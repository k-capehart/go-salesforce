package salesforce

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Auth struct {
	sessionId string
}

func (sf *Salesforce) Login(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) {

	payload := url.Values{
		"grant_type":    {"password"},
		"client_id":     {consumerKey},
		"client_secret": {consumerSecret},
		"username":      {username},
		"password":      {password + securityToken},
	}

	body := strings.NewReader(payload.Encode())
	req, err := http.NewRequest("POST", domain+"/services/oauth2/token", body)
	if err != nil {
		fmt.Println("Error logging in")
		fmt.Println(err.Error())
	}
	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	fmt.Println(resp)
	defer resp.Body.Close()
	return
}
