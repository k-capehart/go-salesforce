package salesforce

import (
	"encoding/json"
	"fmt"
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

func loginPassword(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) *Auth {
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
		fmt.Println("Error logging in")
		fmt.Println(err.Error())
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error with " + resp.Request.Method + " " + endpoint)
		fmt.Println(resp.Status)
		return nil
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response")
		fmt.Println(err.Error())
		return nil
	}

	auth := &Auth{}
	jsonError := json.Unmarshal(respBody, &auth)
	if jsonError != nil {
		fmt.Println("Error decoding response")
		fmt.Println(jsonError.Error())
		return nil
	}

	defer resp.Body.Close()
	return auth
}
