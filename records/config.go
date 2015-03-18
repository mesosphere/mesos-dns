package records

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/mesos/mesos-go/detector"
	_ "github.com/mesos/mesos-go/detector/zoo"
	mesos "github.com/mesos/mesos-go/mesosproto"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/miekg/dns"
)

// Config holds mesos dns configuration
type Config struct {

	// Mesos master(s): a list of IP:port pairs for one or more Mesos masters
	Masters []string

	// Zookeeper: a single Zk url
	Zk []string

	// Refresh frequency: the frequency in seconds of regenerating records (default 60)
	RefreshSeconds int

	// TTL: the TTL value used for SRV and A records (default 60)
	TTL int

	// Resolver port: port used to listen for slave requests (default 53)
	Port int

	//  Domain: name of the domain used (default "mesos", ie .mesos domain)
	Domain string

	// DNS server: IP address of the DNS server for forwarded accesses
	Resolvers []string

	// Timeout is the default connect/read/write timeout for outbound
	// queries
	Timeout int

	// File is the location of the config.json file
	File string

	// Email is the rname for a SOA
	Email string

	// Mname is the mname for a SOA
	Mname string

	// ListenAddr is the server listener address
	Listener string

	// Leading master info, as identified through Zookeeper
	leader     string
	leaderLock sync.RWMutex
	first      bool
}

// SetConfig instantiates a Config struct read in from config.json
func SetConfig(cjson string) (c Config) {
	c = Config{
		RefreshSeconds: 60,
		TTL:            60,
		Domain:         "mesos",
		Port:           53,
		Timeout:        5,
		Email:          "root.mesos-dns.mesos",
		Resolvers:      []string{"8.8.8.8"},
		Listener:       "0.0.0.0",
		leader:         "",
		first:          true,
	}

	usr, _ := user.Current()
	dir := usr.HomeDir + "/"
	cjson = strings.Replace(cjson, "~/", dir, 1)

	path, err := filepath.Abs(cjson)
	if err != nil {
		logging.Error.Println("cannot find configuration file")
		os.Exit(1)
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		logging.Error.Println("missing configuration file")
		os.Exit(1)
	}

	err = json.Unmarshal(b, &c)
	if err != nil {
		logging.Error.Println(err)
	}

	if len(c.Resolvers) == 0 {
		c.Resolvers = GetLocalDNS()
	}

	if len(c.Masters) == 0 && len(c.Zk) == 0 {
		logging.Error.Println("please specify mesos masters or zookeeper in config.json")
		os.Exit(1)
	}

	c.Email = strings.Replace(c.Email, "@", ".", -1)
	if c.Email[len(c.Email)-1:] != "." {
		c.Email = c.Email + "."
	}

	c.Domain = strings.ToLower(c.Domain)
	c.Mname = "mesos-dns." + c.Domain + "."

	logging.Verbose.Println("Mesos-DNS configuration:")
	if len(c.Masters) != 0 {
		logging.Verbose.Println("   - Masters: " + strings.Join(c.Masters, ", "))
	}
	if len(c.Zk) != 0 {
		logging.Verbose.Println("   - Zookeeper: " + strings.Join(c.Zk, ", "))
	}
	logging.Verbose.Println("   - RefreshSeconds: ", c.RefreshSeconds)
	logging.Verbose.Println("   - TTL: ", c.TTL)
	logging.Verbose.Println("   - Domain: " + c.Domain)
	logging.Verbose.Println("   - Port: ", c.Port)
	logging.Verbose.Println("   - Timeout: ", c.Timeout)
	logging.Verbose.Println("   - Listener: " + c.Listener)
	logging.Verbose.Println("   - Resolvers: " + strings.Join(c.Resolvers, ", "))
	logging.Verbose.Println("   - Email: " + c.Email)
	logging.Verbose.Println("   - Mname: " + c.Mname)

	c.first = true
	return c
}

// localAddies returns an array of local ipv4 addresses
func localAddies() []string {
	addies, err := net.InterfaceAddrs()
	if err != nil {
		logging.Error.Println(err)
	}

	bad := []string{}

	for i := 0; i < len(addies); i++ {
		ip, _, err := net.ParseCIDR(addies[i].String())
		if err != nil {
			logging.Error.Println(err)
		}
		t4 := ip.To4()
		if t4 != nil {
			bad = append(bad, t4.String())
		}
	}

	return bad
}

// nonLocalAddies only returns non-local ns entries
func nonLocalAddies(cservers []string) []string {
	bad := localAddies()

	good := []string{}

	for i := 0; i < len(cservers); i++ {
		local := false
		for x := 0; x < len(bad); x++ {
			if cservers[i] == bad[x] {
				local = true
			}
		}

		if !local {
			good = append(good, cservers[i])
		}
	}

	return good
}

// GetLocalDNS returns the first nameserver in /etc/resolv.conf
// used for out of mesos domain queries
func GetLocalDNS() []string {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		logging.Error.Println(err)
		os.Exit(2)
	}

	return nonLocalAddies(conf.Servers)
}

// Start a Zookeeper listener to track leading master
func ZKdetect(c *Config, dr chan bool) {

	// start listener
	logging.Verbose.Println("Starting master detector for ZK ", c.Zk[0])
	md, err := detector.New(c.Zk[0])
	if err != nil {
		logging.Error.Println("failed to create master detector: ", err)
		os.Exit(1)
	}

	// and listen for master changes
	if err := md.Detect(detector.OnMasterChanged(func(info *mesos.MasterInfo) {
		// making this tomic
		c.leaderLock.Lock()
		defer c.leaderLock.Unlock()
		logging.VeryVerbose.Println("Updated Zookeeper info: ", info)
		if info == nil {
			c.leader = ""
			logging.Error.Println("No leader available in Zookeeper.")
		} else if host := info.GetHostname(); host != "" {
			c.leader = host
		} else {
			// unpack IPv4
			octets := make([]byte, 4, 4)
			binary.BigEndian.PutUint32(octets, info.GetIp())
			ipv4 := net.IP(octets)
			c.leader = ipv4.String()
		}
		if len(c.leader) > 0 {
			c.leader = fmt.Sprintf("%s:%d", c.leader, info.GetPort())
		}
		logging.Verbose.Println("New master in Zookeeper ", c.leader)
		if c.first {
			dr <- true
			c.first = false
		}
	})); err != nil {
		logging.Error.Println("failed to initialize master detector ", err)
		os.Exit(1)
	}

}
