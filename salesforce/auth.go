package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Auth struct {
	AccessToken string `json:"access_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
}

type Creds struct {
	Domain         string
	Username       string
	Password       string
	SecurityToken  string
	ConsumerKey    string
	ConsumerSecret string
}

func validateAuth(sf Salesforce) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}
	return nil
}

func loginPassword(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) (*Auth, error) {
	payload := url.Values{
		"grant_type":    {"password"},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
		"username":      {username},
		"password":      {password + securityToken},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	resp, err := http.Post(domain+endpoint, "application/x-www-form-urlencoded", body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(string(resp.Status) + ":" + " failed authentication")
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	auth := &Auth{}
	jsonError := json.Unmarshal(respBody, &auth)
	if jsonError != nil {
		return nil, jsonError
	}

	defer resp.Body.Close()
	return auth, nil
}
