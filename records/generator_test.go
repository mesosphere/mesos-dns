package records

import (
	"encoding/json"
	"io/ioutil"
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

func TestMasterRecord(t *testing.T) {
	// masterRecord(domain string, masters []string, leader string)
	type expectedRR struct {
		name  string
		host  string
		rtype string
	}
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
		{"foo.com", nil, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// single master: leader and fallback
		{"foo.com", []string{"6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master0.foo.com.", "6", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader not in fallback list
		{"foo.com", []string{"8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate fallback masters, leader not in fallback list
		{"foo.com", []string{"8:9", "8:9"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (at the end)
		{"foo.com", []string{"8:9", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// duplicate leading masters in the fallback list
		{"foo.com", []string{"8:9", "6:7", "6:7"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master0.foo.com.", "8", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
		// leader that's also listed in the fallback list (in the middle)
		{"foo.com", []string{"8:9", "6:7", "bob:0"}, "5@6:7",
			[]expectedRR{
				{"leader.foo.com.", "6", "A"},
				{"master.foo.com.", "6", "A"},
				{"master.foo.com.", "8", "A"},
				{"master.foo.com.", "bob", "A"},
				{"master0.foo.com.", "8", "A"},
				{"master1.foo.com.", "6", "A"},
				{"master2.foo.com.", "bob", "A"},
				{"_leader._tcp.foo.com.", "leader.foo.com.:7", "SRV"},
				{"_leader._udp.foo.com.", "leader.foo.com.:7", "SRV"},
			}},
	}
	for i, tc := range tt {
		rg := &RecordGenerator{}
		rg.As = make(rrs)
		rg.SRVs = make(rrs)
		t.Logf("test case %d", i+1)
		rg.masterRecord(tc.domain, tc.masters, tc.leader)
		if tc.expect == nil {
			if len(rg.As) > 0 {
				t.Fatalf("test case %d: unexpected As: %v", i+1, rg.As)
			}
			if len(rg.SRVs) > 0 {
				t.Fatalf("test case %d: unexpected SRVs: %v", i+1, rg.SRVs)
			}
		}
		expectedA := make(rrs)
		expectedSRV := make(rrs)
		for _, e := range tc.expect {
			found := rg.exists(e.name, e.host, e.rtype)
			if !found {
				t.Fatalf("test case %d: missing expected record: name=%q host=%q rtype=%s, As=%v", i+1, e.name, e.host, e.rtype, rg.As)
			}
			if e.rtype == "A" {
				expectedA[e.name] = append(expectedA[e.name], e.host)
			} else {
				expectedSRV[e.name] = append(expectedSRV[e.name], e.host)
			}
		}
		if !reflect.DeepEqual(rg.As, expectedA) {
			t.Fatalf("test case %d: expected As of %v instead of %v", i+1, expectedA, rg.As)
		}
		if !reflect.DeepEqual(rg.SRVs, expectedSRV) {
			t.Fatalf("test case %d: expected SRVs of %v instead of %v", i+1, expectedSRV, rg.SRVs)
		}
	}
}

func TestLeaderIP(t *testing.T) {
	l := "master@144.76.157.37:5050"

	ip := leaderIP(l)

	if ip != "144.76.157.37" {
		t.Error("not parsing ip")
	}
}

type kind int

const (
	a kind = iota
	srv
)

type TestRecord struct {
	kind kind
	name string
	want []string
}

func testRecords(t *testing.T, spec labels.Func, rs []TestRecord) {
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
	if err := rg.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters, spec); err != nil {
		t.Fatal(err)
	}

	for i, tt := range rs {
		var rrs rrs
		switch tt.kind {
		case a:
			rrs = rg.As
		case srv:
			rrs = rg.SRVs
		default:
			t.Fatalf("invalid test record kind %v", tt.kind)
		}

		if got := rrs[tt.name]; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: %d record for %q: got: %q, want: %q", i, tt.kind, tt.name, got, tt.want)
		}
	}
}

// ensure we are parsing what we think we are
func TestInsertState(t *testing.T) {
	testRecords(t, labels.RFC952, []TestRecord{
		{a, "liquor-store.marathon.mesos.", []string{"10.3.0.1", "10.3.0.2"}},
		{a, "liquor-store.marathon.slave.mesos.", []string{"1.2.3.11", "1.2.3.12"}},
		{a, "car-store.marathon.slave.mesos.", []string{"1.2.3.11"}},
		{a, "nginx.marathon.mesos.", []string{"10.3.0.3"}},
		{a, "poseidon.marathon.mesos.", nil},
		{a, "poseidon.marathon.slave.mesos.", nil},
		{a, "master.mesos.", []string{"144.76.157.37"}},
		{a, "master0.mesos.", []string{"144.76.157.37"}},
		{a, "leader.mesos.", []string{"144.76.157.37"}},
		{a, "slave.mesos.", []string{"1.2.3.10", "1.2.3.11", "1.2.3.12"}},
		{a, "some-box.chronoswithaspaceandmixe.mesos.", []string{"1.2.3.11"}}, // ensure we translate the framework name as well
		{a, "marathon.mesos.", []string{"1.2.3.11"}},
		{srv, "_poseidon._tcp.marathon.mesos.", nil},
		{srv, "_leader._tcp.mesos.", []string{"leader.mesos.:5050"}},
		{srv, "_liquor-store._tcp.marathon.mesos.", []string{
			"liquor-store-17700-0.marathon.mesos.:80",
			"liquor-store-17700-0.marathon.mesos.:443",
			"liquor-store-7581-1.marathon.mesos.:80",
			"liquor-store-7581-1.marathon.mesos.:443",
		}},
		{srv, "_liquor-store._udp.marathon.mesos.", nil},
		{srv, "_liquor-store.marathon.mesos.", nil},
		{srv, "_car-store._tcp.marathon.mesos.", []string{
			"car-store-50548-0.marathon.slave.mesos.:31364",
			"car-store-50548-0.marathon.slave.mesos.:31365",
		}},
		{srv, "_car-store._udp.marathon.mesos.", []string{
			"car-store-50548-0.marathon.slave.mesos.:31364",
			"car-store-50548-0.marathon.slave.mesos.:31365",
		}},
		{srv, "_slave._tcp.mesos.", []string{"slave.mesos.:5051"}},
		{srv, "_framework._tcp.marathon.mesos.", []string{"marathon.mesos.:25501"}},
	})
}

// ensure we only generate one A record for each host
func TestNTasks(t *testing.T) {
	rg := &RecordGenerator{}
	rg.As = make(rrs)

	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.2", "A")

	k, _ := rg.As["blah.mesos"]

	if len(k) != 2 {
		t.Error("should only have 2 A records")
	}
}

func TestHashString(t *testing.T) {
	t.Skip("TODO: Increase entropy, fix the bug!")
	fn := func(a, b string) bool { return hashString(a) != hashString(b) }
	if err := quick.Check(fn, &quick.Config{MaxCount: 1e9}); err != nil {
		t.Fatal(err)
	}
}
