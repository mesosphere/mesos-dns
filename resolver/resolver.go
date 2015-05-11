// package resolver contains functions to handle resolving .mesos
// domains
package resolver

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/util"
)

// holds configuration state and the resource records
type Resolver struct {
	version string
	config  records.Config
	rs      *records.RecordSet
	rsLock  sync.RWMutex
	done    chan struct{}
}

func New(version string, config records.Config) *Resolver {
	r := &Resolver{
		version: version,
		config:  config,
		rs:      &records.RecordSet{},
		done:    make(chan struct{}),
	}
	return r
}

func (res *Resolver) getVersion() string {
	return res.version
}

// return the current (read-only) record set. attempts to write to the returned
// object will likely result in a data race.
func (res *Resolver) records() *records.RecordSet {
	res.rsLock.RLock()
	defer res.rsLock.RUnlock()
	return res.rs
}

func (res *Resolver) Run(updates <-chan records.Update) {
	util.Until(func() { res.syncLoop(updates) }, 0, res.done)
}

// syncLoop is the main loop for processing changes. It watches for changes from
// the global update channel. For any new change seen, will run a sync against desired
// state and running state. Never returns.
func (res *Resolver) syncLoop(updates <-chan records.Update) {
	select {
	case <-res.done:
		return
	default:
		// continue
	}
	for u := range updates {
		res.updateRecords(u)
	}
	// only close the done chan if we exit normally, which means that the updates
	// channel has closed and we are to terminate gracefully.
	close(res.done)
}

func (res *Resolver) updateRecords(u records.Update) {
	res.rsLock.Lock()
	defer res.rsLock.Unlock()
	switch u.Op {
	case records.SET:
		logging.Verbose.Println("SET: records changed")
		res.rs = &u.Records
	case records.UPDATE:
		logging.Verbose.Println("UPDATE: records changed")
		res.rs = applyUpdates(&u.Records, res.rs)
	default:
		panic("updateRecords does not support incremental changes")
	}
	timestamp := uint32(time.Now().Unix())
	atomic.StoreUint32(&res.config.SOASerial, timestamp)
}

func (res *Resolver) getSOASerial() uint32 {
	return atomic.LoadUint32(&res.config.SOASerial)
}

func applyUpdates(changed, current *records.RecordSet) *records.RecordSet {
	updated := &records.RecordSet{}
	for name, currentAns := range current.As {
		if ans, found := changed.As.Get(name); found {
			updated.As = updated.As.Put(name, ans)
			logging.VeryVerbose.Printf("record named %q has a new spec %+v", name, ans)
		} else {
			updated.As = updated.As.Put(name, currentAns)
			logging.VeryVerbose.Printf("record named %q stay with the same spec %+v", name, currentAns)
		}
	}
	for name, currentAns := range current.SRVs {
		if ans, found := changed.SRVs.Get(name); found {
			updated.SRVs = updated.SRVs.Put(name, ans)
			logging.VeryVerbose.Printf("record named %q has a new spec %+v", name, ans)
		} else {
			updated.SRVs = updated.SRVs.Put(name, currentAns)
			logging.VeryVerbose.Printf("record named %q stay with the same spec %+v", name, currentAns)
		}
	}
	return updated
}
