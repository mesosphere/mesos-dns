// package records contains functions to generate resource records from
// mesos master states to serve through a dns server
package records

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"strings"
	"hash/fnv"

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
	As     rrs
	SRVs   rrs
	Slaves map[string]string
}

// The following types help parse state.json
// Resources holds our SRV ports
type Resources struct {
	Ports string `json:"ports"`
}

// Tasks holds mesos task information read in from state.json
type Tasks []struct {
	FrameworkId string `json:"framework_id"`
	Id          string `json:"id"`
	Name        string `json:"name"`
	SlaveId     string `json:"slave_id"`
	State       string `json:"state"`
	Resources   `json:"resources"`
}

// Frameworks holds mesos frameworks information read in from state.json
type Frameworks []struct {
	Tasks `json:"tasks"`
	Name  string `json:"name"`
}

// Slaves is a mapping of id to hostname read in from state.json
type slave struct {
	Id       string `json:"id"`
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

	// insert state
	rg.InsertState(sj, c.Domain, c.SOARname, c.Listener, c.Masters)
	return nil
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
		if sj.Leader == "" {
			logging.Verbose.Println("Warning: Zookeeper is wrong about leader")
			if len(masters) == 0 {
				return sj, errors.New("no master")
			} else {
				logging.Verbose.Println("Warning: falling back to Masters config field: ", masters)
			}
		} else {
			return sj, nil
		}
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

// hash two long strings into a short one
func hashString (s string) string {
	var upper, lower, sum uint16
	h:= fnv.New32a()
	h.Write([]byte(s))
	lower = uint16(h.Sum32())
	upper = uint16(h.Sum32() >> 16)
	sum   = uint16(lower + upper)
	return strconv.Itoa(int(sum))
}


// InsertState transforms a StateJSON into RecordGenerator RRs
func (rg *RecordGenerator) InsertState(sj StateJSON, domain string, ns string,
	listener string, masters []string) error {

	// creates a map with slave IP addresses (IPv4)
	rg.Slaves = make(map[string]string)
	for _, slave := range sj.Slaves {
		// if slave is a hostname, translate it
		if net.ParseIP(slave.Hostname) == nil {
			t, err := net.ResolveIPAddr("ip4", slave.Hostname)
			if err != nil {
				logging.Error.Println("cannot translate hostname" + slave.Hostname)
			}
			rg.Slaves[slave.Id] = t.IP.String()
		} else {
			rg.Slaves[slave.Id] = slave.Hostname
		}
	}

	rg.SRVs = make(rrs)
	rg.As = make(rrs)

	// complete crap - refactor me
	for _, f := range sj.Frameworks {
		fname := labels.AsDomainFrag(f.Name)

		for _, task := range f.Tasks {
			host, ok := rg.Slaves[task.SlaveId]
			// skip not running or not discoverable tasks
			if !ok || (task.State != "TASK_RUNNING") {
				continue
			}

			tname := labels.AsDNS952(task.Name)
			sid := slaveIdTail(task.SlaveId)
			tag := hashString(task.Id)
			tail := fname + "." + domain + "."

			// A records for task and task-sid
			arec := tname + "." + tail
			rg.insertRR(arec, host, "A")
			trec := tname + "-" + tag + "-" + sid + "." + tail
			rg.insertRR(trec, host, "A")

			// SRV records
			if task.Resources.Ports != "" {
				ports := yankPorts(task.Resources.Ports)
				for _, port := range ports {
					var srvhost string = trec + ":" + port
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

// A records for the mesos masters
func (rg *RecordGenerator) masterRecord(domain string, masters []string, leader string) {
	// create records for leader
	// A records
	h := strings.Split(leader, "@")
	if len(h) < 2 {
		logging.Error.Println(leader)
	}
	ip, port, err := getProto(h[1])
	if err != nil {
		logging.Error.Println(err)
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
	for i, master := range masters {

		// skip leader
		if leader == master {
			continue
		}

		ip, _, err := getProto(master)
		if err != nil {
			logging.Error.Println(err)
		}

		// A records (master and masterN)
		arec := "master." + domain + "."
		rg.insertRR(arec, ip, "A")
		arec = "master" + strconv.Itoa(i) + "." + domain + "."
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

// insertRR inserts host to name's map
// REFACTOR when storage is updated
func (rg *RecordGenerator) insertRR(name string, host string, rtype string) {
	logging.VeryVerbose.Println("[" + rtype + "]\t" + name + ": " + host)

	if rtype == "A" {
		if val, ok := rg.As[name]; ok {
			// check if A record already exists
			// identical tasks on same slave
			for _, b := range val {
				if b == host {
					return
				}
			}
			rg.As[name] = append(val, host)
		} else {
			rg.As[name] = []string{host}
		}
	} else {
		if val, ok := rg.SRVs[name]; ok {
			// check if SRV record already exists
			for _, b := range val {
				if b == host {
					return
				}
			}
			rg.SRVs[name] = append(val, host)
		} else {
			rg.SRVs[name] = []string{host}
		}
	}
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
func slaveIdTail(slaveID string) string {
	fields := strings.Split(slaveID, "-")
	return strings.ToLower(fields[len(fields)-1])
}

// should be able to accept
// ip:port
// zk://host1:port1,host2:port2,.../path
// zk://username:password@host1:port1,host2:port2,.../path
// file:///path/to/file (where file contains one of the above)
func getProto(pair string) (string, string, error) {
	h := strings.Split(pair, ":")
	return h[0], h[1], nil
}
