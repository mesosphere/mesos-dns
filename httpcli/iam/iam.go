package iam

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mesosphere/mesos-dns/errorutil"
	"github.com/mesosphere/mesos-dns/httpcli"
)

// Register registers a DoerFactory for IAM (JWT-based) authentication
func Register() {
	httpcli.Register(httpcli.AuthIAM, httpcli.DoerFactory(func(cm httpcli.ConfigMap, c *http.Client) (doer httpcli.Doer) {
		obj := cm.FindOrPanic(httpcli.AuthIAM)
		config, ok := obj.(Config)
		if !ok {
			panic(fmt.Errorf("expected Config instead of %#+v", obj))
		}
		validate(config)
		if c != nil {
			doer = Doer(c, config)
		}
		return
	}))
}

func validate(c Config) {
	if c.ID == "" || c.PrivateKey == "" || c.LoginEndpoint == "" {
		panic(ErrInvalidConfiguration)
	}
}

// Doer wraps an HTTP transactor given an IAM configuration
func Doer(client *http.Client, config Config) httpcli.Doer {
	return httpcli.DoerFunc(func(req *http.Request) (*http.Response, error) {
		// TODO if we still have a valid token, try using it first
		token := jwt.New(jwt.SigningMethodRS256)
		token.Claims["uid"] = config.ID
		token.Claims["exp"] = time.Now().Add(time.Hour).Unix()
		// SignedString will treat secret as PEM-encoded key
		tokenStr, err := token.SignedString([]byte(config.PrivateKey))
		if err != nil {
			return nil, err
		}

		authReq := struct {
			UID   string `json:"uid"`
			Token string `json:"token,omitempty"`
		}{
			UID:   config.ID,
			Token: tokenStr,
		}

		b, err := json.Marshal(authReq)
		if err != nil {
			return nil, err
		}

		authBody := bytes.NewBuffer(b)
		resp, err := client.Post(config.LoginEndpoint, "application/json", authBody)
		if err != nil {
			return nil, err
		}
		defer errorutil.Ignore(resp.Body.Close)
		if resp.StatusCode != 200 {
			return nil, httpcli.ErrAuthFailed
		}

		var authResp struct {
			Token string `json:"token"`
		}
		err = json.NewDecoder(resp.Body).Decode(&authResp)
		if err != nil {
			return nil, err
		}

		if req.Header == nil {
			req.Header = make(http.Header)
		}
		req.Header.Set("Authorization", "token="+authResp.Token)

		return client.Do(req)
	})
}
