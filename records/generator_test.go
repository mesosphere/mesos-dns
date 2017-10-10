package records

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records/labels"
	"github.com/mesosphere/mesos-dns/records/state"
)

func init() {
	logging.VerboseFlag = false
	logging.VeryVerboseFlag = false
	logging.SetupLogs()
}

func (rg *RecordGenerator) exists(name, host string, kind rrsKind) bool {
	rrsByKind := kind.rrs(rg)
	if val, ok := rrsByKind[name]; ok {
		_, ok := val[host]
		return ok
	}
	return false
}

func TestParseState_SOAMname(t *testing.T) {
	rg := &RecordGenerator{}
	rg.stateLoader = func(_ []string) (s state.State, err error) {
		s.Leader = "foo@0.1.2.3:45" // required or else ParseState bails
		return
	}
	cfg1 := Config{SOAMname: "jdef123.mesos.", Listener: "4.5.6.7"}
	if err := rg.ParseState(cfg1); err != nil {
		t.Fatal("unexpected error", err)
	} else if !rg.exists("jdef123.mesos.", "4.5.6.7", A) {
		t.Fatalf("failed to locate A record for SOAMname, A records: %#v", rg.As)
	}
	cfg2 := Config{SOAMname: "ack456.mesos.", Listener: "2001:db8::1"}
	if err := rg.ParseState(cfg2); err != nil {
		t.Fatal("unexpected error", err)
	} else if !rg.exists("ack456.mesos.", "2001:db8::1", AAAA) {
		t.Fatalf("failed to locate AAAA record for SOAMname, AAAA records: %#v", rg.AAAAs)
	}
}

type expectedRR struct {
	name string
	host string
	kind rrsKind
}

func TestMasterRecord(t *testing.T) {
	// masterRecord(domain string, masters []string, leader string)
	tt := []struct {
		domain  string
		masters []string
		leader  string
		expect  []expectedRR
	}{
		{"foo.com", nil, "", nil},
		{"foo.com", nil, "@", nil},
		{"foo.com", nil, "1@", nil},
		{"foo.com", nil, "@2", nil},
		{"foo.com", nil, "3@4", nil},
		{"foo.com", nil, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.6", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		{"foo.com", nil, "5@[2001:db8::1]:7",
			[]expectedRR{
				{"leader.foo.com.", "2001:db8::1", AAAA},
				{"master.foo.com.", "2001:db8::1", AAAA},
				{"master0.foo.com.", "2001:db8::1", AAAA},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// single master: leader and fallback
		{"foo.com", []string{"0.0.0.6:7"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.6", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// leader not in fallback list
		{"foo.com", []string{"0.0.0.8:9"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.8", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// duplicate fallback masters, leader not in fallback list
		{"foo.com", []string{"0.0.0.8:9", "0.0.0.8:9"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.8", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// leader that's also listed in the fallback list (at the end)
		{"foo.com", []string{"0.0.0.8:9", "0.0.0.6:7"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.8", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// duplicate leading masters in the fallback list
		{"foo.com", []string{"0.0.0.8:9", "0.0.0.6:7", "0.0.0.6:7"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master0.foo.com.", "0.0.0.8", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		// leader that's also listed in the fallback list (in the middle)
		{"foo.com", []string{"0.0.0.8:9", "0.0.0.6:7", "0.0.0.7:0"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master.foo.com.", "0.0.0.7", A},
				{"master0.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master2.foo.com.", "0.0.0.7", A},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
		{"foo.com", []string{"0.0.0.8:9", "0.0.0.6:7", "[2001:db8::1]:0"}, "5@0.0.0.6:7",
			[]expectedRR{
				{"leader.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.6", A},
				{"master.foo.com.", "0.0.0.8", A},
				{"master.foo.com.", "2001:db8::1", AAAA},
				{"master0.foo.com.", "0.0.0.8", A},
				{"master1.foo.com.", "0.0.0.6", A},
				{"master2.foo.com.", "2001:db8::1", AAAA},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", SRV},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", SRV},
			}},
	}
	for i, tc := range tt {
		rg := &RecordGenerator{}
		rg.As = make(rrs)
		rg.AAAAs = make(rrs)
		rg.SRVs = make(rrs)
		t.Logf("test case %d", i+1)
		rg.masterRecord(tc.domain, tc.masters, tc.leader)
		if tc.expect == nil {
			if len(rg.As) > 0 {
				t.Fatalf("test case %d: unexpected As: %v", i+1, rg.As)
			}
			if len(rg.AAAAs) > 0 {
				t.Fatalf("test case %d: unexpected AAAAs: %v", i+1, rg.AAAAs)
			}
			if len(rg.SRVs) > 0 {
				t.Fatalf("test case %d: unexpected SRVs: %v", i+1, rg.SRVs)
			}
		}
		eA, eAAAA, eSRV, err := expectRecords(rg, tc.expect)
		if err != nil {
			t.Fatalf("test case %d: %s, As=%v, AAAAs=%v, SRVs=%v", i+1, err, rg.As, rg.AAAAs, rg.SRVs)
		}
		if !reflect.DeepEqual(rg.As, eA) {
			t.Fatalf("test case %d: expected As of %v instead of %v", i+1, eA, rg.As)
		}
		if !reflect.DeepEqual(rg.AAAAs, eAAAA) {
			t.Fatalf("test case %d: expected AAAAs of %v instead of %v", i+1, eAAAA, rg.AAAAs)
		}
		if !reflect.DeepEqual(rg.SRVs, eSRV) {
			t.Fatalf("test case %d: expected SRVs of %v instead of %v", i+1, eSRV, rg.SRVs)
		}
	}
}

func expectRecords(rg *RecordGenerator, expect []expectedRR) (eA, eAAAA, eSRV rrs, err error) {
	eA = make(rrs)
	eAAAA = make(rrs)
	eSRV = make(rrs)
	for _, e := range expect {
		found := rg.exists(e.name, e.host, e.kind)
		if !found {
			err = fmt.Errorf("missing expected record: name=%q host=%q kind=%s", e.name, e.host, e.kind)
			return
		}
		switch e.kind {
		case A:
			eA.add(e.name, e.host)
		case AAAA:
			eAAAA.add(e.name, e.host)
		case SRV:
			eSRV.add(e.name, e.host)
		default:
			err = fmt.Errorf("unexpected kind: %q", e.kind)
			return
		}
	}
	return
}

func testRecordGenerator(t *testing.T, spec labels.Func, ipSources []string) RecordGenerator {
	var sj state.State

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		t.Fatal(err)
	} else if err = json.Unmarshal(b, &sj); err != nil {
		t.Fatal(err)
	}

	sj.Leader = "master@144.76.157.37:5050"
	masters := []string{"144.76.157.37:5050"}

	var rg RecordGenerator
	if err := rg.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters, ipSources, spec); err != nil {
		t.Fatal(err)
	}

	return rg
}

// ensure we are parsing what we think we are
func TestInsertState(t *testing.T) {
	rg := testRecordGenerator(t, labels.RFC952, []string{"netinfo", "docker", "mesos", "host"})
	rgDocker := testRecordGenerator(t, labels.RFC952, []string{"docker", "host"})
	rgMesos := testRecordGenerator(t, labels.RFC952, []string{"mesos", "host"})
	rgSlave := testRecordGenerator(t, labels.RFC952, []string{"host"})
	rgNetinfo := testRecordGenerator(t, labels.RFC952, []string{"netinfo"})

	for i, tt := range []struct {
		rrs  rrs
		name string
		want []string
	}{
		{rg.As, "big-dog.marathon.mesos.", []string{"10.3.0.1"}},
		{rg.As, "liquor-store.marathon.mesos.", []string{"10.3.0.1", "10.3.0.2"}},
		{rg.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rg.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},
		{rg.As, "nginx.marathon.mesos.", []string{"10.3.0.3"}},
		{rg.As, "poseidon.marathon.mesos.", nil},
		{rg.As, "poseidon.marathon.slave.mesos.", nil},
		{rg.As, "master.mesos.", []string{"144.76.157.37"}},
		{rg.As, "master0.mesos.", []string{"144.76.157.37"}},
		{rg.As, "leader.mesos.", []string{"144.76.157.37"}},
		{rg.As, "slave.mesos.", []string{"1.2.3.10", "1.2.3.11", "1.2.3.12"}},
		{rg.As, "some-box.chronoswithaspaceandmixe.mesos.", []string{"1.2.3.11"}}, // ensure we translate the framework name as well
		{rg.As, "marathon.mesos.", []string{"1.2.3.11"}},

		{rg.AAAAs, "toy-store.ipv6-framework.mesos.", []string{"fd01:b::1:8000:2"}},
		{rg.AAAAs, "toy-store.ipv6-framework.slave.mesos.", []string{"2001:db8::1"}},
		{rg.AAAAs, "ipv6-framework.mesos.", []string{"2001:db8::1"}},
		{rg.AAAAs, "slave.mesos.", []string{"2001:db8::1"}},

		{rg.SRVs, "_big-dog._tcp.marathon.mesos.", []string{
			"big-dog-4dfjd-0.marathon.mesos.:80",
			"big-dog-4dfjd-0.marathon.mesos.:443",
		}},
		{rg.SRVs, "_poseidon._tcp.marathon.mesos.", nil},
		{rg.SRVs, "_leader._tcp.mesos.", []string{"leader.mesos.:5050"}},
		{rg.SRVs, "_liquor-store._tcp.marathon.mesos.", []string{
			"liquor-store-4dfjd-0.marathon.mesos.:80",
			"liquor-store-4dfjd-0.marathon.mesos.:443",
			"liquor-store-zasmd-1.marathon.mesos.:80",
			"liquor-store-zasmd-1.marathon.mesos.:443",
		}},
		{rg.SRVs, "_liquor-store._udp.marathon.mesos.", nil},
		{rg.SRVs, "_liquor-store.marathon.mesos.", nil},
		{rg.SRVs, "_https._liquor-store._tcp.marathon.mesos.", []string{
			"liquor-store-4dfjd-0.marathon.mesos.:443",
			"liquor-store-zasmd-1.marathon.mesos.:443",
		}},
		{rg.SRVs, "_http._liquor-store._tcp.marathon.mesos.", []string{
			"liquor-store-4dfjd-0.marathon.mesos.:80",
			"liquor-store-zasmd-1.marathon.mesos.:80",
		}},
		{rg.SRVs, "_car-store._tcp.marathon.mesos.", []string{
			"car-store-zinaz-0.marathon.slave.mesos.:31364",
			"car-store-zinaz-0.marathon.slave.mesos.:31365",
		}},
		{rg.SRVs, "_car-store._udp.marathon.mesos.", []string{
			"car-store-zinaz-0.marathon.slave.mesos.:31364",
			"car-store-zinaz-0.marathon.slave.mesos.:31365",
		}},
		{rg.SRVs, "_slave._tcp.mesos.", []string{"slave.mesos.:5051"}},
		{rg.SRVs, "_framework._tcp.marathon.mesos.", []string{"marathon.mesos.:25501"}},

		{rgSlave.As, "liquor-store.marathon.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgSlave.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgSlave.As, "nginx.marathon.mesos.", []string{"1.2.3.11"}},
		{rgSlave.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},

		{rgSlave.AAAAs, "toy-store.ipv6-framework.mesos.", []string{"2001:db8::1"}},
		{rgSlave.AAAAs, "toy-store.ipv6-framework.slave.mesos.", []string{"2001:db8::1"}},

		{rgMesos.As, "liquor-store.marathon.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgMesos.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgMesos.As, "nginx.marathon.mesos.", []string{"10.3.0.3"}},
		{rgMesos.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},

		{rgMesos.AAAAs, "toy-store.ipv6-framework.mesos.", []string{"2001:db8::1"}},
		{rgMesos.AAAAs, "toy-store.ipv6-framework.slave.mesos.", []string{"2001:db8::1"}},

		{rgNetinfo.As, "toy-store.ipv6-framework.mesos.", []string{"12.0.1.2"}},

		{rgNetinfo.AAAAs, "toy-store.ipv6-framework.mesos.", []string{"fd01:b::1:8000:2"}},
		{rgNetinfo.AAAAs, "toy-store.ipv6-framework.slave.mesos.", []string{"2001:db8::1"}},

		{rgDocker.As, "liquor-store.marathon.mesos.", []string{"10.3.0.1", "10.3.0.2"}},
		{rgDocker.As, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{rgDocker.As, "nginx.marathon.mesos.", []string{"1.2.3.11"}},
		{rgDocker.As, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},
		{rgDocker.AAAAs, "toy-store.ipv6-framework.mesos.", []string{"2001:db8::1"}},
		{rgDocker.AAAAs, "toy-store.ipv6-framework.slave.mesos.", []string{"2001:db8::1"}},
	} {
		// convert want into a map[string]struct{} (string set) for simpler comparison
		// via reflect.DeepEqual
		want := map[string]struct{}{}
		for _, x := range tt.want {
			want[x] = struct{}{}
		}
		if got := tt.rrs[tt.name]; !reflect.DeepEqual(got, want) {
			if len(got) == 0 && len(want) == 0 {
				continue
			}
			t.Errorf("test #%d: %q: got: %q, want: %q", i+1, tt.name, got, want)
		}
	}
}

// ensure we only generate one A record for each host
func TestNTasks(t *testing.T) {
	rg := &RecordGenerator{}
	rg.As = make(rrs)

	rg.insertRR("blah.mesos", "10.0.0.1", A)
	rg.insertRR("blah.mesos", "10.0.0.1", A)
	rg.insertRR("blah.mesos", "10.0.0.2", A)

	k := rg.As["blah.mesos"]

	if len(k) != 2 {
		t.Error("should only have 2 A records")
	}
}

func TestHashString(t *testing.T) {
	val := hashString("test")
	if len(val) != 5 {
		t.Fatal("Hash length not 5")
	}
	if val != "iffe9" {
		t.Fatal("hashString(test) != iffe9")
	}
}
func TestHashStringCollisions(t *testing.T) {
	if testing.Short() {
		t.Skip("Quickcheck - skipping for short mode.")
	}

	// Quickcheck config with seed from random.org
	// for deterministic behaviour
	quickConfig := &quick.Config{
		MaxCount: 1e7,
		Rand:     rand.New(rand.NewSource(34553613)),
	}
	fn := func(a, b string) bool { return (a == b) || (hashString(a) != hashString(b)) }
	if err := quick.Check(fn, quickConfig); err != nil {
		t.Fatal("hashString collision encountered ", err)
	}
}

func TestTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("Integration test - skipping for short mode.")
	}

	sleepForeverHandler := func(w http.ResponseWriter, req *http.Request) {
		req.Close = true
		notify := w.(http.CloseNotifier).CloseNotify()
		<-notify
	}
	server := httptest.NewServer(http.HandlerFunc(sleepForeverHandler))
	defer server.Close()

	rg := NewRecordGenerator(WithConfig(Config{StateTimeoutSeconds: 1}))
	_, err := rg.stateLoader([]string{server.Listener.Addr().String()})
	if err == nil {
		t.Fatal("Expect error because of timeout handler")
	}
	urlErr, ok := (err).(*url.Error)
	if !ok {
		t.Fatalf("Expected url.Error, instead: %#v", err)
	}
	netErr, ok := urlErr.Err.(net.Error)
	if !ok {
		t.Fatalf("Expected net.Error, instead: %#v", urlErr)
	}
	if !netErr.Timeout() {
		t.Errorf("Did not receive a timeout, instead: %#v", err)
	}
}
