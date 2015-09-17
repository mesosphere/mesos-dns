package records

import (
	"encoding/json"
	"fmt"
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

	// Zookeeper Detection Timeout: how long in seconds to wait for Zookeeper to be initially responsive (default 30)
	ZkDetectionTimeout int

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

	// NOTE(tsenart): HTTPPort, DNSOn and HTTPOn have defined JSON keys for
	// backwards compatibility with external API clients.
	HTTPPort int `json:"HttpPort"`

	// Enable serving DSN and HTTP requests
	DNSOn  bool `json:"DnsOn"`
	HTTPOn bool `json:"HttpOn"`

	// Enable replies for external requests
	ExternalOn bool

	// EnforceRFC952 will enforce an older, more strict set of rules for DNS labels
	EnforceRFC952 bool

	// IPSources is the prioritized list of task IP sources
	IPSources []string // e.g. ["host", "docker", "mesos", "rkt"]
}

// NewConfig return the default config of the resolver
func NewConfig() Config {
	return Config{
		ZkDetectionTimeout: 30,
		RefreshSeconds:     60,
		TTL:                60,
		Domain:             "mesos",
		Port:               53,
		Timeout:            5,
		SOARname:           "root.ns1.mesos",
		SOAMname:           "ns1.mesos",
		SOARefresh:         60,
		SOARetry:           600,
		SOAExpire:          86400,
		SOAMinttl:          60,
		Resolvers:          []string{"8.8.8.8"},
		Listener:           "0.0.0.0",
		HTTPPort:           8123,
		DNSOn:              true,
		HTTPOn:             true,
		ExternalOn:         true,
		RecurseOn:          true,
		IPSources:          []string{"netinfo", "mesos", "host"},
	}
}

// SetConfig instantiates a Config struct read in from config.json
func SetConfig(cjson string) Config {
	c, err := readConfig(cjson)
	if err != nil {
		logging.Error.Fatal(err)
	}
	// validate and complete configuration file
	if !c.DNSOn && !c.HTTPOn {
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
			logging.Error.Fatalf("Resolvers validation failed: %v", err)
		}
	}

	if err = validateIPSources(c.IPSources); err != nil {
		logging.Error.Fatalf("IPSources validation failed: %v", err)
	}

	c.Domain = strings.ToLower(c.Domain)

	// SOA record fields
	c.SOARname = strings.TrimRight(strings.Replace(c.SOARname, "@", ".", -1), ".") + "."
	c.SOAMname = strings.TrimRight(c.SOAMname, ".") + "."
	c.SOASerial = uint32(time.Now().Unix())

	// print configuration file
	logging.Verbose.Println("Mesos-DNS configuration:")
	logging.Verbose.Println("   - Masters: " + strings.Join(c.Masters, ", "))
	logging.Verbose.Println("   - Zookeeper: ", c.Zk)
	logging.Verbose.Println("   - ZookeeperDetectionTimeout: ", c.ZkDetectionTimeout)
	logging.Verbose.Println("   - RefreshSeconds: ", c.RefreshSeconds)
	logging.Verbose.Println("   - Domain: " + c.Domain)
	logging.Verbose.Println("   - Listener: " + c.Listener)
	logging.Verbose.Println("   - Port: ", c.Port)
	logging.Verbose.Println("   - DnsOn: ", c.DNSOn)
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
	logging.Verbose.Println("   - HttpPort: ", c.HTTPPort)
	logging.Verbose.Println("   - HttpOn: ", c.HTTPOn)
	logging.Verbose.Println("   - ConfigFile: ", c.File)
	logging.Verbose.Println("   - EnforceRFC952: ", c.EnforceRFC952)
	logging.Verbose.Println("   - IPSources: ", c.IPSources)

	return *c
}

func readConfig(file string) (*Config, error) {
	c := NewConfig()

	workingDir := "."
	usr, err := user.Current()
	if err != nil {
		// this can happen (on Linux) if you're running mesos-dns as a non-root user.
		logging.Error.Println("Failed to determine current user, translating ~/ to ./, error was", err)
	} else {
		workingDir = usr.HomeDir
	}

	c.File, err = filepath.Abs(strings.Replace(file, "~/", workingDir+"/", 1))
	if err != nil {
		return nil, fmt.Errorf("cannot find configuration file")
	} else if bs, err := ioutil.ReadFile(c.File); err != nil {
		return nil, fmt.Errorf("missing configuration file: %q", c.File)
	} else if err = json.Unmarshal(bs, &c); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %q: %v", c.File, err)
	}

	return &c, nil
}

func unique(ss []string) []string {
	set := make(map[string]struct{}, len(ss))
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		if _, ok := set[s]; !ok {
			set[s] = struct{}{}
			out = append(out, s)
		}
	}
	return out
}

// GetLocalDNS returns the first nameserver in /etc/resolv.conf
// Used for non-Mesos queries.
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
