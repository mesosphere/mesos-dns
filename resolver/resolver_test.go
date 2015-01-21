package resolver

import (
	"encoding/json"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/miekg/dns"
	"io/ioutil"
	"strconv"
	"testing"
	"time"
)

// dig @127.0.0.1 -p 8053 "bob.*.mesos" ANY
func TestCleanWild(t *testing.T) {
	dom := "bob.*.mesos"

	stripped := cleanWild(dom)

	if stripped != "bob.mesos" {
		t.Error("not stripping domain")
	}
}

func TestSplitDomain(t *testing.T) {
	var res Resolver

	host, port := res.splitDomain("bob.com:12345")

	if host != "bob.com" {
		t.Error("not grabbing host")
	}

	if port != 12345 {
		t.Error("not grabbing port")
	}

}

func TestShuffleAnswers(t *testing.T) {
	var res Resolver

	m := new(dns.Msg)

	for i := 0; i < 10; i++ {
		name := "10.0.0." + strconv.Itoa(i) + ":1234"

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

func fakeDNS(port int) (Resolver, error) {
	logging.VerboseFlag = false
	logging.SetupLogs()

	var res Resolver
	res.Config = records.Config{
		TTL:       60,
		Port:      port,
		Domain:    "mesos",
		Resolvers: records.GetLocalDNS(),
	}

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		return res, err
	}

	var sj records.StateJSON
	err = json.Unmarshal(b, &sj)
	if err != nil {
		return res, err
	}

	res.rs = records.RecordGenerator{}
	res.rs.InsertState(sj, "mesos")

	return res, nil
}

func fakeQuery(dom string, rrHeader uint16, proto string) ([]dns.RR, error) {
	qc := uint16(dns.ClassINET)

	c := new(dns.Client)
	c.Net = proto

	m := new(dns.Msg)
	m.Question = make([]dns.Question, 1)
	m.Question[0] = dns.Question{dns.Fqdn(dom), rrHeader, qc}

	in, _, err := c.Exchange(m, "127.0.0.1:8053")
	if err != nil {
		return in.Answer, err
	}

	return in.Answer, nil
}

func TestHandler(t *testing.T) {
	var msg []dns.RR

	res, err := fakeDNS(8053)
	if err != nil {
		t.Error(err)
	}

	dns.HandleFunc("mesos.", res.HandleMesos)
	go res.Serve("udp")
	go res.Serve("tcp")

	// wait for startup ? lame
	time.Sleep(10 * time.Millisecond)

	// test A records
	msg, err = fakeQuery("chronos.marathon-0.6.0.mesos.", dns.TypeA, "udp")
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 1 {
		t.Error("not serving up A records")
	}

	// test SRV record
	msg, err = fakeQuery("_liquor-store._udp.marathon-0.6.0.mesos.", dns.TypeSRV, "udp")
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 2 {
		t.Error("not serving up SRV records")
	}

	// test tcp
	msg, err = fakeQuery("chronos.marathon-0.6.0.mesos.", dns.TypeA, "tcp")
	if err != nil {
		t.Error(err)
	}

	if len(msg) != 1 {
		t.Error("not serving up A records")
	}

}

func TestNonMesosHandler(t *testing.T) {
	var msg []dns.RR

	res, err := fakeDNS(8054)
	if err != nil {
		t.Error(err)
	}

	dns.HandleFunc(".", res.HandleNonMesos)
	go res.Serve("udp")
	go res.Serve("tcp")

	// wait for startup ? lame
	time.Sleep(10 * time.Millisecond)

	// test A records
	msg, err = fakeQuery("google.com", dns.TypeA, "udp")
	if err != nil {
		t.Error(err)
	}

	if len(msg) < 2 {
		t.Error("not serving up A records")
	}

}
