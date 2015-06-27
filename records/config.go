package records

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"os/user"
	"path/filepath"
	"strings"
	"time"

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
	TTL int32

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

	// SOA record fields (see http://tools.ietf.org/html/rfc1035#page-18)
	SOAMname   string // primary name server
	SOARname   string // email of admin esponsible
	SOASerial  uint32 // initial version number (incremented on refresh)
	SOARefresh uint32 // refresh interval
	SOARetry   uint32 // retry interval
	SOAExpire  uint32 // expiration time
	SOAMinttl  uint32 // minimum TTL

	// Value of RecursionAvailable for responses in Mesos domain
	RecurseOn bool

	// ListenAddr is the server listener address
	Listener string

	// Http port
	HttpPort int

	// Enable serving DSN and HTTP requests
	DnsOn  bool
	HttpOn bool

	// Enable replies for external requests
	ExternalOn bool

	// EnforceRFC952 will enforce an older, more strict set of rules for DNS labels
	EnforceRFC952 bool
}

// SetConfig instantiates a Config struct read in from config.json
func SetConfig(cjson string) (c Config) {
	c = Config{
		RefreshSeconds: 60,
		TTL:            60,
		Domain:         "mesos",
		Port:           53,
		Timeout:        5,
		SOARname:       "root.ns1.mesos",
		SOAMname:       "ns1.mesos",
		SOARefresh:     60,
		SOARetry:       600,
		SOAExpire:      86400,
		SOAMinttl:      60,
		Resolvers:      []string{"8.8.8.8"},
		Listener:       "0.0.0.0",
		HttpPort:       8123,
		DnsOn:          true,
		HttpOn:         true,
		ExternalOn:     true,
		RecurseOn:      true,
	}

	// read configuration file
	usr, _ := user.Current()
	dir := usr.HomeDir + "/"
	cjson = strings.Replace(cjson, "~/", dir, 1)

	path, err := filepath.Abs(cjson)
	if err != nil {
		logging.Error.Fatalf("cannot find configuration file")
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		logging.Error.Fatalf("missing configuration file")
	}
	c.File = path

	err = json.Unmarshal(b, &c)
	if err != nil {
		logging.Error.Println(err)
	}

	// validate and complete configuration file
	if !(c.DnsOn || c.HttpOn) {
		logging.Error.Fatalf("Either DNS or HTTP server should be on")
	}
	if len(c.Masters) == 0 && c.Zk == "" {
		logging.Error.Fatalf("specify mesos masters or zookeeper in config.json")
	}
	if err = validateMasters(c.Masters); err != nil {
		logging.Error.Fatalf("Masters validation failed: %v", err)
	}

	if c.ExternalOn {
		if len(c.Resolvers) == 0 {
			c.Resolvers = GetLocalDNS()
		}
		if err = validateResolvers(c.Resolvers); err != nil {
			logging.Error.Fatalf("Resovlers validation failed: %v", err)
		}
	}

	c.Domain = strings.ToLower(c.Domain)

	// SOA record fields
	c.SOARname = strings.Replace(c.SOARname, "@", ".", -1)
	if c.SOARname[len(c.SOARname)-1:] != "." {
		c.SOARname = c.SOARname + "."
	}
	if c.SOAMname[len(c.SOAMname)-1:] != "." {
		c.SOAMname = c.SOAMname + "."
	}
	c.SOASerial = uint32(time.Now().Unix())

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
	logging.Verbose.Println("   - ExternalOn: ", c.ExternalOn)
	logging.Verbose.Println("   - SOAMname: " + c.SOAMname)
	logging.Verbose.Println("   - SOARname: " + c.SOARname)
	logging.Verbose.Println("   - SOASerial: ", c.SOASerial)
	logging.Verbose.Println("   - SOARefresh: ", c.SOARefresh)
	logging.Verbose.Println("   - SOARetry: ", c.SOARetry)
	logging.Verbose.Println("   - SOAExpire: ", c.SOAExpire)
	logging.Verbose.Println("   - SOAExpire: ", c.SOAMinttl)
	logging.Verbose.Println("   - RecurseOn: ", c.RecurseOn)
	logging.Verbose.Println("   - HttpPort: ", c.HttpPort)
	logging.Verbose.Println("   - HttpOn: ", c.HttpOn)
	logging.Verbose.Println("   - ConfigFile: ", c.File)
	logging.Verbose.Println("   - EnforceRFC952: ", c.EnforceRFC952)

	return c
}

// Returns the first nameserver in /etc/resolv.conf
// used for non-Mesos  queries
func GetLocalDNS() []string {
	conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
	if err != nil {
		logging.Error.Fatalf("%v", err)
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
