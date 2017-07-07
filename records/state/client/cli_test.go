package client

import (
	"testing"
)

func TestInvalidLeaderIP(t *testing.T) {
	l := "master!144.76.157.37;5050"

	ip, err := leaderIP(l)

	if err == nil || ip != "" {
		t.Error("invalid ip was parsed")
	}
}

func TestLeaderIP(t *testing.T) {
	l := "master@144.76.157.37:5050"

	ip, err := leaderIP(l)

	if err != nil {
		t.Error(err)
	}

	if ip != "144.76.157.37" {
		t.Error("not parsing ip")
	}
}
