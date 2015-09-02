package detect

import (
	"reflect"
	"testing"

	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesosphere/mesos-dns/logging"
)

func TestMasters_UpdatedMasters(t *testing.T) {
	// create a new masters detector with an unknown leader and no masters
	ch := make(chan []string, 1)
	m := NewMasters([]string{}, ch)

	for i, tt := range []struct {
		masters []*mesos.MasterInfo
		want    []string
	}{
		{
			// update a single master
			// leave the unknown leader "" unchanged
			newMasterInfos([]string{"a"}),
			[]string{"", "a:5050"},
		},
		{
			// update additional masters,
			// expect them to be appended with the default port number,
			// leave the unknown leader "" unchanged
			newMasterInfos([]string{"b", "c", "d"}),
			[]string{"", "b:5050", "c:5050", "d:5050"},
		},
		{
			// update additional masters with an empty slice
			// expect no update at all (nil)
			newMasterInfos([]string{}),
			nil,
		},
		{
			// update masters with a niladic value
			// expect no update at all (nil)
			nil,
			nil,
		},
	} {
		m.UpdatedMasters(tt.masters)

		if got := recv(ch); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: got %v, want: %v", i, got, tt.want)
		}
	}
}

func TestMasters_OnMasterChanged(t *testing.T) {
	// create a new masters detector with an unknown leader
	// and two initial masters "a:5050", "b:5050"
	ch := make(chan []string, 1)
	m := NewMasters([]string{"a:5050", "b:5050"}, ch)

	for i, tt := range []struct {
		leader *mesos.MasterInfo
		want   []string
	}{
		{
			// update new leader "a",
			// expect an appended port number
			// leaving "b:5050" as the only additional master
			newMasterInfo("a"),
			[]string{"a:5050", "b:5050"},
		},
		{
			// update new leader "c"
			// replacing "a:5050"
			newMasterInfo("c"),
			[]string{"c:5050", "b:5050"},
		},
		{
			// update new leader "b"
			// replacing "c"
			newMasterInfo("b"),
			[]string{"b:5050"},
		},
		{
			// update new leader "", the hostname being the empty string
			// expect to fallback to its ip address,
			// being 0 by default, implying "0.0.0.0"
			newMasterInfo(""),
			[]string{"0.0.0.0:5050"},
		},
		{
			// update new leader with a niladic value
			// expect no update at all (nil)
			nil,
			nil,
		},
	} {
		m.OnMasterChanged(tt.leader)

		if got := recv(ch); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("test #%d: got %v, want: %v", i, got, tt.want)
		}
	}
}

// recv receives from a channel in a non-blocking way, returning the received value or nil.
func recv(ch <-chan []string) []string {
	select {
	case val := <-ch:
		return val
	default:
		return nil
	}
}

func newMasterInfo(hostname string) *mesos.MasterInfo {
	return &mesos.MasterInfo{Hostname: &hostname}
}

func newMasterInfos(hostnames []string) []*mesos.MasterInfo {
	ms := make([]*mesos.MasterInfo, len(hostnames))

	for i, h := range hostnames {
		ms[i] = newMasterInfo(h)
	}

	return ms
}

func init() {
	// TODO(tsenart): Refactor the logging package
	logging.VerboseFlag = true
	logging.SetupLogs()
}
