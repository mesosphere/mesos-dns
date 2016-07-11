package httpcli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/mesosphere/mesos-dns/errorutil"
	"github.com/mesosphere/mesos-dns/httpcli/iam"
)

// NewIAM wraps an HTTP transactor given an IAM configuration
func NewIAM(client *http.Client, config *iam.Config) Doer {
	return &iamAuthClient{
		client: client,
		config: config,
	}
}

type iamAuthClient struct {
	client *http.Client
	config *iam.Config
}

// Do implements Doer for iamAuthClient
func (a *iamAuthClient) Do(req *http.Request) (*http.Response, error) {
	// TODO if we still have a valid token, try using it first
	token := jwt.New(jwt.SigningMethodRS256)
	token.Claims["uid"] = a.config.ID
	token.Claims["exp"] = time.Now().Add(time.Hour).Unix()
	// SignedString will treat secret as PEM-encoded key
	tokenStr, err := token.SignedString([]byte(a.config.Secret))
	if err != nil {
		return nil, err
	}

	authReq := struct {
		UID   string `json:"uid"`
		Token string `json:"token,omitempty"`
	}{
		UID:   a.config.ID,
		Token: tokenStr,
	}

	b, err := json.Marshal(authReq)
	if err != nil {
		return nil, err
	}

	authBody := bytes.NewBuffer(b)
	resp, err := a.client.Post(a.config.LoginEndpoint, "application/json", authBody)
	if err != nil {
		return nil, err
	}
	defer errorutil.Ignore(resp.Body.Close)
	if resp.StatusCode != 200 {
		return nil, ErrAuthFailed
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

	return a.client.Do(req)
}
