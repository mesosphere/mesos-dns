package resolver

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"testing"

	"github.com/kylelemons/godebug/pretty"
	. "github.com/mesosphere/mesos-dns/dnstest"
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
	if err := runHandlers(); err != nil {
		t.Error(err)
	}
}

func BenchmarkHandlers(b *testing.B) {
	for n := 0; n < b.N; n++ {
		if err := runHandlers(); err != nil {
			b.Error(err)
		}
	}
}

func runHandlers() error {
	res, err := fakeDNS()
	if err != nil {
		return err
	}
	fwd := func(m *dns.Msg, net string) (*dns.Msg, error) {
		rr1, err := res.formatA("google.com.", "1.1.1.1")
		if err != nil {
			return nil, err
		}
		rr2, err := res.formatA("google.com.", "2.2.2.2")
		if err != nil {
			return nil, err
		}
		msg := &dns.Msg{Answer: []dns.RR{rr1, rr2}}
		msg.SetReply(m)
		return msg, nil
	}

	for i, tt := range []struct {
		dns.HandlerFunc
		*dns.Msg
	}{
		{
			res.HandleMesos,
			Message(
				Question("chronos.marathon.mesos.", dns.TypeA),
				Header(true, dns.RcodeSuccess),
				Answers(
					A(RRHeader("chronos.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{ // case insensitive
			res.HandleMesos,
			Message(
				Question("cHrOnOs.MARATHON.mesoS.", dns.TypeA),
				Header(true, dns.RcodeSuccess),
				Answers(
					A(RRHeader("chronos.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{ // ipv6
			res.HandleMesos,
			Message(
				Question("toy-store.ipv6-framework.mesos.", dns.TypeAAAA),
				Header(true, dns.RcodeSuccess),
				Answers(
					AAAA(RRHeader("toy-store.ipv6-framework.mesos.", dns.TypeAAAA, 60),
						net.ParseIP("fd01:b::1:8000:2")))),
		},
		{
			res.HandleMesos,
			Message(
				Question("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV),
				Header(true, dns.RcodeSuccess),
				Answers(
					SRV(RRHeader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-4dfjd-0.marathon.mesos.", 443, 0, 0),
					SRV(RRHeader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-zasmd-1.marathon.mesos.", 80, 0, 0),
					SRV(RRHeader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-zasmd-1.marathon.mesos.", 443, 0, 0),
					SRV(RRHeader("_liquor-store._tcp.marathon.mesos.", dns.TypeSRV, 60),
						"liquor-store-4dfjd-0.marathon.mesos.", 80, 0, 0)),
				Extras(
					A(RRHeader("liquor-store-4dfjd-0.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.1")),
					A(RRHeader("liquor-store-zasmd-1.marathon.mesos.", dns.TypeA, 60),
						net.ParseIP("10.3.0.2")))),
		},
		{
			res.HandleMesos,
			Message(
				Question("_car-store._udp.marathon.mesos.", dns.TypeSRV),
				Header(true, dns.RcodeSuccess),
				Answers(
					SRV(RRHeader("_car-store._udp.marathon.mesos.", dns.TypeSRV, 60),
						"car-store-zinaz-0.marathon.slave.mesos.", 31365, 0, 0),
					SRV(RRHeader("_car-store._udp.marathon.mesos.", dns.TypeSRV, 60),
						"car-store-zinaz-0.marathon.slave.mesos.", 31364, 0, 0)),
				Extras(
					A(RRHeader("car-store-zinaz-0.marathon.slave.mesos.", dns.TypeA, 60),
						net.ParseIP("1.2.3.11")))),
		},
		{
			res.HandleMesos,
			Message(
				Question("_car-store._udp.marathon.mesos.", dns.TypeA),
				Header(true, dns.RcodeSuccess),
				NSs(
					SOA(RRHeader("_car-store._udp.marathon.mesos.", dns.TypeSOA, 60),
						"ns1.mesos", "root.ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			Message(
				Question("non-existing.mesos.", dns.TypeSOA),
				Header(true, dns.RcodeSuccess),
				NSs(
					SOA(RRHeader("non-existing.mesos.", dns.TypeSOA, 60),
						"ns1.mesos", "root.ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			Message(
				Question("non-existing.mesos.", dns.TypeNS),
				Header(true, dns.RcodeSuccess),
				NSs(
					NS(RRHeader("non-existing.mesos.", dns.TypeNS, 60), "ns1.mesos"))),
		},
		{
			res.HandleMesos,
			Message(
				Question("missing.mesos.", dns.TypeA),
				Header(true, dns.RcodeNameError),
				NSs(
					SOA(RRHeader("missing.mesos.", dns.TypeSOA, 60),
						"ns1.mesos", "root.ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			Message(
				Question("chronos.marathon.mesos.", dns.TypeAAAA),
				Header(true, dns.RcodeSuccess),
				NSs(
					SOA(RRHeader("chronos.marathon.mesos.", dns.TypeSOA, 60),
						"ns1.mesos", "root.ns1.mesos", 60))),
		},
		{
			res.HandleMesos,
			Message(
				Question("missing.mesos.", dns.TypeAAAA),
				Header(true, dns.RcodeNameError),
				NSs(
					SOA(RRHeader("missing.mesos.", dns.TypeSOA, 60),
						"ns1.mesos", "root.ns1.mesos", 60))),
		},
		{
			res.HandleNonMesos(fwd),
			Message(
				Question("google.com.", dns.TypeA),
				Header(false, dns.RcodeSuccess),
				Answers(
					A(RRHeader("google.com.", dns.TypeA, 60), net.ParseIP("1.1.1.1")),
					A(RRHeader("google.com.", dns.TypeA, 60), net.ParseIP("2.2.2.2")))),
		},
	} {
		var rw ResponseRecorder
		tt.HandlerFunc(&rw, tt.Msg)
		if got, want := rw.Msg, tt.Msg; !(Msg{got}).equivalent(Msg{want}) {
			return fmt.Errorf("test #%d\n%v\n%s", i, pretty.Sprint(tt.Msg.Question), pretty.Compare(got, want))
		}
	}
	return nil
}

type Msg struct{ *dns.Msg }
type RRs []dns.RR

func (m Msg) equivalent(other Msg) bool {
	if m.Msg == nil || other.Msg == nil {
		return m.Msg == other.Msg
	}
	return m.MsgHdr == other.MsgHdr &&
		m.Compress == other.Compress &&
		reflect.DeepEqual(m.Question, other.Question) &&
		RRs(m.Ns).equivalent(RRs(other.Ns)) &&
		RRs(m.Answer).equivalent(RRs(other.Answer)) &&
		RRs(m.Extra).equivalent(RRs(other.Extra))
}

// equivalent RRs have the same records, but not necessarily in the same order
func (rr RRs) equivalent(other RRs) bool {
	if rr == nil || other == nil {
		return rr == nil && other == nil
	}
	type key struct {
		header dns.RR_Header
		text   string
	}

	rrhash := make(map[string]struct{}, len(rr))
	for i := range rr {
		var k key
		header := rr[i].Header()
		if header != nil {
			k.header = *header
		}
		k.text = rr[i].String()
		s := fmt.Sprintf("%+v", k)
		rrhash[s] = struct{}{}
	}

	for i := range other {
		var k key
		header := other[i].Header()
		if header != nil {
			k.header = *header
		}
		k.text = other[i].String()
		s := fmt.Sprintf("%+v", k)
		if _, ok := rrhash[s]; !ok {
			return false
		}
		delete(rrhash, s)
	}
	return len(rrhash) == 0
}

func TestHTTP(t *testing.T) {
	// setup DNS server (just http)
	res, err := fakeDNS()
	if err != nil {
		t.Fatal(err)
	}
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

// TestHTTPAcceptApplicationJson tests that valid requests that specify
// 'Accept: application/json' succeed. This used to fail with
// 406 Not Acceptable.
// See https://jira.mesosphere.com/browse/DCOS_OSS-611
func TestHTTPAcceptApplicationJson(t *testing.T) {
	// setup DNS server (just http)
	res, err := fakeDNS()
	if err != nil {
		t.Fatal(err)
	}

	res.configureHTTP()
	srv := httptest.NewServer(http.DefaultServeMux)
	defer srv.Close()

	path := "/v1/services/_leader._tcp.mesos."
	req, err := http.NewRequest("GET", srv.URL+path, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Add("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET %s: StatusCode: got %d, want %d", path, resp.StatusCode, http.StatusOK)
	}
}

func fakeDNS() (*Resolver, error) {
	config := records.NewConfig()
	config.Masters = []string{"144.76.157.37:5050"}
	config.RecurseOn = false
	config.IPSources = []string{"netinfo", "docker", "mesos", "host"}

	res := New("", config)
	res.rng.Seed(0) // for deterministic tests

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		return nil, err
	}

	var sj state.State
	err = json.Unmarshal(b, &sj)
	if err != nil {
		return nil, err
	}

	spec := labels.RFC952
	err = res.rs.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", res.config.Masters, res.config.IPSources, spec)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func TestMultiError(t *testing.T) {
	me := multiError(nil)
	me.Add()
	me.Add(nil)
	me.Add(multiError(nil))
	if !me.Nil() {
		t.Fatalf("Expected Nil() multiError instead of %q", me.Error())
	}

	me.Add(errors.New("abc"))
	me.Add(errors.New("123"))
	me.Add(multiError(nil))
	me.Add(multiError([]error{errors.New("456")}))
	me.Add(multiError{errors.New("789")})
	me.Add(errors.New("def"))

	const expected = "abc; 123; 456; 789; def"
	actual := me.Error()
	if expected != actual {
		t.Fatalf("expected %q instead of %q", expected, actual)
	}
}

func TestTruncateSetTruncateBit(t *testing.T) {
	testTruncateSetTruncateBit(t, dns.MinMsgSize)
}

func TestTruncateEdns0SetTruncateBit(t *testing.T) {
	testTruncateSetTruncateBit(t, 4096)
}

func TestTruncateNoSetTruncateBit(t *testing.T) {
	testTruncateNoSetTruncateBit(t, dns.MinMsgSize)
}

func TestTruncateEdns0NoSetTruncateBit(t *testing.T) {
	testTruncateNoSetTruncateBit(t, 4096)
}

func testTruncateSetTruncateBit(t *testing.T, max uint16) {
	msg := newMessage(max)
	truncate(msg, max, true)
	if !msg.Truncated {
		t.Fatal("Message not truncated")
	}
	if l := msg.Len(); l > int(max) {
		t.Fatalf("Message too large: %d bytes", l)
	}
	before := msg.Len()
	// test double truncate
	truncate(msg, max, true)
	if !msg.Truncated {
		t.Fatal("Original truncation status was not preserved")
	}
	if msg.Len() != before {
		t.Fatal("Further modification to already truncated message")
	}
	// Add another answer to a truncated message and test that it is then
	// too large.
	msg.Answer = append(msg.Answer, genA(1)...)
	if l := msg.Len(); l < int(max) {
		t.Fatalf("Message to small after adding answers: %d bytes", l)
	}
}

func testTruncateNoSetTruncateBit(t *testing.T, max uint16) {
	msg := newMessage(max)
	size := msg.Len()
	truncate(msg, max, false)
	if msg.Len() == size {
		t.Fatal("truncating a large message did not diminish its size")
	}
	if msg.Len() > int(max) {
		t.Fatal("message not truncated")
	}
	if msg.Truncated {
		t.Fatal("Truncate bit set even though setTruncateBit was false")
	}
	before := msg.Len()
	// We set the Truncate bit on the message and confirm that the bit is
	// cleared even though the message did not need to be truncated.
	// This asserts that no matter the message or its size, truncate() will
	// clear the Truncate bit if setTruncateBit=false is passed to truncate().
	msg.Truncated = true
	truncate(msg, max, false)
	if msg.Truncated {
		t.Fatal("Truncate bit not cleared")
	}
	if msg.Len() != before {
		t.Fatal("Message truncated further")
	}
	msg.Answer = append(msg.Answer, genA(1)...)
	if l := msg.Len(); l < int(max) {
		t.Fatalf("Message too small after adding answers: %d bytes", l)
	}
}

func TestTruncateAnswers(t *testing.T) {
	// Test truncating messages by starting with empty answers and increasing
	// the number of answers until twice the maximum size is reached and test
	// that certain invariants are maintained at all times.
	// Testing boundary conditions only would be typical if the problem space
	// was large, however in this case since the maximum size is so small
	// we test the space exhaustively.
	max := uint16(4096)
	for nn := 0; ; nn++ {
		msg := Message(
			Question("example.com.", dns.TypeA),
			Header(false, dns.RcodeSuccess),
			Answers(genA(nn)...))
		before := msg.Len()
		if before > int(max)*7 {
			// We've really exhausted the problem space by creating
			// messages that have minimal size, all the way too messages
			// that are way bigger they are allowed to be.
			return
		}
		if before <= int(max) {
			// This message is not too large.
			// Assert that we don't truncate messages that
			// don't need to be truncated
			truncateAnswers(msg, max)
			if msg.Len() != before {
				t.Fatal("small message truncated further")
			}
			// Add another answer to check for the case where adding another
			// answer pushes the message size over the max limit. We expect
			// truncateAnswers to drop the last answer.
			msg.Answer = append(msg.Answer, genA(1)...)
			if msg.Len() > int(max) {
				truncateAnswers(msg, max)
				if msg.Len() != before {
					t.Fatal("truncateAnswers did not drop the last answer.")
				}
			}
			continue
		}
		// This message is too large, we truncate its list of answers and
		// check that it narrowly fits afterwards.
		truncateAnswers(msg, max)
		if msg.Len() > int(max) {
			t.Fatal("truncateAnswers did not truncate enough")
		}
		msg.Answer = append(msg.Answer, genA(1)...)
		if msg.Len() <= int(max) {
			t.Fatal("truncateAnswers truncated the message too much")
		}
	}
}

func TestMaxMsgSizeTCP(t *testing.T) {
	msg := newTruncated()
	msg.SetEdns0(4096, false)
	edns0 := msg.IsEdns0()
	if edns0 == nil {
		t.Fatal("(*dns.Msg).SetEdns0() is broken")
	}
	if v := maxMsgSize(false, nil); v != dns.MaxMsgSize {
		t.Fatalf("Wrong max message size: %d", v)
	}
	if v := maxMsgSize(false, edns0); v != dns.MaxMsgSize {
		t.Fatalf("Wrong max message size: %d", v)
	}
}

func TestMaxMsgSizeUDP(t *testing.T) {
	msg := newTruncated()
	msg.SetEdns0(4096, false)
	edns0 := msg.IsEdns0()
	if edns0 == nil {
		t.Fatal("(*dns.Msg).SetEdns0() is broken")
	}
	if v := maxMsgSize(true, nil); v != dns.MinMsgSize {
		t.Fatalf("Wrong max message size: %d", v)
	}
	if v := maxMsgSize(true, edns0); v != 4096 {
		t.Fatalf("Wrong max message size: %d", v)
	}
}

func BenchmarkTruncate(b *testing.B) {
	for n := 0; n < b.N; n++ {
		newTruncated()
	}
}

func newTruncated() *dns.Msg {
	msg := newMessage(dns.MinMsgSize)
	truncate(msg, dns.MinMsgSize, true)
	return msg
}

// newMessage creates a message that is strictly larger the given minimum size
func newMessage(minsize uint16) (msg *dns.Msg) {
	for ii := 1; ii <= int(minsize); ii++ {
		msg = Message(
			Question("example.com.", dns.TypeA),
			Header(false, dns.RcodeSuccess),
			Answers(genA(50*ii)...))
		if msg.Len() > int(minsize) {
			break
		}
	}
	if minsize > dns.MinMsgSize {
		// This is a large response so we set the EDNS0 record.
		msg.SetEdns0(minsize, false)
	}
	return msg
}

// TestNewMessage checks that newMessage returns messages that are larger
// than specified. We do so in its own test so we don't need have this
// assertion in the individual testTruncate* functions.
func TestNewMessage(t *testing.T) {
	msg := newMessage(dns.MinMsgSize)
	if msg.Len() <= dns.MinMsgSize {
		t.Fatal("newMessage result too small")
	}
	msg = newMessage(4096)
	if msg.Len() <= 4096 {
		t.Fatal("newMessage result too small")
	}
}

func genA(n int) []dns.RR {
	records := make([]dns.RR, n)
	ip := []byte{0, 0, 0, 0}
	for i := 0; i < n; i++ {
		binary.PutUvarint(ip, uint64(i))
		records[i] = A(RRHeader("example.com.", dns.TypeA, 60), ip)
	}
	return records
}
