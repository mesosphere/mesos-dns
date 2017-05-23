package state

import (
	"bytes"
	"net"
	"strconv"
	"strings"

	"github.com/mesos/mesos-go/upid"
	"github.com/mesosphere/mesos-dns/logging"
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
		lo, err := strconv.Atoi(pz[0])
		if err != nil {
			logging.Error.Println(err)
			continue
		}
		hi, err := strconv.Atoi(pz[1])
		if err != nil {
			logging.Error.Println(err)
			continue
		}

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
	ContainerStatus ContainerStatus `json:"container_status,omitempty"`
}

// ContainerStatus holds container metadata as defined in the /state.json
// Mesos HTTP endpoint.
type ContainerStatus struct {
	NetworkInfos []NetworkInfo `json:"network_infos,omitempty"`
}

// NetworkInfo holds the network configuration for a single interface
// as defined in the /state.json Mesos HTTP endpoint.
type NetworkInfo struct {
	IPAddresses []IPAddress `json:"ip_addresses,omitempty"`
	// back-compat with 0.25 IPAddress format
	IPAddress string `json:"ip_address,omitempty"`
	Name      string `json:"name,omitempty"`
}

// IPAddress holds a single IP address configured on an interface,
// as defined in the /state.json Mesos HTTP endpoint.
type IPAddress struct {
	IPAddress string `json:"ip_address,omitempty"`
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

// HasDiscoveryInfo return whether the DiscoveryInfo was provided in the state.json
func (t *Task) HasDiscoveryInfo() bool {
	return t.DiscoveryInfo.Name != ""
}

/*
// IP returns the first Task IP found in the given sources.
func (t *Task) IP(srcs ...string) *ScopedIP {
	if ips := t.IPs(srcs...); len(ips) > 0 {
		return ips[0]
	}
	return nil
}
*/

// IPs returns a slice of IPs sourced from the given sources with ascending
// priority.
func (t *Task) IPs(srcs ...string) (ips []*ScopedIP) {
	if t == nil {
		return nil
	}
	for i := range srcs {
		if src, ok := sources[srcs[i]]; ok {
			ips = append(ips, src(t)...)
		}
	}
	// flatten ips to remove nil's (probably a more efficient way to do this)
	for i := range ips {
		for ips[i] == nil {
			copy(ips[i:], ips[i+1:])
			ips[len(ips)-1] = nil
			ips = ips[:len(ips)-1]
		}
	}
	return ips
}

// sources maps the string representation of IP sources to their functions.
var sources = map[string]func(*Task) []*ScopedIP{
	"host":    hostIPs,
	"mesos":   mesosIPs,
	"docker":  dockerIPs,
	"netinfo": networkInfoIPs(""),
	"autoip":  autoIPs,
}

func sanitizedIP(srcIP string) string {
	if ip := net.ParseIP(srcIP); len(ip) > 0 {
		return ip.String()
	}
	return ""
}

// hostIPs is an IPSource which returns the IP addresses of the slave a Task
// runs on.
func hostIPs(t *Task) []*ScopedIP {
	ip := sanitizedIP(t.SlaveIP)
	if ip != "" {
		return []*ScopedIP{{scopeHost, ip}}
	}
	return nil
}

// networkInfoIPs returns IP addresses from a given Task's
// []Status.ContainerStatus.[]NetworkInfos.[]IPAddresses.IPAddress
func networkInfoIPs(networkName string) func(*Task) []*ScopedIP {
	template := ScopedIP{Scope{NetworkScopeContainer, networkName}, ""}
	return func(t *Task) []*ScopedIP {
		return statusIPs(t.Statuses, func(s *Status) []*ScopedIP {
			ips := make([]*ScopedIP, len(s.ContainerStatus.NetworkInfos))
			for i := range s.ContainerStatus.NetworkInfos {
				netinfo := &s.ContainerStatus.NetworkInfos[i]
				if networkName == "" || networkName == netinfo.Name {
					if len(netinfo.IPAddresses) > 0 {
						// In v0.26, we use the IPAddresses field.
						for j := range netinfo.IPAddresses {
							ipAddress := sanitizedIP(netinfo.IPAddresses[j].IPAddress)
							if ipAddress != "" {
								ip := template // copy
								ip.IP = ipAddress
								ips = append(ips, &ip)
							}
						}
					} else {
						// Fall back to v0.25 syntax of single IPAddress if that's being used.
						ipAddress := sanitizedIP(netinfo.IPAddress)
						if ipAddress != "" {
							ip := template // copy
							ip.IP = ipAddress
							ips = append(ips, &ip)
						}
					}
				}
			}
			return ips
		})
	}
}

const (
	// DockerIPLabel is the key of the Label which holds the Docker containerizer IP value.
	DockerIPLabel = "Docker.NetworkSettings.IPAddress"
	// MesosIPLabel is the key of the label which holds the Mesos containerizer IP value.
	MesosIPLabel = "MesosContainerizer.NetworkSettings.IPAddress"
)

// dockerIPs returns IP addresses from the values of all
// Task.[]Status.[]Labels whose keys are equal to "Docker.NetworkSettings.IPAddress".
func dockerIPs(t *Task) []*ScopedIP {
	return statusIPs(t.Statuses, labels2IPs(DockerIPLabel))
}

// mesosIPs returns IP addresses from the values of all
// Task.[]Status.[]Labels whose keys are equal to
// "MesosContainerizer.NetworkSettings.IPAddress".
func mesosIPs(t *Task) []*ScopedIP {
	return statusIPs(t.Statuses, labels2IPs(MesosIPLabel))
}

// statusIPs returns the latest running status IPs extracted with the given src
func statusIPs(st []Status, src func(*Status) []*ScopedIP) []*ScopedIP {
	// the state.json we extract from mesos makes no guarantees re: the order
	// of the task statuses so we should check the timestamps to avoid problems
	// down the line. we can't rely on seeing the same sequence. (@joris)
	// https://github.com/apache/mesos/blob/0.24.0/src/slave/slave.cpp#L5226-L5238
	ts, j := -1.0, -1
	for i := range st {
		if st[i].State == "TASK_RUNNING" && st[i].Timestamp > ts {
			ts, j = st[i].Timestamp, i
		}
	}
	if j >= 0 {
		return src(&st[j])
	}
	return nil
}

// labels returns all given Status.[]Labels' values whose keys are equal
// to the given key
func labels2IPs(key string) func(*Status) []*ScopedIP {
	return func(s *Status) []*ScopedIP {
		vs := make([]*ScopedIP, 0, len(s.Labels))
		for _, l := range s.Labels {
			if l.Key == key {
				if ip := sanitizedIP(l.Value); ip != "" {
					vs = append(vs, &ScopedIP{Scope{NetworkScopeContainer, ""}, ip})
				}
			}
		}
		return vs
	}
}

// ExtractAutoIPLabels returns the `network-scope` and `network-name` port discovery labels.
func ExtractAutoIPLabels(labels []Label) (networkScope string, networkName string) {
	for i := range labels {
		if labels[i].Key == "network-scope" {
			networkScope = labels[i].Value
		}
		if labels[i].Key == "network-name" {
			networkName = labels[i].Value
		}
	}
	return
}

var (
	scopeHost             = Scope{NetworkScopeHost, ""}
	scopeContainerGeneric = Scope{NetworkScopeContainer, ""}
)

// ScopeFrom returns a Scope derived from port discovery info labels, or else nil.
func ScopeFrom(networkScope, networkName string) *Scope {
	if networkScope == "" || networkScope == string(NetworkScopeHost) {
		return &scopeHost
	}
	if networkScope == string(NetworkScopeContainer) {
		if networkName == "" {
			return &scopeContainerGeneric
		}
		return &Scope{NetworkScopeContainer, networkName}
	}
	return nil
}

// autoIPs returns IP addresses associated with a task's port discovery info.
// this implementation makes the assumption that the caller is only going to use
// the first IP address returned, and so the first valid port discovery info
// determines the ip-addresses reported by this func.
// if no port discovery information has been provided, then return nil.
// if an unsupported value for network-scope has been provided, then return nil.
func autoIPs(t *Task) (ips []*ScopedIP) {
	for i := range t.DiscoveryInfo.Ports.DiscoveryPorts {
		port := &t.DiscoveryInfo.Ports.DiscoveryPorts[i]
		networkScope, networkName := ExtractAutoIPLabels(port.Labels.Labels)
		if networkScope == "" || networkScope == string(NetworkScopeHost) {
			return hostIPs(t)
		} else if networkScope == string(NetworkScopeContainer) {
			return networkInfoIPs(networkName)(t)
		}
		// we don't understand the value of the network-scope directive so bail
		break
	}
	return nil
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
		Labels []Label `json:"labels"`
	} `json:"labels"`
	Ports struct {
		DiscoveryPorts []DiscoveryPort `json:"ports"`
	} `json:"ports"`
}

// DiscoveryPort holds a port for a task defined in the /state.json Mesos HTTP endpoint.
type DiscoveryPort struct {
	Protocol string `json:"protocol"`
	Number   int    `json:"number"`
	Name     string `json:"name"`
	Labels   struct {
		Labels []Label `json:"labels"`
	} `json:"labels"`
}

type NetworkScope string

const (
	NetworkScopeHost      = "host"
	NetworkScopeContainer = "container"
)

type Scope struct {
	NetworkScope
	NetworkName string
}

type ScopedIP struct {
	Scope Scope
	IP    string
}
