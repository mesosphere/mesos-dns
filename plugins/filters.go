package plugins

import (
	"github.com/miekg/dns"
)

// Registers a filter that will be invoked prior to the DNS server handling
// the request. A filter may decide to handle the request on behalf of the
// DNS server, in which case the chain is never invoked. To continue processing
// the request a filter should invoke the chain. The chain will never be nil.
type Filter interface {
	ServeDNS(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler)
}

// Func adapter for the Filter interface.
type FilterFunc func(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler)

func (f FilterFunc) ServeDNS(w dns.ResponseWriter, r *dns.Msg, chain dns.Handler) {
	f(w, r, chain)
}

type FilterSet []Filter

// Apply this filter set to the given handler func. Filter implementations are not
// obligated to invoke the chain, so the handler may never actually be called. This
// particular implementation iterates through the filter set in a LIFO manner.
func (fs FilterSet) Handler(handler dns.Handler) dns.Handler {
	index := len(fs)
	var chain dns.HandlerFunc
	chain = dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		if index > 0 {
			index--
			fs[index].ServeDNS(w, r, chain)
		} else {
			handler.ServeDNS(w, r)
		}
	})
	return chain
}
