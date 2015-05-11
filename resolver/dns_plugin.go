// package resolver contains functions to handle resolving .mesos
// domains
package resolver

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/plugins"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/util"
	"github.com/miekg/dns"
)

const (
	recurseCnt = 3
)

type dnsRecordsInterface interface {
	records() *records.RecordSet
	getSOASerial() uint32
}

type DNSPlugin struct {
	dnsi         dnsRecordsInterface
	config       *records.Config
	done         chan struct{}
	applyFilters func(dns.Handler) dns.Handler

	// pluggable external DNS resolution, mainly for unit testing
	extResolver func(r *dns.Msg, nameserver string, proto string, cnt int) (*dns.Msg, error)
}

func NewDNSPlugin(dnsi dnsRecordsInterface, applyFilters func(dns.Handler) dns.Handler) *DNSPlugin {
	plugin := &DNSPlugin{
		dnsi:         dnsi,
		done:         make(chan struct{}),
		applyFilters: applyFilters,
	}
	plugin.extResolver = plugin.defaultExtResolver
	return plugin
}

// starts an http server for mesos-dns queries, returns immediately
func (p *DNSPlugin) Start(ctx plugins.InitialContext) <-chan error {
	p.config = ctx.Config()

	// Handers for Mesos requests
	dns.Handle(p.config.Domain+".", panicRecover(p.applyFilters(dns.HandlerFunc(p.handleMesos))))
	// Handler for nonMesos requests
	dns.Handle(".", panicRecover(p.applyFilters(dns.HandlerFunc(p.handleNonMesos))))

	var doneOnce sync.Once
	closeDone := func() { doneOnce.Do(func() { close(p.done) }) }

	errCh := make(chan error, 2)
	_, e1 := p.serveDNS("tcp")
	go func() {
		defer closeDone()
		errCh <- <-e1
	}()
	_, e2 := p.serveDNS("udp")
	go func() {
		defer closeDone()
		errCh <- <-e2
	}()
	return errCh
}

func (p *DNSPlugin) Stop() {
	//TODO(jdef)
}

func (p *DNSPlugin) Done() <-chan struct{} {
	return p.done
}

// starts a DNS server for net protocol (tcp/udp), returns immediately.
// the returned signal chan is closed upon the server successfully entering the listening phase.
// if the server aborts then an error is sent on the error chan.
func (p *DNSPlugin) serveDNS(proto string) (<-chan struct{}, <-chan error) {
	defer util.HandleCrash()

	var address string
	portString := strconv.Itoa(p.config.Port)
	if p.config.Listener != "" {
		address = net.JoinHostPort(p.config.Listener, portString)
	} else {
		address = ":" + portString
	}
	ch := make(chan struct{})
	server := &dns.Server{
		Addr:              address,
		Net:               proto,
		TsigSecret:        nil,
		NotifyStartedFunc: func() { close(ch) },
	}

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		err := server.ListenAndServe()
		if err != nil {
			errCh <- fmt.Errorf("Failed to setup %q server: %v", proto, err)
		} else {
			logging.Error.Printf("Not listening/serving any more requests.")
		}
	}()
	return ch, errCh
}

// defaultExtResolver queries other nameserver, potentially recurses; callers should probably be invoking extResolver
// instead since that's the pluggable entrypoint into external resolution.
func (p *DNSPlugin) defaultExtResolver(r *dns.Msg, nameserver string, proto string, cnt int) (*dns.Msg, error) {
	var in *dns.Msg
	var err error

	c := new(dns.Client)
	c.Net = proto

	var t time.Duration = 5 * 1e9
	if p.config.Timeout != 0 {
		t = time.Duration(int64(p.config.Timeout * 1e9))
	}

	c.DialTimeout = t
	c.ReadTimeout = t
	c.WriteTimeout = t

	in, _, err = c.Exchange(r, nameserver)
	if err != nil {
		return in, err
	}

	// recurse
	if (in != nil) && (len(in.Answer) == 0) && (!in.MsgHdr.Authoritative) && (len(in.Ns) > 0) && (err != nil) {

		if cnt == recurseCnt {
			logging.CurLog.NonMesosRecursed.Inc()
		}

		if cnt > 0 {
			if soa, ok := (in.Ns[0]).(*dns.SOA); ok {
				return p.defaultExtResolver(r, soa.Ns+":53", proto, cnt-1)
			}
		}

	}

	return in, err
}

// formatSRV returns the SRV resource record for target
func (p *DNSPlugin) formatSRV(name string, target string) (*dns.SRV, error) {
	ttl := uint32(p.config.TTL)

	h, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, errors.New("invalid target")
	}
	iport, _ := strconv.Atoi(port)

	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(iport),
		Target:   h,
	}, nil
}

// returns the A resource record for target
// assumes target is a well formed IPv4 address
func (p *DNSPlugin) formatA(dom string, target string) (*dns.A, error) {
	ttl := uint32(p.config.TTL)

	a := net.ParseIP(target)
	if a == nil {
		return nil, errors.New("invalid target")
	}

	return &dns.A{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    ttl},
		A: a.To4(),
	}, nil
}

// formatSOA returns the SOA resource record for the mesos domain
func (p *DNSPlugin) formatSOA(dom string) (*dns.SOA, error) {
	ttl := uint32(p.config.TTL)

	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      p.config.SOARname,
		Mbox:    p.config.SOAMname,
		Serial:  p.dnsi.getSOASerial(),
		Refresh: p.config.SOARefresh,
		Retry:   p.config.SOARetry,
		Expire:  p.config.SOAExpire,
		Minttl:  ttl,
	}, nil
}

// formatNS returns the NS  record for the mesos domain
func (p *DNSPlugin) formatNS(dom string) (*dns.NS, error) {
	ttl := uint32(p.config.TTL)

	return &dns.NS{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns: p.config.SOAMname,
	}, nil
}

// reorders answers for very basic load balancing
func shuffleAnswers(answers []dns.RR) []dns.RR {
	rand.Seed(time.Now().UTC().UnixNano())

	n := len(answers)
	for i := 0; i < n; i++ {
		r := i + rand.Intn(n-i)
		answers[r], answers[i] = answers[i], answers[r]
	}

	return answers
}

// makes non-mesos queries to external nameserver
func (p *DNSPlugin) handleNonMesos(w dns.ResponseWriter, r *dns.Msg) {
	var err error
	var m *dns.Msg

	// tracing info
	logging.CurLog.NonMesosRequests.Inc()

	// If external request are disabled
	if !p.config.ExternalOn {
		m = new(dns.Msg)
		// set refused
		m.SetRcode(r, 5)
	} else {

		proto := "udp"
		if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			proto = "tcp"
		}

		for _, resolver := range p.config.Resolvers {
			nameserver := resolver + ":53"
			m, err = p.extResolver(r, nameserver, proto, recurseCnt)
			if err == nil {
				break
			}
		}
	}

	// extResolver returns nil Msg sometimes cause of perf
	if m == nil {
		m = new(dns.Msg)
		m.SetRcode(r, 2)
		err = errors.New("nil msg")
	}
	if err != nil {
		logging.Error.Println(r.Question[0].Name)
		logging.Error.Println(err)
		logging.CurLog.NonMesosFailed.Inc()
	} else {
		// nxdomain
		if len(m.Answer) == 0 {
			logging.CurLog.NonMesosNXDomain.Inc()
		} else {
			logging.CurLog.NonMesosSuccess.Inc()
		}
	}

	err = w.WriteMsg(m)
	if err != nil {
		logging.Error.Println(err)
	}
}

// handleMesos is a resolver request handler that responds to a resource
// question with resource answer(s)
// it can handle {A, SRV, ANY}
func (p *DNSPlugin) handleMesos(w dns.ResponseWriter, r *dns.Msg) {
	var err error

	dom := strings.ToLower(cleanWild(r.Question[0].Name))
	qType := r.Question[0].Qtype

	m := new(dns.Msg)
	m.Authoritative = true
	m.RecursionAvailable = p.config.RecurseOn
	m.SetReply(r)

	rs := p.dnsi.records()

	// SRV requests
	if (qType == dns.TypeSRV) || (qType == dns.TypeANY) {
		for _, srv := range rs.SRVs[dom] {
			rr, err := p.formatSRV(r.Question[0].Name, srv)
			if err != nil {
				logging.Error.Println(err)
			} else {
				m.Answer = append(m.Answer, rr)
				// return one corresponding A record add additional info
				host := strings.Split(srv, ":")[0]
				if len(rs.As[host]) != 0 {
					rr, err := p.formatA(host, rs.As[host][0])
					if err != nil {
						logging.Error.Println(err)
					} else {
						m.Extra = append(m.Extra, rr)
					}
				}

			}
		}
	}

	// A requests
	if (qType == dns.TypeA) || (qType == dns.TypeANY) {
		for _, a := range rs.As[dom] {
			rr, err := p.formatA(dom, a)
			if err != nil {
				logging.Error.Println(err)
			} else {
				m.Answer = append(m.Answer, rr)
			}

		}
	}

	// SOA requests
	if (qType == dns.TypeSOA) || (qType == dns.TypeANY) {
		rr, err := p.formatSOA(r.Question[0].Name)
		if err != nil {
			logging.Error.Println(err)
		} else {
			m.Ns = append(m.Ns, rr)
		}
	}

	// NS requests
	if (qType == dns.TypeNS) || (qType == dns.TypeANY) {
		rr, err := p.formatNS(r.Question[0].Name)
		logging.Error.Println("NS request")
		if err != nil {
			logging.Error.Println(err)
		} else {
			m.Ns = append(m.Ns, rr)
		}
	}

	// shuffle answers
	m.Answer = shuffleAnswers(m.Answer)
	// tracing info
	logging.CurLog.MesosRequests.Inc()

	if err != nil {
		logging.CurLog.MesosFailed.Inc()
	} else if (qType == dns.TypeAAAA) && (len(rs.SRVs[dom]) > 0 || len(rs.As[dom]) > 0) {
		// correct handling of AAAA if there are A or SRV records
		m.Authoritative = true
		// set NOERROR
		m.SetRcode(r, 0)
		// leave answer empty (NOERROR --> NODATA)

	} else {
		// no answers but not a {SOA,SRV} request
		if len(m.Answer) == 0 && (qType != dns.TypeSOA) && (qType != dns.TypeNS) && (qType != dns.TypeSRV) {
			// set NXDOMAIN
			m.SetRcode(r, 3)

			rr, err := p.formatSOA(r.Question[0].Name)
			if err != nil {
				logging.Error.Println(err)
			} else {
				m.Ns = append(m.Ns, rr)
			}

			logging.CurLog.MesosNXDomain.Inc()
			logging.VeryVerbose.Println("total A rrs:\t" + strconv.Itoa(len(rs.As)))
			logging.VeryVerbose.Println("failed looking for " + r.Question[0].String())
		} else {
			logging.CurLog.MesosSuccess.Inc()
		}
	}

	err = w.WriteMsg(m)
	if err != nil {
		logging.Error.Println(err)
	}
}

// panicRecover catches any panics from the resolvers and sets an error
// code of server failure
func panicRecover(h dns.Handler) dns.Handler {
	return dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
		defer func() {
			if rec := recover(); rec != nil {
				m := new(dns.Msg)
				m.SetRcode(r, 2)
				_ = w.WriteMsg(m)
				logging.Error.Println(rec)
			}
		}()
		h.ServeDNS(w, r)
	})
}

// cleanWild strips any wildcards out thus mapping cleanly to the
// original serviceName
func cleanWild(dom string) string {
	if strings.Contains(dom, ".*") {
		return strings.Replace(dom, ".*", "", -1)
	} else {
		return dom
	}
}
