package config

// largely inspired by the Kubernetes pkg/kubelet/config package

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/mesosphere/mesos-dns/util"
	"github.com/mesosphere/mesos-dns/util/config"
)

// RecordConfigNotificationMode describes how changes are sent to the update channel.
type RecordConfigNotificationMode int

const (
	// RecordConfigNotificationSnapshot delivers the full configuration as a SET whenever
	// any change occurs.
	RecordConfigNotificationSnapshot = iota
	// RecordConfigNotificationSnapshotAndUpdates delivers an UPDATE message whenever records are
	// changed, and a SET message if there are any additions or removals.
	RecordConfigNotificationSnapshotAndUpdates
	// RecordConfigNotificationIncremental delivers ADD, UPDATE, and REMOVE to the update channel.
	RecordConfigNotificationIncremental

	UpdatesBacklog = 50
)

// RecordConfig is a configuration mux that merges many sources of record configuration into a single
// consistent structure, and then delivers incremental change notifications to listeners
// in order.
type RecordConfig struct {
	records *recordStorage
	mux     *config.Mux

	// the channel of denormalized changes passed to listeners
	updates chan records.Update

	// contains the list of all configured sources
	sourcesLock sync.Mutex
	sources     util.StringSet
}

// New creates an object that can merge many configuration sources into a stream
// of normalized updates to a record configuration.
func New(mode RecordConfigNotificationMode) *RecordConfig {
	updates := make(chan records.Update, UpdatesBacklog)
	storage := newRecordStorage(updates, mode)
	recordConfig := &RecordConfig{
		records: storage,
		mux:     config.NewMux(storage),
		updates: updates,
		sources: util.StringSet{},
	}
	return recordConfig
}

// Channel creates or returns a config source channel.  The channel
// only accepts records Updates
func (c *RecordConfig) Channel(source string) chan<- interface{} {
	c.sourcesLock.Lock()
	defer c.sourcesLock.Unlock()
	c.sources.Insert(source)
	return c.mux.Channel(source)
}

// SeenAllSources returns true if this config has received a SET
// message from all configured sources, false otherwise.
func (c *RecordConfig) SeenAllSources() bool {
	if c.records == nil {
		return false
	}
	logging.VeryVerbose.Printf("Looking for %v, have seen %v", c.sources.List(), c.records.sourcesSeen)
	return c.records.seenSources(c.sources.List()...)
}

// Updates returns a channel of updates to the configuration, properly denormalized.
func (c *RecordConfig) Updates() <-chan records.Update {
	return c.updates
}

// Sync requests the full configuration be delivered to the update channel.
func (c *RecordConfig) Sync() {
	c.records.Sync()
}

// recordStorage manages the current record state at any point in time and ensures updates
// to the channel are delivered in order.  Note that this object is an in-memory source of
// "truth" and on creation contains zero entries.  Once all previously read sources are
// available, then this object should be considered authoritative.
type recordStorage struct {
	recordLock sync.RWMutex
	// map of source name to record set
	records map[string]*records.RecordSet
	mode    RecordConfigNotificationMode

	// ensures that updates are delivered in strict order
	// on the updates channel
	updateLock sync.Mutex
	updates    chan<- records.Update

	// contains the set of all sources that have sent at least one SET
	sourcesSeenLock sync.Mutex
	sourcesSeen     util.StringSet
}

// TODO: RecordConfigNotificationMode could be handled by a listener to the updates channel
// in the future, especially with multiple listeners.
// TODO: allow initialization of the current state of the store with snapshotted version.
func newRecordStorage(updates chan<- records.Update, mode RecordConfigNotificationMode) *recordStorage {
	return &recordStorage{
		records:     make(map[string]*records.RecordSet),
		mode:        mode,
		updates:     updates,
		sourcesSeen: util.StringSet{},
	}
}

// Merge normalizes a set of incoming changes from different sources into a map of all RecordSets
// and ensures that redundant changes are filtered out, and then pushes zero or more minimal
// updates onto the update channel.  Ensures that updates are delivered in order.
func (s *recordStorage) Merge(source string, change interface{}) error {
	s.updateLock.Lock()
	defer s.updateLock.Unlock()

	adds, updates, deletes := s.merge(source, change)

	// deliver update notifications
	switch s.mode {
	case RecordConfigNotificationIncremental:
		if deletes.Records.Size() > 0 {
			s.updates <- *deletes
		}
		if adds.Records.Size() > 0 {
			s.updates <- *adds
		}
		if updates.Records.Size() > 0 {
			s.updates <- *updates
		}

	case RecordConfigNotificationSnapshotAndUpdates:
		if updates.Records.Size() > 0 {
			s.updates <- *updates
		}
		if deletes.Records.Size() > 0 || adds.Records.Size() > 0 {
			upd := s.MergedState().(*records.RecordSet)
			s.updates <- records.Update{Records: *upd, Op: records.SET, Source: source}
		}

	case RecordConfigNotificationSnapshot:
		if updates.Records.Size() > 0 || deletes.Records.Size() > 0 || adds.Records.Size() > 0 {
			upd := s.MergedState().(*records.RecordSet)
			s.updates <- records.Update{Records: *upd, Op: records.SET, Source: source}
		}

	default:
		panic(fmt.Sprintf("unsupported RecordConfigNotificationMode: %#v", s.mode))
	}

	return nil
}

func (s *recordStorage) merge(source string, change interface{}) (adds, updates, deletes *records.Update) {
	s.recordLock.Lock()
	defer s.recordLock.Unlock()

	adds = &records.Update{Op: records.ADD}
	updates = &records.Update{Op: records.UPDATE}
	deletes = &records.Update{Op: records.REMOVE}

	recordSets := s.records[source]
	if recordSets == nil {
		recordSets = &records.RecordSet{}
	}

	update := change.(records.Update)
	switch update.Op {
	case records.ADD, records.UPDATE:
		if update.Op == records.ADD {
			logging.VeryVerbose.Printf("Adding new records from source %s : %v", source, update.Records)
		} else {
			logging.VeryVerbose.Printf("Updating records from source %s : %v", source, update.Records)
		}

		//TODO(jdef) reinstitute this at some point
		//filtered := filterInvalidRecords(update.Records, source)

		for name, ref := range update.Records.As {
			if existing, found := recordSets.As.Get(name); found {
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					existing = ref
					update.Records.As = update.Records.As.Put(name, existing)
					continue
				}
				// this is a no-op
				continue
			}
			recordSets.As = recordSets.As.Put(name, ref)
			adds.Records.As = adds.Records.As.Put(name, ref)
		}
		for name, ref := range update.Records.SRVs {
			if existing, found := recordSets.SRVs.Get(name); found {
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					existing = ref
					update.Records.SRVs = update.Records.SRVs.Put(name, existing)
					continue
				}
				// this is a no-op
				continue
			}
			recordSets.SRVs = recordSets.SRVs.Put(name, ref)
			adds.Records.SRVs = adds.Records.SRVs.Put(name, ref)
		}

	case records.REMOVE:
		logging.VeryVerbose.Printf("Removing records %v", update)
		for name, _ := range update.Records.As {
			if existing, found := recordSets.As.Get(name); found {
				// this is a delete
				recordSets.As.Delete(name)
				deletes.Records.As = deletes.Records.As.Put(name, existing)
				continue
			}
			// this is a no-op
		}
		for name, _ := range update.Records.SRVs {
			if existing, found := recordSets.SRVs.Get(name); found {
				// this is a delete
				recordSets.SRVs.Delete(name)
				deletes.Records.SRVs = deletes.Records.SRVs.Put(name, existing)
				continue
			}
			// this is a no-op
		}

	case records.SET:
		logging.VeryVerbose.Printf("Setting records for source %s : %v", source, update)
		s.markSourceSet(source)

		// Clear the old entries
		oldAs := recordSets.As
		recordSets.As = nil

		//TODO(jdef) reinstitute this at some point
		//filtered := filterInvalidRecords(update.RecordSets, source)

		for name, ref := range update.Records.As {
			if existing, found := oldAs.Get(name); found {
				recordSets.As.Put(name, existing)
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					existing = ref
					updates.Records.As = updates.Records.As.Put(name, existing)
					continue
				}
				// this is a no-op
				continue
			}
			recordSets.As = recordSets.As.Put(name, ref)
			adds.Records.As = adds.Records.As.Put(name, ref)
		}
		for name, existing := range oldAs {
			if _, found := recordSets.As.Get(name); !found {
				// this is a delete
				deletes.Records.As = deletes.Records.As.Put(name, existing)
			}
		}

		// Clear the old entries
		oldSRVs := recordSets.SRVs
		recordSets.SRVs = nil

		for name, ref := range update.Records.SRVs {
			if existing, found := oldSRVs.Get(name); found {
				recordSets.SRVs.Put(name, existing)
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					existing = ref
					updates.Records.SRVs = updates.Records.SRVs.Put(name, existing)
					continue
				}
				// this is a no-op
				continue
			}
			recordSets.SRVs = recordSets.SRVs.Put(name, ref)
			adds.Records.SRVs = adds.Records.SRVs.Put(name, ref)
		}
		for name, existing := range oldSRVs {
			if _, found := recordSets.SRVs.Get(name); !found {
				// this is a delete
				deletes.Records.SRVs = deletes.Records.SRVs.Put(name, existing)
			}
		}

	default:
		logging.Error.Printf("Received invalid update type: %v", update)

	}

	s.records[source] = recordSets
	return adds, updates, deletes
}

func (s *recordStorage) markSourceSet(source string) {
	s.sourcesSeenLock.Lock()
	defer s.sourcesSeenLock.Unlock()
	s.sourcesSeen.Insert(source)
}

func (s *recordStorage) seenSources(sources ...string) bool {
	s.sourcesSeenLock.Lock()
	defer s.sourcesSeenLock.Unlock()
	return s.sourcesSeen.HasAll(sources...)
}

// Sync sends a copy of the current state through the update channel.
func (s *recordStorage) Sync() {
	s.updateLock.Lock()
	defer s.updateLock.Unlock()
	upd := s.MergedState().(*records.RecordSet)
	s.updates <- records.Update{Records: *upd, Op: records.SET, Source: records.AllSource}
}

// Object implements config.Accessor
func (s *recordStorage) MergedState() interface{} {
	s.recordLock.RLock()
	defer s.recordLock.RUnlock()
	recordSet := &records.RecordSet{}
	for _, sourceRecords := range s.records {
		for name, recordRef := range sourceRecords.As {
			ans := recordRef.Clone()
			recordSet.As = recordSet.As.Put(name, ans)
		}
		for name, recordRef := range sourceRecords.SRVs {
			ans := recordRef.Clone()
			recordSet.SRVs = recordSet.SRVs.Put(name, ans)
		}
	}
	return recordSet
}
