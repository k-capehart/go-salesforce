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

type SObject struct {
	Id string
}

type QueryResponse struct {
	Done           bool      `json:"done"`
	NextRecordsURL string    `json:"nextRecordsUrl"`
	SObjects       []SObject `json:"records"`
	TotalSize      int       `json:"totalSize"`
}

type Salesforce struct {
	auth *Auth
}

func Init(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) *Salesforce {
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

	sf := &Salesforce{}
	jsonError := json.Unmarshal(respBody, &sf.auth)
	if jsonError != nil {
		fmt.Println("Error decoding response")
		fmt.Println(jsonError.Error())
		return nil
	}

	defer resp.Body.Close()
	return sf
}

func (sf *Salesforce) Query(query string) []byte {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return nil
	}
	query = url.QueryEscape(query)
	endpoint := sf.auth.InstanceUrl + "/services/data/v60.0/query/?q=" + query
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Println("Error creating request")
		fmt.Println(err.Error())
		return nil
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+sf.auth.AccessToken)
	resp, err := http.DefaultClient.Do(req)
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
	defer resp.Body.Close()
	return respBody
}
