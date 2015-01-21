package records

import (
	"encoding/json"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/miekg/dns"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// Config holds mesos dns configuration
type Config struct {

	// Mesos master(s): a list of IP:port/zk pairs for one or more Mesos masters
	Masters []string

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
}

// SetConfig instantiates a Config struct read in from config.json
func SetConfig(cjson string) (c Config) {
	c = Config{
		RefreshSeconds: 60,
		TTL:            60,
		Domain:         "mesos",
		Port:           53,
		Timeout:        5,
		Resolvers:      []string{"8.8.8.8"},
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

	if len(c.Masters) == 0 {
		logging.Error.Println("please specify mesos masters in config.json")
		os.Exit(1)
	}

    logging.Verbose.Println("Mesos-DNS configuration:")
    logging.Verbose.Println("   - Masters: " + strings.Join(c.Masters,", "))
    logging.Verbose.Println("   - RefreshSeconds: ", c.RefreshSeconds)
    logging.Verbose.Println("   - TTL: ", c.TTL)
    logging.Verbose.Println("   - Domain: " + c.Domain)
    logging.Verbose.Println("   - Port: ", c.Port)
    logging.Verbose.Println("   - Timeout: ",  c.Timeout)
    logging.Verbose.Println("   - Resolvers: " + strings.Join(c.Resolvers,", "))

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
