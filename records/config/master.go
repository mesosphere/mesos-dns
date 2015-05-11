package config

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/mesos/mesos-go/detector"
	_ "github.com/mesos/mesos-go/detector/zoo"
	mesos "github.com/mesos/mesos-go/mesosproto"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/util"
)

const (
	zkInitialDetectionTimeout = 30 * time.Second
)

type RecordLoader func(*records.RecordGenerator) *records.RecordGenerator

type MasterSource struct {
	config     records.Config
	errHandler func(string, error)

	// always points to the leading master
	leader     string
	leaderLock sync.Mutex

	// preLoaders are invoked at the beginning of the Reload cycle, just prior to state generation
	preLoaders []RecordLoader

	// postLoaders are invoked at the end of the Reload cycle, after state generation
	postLoaders []RecordLoader

	startZKdetection func(zkurl string, leaderChanged func(string)) error
}

// mesos master config source
func NewSourceMaster(config records.Config, eh func(string, error), updates chan<- interface{}) *MasterSource {
	m := &MasterSource{
		config:           config,
		errHandler:       eh,
		startZKdetection: startDefaultZKdetector,
	}
	go m.run(updates)
	return m
}

// execute a RecordLoader func at Reload time. this func should only be invoked during
// bootstrapping (before processing begins) since this is not "thread-safe".
func (c *MasterSource) OnPreload(r RecordLoader) {
	if r != nil {
		c.preLoaders = append(c.preLoaders, r)
	}
}

// execute a RecordLoader func at Reload time. this func should only be invoked during
// bootstrapping (before processing begins) since this is not "thread-safe".
func (c *MasterSource) OnPostload(r RecordLoader) {
	if r != nil {
		c.postLoaders = append(c.postLoaders, r)
	}
}

func (c *MasterSource) getLeader() string {
	c.leaderLock.Lock()
	defer c.leaderLock.Unlock()
	return c.leader
}

// launch Zookeeper listener
func (c *MasterSource) beginLeaderWatch() (newLeader <-chan struct{}, zkErr <-chan error) {
	if c.config.Zk != "" {
		newLeader, zkErr = c.launchZK(zkInitialDetectionTimeout)
	} else {
		// uniform behavior when new leader from masters field
		leader := make(chan struct{}, 1)
		leader <- struct{}{}
		newLeader = leader
	}
	return
}

// periodically reload DNS records, either because the reload timer expired or else
// because a caller invoked the tryReload func returned by this func.
func (c *MasterSource) launchReloader(updates chan<- interface{}) (tryReload func()) {
	// generate reload signal; up to 1 reload pending at any time
	reloadSignal := make(chan struct{}, 1)
	tryReload = func() {
		// non-blocking, attempt to queue a reload
		select {
		case reloadSignal <- struct{}{}:
		default:
		}
	}

	// periodic loading of DNS state (pull from Master)
	go func() {
		defer util.HandleCrash()
		reloadTimeout := time.Second * time.Duration(c.config.RefreshSeconds)
		reloadTimer := time.AfterFunc(reloadTimeout, tryReload)
		for _ = range reloadSignal {
			c.reload(updates)
			logging.PrintCurLog()
			reloadTimer.Reset(reloadTimeout)
		}
	}()
	return
}

func (c *MasterSource) run(updates chan<- interface{}) {
	newLeader, zkErr := c.beginLeaderWatch()
	tryReload := c.launchReloader(updates)

	// infinite loop until there is fatal error
	for {
		select {
		case <-newLeader:
			tryReload()
		case err := <-zkErr:
			c.errHandler("ZK watcher", err)
		}
	}
}

// launches Zookeeper detector, returns immediately two chans: the first fires an empty
// struct whenever there's a new (non-nil) mesos leader, the second if there's an unrecoverable
// error in the master detector.
func (c *MasterSource) launchZK(initialDetectionTimeout time.Duration) (<-chan struct{}, <-chan error) {
	var startedOnce sync.Once
	startedCh := make(chan struct{})
	errCh := make(chan error, 1)
	leaderCh := make(chan struct{}, 1) // the first write never blocks

	listenerFunc := func(newLeader string) {
		defer func() {
			if newLeader != "" {
				leaderCh <- struct{}{}
				startedOnce.Do(func() { close(startedCh) })
			}
		}()
		c.leaderLock.Lock()
		defer c.leaderLock.Unlock()
		c.leader = newLeader
	}
	go func() {
		defer util.HandleCrash()

		err := c.startZKdetection(c.config.Zk, listenerFunc)
		if err != nil {
			errCh <- err
			return
		}

		logging.VeryVerbose.Println("Warning: waiting for initial information from Zookeper.")
		select {
		case <-startedCh:
			logging.VeryVerbose.Println("Info: got initial information from Zookeper.")
		case <-time.After(initialDetectionTimeout):
			errCh <- fmt.Errorf("timed out waiting for initial ZK detection, exiting")
		}
	}()
	return leaderCh, errCh
}

// Start a Zookeeper listener to track leading master, invokes callback function when
// master changes are reported.
func startDefaultZKdetector(zkurl string, leaderChanged func(string)) error {

	// start listener
	logging.Verbose.Println("Starting master detector for ZK ", zkurl)
	md, err := detector.New(zkurl)
	if err != nil {
		return fmt.Errorf("failed to create master detector: %v", err)
	}

	// and listen for master changes
	if err := md.Detect(detector.OnMasterChanged(func(info *mesos.MasterInfo) {
		leader := ""
		if leaderChanged != nil {
			defer func() {
				leaderChanged(leader)
			}()
		}
		logging.VeryVerbose.Println("Updated Zookeeper info: ", info)
		if info == nil {
			logging.Error.Println("No leader available in Zookeeper.")
		} else {
			if host := info.GetHostname(); host != "" {
				leader = host
			} else {
				// unpack IPv4
				octets := make([]byte, 4, 4)
				binary.BigEndian.PutUint32(octets, info.GetIp())
				ipv4 := net.IP(octets)
				leader = ipv4.String()
			}
			leader = fmt.Sprintf("%s:%d", leader, info.GetPort())
			logging.Verbose.Println("new master in Zookeeper ", leader)
		}
	})); err != nil {
		return fmt.Errorf("failed to initialize master detector: %v", err)
	}
	return nil
}

// triggers a new refresh from mesos master
func (c *MasterSource) reload(updates chan<- interface{}) {
	t := records.RecordGenerator{}
	t.TaskRecordGeneratorFn = t.BuildTaskRecords

	// pre-ParseState phase, preloader plugins can wrap around, or otherwise customize,
	// a RecordGenerator.TaskRecordGeneratorFn. useful if plugins want to create additional
	// records based on, for example, task labels.
	state := &t
	for _, g := range c.preLoaders {
		state = g(state)
	}

	if err := state.ParseState(c.getLeader(), c.config); err != nil {
		logging.VeryVerbose.Printf("Warning: master not found; keeping old DNS state: %v", err)
		return
	}

	// post-ParseState phase, postloader plugins can modify generated records.
	//TODO(jdef) is this really useful?!
	for _, r := range c.postLoaders {
		state = r(state)
	}

	updates <- records.Update{
		Records: state.RecordSet,
		Op:      records.SET,
		Source:  records.MasterSource,
	}
}
