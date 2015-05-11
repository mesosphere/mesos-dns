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

func mergeAddUpdate(sourceRecords *records.RecordSet, addUpdateEvent, adds, updates *records.Update) {
	addUpdate := func(eventRRS records.RRS, sourceRRS, addsRRS, updatesRRS *records.RRS) {
		for name, ref := range eventRRS {
			if existing, found := (*sourceRRS).Get(name); found {
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					records.Put(sourceRRS, name, ref)
					records.Put(updatesRRS, name, ref)
					continue
				}
				// this is a no-op
				continue
			}
			records.Put(sourceRRS, name, ref)
			records.Put(addsRRS, name, ref)
		}
	}
	addUpdate(addUpdateEvent.Records.As, &sourceRecords.As, &adds.Records.As, &updates.Records.As)
	addUpdate(addUpdateEvent.Records.SRVs, &sourceRecords.SRVs, &adds.Records.SRVs, &updates.Records.SRVs)
}

func mergeRemove(sourceRecords *records.RecordSet, removeEvent, deletes *records.Update) {
	deleteFn := func(eventRRS, sourceRRS records.RRS, deletesRRS *records.RRS) {
		for name, _ := range eventRRS {
			if existing, found := sourceRRS.Get(name); found {
				// this is a delete
				sourceRRS.Delete(name)
				records.Put(deletesRRS, name, existing)
				continue
			}
			// this is a no-op
		}
	}
	deleteFn(removeEvent.Records.As, sourceRecords.As, &deletes.Records.As)
	deleteFn(removeEvent.Records.SRVs, sourceRecords.SRVs, &deletes.Records.SRVs)
}

func mergeSet(sourceRecords *records.RecordSet, setEvent, adds, updates, deletes *records.Update) {
	addUpdateDelete := func(eventRRS records.RRS, sourceRRS, addsRRS, updatesRRS, deletesRRS *records.RRS) {
		// Clear the old entries
		old := *sourceRRS
		*sourceRRS = nil

		for name, ref := range eventRRS {
			if existing, found := old.Get(name); found {
				records.Put(sourceRRS, name, existing)
				if !reflect.DeepEqual(existing, ref) {
					// this is an update
					records.Put(sourceRRS, name, ref)
					records.Put(updatesRRS, name, ref)
					continue
				}
				// this is a no-op
				continue
			}
			records.Put(sourceRRS, name, ref)
			records.Put(addsRRS, name, ref)
		}
		for name, existing := range old {
			if _, found := (*sourceRRS).Get(name); !found {
				// this is a delete
				records.Put(deletesRRS, name, existing)
			}
		}
	}
	addUpdateDelete(setEvent.Records.As, &sourceRecords.As, &adds.Records.As, &updates.Records.As, &deletes.Records.As)
	addUpdateDelete(setEvent.Records.SRVs, &sourceRecords.SRVs, &adds.Records.SRVs, &updates.Records.SRVs, &deletes.Records.SRVs)
}

func (s *recordStorage) merge(source string, change interface{}) (adds, updates, deletes *records.Update) {
	s.recordLock.Lock()
	defer s.recordLock.Unlock()

	adds = &records.Update{Op: records.ADD}
	updates = &records.Update{Op: records.UPDATE}
	deletes = &records.Update{Op: records.REMOVE}

	sourceRecords := s.records[source]
	if sourceRecords == nil {
		sourceRecords = &records.RecordSet{}
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

		mergeAddUpdate(sourceRecords, &update, adds, updates)

	case records.REMOVE:
		logging.VeryVerbose.Printf("Removing records %v", update)
		mergeRemove(sourceRecords, &update, deletes)

	case records.SET:
		logging.VeryVerbose.Printf("Setting records for source %s : %v", source, update)
		s.markSourceSet(source)

		//TODO(jdef) reinstitute this at some point
		//filtered := filterInvalidRecords(update.RecordSets, source)

		s.markSourceSet(source)
		mergeSet(sourceRecords, &update, adds, updates, deletes)

	default:
		logging.Error.Printf("Received invalid update type: %v", update)
	}

	s.records[source] = sourceRecords
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
			recordSet.As = recordSet.As.Put(name, recordRef)
		}
		for name, recordRef := range sourceRecords.SRVs {
			recordSet.SRVs = recordSet.SRVs.Put(name, recordRef)
		}
	}
	return recordSet
}
