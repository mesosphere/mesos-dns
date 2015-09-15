package state

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/mesos/mesos-go/upid"
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

func TestTask_containerIP(t *testing.T) {
	makeTask := func(networkInfoAddress string, label Label) Task {
		labels := make([]Label, 0, 1)
		if label.Key != "" {
			labels = append(labels, label)
		}

		var containerStatus ContainerStatus
		if networkInfoAddress != "" {
			containerStatus = ContainerStatus{
				NetworkInfos: []NetworkInfo{
					NetworkInfo{
						IPAddress: networkInfoAddress,
					},
				},
			}
		}

		return Task{
			State: "TASK_RUNNING",
			Statuses: []Status{
				Status{
					Timestamp:       1.0,
					State:           "TASK_RUNNING",
					Labels:          labels,
					ContainerStatus: containerStatus,
				},
			},
		}
	}

	// Verify IP extraction from NetworkInfo
	task := makeTask("1.2.3.4", Label{})
	if task.containerIP("mesos") != "1.2.3.4" {
		t.Errorf("Failed to extract IP from NetworkInfo")
	}

	// Verify IP extraction from NetworkInfo takes precedence over
	// labels
	task = makeTask("1.2.3.4",
		Label{Key: ipLabels["mesos"], Value: "2.4.6.8"})
	if task.containerIP("mesos") != "1.2.3.4" {
		t.Errorf("Failed to extract IP from NetworkInfo when label also supplied")
	}

	// Verify IP extraction from the Mesos label without NetworkInfo
	task = makeTask("",
		Label{Key: ipLabels["mesos"], Value: "1.2.3.4"})
	if task.containerIP("mesos") != "1.2.3.4" {
		t.Errorf("Failed to extract IP from Mesos label")
	}

	// Verify IP extraction from the Docker label without NetworkInfo
	task = makeTask("",
		Label{Key: ipLabels["docker"], Value: "1.2.3.4"})
	if task.containerIP("docker") != "1.2.3.4" {
		t.Errorf("Failed to extract IP from Docker label")
	}
}
