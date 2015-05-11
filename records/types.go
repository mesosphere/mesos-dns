package records

type Answer []string

// REFACTOR - when discoveryinfo is integrated
func (a Answer) Clone() Answer {
	if a == nil {
		return nil
	}
	b := make([]string, len(a))
	copy(b, a)
	return Answer(b)
}

// Map host/service name to DNS answer
type RRS map[string]Answer

// Mesos-DNS state
// Refactor when discovery id is available
type RecordSet struct {
	As   RRS
	SRVs RRS
}

func (rrs RRS) Put(name string, ans Answer) RRS {
	if rrs == nil {
		rrs = make(RRS)
	}
	rrs[name] = ans
	return rrs
}

func (rrs RRS) Get(name string) (v Answer, found bool) {
	if rrs != nil {
		v, found = rrs[name]
	}
	return
}

func (rrs RRS) Delete(name string) {
	if rrs == nil {
		return
	}
	delete(rrs, name)
}

func (rs *RecordSet) Size() int {
	if rs == nil {
		return 0
	}
	return len(rs.As) + len(rs.SRVs)
}

// Operation defines what changes will be made on a pod configuration.
type Operation int

const (
	// This is the current pod configuration
	SET Operation = iota
	// RecordSets with the given ids are new to this source
	ADD
	// RecordSets with the given ids have been removed from this source
	REMOVE
	// RecordSets with the given ids have been updated in this source
	UPDATE

	// These constants identify the sources of records

	// Standard record updates generated from Mesos Master tasks
	MasterSource = "master"
	// Updates from all sources
	AllSource = "*"
)

// Update defines an operation sent on the channel. You can add or remove single services by
// sending an array of size one and Op == ADD|REMOVE (with REMOVE, only the ID is required).
// For setting the state of the system to a given state for this source configuration, set
// Records as desired and Op to SET, which will reset the system state to that specified in this
// operation for this source channel. To remove all pods, set Records to empty object and Op to SET.
type Update struct {
	Records RecordSet
	Op      Operation
	Source  string
}
