package salesforce

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const API_VERSION = "v60.0"

type Attributes struct {
	Type string `json:"type"`
	Url  string `json:"url"`
}

type QueryResponse struct {
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Records   []map[string]any `json:"records"`
}

type Salesforce struct {
	auth *Auth
}

type SObject struct {
	SObjectName string
	Fields      map[string]string
}

func Init(domain string, username string, password string, securityToken string, consumerKey string, consumerSecret string) *Salesforce {
	auth := loginPassword(domain, username, password, securityToken, consumerKey, consumerSecret)
	return &Salesforce{auth: auth}
}

func (sf *Salesforce) QueryUnstructured(query string) *QueryResponse {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return nil
	}
	query = url.QueryEscape(query)
	endpoint := sf.auth.InstanceUrl + "/services/data/" + API_VERSION + "/query/?q=" + query
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
		fmt.Println("Error authenticating")
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

	queryResponse := &QueryResponse{}
	jsonError := json.Unmarshal(respBody, &queryResponse)
	if jsonError != nil {
		fmt.Println("Error decoding response")
		fmt.Println(jsonError.Error())
		return nil
	}

	defer resp.Body.Close()
	return queryResponse
}

func (sf *Salesforce) Insert(sObject SObject) {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return
	}
	json, err := json.Marshal(sObject.Fields)
	body := strings.NewReader(string(json))
	if err != nil {
		fmt.Println("Error converting object to json")
		fmt.Println(err.Error())
		return
	}
	endpoint := sf.auth.InstanceUrl + "/services/data/" + API_VERSION + "/sobjects/" + sObject.SObjectName
	req, err := http.NewRequest("POST", endpoint, body)
	if err != nil {
		fmt.Println("Error creating request")
		fmt.Println(err.Error())
		return
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+sf.auth.AccessToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Error with DML operation")
		fmt.Println(err.Error())
		return
	}
	if resp.StatusCode != http.StatusCreated {
		fmt.Println("Error with " + resp.Request.Method + " " + endpoint)
		fmt.Println(resp.Status)
		return
	}
}
