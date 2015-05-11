package config

import (
	"reflect"
	"testing"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records"
	"github.com/stretchr/testify/assert"
)

func init() {
	logging.VerboseFlag = false
	logging.SetupLogs()
}

func TestMergeAddUpdate(t *testing.T) {
	assert := assert.New(t)
	sourceRecords := records.RecordSet{}
	adds := &records.Update{}
	updates := &records.Update{}

	event := &records.Update{
		Records: records.RecordSet{
			As:   records.RRS{"james": records.Answer{"foo", "bar"}},
			SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	mergeAddUpdate(&sourceRecords, event, adds, updates)
	assert.True(reflect.DeepEqual(sourceRecords, event.Records), "expected %v instead of %v", event.Records, sourceRecords)
	assert.True(reflect.DeepEqual(adds.Records, event.Records))
	assert.True(reflect.DeepEqual(updates.Records, records.RecordSet{}))

	// add a new record to source record set that's non-empty
	event2 := &records.Update{
		Records: records.RecordSet{
			As: records.RRS{"bill": records.Answer{"june", "july"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	expected := &records.Update{
		Records: records.RecordSet{
			As: records.RRS{
				"james": records.Answer{"foo", "bar"},
				"bill":  records.Answer{"june", "july"},
			},
			SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	adds = &records.Update{}
	updates = &records.Update{}
	mergeAddUpdate(&sourceRecords, event2, adds, updates)
	assert.True(reflect.DeepEqual(sourceRecords, expected.Records), "expected %v instead of %v", expected.Records, sourceRecords)
	assert.True(reflect.DeepEqual(adds.Records, event2.Records))
	assert.True(reflect.DeepEqual(updates.Records, records.RecordSet{}))

	// update an existing record
	event3 := &records.Update{
		Records: records.RecordSet{
			As: records.RRS{"bill": records.Answer{"june", "october"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	expected = &records.Update{
		Records: records.RecordSet{
			As: records.RRS{
				"james": records.Answer{"foo", "bar"},
				"bill":  records.Answer{"june", "october"},
			},
			SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	adds = &records.Update{}
	updates = &records.Update{}
	mergeAddUpdate(&sourceRecords, event3, adds, updates)
	assert.True(reflect.DeepEqual(sourceRecords, expected.Records), "expected %v instead of %v", expected.Records, sourceRecords)
	assert.True(reflect.DeepEqual(adds.Records, records.RecordSet{}))
	assert.True(reflect.DeepEqual(updates.Records, event3.Records))
}

func TestMergeRemove(t *testing.T) {
	assert := assert.New(t)
	sourceRecords := records.RecordSet{
		As:   records.RRS{"james": records.Answer{"foo", "bar"}},
		SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
	}
	deletes := &records.Update{}

	event := &records.Update{
		Records: records.RecordSet{
			As: records.RRS{"james": records.Answer{"foo", "bar"}},
		},
		Op:     records.REMOVE,
		Source: records.MasterSource,
	}
	mergeRemove(&sourceRecords, event, deletes)

	expected := &records.Update{
		Records: records.RecordSet{
			As:   records.RRS{},
			SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
		},
		Op:     records.REMOVE,
		Source: records.MasterSource,
	}
	assert.True(reflect.DeepEqual(sourceRecords, expected.Records), "expected %v instead of %v", expected.Records, sourceRecords)
	assert.True(reflect.DeepEqual(deletes.Records, event.Records), "expected %v instead of %v", event.Records, deletes.Records)
}

func TestMergeSet(t *testing.T) {
	assert := assert.New(t)
	adds := &records.Update{}
	updates := &records.Update{}
	deletes := &records.Update{}

	sourceRecords := records.RecordSet{
		As: records.RRS{
			"james": records.Answer{"zebra", "bar"},
			"kat":   records.Answer{"jen", "carrie"},
		},
		SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
	}
	event := &records.Update{
		Records: records.RecordSet{
			As: records.RRS{
				"james": records.Answer{"foo", "bar"},   // updated
				"joe":   records.Answer{"nick", "chad"}, // added
				// kat was deleted
			},
			SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
		},
		Op:     records.ADD,
		Source: records.MasterSource,
	}
	mergeSet(&sourceRecords, event, adds, updates, deletes)

	expected := records.RecordSet{
		As: records.RRS{
			"james": records.Answer{"foo", "bar"},
			"joe":   records.Answer{"nick", "chad"},
		},
		SRVs: records.RRS{"anna": records.Answer{"cat", "dog"}},
	}
	assert.True(reflect.DeepEqual(sourceRecords, expected), "expected %v instead of %v", expected, sourceRecords)
	assert.True(reflect.DeepEqual(adds.Records, records.RecordSet{
		As: records.RRS{"joe": records.Answer{"nick", "chad"}},
	}))
	assert.True(reflect.DeepEqual(updates.Records, records.RecordSet{
		As: records.RRS{"james": records.Answer{"foo", "bar"}},
	}))
	assert.True(reflect.DeepEqual(deletes.Records, records.RecordSet{
		As: records.RRS{"kat": records.Answer{"jen", "carrie"}},
	}))
}
