package iam

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/mesosphere/mesos-dns/errorutil"
)

// LoadFromFile reads an IAM Config from a file (JSON format) on the local filesystem.
func LoadFromFile(filename string) (config Config, err error) {
	var f *os.File
	if f, err = os.Open(filename); err != nil {
		err = fmt.Errorf("failed to load IAM config from file %q: %+v", filename, err)
		return
	}

	defer errorutil.Ignore(f.Close)
	dec := json.NewDecoder(f)
	if err = dec.Decode(&config); err != nil {
		err = fmt.Errorf("invalid IAM JSON: %+v", err)
		return
	}
	if _, err = url.Parse(config.LoginEndpoint); err != nil {
		err = fmt.Errorf("invalid LoginEndpoint URL: %+v", err)
	}
	return
}
