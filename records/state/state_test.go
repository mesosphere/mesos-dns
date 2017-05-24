package state_test

import (
	"bytes"
	"encoding/json"
	"net"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/upid"
	. "github.com/mesosphere/mesos-dns/records/state"
)

func TestResources_Ports(t *testing.T) {
	r := Resources{PortRanges: "[31111-31111, 31115-31117]"}
	want := []string{"31111", "31115", "31116", "31117"}
	if got := r.Ports(); !reflect.DeepEqual(got, want) {
		t.Fatalf("got: %v, want: %v", got, want)
	}
}

func TestPID_UnmarshalJSON(t *testing.T) {
	makePID := func(id, host, port string) PID {
		return PID{UPID: &upid.UPID{ID: id, Host: host, Port: port}}
	}
	for i, tt := range []struct {
		data string
		want PID
		err  error
	}{
		{`"slave(1)@127.0.0.1:5051"`, makePID("slave(1)", "127.0.0.1", "5051"), nil},
		{`  "slave(1)@127.0.0.1:5051"  `, makePID("slave(1)", "127.0.0.1", "5051"), nil},
		{`"  slave(1)@127.0.0.1:5051  "`, makePID("slave(1)", "127.0.0.1", "5051"), nil},
	} {
		var pid PID
		if err := json.Unmarshal([]byte(tt.data), &pid); !reflect.DeepEqual(err, tt.err) {
			t.Errorf("test #%d: got err: %v, want: %v", i, err, tt.want)
		}
		if got := pid; !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: got: %v, want: %v", i, got, tt.want)
		}
	}
}

func TestTask_IPs(t *testing.T) {
	for i, tt := range []struct {
		*Task
		srcs []string
		want []*ScopedIP
	}{
		{nil, nil, nil},
		{nil, []string{}, nil},
		{nil, []string{"host"}, nil},
		{ // no IPs for the given sources
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"))))),
			srcs: []string{"host", "mesos"},
			want: nil,
		},
		{ // unknown IP sources are ignored
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"))))),
			srcs: []string{"foo", "netinfo", "bar"},
			want: ips(containerScope(""), "1.2.3.4"),
		},
		{ // multiple IPs on a NetworkInfo
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"), netinfo("2.3.4.5"))))),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.3.4", "2.3.4.5"),
		},
		{ // multiple NetworkInfos each with one IP
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4", "2.3.4.5"))))),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.3.4", "2.3.4.5"),
		},
		{ // back-compat with 0.25 IPAddress format
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(oldnetinfo("1.2.3.4"))))),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.3.4"),
		},
		{ // check back-compat doesn't break multi-netinfo case
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(oldnetinfo(""), netinfo("1.2.3.4"))))),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.3.4"),
		},
		{ // check that we prefer 0.26 IPAddresses over 0.25 IPAddress
			Task: task(statuses(status(state("TASK_RUNNING"), netinfos(oldnewnetinfo("1.2.3.4", "1.2.4.8"))))),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.4.8"),
		},
		{ // source order
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4")))),
			),
			srcs: []string{"host", "netinfo"},
			want: append(ips(hostScope(), "2.3.4.5"), ips(containerScope(""), "1.2.3.4")...),
		},
		{ // autoip only, w/o port discovery
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4")))),
			),
			srcs: []string{"autoip"},
			want: ips(hostScope()),
		},
		{ // autoip only, host scope
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4")))),
				discovery("task1", hostPort(80, "web")),
			),
			srcs: []string{"autoip"},
			want: ips(hostScope(), "2.3.4.5"),
		},
		{ // autoip only, network scope (unnamed), single network
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4")))),
				discovery("task1", containerPort(80, "web", "")),
			),
			srcs: []string{"autoip"},
			want: ips(containerScope(""), "1.2.3.4"),
		},
		{ // autoip only, network scope (unnamed), multi network
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"), netinfo("4.5.6.7")))),
				discovery("task1", containerPort(80, "web", "")),
			),
			srcs: []string{"autoip"},
			want: ips(containerScope(""), "1.2.3.4", "4.5.6.7"),
		},
		{ // autoip only, network scope (unnamed), multi network (1 named)
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"), namedNetinfo("net1", "4.5.6.7")))),
				discovery("task1", containerPort(80, "web", "")),
			),
			srcs: []string{"autoip"},
			want: ips(containerScope(""), "1.2.3.4", "4.5.6.7"),
		},
		{ // autoip only, network scope (named), multi network (1 named)
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"), namedNetinfo("net1", "4.5.6.7")))),
				discovery("task1", containerPort(80, "web", "net1")),
			),
			srcs: []string{"autoip"},
			want: ips(containerScope("net1"), "4.5.6.7"),
		},
		{ // autoip only, network scope (named + unnamed), multi network (1 named)
			Task: task(
				slaveIP("2.3.4.5"),
				statuses(status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"), namedNetinfo("net1", "4.5.6.7")))),
				discovery("task1", containerPort(80, "web", "net1"), containerPort(443, "tls", "")),
			),
			srcs: []string{"autoip"},
			want: append(ips(containerScope("net1"), "4.5.6.7"), ips(containerScope(""), "1.2.3.4", "4.5.6.7")...),
		},
		{ // statuses state
			Task: task(
				statuses(
					status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4"))),
					status(state("TASK_STOPPED"), netinfos(netinfo("2.3.4.5"))),
				),
			),
			srcs: []string{"netinfo"},
			want: ips(containerScope(""), "1.2.3.4"),
		},
		{ // statuses ordering
			Task: task(
				statuses(
					status(state("TASK_RUNNING"), netinfos(netinfo("1.2.3.4")), timestamp(1)),
					status(state("TASK_RUNNING"), netinfos(netinfo("1.3.5.7")), timestamp(4)),
					status(state("TASK_RUNNING"), labels(DockerIPLabel, "2.3.4.5"), timestamp(3)),
					status(state("TASK_RUNNING"), labels(DockerIPLabel, "2.4.6.8"), timestamp(5)),
					status(state("TASK_RUNNING"), labels(DockerIPLabel, "2.5.8.1"), timestamp(2)),
				),
			),
			srcs: []string{"docker", "netinfo"},
			want: ips(containerScope(""), "2.4.6.8"),
		},
		{ // label ordering
			Task: task(
				statuses(
					status(
						state("TASK_RUNNING"),
						labels(DockerIPLabel, "1.2.3.4", DockerIPLabel, "2.3.4.5"),
					),
				),
			),
			srcs: []string{"docker"},
			want: ips(containerScope(""), "1.2.3.4", "2.3.4.5"),
		},
	} {
		if got := tt.IPs(tt.srcs...); !reflect.DeepEqual(got, tt.want) {
			t.Logf("%+v", tt.Task)
			t.Errorf("test #%d: got %v, want %v", i, scopedIPs(got), scopedIPs(tt.want))
		}
	}
}

// test helpers

type (
	taskOpt   func(*Task)
	statusOpt func(*Status)
)

func scopedIPs(iplist []*ScopedIP) string {
	var buf bytes.Buffer
	for i := range iplist {
		if i > 0 {
			buf.WriteString(",")
		}
		buf.WriteString("[")
		buf.WriteString(string(iplist[i].Scope.NetworkScope))
		buf.WriteString(":")
		buf.WriteString(iplist[i].Scope.NetworkName)
		buf.WriteString("]")
		buf.WriteString(iplist[i].IP)
	}
	return buf.String()
}

func hostScope() Scope {
	return Scope{NetworkScopeHost, ""}
}

func containerScope(networkName string) Scope {
	return Scope{NetworkScopeContainer, networkName}
}

func ips(scope Scope, ss ...string) []*ScopedIP {
	if len(ss) == 0 {
		return nil
	}
	addrs := make([]*ScopedIP, len(ss))
	for i := range ss {
		addrs[i] = &ScopedIP{scope, net.ParseIP(ss[i]).String()}
	}
	return addrs
}

func task(opts ...taskOpt) *Task {
	var t Task
	for _, opt := range opts {
		opt(&t)
	}
	return &t
}

func statuses(st ...Status) taskOpt {
	return func(t *Task) {
		t.Statuses = append(t.Statuses, st...)
	}
}

func slaveIP(ip string) taskOpt {
	return func(t *Task) { t.SlaveIP = ip }
}

func discovery(name string, opts ...discoveryOpt) taskOpt {
	return func(t *Task) {
		t.DiscoveryInfo = DiscoveryInfo{Name: name}
		for _, f := range opts {
			f(&t.DiscoveryInfo)
		}
	}
}

type discoveryOpt func(*DiscoveryInfo)

func hostPort(port int, name string) discoveryOpt {
	return func(d *DiscoveryInfo) {
		d.Ports.DiscoveryPorts = append(d.Ports.DiscoveryPorts, DiscoveryPort{Number: port, Name: name})
	}
}

func containerPort(port int, name, networkName string) discoveryOpt {
	return func(d *DiscoveryInfo) {
		labels := []Label{
			Label{Key: "network-scope", Value: "container"},
		}
		if networkName != "" {
			labels = append(labels, Label{Key: "network-name", Value: networkName})
		}
		d.Ports.DiscoveryPorts = append(d.Ports.DiscoveryPorts,
			DiscoveryPort{
				Number: port,
				Name:   name,
				Labels: Labels{Labels: labels},
			})
	}
}

func status(opts ...statusOpt) Status {
	var s Status
	for _, opt := range opts {
		opt(&s)
	}
	return s
}

func labels(kvs ...string) statusOpt {
	if len(kvs)%2 != 0 {
		panic("odd number")
	}
	return func(s *Status) {
		for i := 0; i < len(kvs); i += 2 {
			s.Labels = append(s.Labels, Label{Key: kvs[i], Value: kvs[i+1]})
		}
	}
}

func state(st string) statusOpt {
	return func(s *Status) { s.State = st }
}

func netinfos(netinfos ...NetworkInfo) statusOpt {
	return func(s *Status) {
		s.ContainerStatus.NetworkInfos = append(s.ContainerStatus.NetworkInfos, netinfos...)
	}
}

func netinfo(ips ...string) NetworkInfo {
	netinfo := NetworkInfo{}
	for _, ip := range ips {
		netinfo.IPAddresses = append(netinfo.IPAddresses, IPAddress{ip})
	}
	return netinfo
}

func namedNetinfo(name string, ips ...string) NetworkInfo {
	netinfo := NetworkInfo{Name: name}
	for _, ip := range ips {
		netinfo.IPAddresses = append(netinfo.IPAddresses, IPAddress{ip})
	}
	return netinfo
}

// NetworkInfo using v0.25 syntax for storing a single IP.
func oldnetinfo(ip string) NetworkInfo {
	netinfo := NetworkInfo{}
	netinfo.IPAddress = ip
	return netinfo
}

// NetworkInfo using both 0.25 and 0.26 syntax for IPs.
func oldnewnetinfo(oldip string, newip string) NetworkInfo {
	netinfo := NetworkInfo{}
	netinfo.IPAddress = oldip
	netinfo.IPAddresses = append(netinfo.IPAddresses, IPAddress{newip})
	return netinfo
}

func timestamp(t float64) statusOpt {
	return func(s *Status) { s.Timestamp = t }
}
