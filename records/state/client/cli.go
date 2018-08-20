package client

import (
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/mesosphere/mesos-dns/errorutil"
	"github.com/mesosphere/mesos-dns/httpcli"
	"github.com/mesosphere/mesos-dns/logging"
	"github.com/mesosphere/mesos-dns/records/state"
	"github.com/mesosphere/mesos-dns/urls"
)

type (
	// StateLoader attempts to read state from the leading Mesos master and return the parsed content.
	StateLoader func(masters []string) (state.State, error)

	// Unmarshaler parses raw byte content into a State object
	Unmarshaler func([]byte, *state.State) error
)

// NewStateLoader generates a new Mesos master state loader using the given http client and initial endpoint.
func NewStateLoader(doer httpcli.Doer, initialEndpoint urls.Builder, unmarshal Unmarshaler) StateLoader {
	return func(masters []string) (state.State, error) {
		return LoadMasterStateTryAll(masters, func(ip, port string) (state.State, error) {
			return LoadMasterStateFailover(ip, func(tryIP string) (state.State, error) {
				return LoadMasterState(doer, initialEndpoint, tryIP, port, unmarshal)
			})
		})
	}

}

// LoadMasterStateTryAll tries each master and looks for the leader; if no leader responds it errors.
// The first master in the list is assumed to be the leading mesos master.
func LoadMasterStateTryAll(masters []string, stateLoader func(ip, port string) (state.State, error)) (state.State, error) {
	var sj state.State
	var leader string

	if len(masters) > 0 {
		leader, masters = masters[0], masters[1:]
	}

	// Check if ZK leader is correct
	if leader != "" {
		logging.VeryVerbose.Println("Zookeeper says the leader is: ", leader)
		ip, port, err := urls.SplitHostPort(leader)
		if err != nil {
			logging.Error.Println(err)
		} else {
			if sj, err = stateLoader(ip, port); err == nil {
				return sj, nil
			}
			logging.Error.Println("Failed to fetch state from leader. Error: ", err)
			if len(masters) == 0 {
				logging.Error.Println("No more masters to try, returning last error")
				return sj, err
			}
			logging.Error.Println("Falling back to remaining masters: ", masters)
		}
	}

	// try each listed mesos master before dying
	var (
		ip, port string
		err      error
	)
	for _, master := range masters {
		ip, port, err = urls.SplitHostPort(master)
		if err != nil {
			logging.Error.Println(err)
			continue
		}

		if sj, err = stateLoader(ip, port); err != nil {
			logging.Error.Println("Failed to fetch state - trying next one. Error: ", err)
			continue
		}
		return sj, nil
	}

	logging.Error.Println("No more masters eligible for state query, returning last error")
	return sj, err
}

// LoadMasterStateFailover catches an attempt to load state from a mesos master.
// Attempts can fail from due to a down server or if contacting a mesos master secondary.
// It reloads from a different master if the contacted master is a secondary.
func LoadMasterStateFailover(initialMasterIP string, stateLoader func(ip string) (state.State, error)) (state.State, error) {
	var err error
	var sj state.State

	logging.VeryVerbose.Println("reloading from master " + initialMasterIP)
	sj, err = stateLoader(initialMasterIP)
	if err != nil {
		return state.State{}, err
	}
	if sj.Leader != "" {
		var stateLeaderIP string

		stateLeaderIP, err = leaderIP(sj.Leader)
		if err != nil {
			return sj, err
		}
		if stateLeaderIP != initialMasterIP {
			logging.VeryVerbose.Println("Warning: master changed to " + stateLeaderIP)
			return stateLoader(stateLeaderIP)
		}
		return sj, nil
	}
	err = errors.New("Fetched state does not contain leader information")
	return sj, err
}

// LoadMasterState loads state from mesos master
func LoadMasterState(client httpcli.Doer, stateEndpoint urls.Builder, ip, port string, unmarshal Unmarshaler) (sj state.State, _ error) {
	// REFACTOR: state security

	u := url.URL(stateEndpoint.With(urls.Host(net.JoinHostPort(ip, port))))

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		logging.Error.Println(err)
		return state.State{}, err
	}

	req.Header.Set("Content-Type", "application/json") // TODO(jdef) unclear why Content-Type vs. Accept
	req.Header.Set("User-Agent", "Mesos-DNS")

	resp, err := client.Do(req)
	if err != nil {
		logging.Error.Println(err)
		return sj, err
	}

	defer errorutil.Ignore(resp.Body.Close)
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logging.Error.Println(err)
		return sj, err
	}

	err = unmarshal(body, &sj)
	if err != nil {
		logging.Error.Println(err)
		return sj, err
	}

	return
}

// leaderIP returns the ip for the mesos master
// input format master@ip:port
func leaderIP(leader string) (string, error) {
	// TODO(jdef) it's unclear why we drop the port here
	nameAddressPair := strings.Split(leader, "@")
	if len(nameAddressPair) != 2 {
		return "", errors.New("Invalid leader address: " + leader)
	}
	hostPort := nameAddressPair[1]
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return "", err
	}
	return host, nil
}
