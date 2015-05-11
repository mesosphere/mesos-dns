package config

import (
	"sync"
	"testing"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/util"
)

func init() {
	logging.VerboseFlag = false
	logging.SetupLogs()
}

func TestLaunchZK(t *testing.T) {
	var closeOnce sync.Once
	ch := make(chan struct{})
	closer := func() { closeOnce.Do(func() { close(ch) }) }
	res := &MasterSource{
		startZKdetection: func(zkurl string, leaderChanged func(string)) error {
			go func() {
				defer closer()
				leaderChanged("")
				leaderChanged("")
				leaderChanged("a")
				leaderChanged("")
				leaderChanged("")
				leaderChanged("b")
				leaderChanged("")
				leaderChanged("")
				leaderChanged("c")
			}()
			return nil
		},
	}
	leaderSig, errCh := res.launchZK(1 * time.Second)
	util.OnError(ch, errCh, func(err error) { t.Fatalf("unexpected error: %v", err) })
	getLeader := func() string {
		res.leaderLock.Lock()
		defer res.leaderLock.Unlock()
		return res.leader
	}
	for i := 0; i < 3; i++ {
		select {
		case <-leaderSig:
			t.Logf("new leader %d: %s", i, getLeader())
		case <-time.After(1 * time.Second):
			t.Fatalf("timed out waiting for new leader")
		}
	}
	select {
	case <-ch:
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for detector death")
	}
	// there should be nothing left in the leader signal chan
	select {
	case <-leaderSig:
		t.Fatalf("unexpected new leader")
	case <-time.After(200 * time.Millisecond):
		// expected
	}
}
