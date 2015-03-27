package records

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/mesosphere/mesos-dns/logging"
	"github.com/miekg/dns"
)

// Config holds mesos dns configuration
type Config struct {

	// Mesos master(s): a list of IP:port pairs for one or more Mesos masters
	Masters []string

	// Zookeeper: a single Zk url
	Zk string

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

	// Http port
	HttpPort int

	// Enable/discable servers
	DnsOn	bool
	HttpOn	bool
}

// SetConfig instantiates a Config struct read in from config.json
func SetConfig(cjson string) (c Config) {
	c = Config{
		Zk:             "",
		RefreshSeconds: 60,
		TTL:            60,
		Domain:         "mesos",
		Port:           53,
		Timeout:        5,
		Email:          "root.mesos-dns.mesos",
		Resolvers:      []string{"8.8.8.8"},
		Listener:       "0.0.0.0",
		HttpPort:		8123,
		DnsOn:			true,
		HttpOn:			true,
	}

	// read configuration file
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
	c.File = path

	err = json.Unmarshal(b, &c)
	if err != nil {
		logging.Error.Println(err)
	}

	// validate and complete configuration file
	if !(c.DnsOn || c.HttpOn) {
		logging.Error.Println("Either DNS or HTTP server should be on")
		os.Exit(1)
	}
	if len(c.Masters) == 0 && c.Zk == "" {
		logging.Error.Println("specify mesos masters or zookeeper in config.json")
		os.Exit(1)
	}

	if len(c.Resolvers) == 0 {
		c.Resolvers = GetLocalDNS()
	}

	c.Email = strings.Replace(c.Email, "@", ".", -1)
	if c.Email[len(c.Email)-1:] != "." {
		c.Email = c.Email + "."
	}

	c.Domain = strings.ToLower(c.Domain)
	c.Mname = "mesos-dns." + c.Domain + "."

	// print configuration file
	logging.Verbose.Println("Mesos-DNS configuration:")
	if len(c.Masters) != 0 {
		logging.Verbose.Println("   - Masters: " + strings.Join(c.Masters, ", "))
	}
	if c.Zk != "" {
		logging.Verbose.Println("   - Zookeeper: ", c.Zk)
	}
	logging.Verbose.Println("   - RefreshSeconds: ", c.RefreshSeconds)
	logging.Verbose.Println("   - Domain: " + c.Domain)
	logging.Verbose.Println("   - Listener: " + c.Listener)
	logging.Verbose.Println("   - Port: ", c.Port)
	logging.Verbose.Println("   - DnsOn: ", c.DnsOn)
	logging.Verbose.Println("   - TTL: ", c.TTL)
	logging.Verbose.Println("   - Timeout: ", c.Timeout)
	logging.Verbose.Println("   - Resolvers: " + strings.Join(c.Resolvers, ", "))
	logging.Verbose.Println("   - Email: " + c.Email)
	logging.Verbose.Println("   - Mname: " + c.Mname)
	logging.Verbose.Println("   - HttpPort: ", c.HttpPort)
	logging.Verbose.Println("   - HttpOn: ", c.HttpOn)
	logging.Verbose.Println("   - ConfigFile: ", c.File)

	return c
}

// Returns the first nameserver in /etc/resolv.conf
// used for non-Mesos  queries
func GetLocalDNS() []string {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		logging.Error.Println(err)
		os.Exit(2)
	}

	return nonLocalAddies(conf.Servers)
}

// Returns non-local nameserver entries
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

// Returns an array of local ipv4 addresses
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

