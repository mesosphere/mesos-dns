package exchanger

import (
	"log"
	"net"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/miekg/dns"
)

// Exchanger is an interface capturing a dns.Client Exchange method.
type Exchanger interface {
	// Exchange performs an synchronous query. It sends the message m to the address
	// contained in addr (host:port) and waits for a reply.
	Exchange(m *dns.Msg, addr string) (r *dns.Msg, rtt time.Duration, err error)
}

// Func is a function type that implements the Exchanger interface.
type Func func(*dns.Msg, string) (*dns.Msg, time.Duration, error)

// Exchange implements the Exchanger interface.
func (f Func) Exchange(m *dns.Msg, addr string) (*dns.Msg, time.Duration, error) {
	return f(m, addr)
}

// A Decorator adds a layer of behaviour to a given Exchanger.
type Decorator func(Exchanger) Exchanger

// Decorate decorates an Exchanger with the given Decorators.
func Decorate(ex Exchanger, ds ...Decorator) Exchanger {
	decorated := ex
	for _, decorate := range ds {
		decorated = decorate(decorated)
	}
	return decorated
}

// Pred is a predicate function type for dns.Msgs.
type Pred func(*dns.Msg) bool

// While returns an Exchanger which attempts the given Exchangers while the given
// predicate function returns true for the returned dns.Msg, an error is returned,
// or all Exchangers are attempted, in which case the return values of the last
// one are returned.
func While(p Pred, exs ...Exchanger) Exchanger {
	return Func(func(m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error) {
		for _, ex := range exs {
			if r, rtt, err = ex.Exchange(m, a); err != nil || !p(r) {
				break
			}
		}
		return
	})
}

// ErrorLogging returns a Decorator which logs an Exchanger's errors to the given
// logger.
func ErrorLogging(l *log.Logger) Decorator {
	return func(ex Exchanger) Exchanger {
		return Func(func(m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error) {
			defer func() {
				if err != nil {
					l.Printf("error exchanging with %q: %v", a, err)
				}
			}()
			return ex.Exchange(m, a)
		})
	}
}

// Instrumentation returns a Decorator which instruments an Exchanger with the given
// counter.
func Instrumentation(c logging.Counter) Decorator {
	return func(ex Exchanger) Exchanger {
		return Func(func(m *dns.Msg, a string) (*dns.Msg, time.Duration, error) {
			defer c.Inc()
			return ex.Exchange(m, a)
		})
	}
}

// A Recurser returns the addr (host:port) of the next DNS server to recurse a
// Msg to. Empty returns signal that further recursion isn't possible or needed.
type Recurser func(*dns.Msg) string

// Recurse is the default Mesos-DNS Recurser which returns an addr (host:port)
// only when the given dns.Msg doesn't contain authoritative answers and has at
// least one SOA record in its NS section.
func Recurse(r *dns.Msg) string {
	if r.Authoritative && len(r.Answer) > 0 {
		return ""
	}

	for _, ns := range r.Ns {
		if soa, ok := ns.(*dns.SOA); ok {
			return net.JoinHostPort(soa.Ns, "53")
		}
	}

	return ""
}

// Recursion returns a Decorator which recurses until the given Recurser returns
// an empty string or max attempts have been reached.
func Recursion(max int, rec Recurser) Decorator {
	return func(ex Exchanger) Exchanger {
		return Func(func(m *dns.Msg, a string) (r *dns.Msg, rtt time.Duration, err error) {
			for i := 0; i <= max; i++ {
				if r, rtt, err = ex.Exchange(m, a); err != nil {
					break
				} else if a = rec(r); a == "" {
					break
				}
			}
			return r, rtt, err
		})
	}
}
