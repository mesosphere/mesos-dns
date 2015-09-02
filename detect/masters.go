package detect

import (
	"encoding/binary"
	"net"
	"strconv"

	"github.com/mesos/mesos-go/detector"
	mesos "github.com/mesos/mesos-go/mesosproto"

	"github.com/mesosphere/mesos-dns/logging"
)

var (
	_ detector.MasterChanged = (*Masters)(nil)
	_ detector.AllMasters    = (*Masters)(nil)
)

// Masters detects changes of leader and/or master elections
// and sends these changes to a channel.
type Masters struct {
	// current masters list,
	// 1st item represents the leader,
	// the rest remaining masters
	masters []string

	// the channel leader/master changes are being sent to
	changed chan<- []string
}

// NewMasters returns a new Masters detector with the given initial masters
// and the given changed channel to which master changes will be sent to.
// Initially the leader is unknown which is represented by
// setting the first item of the sent masters slice to be empty.
func NewMasters(masters []string, changed chan<- []string) *Masters {
	return &Masters{
		masters: append([]string{""}, masters...),
		changed: changed,
	}
}

// OnMasterChanged sets the given MasterInfo as the current leader
// leaving the remaining masters unchanged and emits the current masters state.
// It implements the detector.MasterChanged interface.
func (ms *Masters) OnMasterChanged(leader *mesos.MasterInfo) {
	logging.VeryVerbose.Println("Updated leader: ", leader)

	if leader == nil {
		logging.Error.Println("No master available in Zookeeper.")
		return
	}

	ms.masters = ordered(masterHostPort(leader), ms.masters[1:])
	emit(ms.changed, ms.masters)
}

// UpdatedMasters sets the given slice of MasterInfo as the current remaining masters
// leaving the current leader unchanged and emits the current masters state.
// It implements the detector.AllMasters interface.
func (ms *Masters) UpdatedMasters(infos []*mesos.MasterInfo) {
	logging.VeryVerbose.Println("Updated masters: ", infos)

	if infos == nil {
		logging.Error.Println("No masters available in Zookeeper.")
		return
	}

	masters := make([]string, 0, len(infos))
	for _, info := range infos {
		if validMasterInfo(info) {
			masters = append(masters, masterHostPort(info))
		}
	}

	if len(masters) == 0 {
		logging.Error.Println("No valid masters available in Zookeeper.")
		return
	}

	ms.masters = ordered(ms.masters[0], masters)
	emit(ms.changed, ms.masters)
}

func emit(ch chan<- []string, s []string) {
	ch <- append(make([]string, 0, len(s)), s...)
}

// ordered returns a slice of masters with the given leader in the first position
func ordered(leader string, masters []string) []string {
	ms := append(make([]string, 0, len(masters)+1), leader)
	for _, m := range masters {
		if m != leader {
			ms = append(ms, m)
		}
	}
	return ms
}

func validMasterInfo(info *mesos.MasterInfo) bool {
	return info.GetHostname() != "" || info.GetIp() != 0
}

func masterHostPort(info *mesos.MasterInfo) string {
	host := info.GetHostname()

	if host == "" {
		// unpack IPv4
		octets := make([]byte, 4)
		binary.BigEndian.PutUint32(octets, info.GetIp())
		ipv4 := net.IP(octets)
		host = ipv4.String()
	}

	return net.JoinHostPort(host, masterPort(info))
}

func masterPort(info *mesos.MasterInfo) string {
	return strconv.FormatUint(uint64(info.GetPort()), 10)
}
