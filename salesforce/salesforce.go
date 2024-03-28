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
	Fields      map[string]any
}

func doRequest(method string, uri string, auth Auth, body []byte) (*http.Response, error) {
	var reader *strings.Reader
	var req *http.Request
	var err error
	endpoint := auth.InstanceUrl + "/services/data/" + API_VERSION + uri

	if body != nil {
		reader = strings.NewReader(string(body))
		req, err = http.NewRequest(method, endpoint, reader)
	} else {
		req, err = http.NewRequest(method, endpoint, nil)
	}
	if err != nil {
		fmt.Println("Error creating request")
		fmt.Println(err.Error())
		return nil, err
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)

	return http.DefaultClient.Do(req)
}

func Init(creds Creds) *Salesforce {
	var auth *Auth
	if creds.Domain != "" {
		auth = loginPassword(
			creds.Domain,
			creds.Username,
			creds.Password,
			creds.SecurityToken,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	}

	if auth == nil {
		panic("Please refer to Salesforce REST API Developer Guide for proper authentication: https://developer.salesforce.com/docs/atlas.en-us.chatterapi.meta/chatterapi/intro_using_oauth.htm")
	}
	return &Salesforce{auth: auth}
}

func (sf *Salesforce) QueryUnstructured(query string) []SObject {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return nil
	}
	query = url.QueryEscape(query)
	resp, err := doRequest("GET", "/query/?q="+query, *sf.auth, nil)
	if err != nil {
		fmt.Println("Error authenticating")
		fmt.Println(err.Error())
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		fmt.Println("Error with " + resp.Request.Method + " " + "/query/?q=" + query)
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

	var results []SObject
	for _, record := range queryResponse.Records {
		newsObject := SObject{
			SObjectName: record["attributes"].(map[string]any)["type"].(string),
			Fields:      record,
		}
		results = append(results, newsObject)
	}

	defer resp.Body.Close()
	return results
}

func (sf *Salesforce) Insert(sObject SObject) {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return
	}
	json, err := json.Marshal(sObject.Fields)
	if err != nil {
		fmt.Println("Error converting object to json")
		fmt.Println(err.Error())
		return
	}

	resp, err := doRequest("POST", "/sobjects/"+sObject.SObjectName, *sf.auth, json)
	if err != nil {
		fmt.Println("Error with DML operation")
		fmt.Println(err.Error())
		return
	}
	if resp.StatusCode != http.StatusCreated {
		fmt.Println("Error with " + resp.Request.Method + " " + "/sobjects/" + sObject.SObjectName)
		fmt.Println(resp.Status)
		return
	}
}

func (sf *Salesforce) Update(sObject SObject) {
	if sf.auth == nil {
		fmt.Println("Not authenticated. Please use salesforce.Init().")
		return
	}

	recordId := sObject.Fields["Id"].(string)
	delete(sObject.Fields, "Id")

	json, err := json.Marshal(sObject.Fields)
	if err != nil {
		fmt.Println("Error converting object to json")
		fmt.Println(err.Error())
		return
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObject.SObjectName+"/"+recordId, *sf.auth, json)
	if err != nil {
		fmt.Println("Error with DML operation")
		fmt.Println(err.Error())
		return
	}
	if resp.StatusCode != http.StatusNoContent {
		fmt.Println("Error with " + resp.Request.Method + " " + "/sobjects/" + sObject.SObjectName)
		fmt.Println(resp.Status)
		return
	}
}
