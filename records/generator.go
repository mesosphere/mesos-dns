// package records contains functions to generate resource records from
// mesos master states to serve through a dns server
package records

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records/labels"
)

// Map host/service name to DNS answer
// REFACTOR - when discoveryinfo is integrated
// Will likely become map[string][]discoveryinfo
type rrs map[string][]string

// Mesos-DNS state
// Refactor when discovery id is available
type RecordGenerator struct {
	As       rrs
	SRVs     rrs
	SlaveIPs map[string]string
}

// The following types help parse state.json
// Resources holds our SRV ports
type Resources struct {
	Ports string `json:"ports"`
}

// Label holds a key/value pair
type Label struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// Status holds a task status
type Status struct {
	Timestamp float64 `json:"timestamp"`
	State     string  `json:"state"`
	Labels    []Label `json:"labels,omitempty"`
}

// Tasks holds mesos task information read in from state.json
type Task struct {
	FrameworkID string   `json:"framework_id"`
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	SlaveID     string   `json:"slave_id"`
	State       string   `json:"state"`
	Statuses    []Status `json:"statuses"`
	Resources   `json:"resources"`
}

// Frameworks holds mesos frameworks information read in from state.json
type Frameworks []struct {
	Tasks []Task `json:"tasks"`
	Name  string `json:"name"`
}

// Slaves is a mapping of id to hostname read in from state.json
type slave struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
}
type Slaves []slave

// StateJSON is a representation of mesos master state.json
type StateJSON struct {
	Frameworks `json:"frameworks"`
	Slaves     `json:"slaves"`
	Leader     string `json:"leader"`
}

// Finds the master and inserts DNS state
func (rg *RecordGenerator) ParseState(leader string, c Config) error {

	// find master -- return if error
	sj, err := rg.findMaster(leader, c.Masters)
	if err != nil {
		logging.Error.Println("no master")
		return err
	}
	if sj.Leader == "" {
		logging.Error.Println("Unexpected error")
		err = errors.New("empty master")
		return err
	}

	hostSpec := labels.ForRFC1123()
	if c.EnforceRFC952 {
		hostSpec = labels.ForRFC952()
	}

	return rg.InsertState(sj, c.Domain, c.SOARname, c.Listener, c.Masters, hostSpec)
}

// Tries each master and looks for the leader
// if no leader responds it errors
func (rg *RecordGenerator) findMaster(leader string, masters []string) (StateJSON, error) {
	var sj StateJSON

	// Check if ZK master is correct
	if leader != "" {
		logging.VeryVerbose.Println("Zookeeper says the leader is: ", leader)
		ip, port, err := getProto(leader)
		if err != nil {
			logging.Error.Println(err)
		}

		sj, _ = rg.loadWrap(ip, port)
		if sj.Leader != "" {
			return sj, nil
		}
		logging.Verbose.Println("Warning: Zookeeper is wrong about leader")
		if len(masters) == 0 {
			return sj, errors.New("no master")
		}
		logging.Verbose.Println("Warning: falling back to Masters config field: ", masters)
	}

	// try each listed mesos master before dying
	for i, master := range masters {
		ip, port, err := getProto(master)
		if err != nil {
			logging.Error.Println(err)
		}

		sj, _ = rg.loadWrap(ip, port)
		if sj.Leader == "" {
			logging.VeryVerbose.Println("Warning: not a leader - trying next one")
			if len(masters)-1 == i {
				return sj, errors.New("no master")
			}

		} else {
			return sj, nil
		}

	}

	return sj, errors.New("no master")
}

// Loads state.json from mesos master
func (rg *RecordGenerator) loadFromMaster(ip string, port string) (sj StateJSON) {
	// REFACTOR: state.json security
	url := "http://" + ip + ":" + port + "/master/state.json"

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logging.Error.Println(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logging.Error.Println(err)
	}

	err = json.Unmarshal(body, &sj)
	if err != nil {
		logging.Error.Println(err)
	}

	return sj
}

// Catches an attempt to load state.json from a mesos master
// attempts can fail from down server or mesos master secondary
// it also reloads from a different master if the master it attempted to
// load from was not the leader
func (rg *RecordGenerator) loadWrap(ip string, port string) (StateJSON, error) {
	var err error
	var sj StateJSON

	defer func() {
		if rec := recover(); rec != nil {
			err = errors.New("can't connect to master")
		}

	}()

	logging.VeryVerbose.Println("reloading from master " + ip)
	sj = rg.loadFromMaster(ip, port)

	if rip := leaderIP(sj.Leader); rip != ip {
		logging.VeryVerbose.Println("Warning: master changed to " + ip)
		sj = rg.loadFromMaster(rip, port)
	}

	return sj, err
}

// BUG: The probability of hashing collisions is too high with only 17 bits.
// NOTE: Using a numerical base as high as valid characters in DNS names would
// reduce the resulting length without risking more collisions.
func hashString(s string) string {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s))
	sum := h.Sum32()
	lower, upper := uint16(sum), uint16(sum>>16)
	return strconv.FormatUint(uint64(lower+upper), 10)
}

// attempt to translate the hostname into an IPv4 address. logs an error if IP
// lookup fails. if an IP address cannot be found, returns the same hostname
// that was given. upon success returns the IP address as a string.
func hostToIP4(hostname string) (string, bool) {
	ip := net.ParseIP(hostname)
	if ip == nil {
		t, err := net.ResolveIPAddr("ip4", hostname)
		if err != nil {
			logging.Error.Printf("cannot translate hostname %q into an ip4 address", hostname)
			return hostname, false
		}
		ip = t.IP
	}
	return ip.String(), true
}

// attempt to convert the slave hostname to an IP4 address. if that fails, then
// sanitize the hostname for DNS compat.
func sanitizedSlaveAddress(hostname string, spec labels.HostNameSpec) (string, bool) {
	address, ok := hostToIP4(hostname)
	if !ok {
		address = labels.AsDomainFrag(address, spec)
	}
	return address, ok
}

func (t *Task) containerIP() string {
	const containerIPTaskStatusLabel = "Docker.NetworkSettings.IPAddress"

	// find TASK_RUNNING statuses
	var latestContainerIP string
	var latestTimestamp float64
	for _, status := range t.Statuses {
		if status.State != "TASK_RUNNING" {
			continue
		}

		// find the latest docker-inspect label
		for _, label := range status.Labels {
			if label.Key == containerIPTaskStatusLabel && status.Timestamp > latestTimestamp {
				latestContainerIP = label.Value
				latestTimestamp = status.Timestamp
				break
			}
		}
	}

	return latestContainerIP
}

// InsertState transforms a StateJSON into RecordGenerator RRs
func (rg *RecordGenerator) InsertState(sj StateJSON, domain string, ns string,
	listener string, masters []string, spec labels.HostNameSpec) error {

	// creates a map with slave IP addresses (IPv4)
	rg.SlaveIPs = make(map[string]string)
	rg.SRVs = make(rrs)
	rg.As = make(rrs)

	for _, slave := range sj.Slaves {
		ssa, ok := sanitizedSlaveAddress(slave.Hostname, spec)
		rg.SlaveIPs[slave.ID] = ssa

		if ok {
			rg.insertRR("slave."+domain+".", ssa, "A")
		} else {
			logging.VeryVerbose.Printf("string '%q' for slave with id %q is not a valid IP address", ssa, slave.ID)
		}
	}


	// complete crap - refactor me
	for _, f := range sj.Frameworks {
		fname := labels.AsDomainFrag(f.Name, spec)
		for _, task := range f.Tasks {
			hostIP, ok := rg.SlaveIPs[task.SlaveID]
			// skip not running or not discoverable tasks
			if !ok || (task.State != "TASK_RUNNING") {
				continue
			}

			tname := spec.Mangle(task.Name)
			sid := slaveIDTail(task.SlaveID)
			tag := hashString(task.ID)
			tail := fname + "." + domain + "."

			// A records for task and task-sid
			arec := tname + "." + tail
			rg.insertRR(arec, hostIP, "A")
			trec := tname + "-" + tag + "-" + sid + "." + tail
			rg.insertRR(trec, hostIP, "A")

			// A records with container IP
			if containerIP := task.containerIP(); containerIP != "" {
				rg.insertRR("_container."+arec, containerIP, "A")
				rg.insertRR("_container."+trec, containerIP, "A")
			}

			// SRV records
			if task.Resources.Ports != "" {
				ports := yankPorts(task.Resources.Ports)
				for _, port := range ports {
					srvhost := trec + ":" + port
					tcp := "_" + tname + "._tcp." + tail
					udp := "_" + tname + "._udp." + tail
					rg.insertRR(tcp, srvhost, "SRV")
					rg.insertRR(udp, srvhost, "SRV")
				}
			}
		}
	}

	rg.listenerRecord(listener, ns)
	rg.masterRecord(domain, masters, sj.Leader)
	return nil
}

// masterRecord injects A and SRV records into the generator store:
//     master.domain.  // resolves to IPs of all masters
//     masterN.domain. // one IP address for each master
//     leader.domain.  // one IP address for the leading master
//
// The current func implementation makes an assumption about the order of masters:
// it's the order in which you expect the enumerated masterN records to be created.
// This is probably important: if a new leader is elected, you may not want it to
// become master0 simply because it's the leader. You probably want your DNS records
// to change as little as possible. And this func should have the least impact on
// enumeration order, or name/IP mappings - it's just creating the records. So let
// the caller do the work of ordering/sorting (if desired) the masters list if a
// different outcome is desired.
//
// Another consequence of the current overall mesos-dns app implementation is that
// the leader may not even be in the masters list at some point in time. masters is
// really fallback-masters (only consider these to be masters if I can't find a
// leader via ZK). At some point in time, they may not actually be masters any more.
// Consider a cluster of 3 nodes that suffers the loss of a member, and gains a new
// member (VM crashed, was replaced by another VM). And the cycle repeats several
// times. You end up with a set of running masters (and leader) that's different
// than the set of statically configured fallback masters.
//
// So the func tries to index the masters as they're listed and begrudgingly assigns
// the leading master an index out-of-band if it's not actually listed in the masters
// list. There are probably better ways to do it.
func (rg *RecordGenerator) masterRecord(domain string, masters []string, leader string) {
	// create records for leader
	// A records
	h := strings.Split(leader, "@")
	if len(h) < 2 {
		logging.Error.Println(leader)
		return // avoid a panic later
	}
	leaderAddress := h[1]
	ip, port, err := getProto(leaderAddress)
	if err != nil {
		logging.Error.Println(err)
		return
	}
	arec := "leader." + domain + "."
	rg.insertRR(arec, ip, "A")
	arec = "master." + domain + "."
	rg.insertRR(arec, ip, "A")

	// SRV records
	tcp := "_leader._tcp." + domain + "."
	udp := "_leader._udp." + domain + "."
	host := "leader." + domain + "." + ":" + port
	rg.insertRR(tcp, host, "SRV")
	rg.insertRR(udp, host, "SRV")

	// if there is a list of masters, insert that as well
	addedLeaderMasterN := false
	idx := 0
	for _, master := range masters {

		ip, _, err := getProto(master)
		if err != nil {
			logging.Error.Println(err)
			continue
		}

		// A records (master and masterN)
		if master != leaderAddress {
			arec := "master." + domain + "."
			added := rg.insertRR(arec, ip, "A")
			if !added {
				// duplicate master?!
				continue
			}
		}

		if master == leaderAddress && addedLeaderMasterN {
			// duplicate leader in masters list?!
			continue
		}

		arec := "master" + strconv.Itoa(idx) + "." + domain + "."
		rg.insertRR(arec, ip, "A")
		idx++

		if master == leaderAddress {
			addedLeaderMasterN = true
		}
	}
	// flake: we ended up with a leader that's not in the list of all masters?
	if !addedLeaderMasterN {
		// only a flake if there were fallback masters configured
		if len(masters) > 0 {
			logging.Error.Printf("warning: leader %q is not in master list", leader)
		}
		arec = "master" + strconv.Itoa(idx) + "." + domain + "."
		rg.insertRR(arec, ip, "A")
	}
}

// A record for mesos-dns (the name is listed in SOA replies)
func (rg *RecordGenerator) listenerRecord(listener string, ns string) {
	if listener == "0.0.0.0" {
		rg.setFromLocal(listener, ns)
	} else if listener == "127.0.0.1" {
		rg.insertRR(ns, "127.0.0.1", "A")
	} else {
		rg.insertRR(ns, listener, "A")
	}
}

// A records for each local interface
// If this causes problems you should explicitly set the
// listener address in config.json
func (rg *RecordGenerator) setFromLocal(host string, ns string) {

	ifaces, err := net.Interfaces()
	if err != nil {
		logging.Error.Println(err)
	}

	// handle err
	for _, i := range ifaces {

		addrs, err := i.Addrs()
		if err != nil {
			logging.Error.Println(err)
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ip = ip.To4()
			if ip == nil {
				continue
			}

			rg.insertRR(ns, ip.String(), "A")
		}
	}
}

func (rg *RecordGenerator) exists(name, host, rtype string) bool {
	if rtype == "A" {
		if val, ok := rg.As[name]; ok {
			// check if A record already exists
			// identical tasks on same slave
			for _, b := range val {
				if b == host {
					return true
				}
			}
		}
	} else {
		if val, ok := rg.SRVs[name]; ok {
			// check if SRV record already exists
			for _, b := range val {
				if b == host {
					return true
				}
			}
		}
	}
	return false
}

// insertRR adds a record to the appropriate record map for the given name/host pair,
// but only if the pair is unique. returns true if added, false otherwise.
// TODO(???): REFACTOR when storage is updated
func (rg *RecordGenerator) insertRR(name, host, rtype string) bool {
	logging.VeryVerbose.Println("[" + rtype + "]\t" + name + ": " + host)

	if rg.exists(name, host, rtype) {
		return false
	}
	if rtype == "A" {
		val := rg.As[name]
		rg.As[name] = append(val, host)
	} else {
		val := rg.SRVs[name]
		rg.SRVs[name] = append(val, host)
	}
	return true
}

// returns an array of ports from a range
func yankPorts(ports string) []string {
	rhs := strings.Split(ports, "[")[1]
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

// leaderIP returns the ip for the mesos master
// input format master@ip:port
func leaderIP(leader string) string {
	pair := strings.Split(leader, "@")[1]
	return strings.Split(pair, ":")[0]
}

// return the slave number from a Mesos slave id
func slaveIDTail(slaveID string) string {
	fields := strings.Split(slaveID, "-")
	return strings.ToLower(fields[len(fields)-1])
}

// should be able to accept
// ip:port
// zk://host1:port1,host2:port2,.../path
// zk://username:password@host1:port1,host2:port2,.../path
// file:///path/to/file (where file contains one of the above)
func getProto(pair string) (string, string, error) {
	h := strings.SplitN(pair, ":", 2)
	if len(h) != 2 {
		return "", "", fmt.Errorf("unable to parse proto from %q", pair)
	}
	return h[0], h[1], nil
}
