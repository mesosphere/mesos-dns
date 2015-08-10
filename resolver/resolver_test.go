package resolver

import (
	"encoding/json"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/records/labels"
	"github.com/mesosphere/mesos-dns/records/state"
	"github.com/miekg/dns"
)

func init() {
	logging.VerboseFlag = false
	logging.SetupLogs()
}

// dig @127.0.0.1 -p 8053 "bob.*.mesos" ANY
func TestCleanWild(t *testing.T) {
	dom := "bob.*.mesos"

	stripped := cleanWild(dom)

	if stripped != "bob.mesos" {
		t.Error("not stripping domain")
	}
}

func TestShuffleAnswers(t *testing.T) {
	var res Resolver

	m := new(dns.Msg)

	for i := 0; i < 10; i++ {
		name := "10.0.0." + strconv.Itoa(i)
		rr, err := res.formatA("blah.com", name)
		if err != nil {
			t.Error(err)
		}
		m.Answer = append(m.Answer, rr)
	}

	n := new(dns.Msg)
	c := make([]dns.RR, len(m.Answer))
	copy(c, m.Answer)
	n.Answer = c

	rng := rand.New(rand.NewSource(0))
	_ = shuffleAnswers(rng, m.Answer)

	sflag := false
	// 10! chance of failing here
	for i := 0; i < 10; i++ {
		if n.Answer[i] != m.Answer[i] {
			sflag = true
			break
		}
	}

	if !sflag {
		t.Error("not shuffling")
	}
}

func TestHandlers(t *testing.T) {
	res := fakeDNS(t)
	res.extResolver = func(r *dns.Msg, nameserver string, proto string, cnt int) (*dns.Msg, error) {
		rr1, err := res.formatA("google.com.", "1.1.1.1")
		if err != nil {
			return nil, err
		}
		rr2, err := res.formatA("google.com.", "2.2.2.2")
		if err != nil {
			return nil, err
		}
		msg := &dns.Msg{Answer: []dns.RR{rr1, rr2}}
		msg.SetReply(r)
		return msg, nil
	}

	for i, tt := range []struct {
		dns.HandlerFunc
		*dns.Msg
	}{
		{
			res.HandleMesos,
			message(
				question("chronos.marathon.mesos.", dns.TypeA),
				header(true, dns.RcodeSuccess),
				answers(
					a(rrheader("chronos.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{ // case insensitive
			res.HandleMesos,
			message(
				question("cHrOnOs.MARATHON.mesoS.", dns.TypeA),
				header(true, dns.RcodeSuccess),
				answers(
					a(rrheader("chronos.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{
			res.HandleMesos,
			message(
				question("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV),
				header(true, dns.RcodeSuccess),
				answers(
					srv(rrheader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-17700-0.marathon.mesos.", 443, 0, 0),
					srv(rrheader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-7581-1.marathon.mesos.", 80, 0, 0),
					srv(rrheader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-7581-1.marathon.mesos.", 443, 0, 0),
					srv(rrheader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-17700-0.marathon.mesos.", 80, 0, 0)),
				extras(
					a(rrheader("liquor-store-17700-0.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.1")),
					a(rrheader("liquor-store-17700-0.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.1")),
					a(rrheader("liquor-store-7581-1.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.2")),
					a(rrheader("liquor-store-7581-1.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.2")))),
		},
		{
			res.HandleMesos,
			message(
				question("_car-store._udp.marathon.mesos.", dns.TypeSRV),
				header(true, dns.RcodeSuccess),
				answers(
					srv(rrheader("_car-store._udp.marathon.mesos.", dns.TypeSRV, 60),
						"car-store-50548-0.marathon.slave.mesos.", 31365, 0, 0),
					srv(rrheader("_car-store._udp.marathon.mesos.", dns.TypeSRV, 60),
						"car-store-50548-0.marathon.slave.mesos.", 31364, 0, 0)),
				extras(
					a(rrheader("car-store-50548-0.marathon.slave.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")),
					a(rrheader("car-store-50548-0.marathon.slave.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{
			res.HandleMesos,
			message(
				question("non-existing.mesos.", dns.TypeSOA),
				header(true, dns.RcodeSuccess),
				nss(
					soa(rrheader("non-existing.mesos.", dns.TypeSOA, 60),
						"root.ns1.mesos", "ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			message(
				question("non-existing.mesos.", dns.TypeNS),
				header(true, dns.RcodeSuccess),
				nss(
					ns(rrheader("non-existing.mesos.", dns.TypeNS, 60), "ns1.mesos"))),
		},
		{
			res.HandleMesos,
			message(
				question("missing.mesos.", dns.TypeA),
				header(true, dns.RcodeNameError),
				nss(
					soa(rrheader("missing.mesos.", dns.TypeSOA, 60),
						"root.ns1.mesos", "ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			message(
				question("chronos.marathon.mesos.", dns.TypeAAAA),
				header(true, dns.RcodeSuccess),
				nss(
					soa(rrheader("chronos.marathon.mesos.", dns.TypeSOA, 60),
						"root.ns1.mesos", "ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			message(
				question("missing.mesos.", dns.TypeAAAA),
				header(true, dns.RcodeNameError),
				nss(
					soa(rrheader("missing.mesos.", dns.TypeSOA, 60),
						"root.ns1.mesos", "ns1.mesos", 60))),
		},
		{
			res.HandleNonMesos,
			message(
				question("google.com.", dns.TypeA),
				header(false, dns.RcodeSuccess),
				answers(
					a(rrheader("google.com.", dns.TypeA, 60), net.ParseIP("1.1.1.1")),
					a(rrheader("google.com.", dns.TypeA, 60), net.ParseIP("2.2.2.2")))),
		},
	} {
		var rw responseRecorder
		tt.HandlerFunc(&rw, tt.Msg)
		if got, want := rw.msg, tt.Msg; !reflect.DeepEqual(got, want) {
			t.Logf("Test #%d\n%v\n", i, pretty.Sprint(tt.Msg.Question))
			t.Error(pretty.Compare(got, want))
		}
	}
}

func TestHTTP(t *testing.T) {
	// setup DNS server (just http)
	res := fakeDNS(t)
	res.version = "0.1.1"

	res.configureHTTP()
	srv := httptest.NewServer(http.DefaultServeMux)
	defer srv.Close()

	for _, tt := range []struct {
		path      string
		code      int
		got, want interface{}
	}{
		{"/v1/version", http.StatusOK, map[string]interface{}{},
			map[string]interface{}{
				"Service": "Mesos-DNS",
				"URL":     "https://github.com/mesosphere/mesos-dns",
				"Version": "0.1.1",
			},
		},
		{"/v1/config", http.StatusOK, &records.Config{}, &res.config},
		{"/v1/services/_leader._tcp.mesos.", http.StatusOK, []interface{}{},
			[]interface{}{map[string]interface{}{
				"service": "_leader._tcp.mesos.",
				"host":    "leader.mesos.",
				"ip":      "1.2.3.4",
				"port":    "5050",
			}},
		},
		{"/v1/services/_myservice._tcp.mesos.", http.StatusOK, []interface{}{},
			[]interface{}{map[string]interface{}{
				"service": "",
				"host":    "",
				"ip":      "",
				"port":    "",
			}},
		},
		{"/v1/hosts/leader.mesos", http.StatusOK, []interface{}{},
			[]interface{}{map[string]interface{}{
				"host": "leader.mesos.",
				"ip":   "1.2.3.4",
			}},
		},
	} {
		if resp, err := http.Get(srv.URL + tt.path); err != nil {
			t.Error(err)
		} else if got, want := resp.StatusCode, tt.code; got != want {
			t.Errorf("GET %s: StatusCode: got %d, want %d", tt.path, got, want)
		} else if err := json.NewDecoder(resp.Body).Decode(&tt.got); err != nil {
			t.Error(err)
		} else if got, want := tt.got, tt.want; !reflect.DeepEqual(got, want) {
			t.Errorf("GET %s: Body:\ngot:  %#v\nwant: %#v", tt.path, got, want)
		} else {
			_ = resp.Body.Close()
		}
	}
}

func TestLaunchZK(t *testing.T) {
	var closeOnce sync.Once
	ch := make(chan struct{})
	closer := func() { closeOnce.Do(func() { close(ch) }) }
	res := &Resolver{
		startZKdetection: func(zkurl string, leaderChanged func(string)) error {
			go func() {
				defer closer()
				leaderChanged("")
				leaderChanged("")
				leaderChanged("a")
				leaderChanged("")
				leaderChanged("")
				leaderChanged("b")
				leaderChanged("")
				leaderChanged("")
				leaderChanged("c")
			}()
			return nil
		},
	}
	leaderSig, errCh := res.LaunchZK(1 * time.Second)
	onError(ch, errCh, func(err error) { t.Fatalf("unexpected error: %v", err) })
	getLeader := func() string {
		res.leaderLock.Lock()
		defer res.leaderLock.Unlock()
		return res.leader
	}
	for i := 0; i < 3; i++ {
		select {
		case <-leaderSig:
			t.Logf("new leader %d: %s", i, getLeader())
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for new leader")
		}
	}
	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for detector death")
	}
	// there should be nothing left in the leader signal chan
	select {
	case <-leaderSig:
		t.Fatalf("unexpected new leader")
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}

func message(opts ...func(*dns.Msg)) *dns.Msg {
	var m dns.Msg
	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

func header(auth bool, rcode int) func(*dns.Msg) {
	return func(m *dns.Msg) {
		m.Authoritative = auth
		m.Response = true
		m.Rcode = rcode
		m.Compress = true
	}
}

func question(name string, qtype uint16) func(*dns.Msg) {
	return func(m *dns.Msg) { m.SetQuestion(name, qtype) }
}

func answers(rrs ...dns.RR) func(*dns.Msg) {
	return func(m *dns.Msg) { m.Answer = append(m.Answer, rrs...) }
}

func nss(rrs ...dns.RR) func(*dns.Msg) {
	return func(m *dns.Msg) { m.Ns = append(m.Ns, rrs...) }
}

func extras(rrs ...dns.RR) func(*dns.Msg) {
	return func(m *dns.Msg) { m.Extra = append(m.Extra, rrs...) }
}

func rrheader(name string, rrtype uint16, ttl uint32) dns.RR_Header {
	return dns.RR_Header{
		Name:   name,
		Rrtype: rrtype,
		Class:  dns.ClassINET,
		Ttl:    ttl,
	}
}

func a(hdr dns.RR_Header, ip net.IP) dns.RR {
	return &dns.A{
		Hdr: hdr,
		A:   ip.To4(),
	}
}

func srv(hdr dns.RR_Header, target string, port, priority, weight uint16) dns.RR {
	return &dns.SRV{
		Hdr:      hdr,
		Target:   target,
		Port:     port,
		Priority: priority,
		Weight:   weight,
	}
}

func ns(hdr dns.RR_Header, s string) dns.RR {
	return &dns.NS{
		Hdr: hdr,
		Ns:  s,
	}
}

func soa(hdr dns.RR_Header, ns, mbox string, minttl uint32) dns.RR {
	return &dns.SOA{
		Hdr:     hdr,
		Ns:      ns,
		Mbox:    mbox,
		Minttl:  minttl,
		Refresh: 60,
		Retry:   600,
		Expire:  86400,
	}
}

// responseRecorder implements the dns.ResponseWriter interface. It's used in
// tests only.
type responseRecorder struct {
	localAddr  net.IPAddr
	remoteAddr net.IPAddr
	msg        *dns.Msg
}

func (r responseRecorder) LocalAddr() net.Addr  { return &r.localAddr }
func (r responseRecorder) RemoteAddr() net.Addr { return &r.remoteAddr }
func (r *responseRecorder) WriteMsg(m *dns.Msg) error {
	r.msg = m
	return nil
}

func (r *responseRecorder) Write([]byte) (int, error) { return 0, nil }
func (r *responseRecorder) Close() error              { return nil }
func (r responseRecorder) TsigStatus() error          { return nil }
func (r responseRecorder) TsigTimersOnly(bool)        {}
func (r *responseRecorder) Hijack()                   {}

func fakeDNS(t *testing.T) *Resolver {
	config := records.NewConfig()
	config.Masters = []string{"144.76.157.37:5050"}
	config.RecurseOn = false
	config.IPSources = []string{"docker", "mesos", "host"}

	res := New("", config)
	res.rng.Seed(0) // for deterministic tests

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		t.Fatal(err)
	}

	var sj state.State
	err = json.Unmarshal(b, &sj)
	if err != nil {
		t.Fatal(err)
	}

	spec := labels.RFC952
	err = res.rs.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", res.config.Masters, res.config.IPSources, spec)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func onError(abort <-chan struct{}, errCh <-chan error, f func(error)) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		select {
		case <-abort:
		case e := <-errCh:
			if e != nil {
				defer close(ch)
				f(e)
			}
		}
	}()
	return ch
}
