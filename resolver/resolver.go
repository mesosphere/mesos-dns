// package resolver contains functions to handle resolving .mesos
// domains
package resolver

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"io"
	"math/rand"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mesos/mesos-go/detector"
	_ "github.com/mesos/mesos-go/detector/zoo"
	mesos "github.com/mesos/mesos-go/mesosproto"

	"github.com/emicklei/go-restful"
	"github.com/miekg/dns"
)

var (
	recurseCnt = 3
)

// holds configuration state and the resource records
type Resolver struct {
	version    string
	config     records.Config
	rs         *records.RecordGenerator
	rsLock     sync.RWMutex
	leader     string
	leaderLock sync.RWMutex
}

func New(version string, config records.Config) *Resolver {
	return &Resolver{
		version: version,
		config:  config,
		rs:      &records.RecordGenerator{},
	}
}

// return the current (read-only) record set. attempts to write to the returned
// object will likely result in a data race.
func (res *Resolver) records() *records.RecordGenerator {
	res.rsLock.RLock()
	defer res.rsLock.RUnlock()
	return res.rs
}

// launches DNS server for a resolver
func (res *Resolver) LaunchDNS() {
	// Handers for Mesos requests
	dns.HandleFunc(res.config.Domain+".", panicRecover(res.HandleMesos))
	// Handler for nonMesos requests
	dns.HandleFunc(".", panicRecover(res.HandleNonMesos))

	go res.Serve("tcp")
	go res.Serve("udp")
}

// starts a DNS server for net protocol (tcp/udp)
func (res *Resolver) Serve(net string) {
	defer func() {
		if rec := recover(); rec != nil {
			logging.Error.Printf("%s\n", rec)
			os.Exit(1)
		}
	}()

	server := &dns.Server{
		Addr:       res.config.Listener + ":" + strconv.Itoa(res.config.Port),
		Net:        net,
		TsigSecret: nil,
	}

	err := server.ListenAndServe()
	if err != nil {
		logging.Error.Printf("Failed to setup "+net+" server: %s\n", err.Error())
	} else {
		logging.Error.Printf("Not listening/serving any more requests.")
	}

	os.Exit(1)
}

// launches Zookeeper detector
func (res *Resolver) LaunchZK() {
	dr, err := res.ZKdetect()
	if err != nil {
		logging.Error.Println(err.Error())
		os.Exit(1)
	}

	logging.VeryVerbose.Println("Warning: waiting for initial information from Zookeper.")
	select {
	case <-dr:
		logging.VeryVerbose.Println("Warning: got initial information from Zookeper.")
	case <-time.After(4 * time.Minute):
		logging.Error.Println("timed out waiting for initial ZK detection, exiting")
		os.Exit(1)
	}
}

// triggers a new refresh from mesos master
func (res *Resolver) Reload() {
	t := records.RecordGenerator{}
	// Being very conservative
	res.leaderLock.RLock()
	currentLeader := res.leader
	res.leaderLock.RUnlock()
	err := t.ParseState(currentLeader, res.config)

	if err == nil {
		// may need to refactor for fairness
		res.rsLock.Lock()
		defer res.rsLock.Unlock()
		res.rs = &t
	} else {
		logging.VeryVerbose.Println("Warning: master not found; keeping old DNS state")
	}
}

// resolveOut queries other nameserver, potentially recurses
func (res *Resolver) resolveOut(r *dns.Msg, nameserver string, proto string, cnt int) (*dns.Msg, error) {
	var in *dns.Msg
	var err error

	c := new(dns.Client)
	c.Net = proto

	var t time.Duration = 5 * 1e9
	if res.config.Timeout != 0 {
		t = time.Duration(int64(res.config.Timeout * 1e9))
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
				return res.resolveOut(r, soa.Ns+":53", proto, cnt-1)
			}
		}

	}

	return in, err
}

// formatSRV returns the SRV resource record for target
func (res *Resolver) formatSRV(name string, target string) (*dns.SRV, error) {
	ttl := uint32(res.config.TTL)

	h, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, errors.New("invalid target")
	}
	p, _ := strconv.Atoi(port)

	return &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   name,
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Priority: 0,
		Weight:   0,
		Port:     uint16(p),
		Target:   h,
	}, nil
}

// returns the A resource record for target
// assumes target is a well formed IPv4 address
func (res *Resolver) formatA(dom string, target string) (*dns.A, error) {
	ttl := uint32(res.config.TTL)

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
func (res *Resolver) formatSOA(dom string) (*dns.SOA, error) {
	ttl := uint32(res.config.TTL)

	return &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   dom,
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    ttl,
		},
		Ns:      res.config.Mname,
		Mbox:    res.config.Email,
		Serial:  uint32(time.Now().Unix()),
		Refresh: ttl,
		Retry:   600,
		Expire:  86400,
		Minttl:  ttl,
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
func (res *Resolver) HandleNonMesos(w dns.ResponseWriter, r *dns.Msg) {
	var err error
	var m *dns.Msg

	// tracing info
	logging.CurLog.NonMesosRequests.Inc()

	// If external request are disabled
	if !res.config.ExternalOn {
		m = new(dns.Msg)
		// set refused
		m.SetRcode(r, 5)
	} else {

		proto := "udp"
		if _, ok := w.RemoteAddr().(*net.TCPAddr); ok {
			proto = "tcp"
		}
		
		for _, resolver := range res.config.Resolvers {
			nameserver := resolver + ":53"
			m, err = res.resolveOut(r, nameserver, proto, recurseCnt)
			if err == nil {
				break
			}
		}
	}
		
	// resolveOut returns nil Msg sometimes cause of perf
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

// HandleMesos is a resolver request handler that responds to a resource
// question with resource answer(s)
// it can handle {A, SRV, ANY}
func (res *Resolver) HandleMesos(w dns.ResponseWriter, r *dns.Msg) {
	var err error

	dom := strings.ToLower(cleanWild(r.Question[0].Name))
	qType := r.Question[0].Qtype

	m := new(dns.Msg)
	m.Authoritative = true
	m.RecursionAvailable = true
	m.SetReply(r)

	rs := res.records()

	// SRV requests
	if (qType == dns.TypeSRV) || (qType == dns.TypeANY) {
		for _, srv := range rs.SRVs[dom] {
			rr, err := res.formatSRV(r.Question[0].Name, srv)
			if err != nil {
				logging.Error.Println(err)
			} else {
				m.Answer = append(m.Answer, rr)
				// return one corresponding A record add additional info
				host := strings.Split(srv, ":")[0]
				if len(rs.As[host]) != 0 {
					rr, err := res.formatA(host, rs.As[host][0])
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
			rr, err := res.formatA(dom, a)
			if err != nil {
				logging.Error.Println(err)
			} else {
				m.Answer = append(m.Answer, rr)
			}

		}
	}

	// SOA requests
	if qType == dns.TypeSOA {
		m = new(dns.Msg)
		m.SetReply(r)

		rr, err := res.formatSOA(r.Question[0].Name)
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
		m = new(dns.Msg)
		m.Authoritative = true
		// set NOERROR
		m.SetRcode(r, 0)
		// leave answer empty (NOERROR --> NODATA)

	} else {
		// no answers but not a {SOA,SRV} request
		if len(m.Answer) == 0 && (qType != dns.TypeSOA) && (qType != dns.TypeSRV) {

			m = new(dns.Msg)
			// set NXDOMAIN
			m.SetRcode(r, 3)

			rr, err := res.formatSOA(r.Question[0].Name)
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

// Hdns starts an http server for mesos-dns queries
func (res *Resolver) LaunchHTTP() {
	defer func() {
		if rec := recover(); rec != nil {
			logging.Error.Printf("%s\n", rec)
			os.Exit(1)
		}
	}()

	// webserver + available routes
	ws := new(restful.WebService)
	ws.Route(ws.GET("/v1/version").To(res.RestVersion))
	ws.Route(ws.GET("/v1/config").To(res.RestConfig))
	ws.Route(ws.GET("/v1/hosts/{host}").To(res.RestHost))
	ws.Route(ws.GET("/v1/hosts/{host}/ports").To(res.RestPorts))
	ws.Route(ws.GET("/v1/services/{service}").To(res.RestService))
	restful.Add(ws)

	portString := ":" + strconv.Itoa(res.config.HttpPort)
	if err := http.ListenAndServe(portString, nil); err != nil {
		logging.Error.Printf("Failed to setup http server: %s\n", err.Error())
	} else {
		logging.Error.Printf("Not serving http requests any more .")
	}
	os.Exit(1)
}

// Reports configuration through REST interface
func (res *Resolver) RestConfig(req *restful.Request, resp *restful.Response) {
	output, err := json.Marshal(res.config)
	if err != nil {
		logging.Error.Println(err)
	}

	io.WriteString(resp, string(output))
}

// Reports Mesos-DNS version through REST interface
func (res *Resolver) RestVersion(req *restful.Request, resp *restful.Response) {
	mapV := map[string]string{"Service": "Mesos-DNS",
		"Version": res.version,
		"URL":     "https://github.com/mesosphere/mesos-dns"}
	output, err := json.Marshal(mapV)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))
}

// Reports Mesos-DNS version through http interface
func (res *Resolver) RestHost(req *restful.Request, resp *restful.Response) {

	host := req.PathParameter("host")
	// clean up host name
	dom := strings.ToLower(cleanWild(host))
	if dom[len(dom)-1] != '.' {
		dom += "."
	}

	mapH := make([]map[string]string, 0)
	rs := res.records()

	for _, ip := range rs.As[dom] {
		t := map[string]string{"host": dom, "ip": ip}
		mapH = append(mapH, t)
	}
	empty := (len(rs.As[dom]) == 0)
	if empty {
		t := map[string]string{"host": "", "ip": ""}
		mapH = append(mapH, t)
	}

	output, err := json.Marshal(mapH)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))

	// stats
	mesosrq := strings.HasSuffix(dom, res.config.Domain+".")
	if mesosrq {
		logging.CurLog.MesosRequests.Inc()
		if empty {
			logging.CurLog.MesosNXDomain.Inc()
		} else {
			logging.CurLog.MesosSuccess.Inc()
		}
	} else {
		logging.CurLog.NonMesosRequests.Inc()
		logging.CurLog.NonMesosFailed.Inc()
	}

}

// Reports Mesos-DNS version through http interface
func (res *Resolver) RestPorts(req *restful.Request, resp *restful.Response) {
	io.WriteString(resp, "To be implemented...")
}

// Reports Mesos-DNS version through http interface
func (res *Resolver) RestService(req *restful.Request, resp *restful.Response) {

	var ip string

	service := req.PathParameter("service")

	// clean up service name
	dom := strings.ToLower(cleanWild(service))
	if dom[len(dom)-1] != '.' {
		dom += "."
	}

	mapS := make([]map[string]string, 0)
	rs := res.records()

	for _, srv := range rs.SRVs[dom] {
		h, port, _ := net.SplitHostPort(srv)
		p, _ := strconv.Atoi(port)
		if len(rs.As[h]) != 0 {
			ip = rs.As[h][0]
		} else {
			ip = ""
		}

		t := map[string]string{"service": service, "host": h, "ip": ip, "port": strconv.Itoa(p)}
		mapS = append(mapS, t)
	}

	empty := (len(rs.SRVs[dom]) == 0)
	if empty {
		t := map[string]string{"service": "", "host": "", "ip": "", "port": ""}
		mapS = append(mapS, t)
	}

	output, err := json.Marshal(mapS)
	if err != nil {
		logging.Error.Println(err)
	}
	io.WriteString(resp, string(output))

	// stats
	mesosrq := strings.HasSuffix(dom, res.config.Domain+".")
	if mesosrq {
		logging.CurLog.MesosRequests.Inc()
		if empty {
			logging.CurLog.MesosNXDomain.Inc()
		} else {
			logging.CurLog.MesosSuccess.Inc()
		}
	} else {
		logging.CurLog.NonMesosRequests.Inc()
		logging.CurLog.NonMesosFailed.Inc()
	}

}

// panicRecover catches any panics from the resolvers and sets an error
// code of server failure
func panicRecover(f func(w dns.ResponseWriter, r *dns.Msg)) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		defer func() {
			if rec := recover(); rec != nil {
				m := new(dns.Msg)
				m.SetRcode(r, 2)
				_ = w.WriteMsg(m)
				logging.Error.Println(rec)
			}
		}()
		f(w, r)
	}
}

// Start a Zookeeper listener to track leading master, returns a signal chan
// that closes upon the first leader detection notification (and c.leader is
// meaningfully readable).
func (res *Resolver) ZKdetect() (<-chan struct{}, error) {

	// start listener
	logging.Verbose.Println("Starting master detector for ZK ", res.config.Zk)
	md, err := detector.New(res.config.Zk)
	if err != nil {
		return nil, fmt.Errorf("failed to create master detector: %v", err)
	}

	// and listen for master changes
	var startedOnce sync.Once
	started := make(chan struct{})
	if err := md.Detect(detector.OnMasterChanged(func(info *mesos.MasterInfo) {
		// making this atomic
		res.leaderLock.Lock()
		defer res.leaderLock.Unlock()
		logging.VeryVerbose.Println("Updated Zookeeper info: ", info)
		if info == nil {
			res.leader = ""
			logging.Error.Println("No leader available in Zookeeper.")
		} else if host := info.GetHostname(); host != "" {
			res.leader = host
		} else {
			// unpack IPv4
			octets := make([]byte, 4, 4)
			binary.BigEndian.PutUint32(octets, info.GetIp())
			ipv4 := net.IP(octets)
			res.leader = ipv4.String()
		}
		if len(res.leader) > 0 {
			res.leader = fmt.Sprintf("%s:%d", res.leader, info.GetPort())
		}
		logging.Verbose.Println("New master in Zookeeper ", res.leader)
		startedOnce.Do(func() { close(started) })
	})); err != nil {
		return nil, fmt.Errorf("failed to initialize master detector: %v", err)
	}
	return started, nil
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
