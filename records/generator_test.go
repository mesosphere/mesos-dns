package records

import (
	"encoding/json"
	"github.com/mesosphere/mesos-dns/logging"
	"io/ioutil"
	"testing"
)

func init() {
	logging.VerboseFlag = false
	logging.SetupLogs()
}

func TestHostBySlaveId(t *testing.T) {

	slaves := []slave{
		{Id: "20140827-000744-3041283216-5050-2116-1", Hostname: "blah.com"},
		{Id: "33333333-333333-3333333333-3333-3333-2", Hostname: "blah.blah.com"},
	}

	rg := RecordGenerator{Slaves: slaves}

	for i := 0; i < len(slaves); i++ {
		host, err := rg.hostBySlaveId(slaves[i].Id)
		if err != nil {
			t.Error(err)
		}

		if host != slaves[i].Hostname {
			t.Error("wrong slave/hostname")
		}
	}

}

func TestYankPorts(t *testing.T) {
	p := "[31328-31328]"

	ports := yankPorts(p)

	if ports[0] != "31328" {
		t.Error("not parsing port")
	}
}

func TestMultipleYankPorts(t *testing.T) {
	p := "[31111-31111, 31113-31113]"

	ports := yankPorts(p)

	if len(ports) != 2 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31111" {
		t.Error("not parsing port")
	}

	if ports[1] != "31113" {
		t.Error("not parsing port")
	}
}

func TestRangePorts(t *testing.T) {
	p := "[31115-31117]"

	ports := yankPorts(p)

	if len(ports) != 3 {
		t.Error("not parsing ports")
	}

	if ports[0] != "31115" {
		t.Error("not parsing port")
	}

	if ports[1] != "31116" {
		t.Error("not parsing port")
	}

	if ports[2] != "31117" {
		t.Error("not parsing port")
	}

}

func TestLeaderIP(t *testing.T) {
	l := "master@144.76.157.37:5050"

	ip := leaderIP(l)

	if ip != "144.76.157.37" {
		t.Error("not parsing ip")
	}
}

type invalidHosts struct {
	host     string
	expected string
}

func TestStripInvalid(t *testing.T) {

	var tests = []invalidHosts{
		{"host.com", "host.com"},
		{"space space.com", "spacespace.com"},
		{"blah-dash.com", "blah-dash.com"},
		{"not$1234.com", "not1234.com"},
		{"(@ host . com", "host.com"},
		{"MiXeDcase.CoM", "mixedcase.com"},
	}

	for _, pair := range tests {
		url := stripInvalid(pair.host)
		if url != pair.expected {
			t.Error(
				"For", pair.host,
				"expected", pair.expected,
				"got", url,
			)
		}
	}

}

// ensure we are parsing what we think we are
func TestInsertState(t *testing.T) {

	var sj StateJSON

	b, err := ioutil.ReadFile("../factories/fake.json")
	if err != nil {
		t.Error("missing test data")
	}

	err = json.Unmarshal(b, &sj)
	if err != nil {
		t.Error(err)
	}
	sj.Leader = "master@144.76.157.37:5050"

	masters := []string{"144.76.157.37:5050"}
	rg := RecordGenerator{}
	rg.InsertState(sj, "mesos", "mesos-dns.mesos.", "127.0.0.1", masters)

	// ensure we are only collecting running tasks
	_, ok := rg.SRVs["_poseidon._tcp.marathon-0.6.0.mesos."]
	if ok {
		t.Error("should not find this not-running task - SRV record")
	}

	_, ok = rg.As["liquor-store.marathon-0.6.0.mesos."]
	if !ok {
		t.Error("should find this running task - A record")
	}

	_, ok = rg.As["poseidon.marathon-0.6.0.mesos."]
	if ok {
		t.Error("should not find this not-running task - A record")
	}

	_, ok = rg.As["master.mesos."]
	if !ok {
		t.Error("should find a running master - A record")
	}

	_, ok = rg.As["master0.mesos."]
	if !ok {
		t.Error("should find a running master0 - A record")
	}

	_, ok = rg.SRVs["_master._tcp.mesos."]
	if !ok {
		t.Error("should find a running master - SRV record")
	}

	_, ok = rg.As["leader.mesos."]
	if !ok {
		t.Error("should find a leading master - A record")
	}

	_, ok = rg.SRVs["_leader._tcp.mesos."]
	if !ok {
		t.Error("should find a leading master - SRV record")
	}

	// test for 12 SRV names
	if len(rg.SRVs) != 12 {
		t.Error("not enough SRVs")
	}

	// test for 5 A names
	if len(rg.As) != 8 {
		t.Error("not enough As")
	}

	// ensure we translate the framework name as well
	_, ok = rg.As["some-box.chronoswithaspaceandmixedcase-2.0.1.mesos."]
	if !ok {
		t.Error("should find this task w/a space in the framework name - A record")
	}

	// ensure we find this SRV
	rrs := rg.SRVs["_liquor-store._tcp.marathon-0.6.0.mesos."]

	// ensure there are 3 RRDATA answers for this SRV name
	if len(rrs) != 3 {
		t.Error("not enough SRV records")
	}

	// ensure we don't find this as a SRV record
	rrs = rg.SRVs["_liquor-store.marathon-0.6.0.mesos."]
	if len(rrs) != 0 {
		t.Error("not a proper SRV record")
	}

}

// ensure we only generate one A record for each host
func TestNTasks(t *testing.T) {
	rg := RecordGenerator{}
	rg.As = make(rrs)

	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.1", "A")
	rg.insertRR("blah.mesos", "10.0.0.2", "A")

	k, _ := rg.As["blah.mesos"]

	if len(k) != 2 {
		t.Error("should only have 2 A records")
	}
}
