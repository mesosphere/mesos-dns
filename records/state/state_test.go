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
