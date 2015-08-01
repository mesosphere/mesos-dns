package tap

import (
	"github.com/miekg/dns"
)

type Filter interface {
	FilterDNS(dns.ResponseWriter, *dns.Msg)
}

type FilterFunc func(dns.ResponseWriter, *dns.Msg)

func (ff FilterFunc) FilterDNS(w dns.ResponseWriter, r *dns.Msg) {
	ff(w, r)
}
