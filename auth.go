package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type authentication struct {
	AccessToken string `json:"access_token"`
	InstanceUrl string `json:"instance_url"`
	Id          string `json:"id"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
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
	AccessToken    string
}

const (
	grantTypePassword          = "password"
	grantTypeClientCredentials = "client_credentials"
)

func validateAuth(sf Salesforce) error {
	if sf.Auth == nil || sf.Auth.AccessToken == "" {
		return errors.New("not authenticated: please use salesforce.Init()")
	}
	return nil
}

func validateSession(auth authentication) error {
	if err := validateAuth(Salesforce{Auth: &auth}); err != nil {
		return err
	}
	_, err := doRequest(http.MethodGet, "/limits", jsonType, auth, "", http.StatusOK)
	if err != nil {
		return err
	}

	return nil
}

func doAuth(url string, body *strings.Reader) (*authentication, error) {
	resp, err := http.Post(url, "application/x-www-form-urlencoded", body)
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

	auth := &authentication{}
	jsonError := json.Unmarshal(respBody, &auth)
	if jsonError != nil {
		return nil, jsonError
	}

	defer resp.Body.Close()
	return auth, nil
}

func usernamePasswordFlow(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) (*authentication, error) {
	payload := url.Values{
		"grant_type":    {grantTypePassword},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
		"username":      {username},
		"password":      {password + securityToken},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	auth, err := doAuth(domain+endpoint, body)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func clientCredentialsFlow(domain string, consumerKey string, consumerSecret string) (*authentication, error) {
	payload := url.Values{
		"grant_type":    {grantTypeClientCredentials},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
	}
	endpoint := "/services/oauth2/token"
	body := strings.NewReader(payload.Encode())
	auth, err := doAuth(domain+endpoint, body)
	if err != nil {
		return nil, err
	}
	return auth, nil
}

func setAccessToken(domain string, accessToken string) (*authentication, error) {
	auth := &authentication{InstanceUrl: domain, AccessToken: accessToken}
	if err := validateSession(*auth); err != nil {
		return nil, err
	}
	return auth, nil
}
