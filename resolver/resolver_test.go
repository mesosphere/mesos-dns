package resolver

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/records/labels"
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

	_ = shuffleAnswers(m.Answer)

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

func fakeDNS(port int) (*Resolver, error) {
	res := New("", records.Config{
		Masters:    []string{"144.76.157.37:5050"},
		TTL:        60,
		Port:       port,
		Domain:     "mesos",
		Resolvers:  records.GetLocalDNS(),
		Listener:   "127.0.0.1",
		SOARname:   "root.ns1.mesos.",
		SOAMname:   "ns1.mesos.",
		HTTPPort:   8123,
		ExternalOn: true,
	})

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		return res, err
	}

	var sj records.StateJSON
	err = json.Unmarshal(b, &sj)
	if err != nil {
		return res, err
	}

	masters := []string{"144.76.157.37:5050"}
	spec := labels.RFC952
	res.rs = &records.RecordGenerator{}
	res.rs.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters, spec)

	return res, nil
}

func fakeMsg(dom string, rrHeader uint16, proto string, serverPort int) (*dns.Msg, error) {
	qc := uint16(dns.ClassINET)

	c := new(dns.Client)
	c.Net = proto

	m := new(dns.Msg)
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{
		Name:   dns.Fqdn(dom),
		Qtype:  rrHeader,
		Qclass: qc,
	}
	m.RecursionDesired = true
	in, _, err := c.Exchange(m, "127.0.0.1:"+strconv.Itoa(serverPort))
	return in, err

}

func fakeQuery(dom string, rrHeader uint16, proto string, serverPort int) ([]dns.RR, error) {
	in, err := fakeMsg(dom, rrHeader, proto, serverPort)
	if err != nil {
		return nil, err
	}

	return in.Answer, nil
}

func identicalResults(a []dns.RR, b []dns.RR) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].String() != b[i].String() {
			return false
		}
	}
	return true
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

func onSignal(abort <-chan struct{}, s <-chan struct{}, f func()) {
	go func() {
		select {
		case <-abort:
		case <-s:
			f()
		}
	}()
}

// start TCP and UDP DNS servers and block, waiting for the server listeners to come up before returning
func safeStartDNSResolver(t *testing.T, res *Resolver) {
	var abortOnce sync.Once
	abort := make(chan struct{})
	doAbort := func() {
		abortOnce.Do(func() { close(abort) })
	}

	s1, e := res.Serve("udp")
	f1 := onError(abort, e, func(err error) { t.Fatalf("udp server failed: %v", err) })
	s2, e := res.Serve("tcp")
	f2 := onError(abort, e, func(err error) { t.Fatalf("tcp server failed: %v", err) })

	var wg sync.WaitGroup
	wg.Add(2)

	onSignal(abort, s1, wg.Done)
	onSignal(abort, s2, wg.Done)
	onSignal(abort, f1, doAbort)
	onSignal(abort, f2, doAbort)

	wg.Wait()
}

func TestHandler(t *testing.T) {
	var msg []dns.RR

	const port = 8053
	res, err := fakeDNS(port)
	if err != nil {
		t.Error(err)
	}

	dns.HandleFunc("mesos.", res.HandleMesos)
	safeStartDNSResolver(t, res)

	// test A records
	msg, err = fakeQuery("chronos.marathon.mesos.", dns.TypeA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 1 {
		t.Error("not serving up A records")
	}

	// Test case sensitivity -- this test depends on one above
	dup := msg
	msg, err = fakeQuery("cHrOnOs.MARATHON.mesoS.", dns.TypeA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if !identicalResults(msg, dup) {
		t.Errorf("Case sensitivity failure:\n%s\n!=\n%s", msg, dup)
	}

	// test SRV record
	msg, err = fakeQuery("_liquor-store._udp.marathon.mesos.", dns.TypeSRV, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 3 {
		t.Error("not serving up SRV records")
	}

	// test SOA
	m, err := fakeMsg("non-existing.mesos.", dns.TypeSOA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if m.Ns == nil {
		t.Error("not serving up SOA")
	}

	// test NS
	m, err = fakeMsg("non-existing2.mesos.", dns.TypeNS, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if m.Ns == nil {
		t.Error("not serving up NS")
	}

	// test non-existing host
	m, err = fakeMsg("missing.mesos.", dns.TypeA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if got, want := m.Rcode, dns.RcodeNameError; got != want {
		t.Errorf("not setting NXDOMAIN, got Rcode: %v, want: %v", got, want)
	}

	// test tcp
	msg, err = fakeQuery("chronos.marathon.mesos.", dns.TypeA, "tcp", port)
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 1 {
		t.Error("not serving up A records")
	}

	// test AAAA --> NODATA
	m, err = fakeMsg("chronos.marathon.mesos.", dns.TypeAAAA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if m.Rcode != 0 || len(m.Answer) > 0 {
		t.Errorf("not setting NODATA for AAAA requests: Rcode: %d, Answer: %+v", m.Rcode, m.Answer)
	}

	// test AAAA --> NXDOMAIN
	m, err = fakeMsg("missing.mesos.", dns.TypeAAAA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if got, want := m.Rcode, dns.RcodeNameError; got != want {
		t.Errorf("not setting NXDOMAIN for AAAA requests: got Rcode: %v, want: %v", got, want)
	}
}

func TestNonMesosHandler(t *testing.T) {
	var msg []dns.RR

	const port = 8054
	res, err := fakeDNS(port)
	res.extResolver = func(r *dns.Msg, nameserver string, proto string, cnt int) (*dns.Msg, error) {
		t.Logf("ext-resolver: r=%v, nameserver=%s, proto=%s, cnt=%d", r, nameserver, proto, cnt)
		rr1, err := res.formatA("google.com.", "1.1.1.1")
		if err != nil {
			return nil, err
		}
		rr2, err := res.formatA("google.com.", "2.2.2.2")
		if err != nil {
			return nil, err
		}
		msg := &dns.Msg{
			Answer: []dns.RR{rr1, rr2},
		}
		msg.SetReply(r)
		return msg, nil
	}
	if err != nil {
		t.Error(err)
	}

	dns.HandleFunc(".", res.HandleNonMesos)
	safeStartDNSResolver(t, res)

	// test A records
	msg, err = fakeQuery("google.com", dns.TypeA, "udp", port)
	if err != nil {
		t.Error(err)
	}

	if len(msg) < 1 {
		t.Errorf("not serving up A records, expected 2 records instead of %d: %+v", len(msg), msg)
	}
}

func TestHTTP(t *testing.T) {

	// setup DNS server (just http)
	res, err := fakeDNS(8053)
	if err != nil {
		t.Error(err)
	}
	res.version = "0.1.1"

	res.configureHTTP()
	ts := httptest.NewServer(http.DefaultServeMux)
	defer ts.Close()

	// test /v1/version
	r1, err := http.Get(ts.URL + "/v1/version")
	if err != nil {
		t.Error(err)
	}
	g1, err := ioutil.ReadAll(r1.Body)
	if err != nil {
		t.Error(err)
	}
	var got1 map[string]interface{}
	err = json.Unmarshal(g1, &got1)
	correct1 := map[string]interface{}{"Service": "Mesos-DNS", "Version": "0.1.1", "URL": "https://github.com/mesosphere/mesos-dns"}
	eq1 := reflect.DeepEqual(got1, correct1)
	if !eq1 {
		t.Error("HTTP version API failure")
	}

	// test /v1/config
	r2, err := http.Get(ts.URL + "/v1/config")
	if err != nil {
		t.Error(err)
	}
	g2, err := ioutil.ReadAll(r2.Body)
	if err != nil {
		t.Error(err)
	}
	var got2 records.Config
	err = json.Unmarshal(g2, &got2)
	eq2 := reflect.DeepEqual(got2, res.config)
	if !eq2 {
		t.Error("HTTP config API failure")
	}

	// test /v1/services -- existing record
	r3, err := http.Get(ts.URL + "/v1/services/_leader._tcp.mesos.")
	if err != nil {
		t.Error(err)
	}
	g3, err := ioutil.ReadAll(r3.Body)
	if err != nil {
		t.Error(err)
	}
	var got3 []map[string]interface{}
	err = json.Unmarshal(g3, &got3)
	correct3 := []map[string]interface{}{{"host": "leader.mesos.", "port": "5050", "service": "_leader._tcp.mesos.", "ip": "1.2.3.4"}}
	eq3 := reflect.DeepEqual(got3, correct3)
	if !eq3 {
		t.Error("HTTP services API failure")
	}

	// test /v1/services -- non existing record
	r4, err := http.Get(ts.URL + "/v1/services/_myservice._tcp.mesos.")
	if err != nil {
		t.Error(err)
	}
	g4, err := ioutil.ReadAll(r4.Body)
	if err != nil {
		t.Error(err)
	}
	var got4 []map[string]interface{}
	err = json.Unmarshal(g4, &got4)
	correct4 := []map[string]interface{}{{"host": "", "port": "", "service": "", "ip": ""}}
	eq4 := reflect.DeepEqual(got4, correct4)
	if !eq4 {
		t.Error("HTTP services API failure")
	}

	// test /v1/host -- existing record
	r5, err := http.Get(ts.URL + "/v1/hosts/leader.mesos")
	if err != nil {
		t.Error(err)
	}
	g5, err := ioutil.ReadAll(r5.Body)
	if err != nil {
		t.Error(err)
	}
	var got5 []map[string]interface{}
	err = json.Unmarshal(g5, &got5)
	correct5 := []map[string]interface{}{{"host": "leader.mesos.", "ip": "1.2.3.4"}}
	eq5 := reflect.DeepEqual(got5, correct5)
	if !eq5 {
		t.Error("HTTP hosts API failure")
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
