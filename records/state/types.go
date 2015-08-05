package state

import (
	"bytes"

	"github.com/mesos/mesos-go/upid"
)

// Resources holds resources as defined in the /state.json Mesos HTTP endpoint.
type Resources struct {
	Ports string `json:"ports"`
}

// Label holds a label as defined in the /state.json Mesos HTTP endpoint.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Status holds a task status as defined in the /state.json Mesos HTTP endpoint.
type Status struct {
	Timestamp float64 `json:"timestamp"`
	State     string  `json:"state"`
	Labels    []Label `json:"labels,omitempty"`
}

// Task holds a task as defined in the /state.json Mesos HTTP endpoint.
type Task struct {
	FrameworkID string   `json:"framework_id"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	SlaveID     string   `json:"slave_id"`
	State       string   `json:"state"`
	Statuses    []Status `json:"statuses"`
	Resources   `json:"resources"`
	Discovery   *DiscoveryInfo `json:"discovery"`
}

// Framework holds a framework as defined in the /state.json Mesos HTTP endpoint.
type Framework struct {
	Tasks []Task `json:"tasks"`
	PID   PID    `json:"pid"`
	Name  string `json:"name"`
}

// Slave holds a slave as defined in the /state.json Mesos HTTP endpoint.
type Slave struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	PID      PID    `json:"pid"`
}

// PID holds a Mesos PID and implements the json.Unmarshaler interface.
type PID struct{ *upid.UPID }

// UnmarshalJSON implements the json.Unmarshaler interface for PIDs.
func (p *PID) UnmarshalJSON(data []byte) (err error) {
	p.UPID, err = upid.Parse(string(bytes.Trim(data, `" `)))
	return err
}

// State holds the state defined in the /state.json Mesos HTTP endpoint.
type State struct {
	Frameworks []Framework `json:"frameworks"`
	Slaves     []Slave     `json:"slaves"`
	Leader     string      `json:"leader"`
}

// DiscoveryInfo holds the discovery meta data for a task defined in the /state.json Mesos HTTP endpoint.
type DiscoveryInfo struct {
	Visibilty   string  `json:"visibility"`
	Version     *string `json:"version,omitempty"`
	Name        *string `json:"name,omitempty"`
	Location    *string `json:"location,omitempty"`
	Environment *string `json:"environment,omitempty"`
	Labels      struct {
		Labels `json:"labels"`
	} `json:"labels"`
	Ports struct {
		DiscoveryPorts `json:"ports"`
	} `json:"ports"`
}

// Labels holds the key/value labels of a task defined in the /state.json Mesos HTTP endpoint.
type Labels []struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// DiscoveryPorts holds the ports for a task defined in the /state.json Mesos HTTP endpoint.
type DiscoveryPorts []struct {
	Protocol string `json:"protocol"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
}
