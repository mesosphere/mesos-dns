package state

import (
	"bytes"
	"strconv"
	"strings"

	"github.com/mesos/mesos-go/upid"
)

// Resources holds resources as defined in the /state.json Mesos HTTP endpoint.
type Resources struct {
	PortRanges string `json:"ports"`
}

// Ports returns a slice of individual ports expanded from PortRanges.
func (r Resources) Ports() []string {
	if r.PortRanges == "" || r.PortRanges == "[]" {
		return []string{}
	}

	rhs := strings.Split(r.PortRanges, "[")[1]
	lhs := strings.Split(rhs, "]")[0]

	yports := []string{}

	mports := strings.Split(lhs, ",")
	for _, port := range mports {
		tmp := strings.TrimSpace(port)
		pz := strings.Split(tmp, "-")
		lo, _ := strconv.Atoi(pz[0])
		hi, _ := strconv.Atoi(pz[1])

		for t := lo; t <= hi; t++ {
			yports = append(yports, strconv.Itoa(t))
		}
	}
	return yports
}

// Label holds a label as defined in the /state.json Mesos HTTP endpoint.
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Status holds a task status as defined in the /state.json Mesos HTTP endpoint.
type Status struct {
	Timestamp       float64         `json:"timestamp"`
	State           string          `json:"state"`
	Labels          []Label         `json:"labels,omitempty"`
	ContainerStatus ContainerStatus `json:"container_status"`
}

// ContainerStatus holds container metadata as defined in the /state.json
// Mesos HTTP endpoint.
type ContainerStatus struct {
	NetworkInfos []NetworkInfo `json:"network_infos"`
}

// NetworkInfo holds a network address resolution as defined in the
// /state.json Mesos HTTP endpoint.
type NetworkInfo struct {
	IPAddress string `json:"ip_address"`
}

// Task holds a task as defined in the /state.json Mesos HTTP endpoint.
type Task struct {
	FrameworkID   string   `json:"framework_id"`
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SlaveID       string   `json:"slave_id"`
	State         string   `json:"state"`
	Statuses      []Status `json:"statuses"`
	Resources     `json:"resources"`
	DiscoveryInfo DiscoveryInfo `json:"discovery"`

	SlaveIP string `json:"-"`
}

var ipLabels = map[string]string{
	"docker": "Docker.NetworkSettings.IPAddress",
	"mesos":  "MesosContainerizer.NetworkSettings.IPAddress",
}

// IP extracts the IP from a task given the prioritized list of IP sources
func (t *Task) IP(srcs ...string) string {
	for _, src := range srcs {
		switch src {
		case "host":
			return t.SlaveIP
		case "docker", "mesos":
			if ip := t.containerIP(src); ip != "" {
				return ip
			}
		}
	}
	return ""
}

// HasDiscoveryInfo return whether the DiscoveryInfo was provided in the state.json
func (t *Task) HasDiscoveryInfo() bool {
	return t.DiscoveryInfo.Name != ""
}

// containerIP extracts a container ip from a Mesos state.json task. If no
// container ip is provided, an empty string is returned.
func (t *Task) containerIP(src string) string {
	ipLabel := ipLabels[src]

	// find TASK_RUNNING statuses
	var latestContainerIP string
	var latestTimestamp float64
	for _, status := range t.Statuses {
		if status.State != "TASK_RUNNING" || status.Timestamp <= latestTimestamp {
			continue
		}

		// first try to extract the address from the NetworkInfo structure
		if len(status.ContainerStatus.NetworkInfos) > 0 {
			// TODO(CD): handle multiple NetworkInfo objects
			// TODO(CD): only create A records if the address is IPv4
			// TODO(CD): create AAAA records if the address is IPv6
			latestContainerIP = status.ContainerStatus.NetworkInfos[0].IPAddress
			latestTimestamp = status.Timestamp
			break
		}

		// next, fall back to the docker-inspect label
		// TODO(CD): deprecate and then remove this address discovery method
		for _, label := range status.Labels {
			if label.Key == ipLabel {
				latestContainerIP = label.Value
				latestTimestamp = status.Timestamp
				break
			}
		}
	}

	return latestContainerIP
}

// Framework holds a framework as defined in the /state.json Mesos HTTP endpoint.
type Framework struct {
	Tasks    []Task `json:"tasks"`
	PID      PID    `json:"pid"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
}

// HostPort returns the hostname and port where a framework's scheduler is
// listening on.
func (f Framework) HostPort() (string, string) {
	if f.PID.UPID != nil {
		return f.PID.Host, f.PID.Port
	}
	return f.Hostname, ""
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
	Visibilty   string `json:"visibility"`
	Version     string `json:"version,omitempty"`
	Name        string `json:"name,omitempty"`
	Location    string `json:"location,omitempty"`
	Environment string `json:"environment,omitempty"`
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
