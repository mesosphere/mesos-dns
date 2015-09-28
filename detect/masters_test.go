package detect

import (
	"encoding/binary"
	"net"
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
			newMasterInfos([]string{"1.1.1.1"}),
			[]string{"", "1.1.1.1:5050"},
		},
		{
			// update additional masters,
			// expect them to be appended with the default port number,
			// leave the unknown leader "" unchanged
			newMasterInfos([]string{"1.1.1.1", "1.1.1.2", "1.1.1.3"}),
			[]string{"", "1.1.1.1:5050", "1.1.1.2:5050", "1.1.1.3:5050"},
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
	// and two initial masters "1.1.1.1:5050", "1.1.1.2:5050"
	ch := make(chan []string, 1)
	m := NewMasters([]string{"1.1.1.1:5050", "1.1.1.2:5050"}, ch)

	for i, tt := range []struct {
		leader *mesos.MasterInfo
		want   []string
	}{
		{
			// update new leader "1.1.1.1",
			// expect an appended port number
			// leaving "1.1.1.2:5050" as the only additional master
			newMasterInfo("1.1.1.1"),
			[]string{"1.1.1.1:5050", "1.1.1.2:5050"},
		},
		{
			// update new leader "1.1.1.3"
			// replacing "1.1.1.1:5050"
			newMasterInfo("1.1.1.3"),
			[]string{"1.1.1.3:5050", "1.1.1.2:5050"},
		},
		{
			// update new leader "1.1.1.2"
			// replacing "1.1.1.3"
			newMasterInfo("1.1.1.2"),
			[]string{"1.1.1.2:5050"},
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

func newMasterInfo(ip string) *mesos.MasterInfo {
	ip4 := net.ParseIP(ip).To4()
	ipr := binary.LittleEndian.Uint32(ip4)

	return &mesos.MasterInfo{
		Ip: &ipr,
	}
}

func newMasterInfos(ips []string) []*mesos.MasterInfo {
	ms := make([]*mesos.MasterInfo, len(ips))

	for i, ip := range ips {
		ms[i] = newMasterInfo(ip)
	}

	return ms
}

func init() {
	// TODO(tsenart): Refactor the logging package
	logging.VerboseFlag = true
	logging.SetupLogs()
}
