package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/forcedotcom/go-soql"
	"github.com/mitchellh/mapstructure"
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
		return nil, err
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)

	return http.DefaultClient.Do(req)
}

func convertToMap(obj any) (map[string]any, error) {
	var recordMap map[string]any
	if _, ok := obj.(map[string]any); ok {
		recordMap = obj.(map[string]any)
	} else {
		err := mapstructure.Decode(obj, &recordMap)
		if err != nil {
			return nil, err
		}
	}
	return recordMap, nil
}

func convertToSliceOfMaps(obj any) ([]map[string]any, error) {
	var recordMap []map[string]any
	if _, ok := obj.(map[string]any); ok {
		recordMap = obj.([]map[string]any)
	} else {
		err := mapstructure.Decode(obj, &recordMap)
		if err != nil {
			return nil, err
		}
	}
	return recordMap, nil
}

func processSalesforceError(resp http.Response) error {
	var errorMessage string
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		errorMessage = "error processing failed http call"
	} else {
		errorMessage = string(resp.Status) + ": " + string(responseData)
	}
	return errors.New(errorMessage)
}

func processSalesforceResponse(resp http.Response) error {
	responseData, err := io.ReadAll(resp.Body)
	responseMap := []map[string]any{}
	salesforceErrors := ""
	if err != nil {
		return errors.New("error processing http response")
	}
	jsonError := json.Unmarshal(responseData, &responseMap)
	if jsonError != nil {
		return errors.New("error processing http response")
	}
	for _, val := range responseMap {
		if !val["success"].(bool) {
			salesforceErrors = salesforceErrors + "{id: " + val["id"].(string) + " message: " + val["errors"].([]any)[0].(map[string]any)["message"].(string) + "} "
		}
	}
	if salesforceErrors != "" {
		strings.TrimSpace(salesforceErrors)
		return errors.New("salesforce errors: " + salesforceErrors)
	}
	return nil
}

func Init(creds Creds) (*Salesforce, error) {
	var auth *Auth
	var err error
	if creds != (Creds{}) &&
		creds.Domain != "" && creds.Username != "" &&
		creds.Password != "" && creds.SecurityToken != "" &&
		creds.ConsumerKey != "" && creds.ConsumerSecret != "" {

		auth, err = loginPassword(
			creds.Domain,
			creds.Username,
			creds.Password,
			creds.SecurityToken,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	}

	if err != nil || auth == nil {
		return nil, errors.New("please refer to salesforce REST API developer guide for proper authentication: https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5")
	}
	return &Salesforce{auth: auth}, nil
}

func (sf *Salesforce) Query(query string, sObject any) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}
	query = url.QueryEscape(query)
	resp, err := doRequest("GET", "/query/?q="+query, *sf.auth, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	queryResponse := &QueryResponse{}
	queryResponseError := json.Unmarshal(respBody, &queryResponse)
	if queryResponseError != nil {
		return err
	}

	sObjectError := mapstructure.Decode(queryResponse.Records, sObject)
	if sObjectError != nil {
		return err
	}

	return nil
}

func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error {
	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}
	sf.Query(soqlQuery, sObject)
	return nil
}

func (sf *Salesforce) InsertOne(sObjectName string, record any) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}
	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("POST", "/sobjects/"+sObjectName, *sf.auth, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}
	recordId := recordMap["Id"].(string)
	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObjectName+"/"+recordId, *sf.auth, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}
	recordId := recordMap["Id"].(string)

	resp, err := doRequest("DELETE", "/sobjects/"+sObjectName+"/"+recordId, *sf.auth, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) InsertCollection(sObjectName string, records any, allOrNone bool) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	payload := map[string]any{
		"allOrNone": allOrNone,
		"records":   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("POST", "/composite/sobjects/", *sf.auth, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}
	return nil
}

func (sf *Salesforce) UpdateCollection(sObjectName string, records any, allOrNone bool) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	payload := map[string]any{
		"allOrNone": allOrNone,
		"records":   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/composite/sobjects/", *sf.auth, body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}
	return nil
}

func (sf *Salesforce) DeleteCollection(sObjectName string, records any, allOrNone bool) error {
	if sf.auth == nil {
		return errors.New("not authenticated: please use salesforce.Init()")
	}

	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	if len(recordMap) > 200 {
		return errors.New("salesforce composite api call supports up to 200 records at once")
	}

	var ids string
	for i := 0; i < len(recordMap); i++ {
		if i == len(recordMap)-1 {
			ids = ids + recordMap[i]["Id"].(string)
		} else {
			ids = ids + recordMap[i]["Id"].(string) + ","
		}
	}

	resp, err := doRequest("DELETE", "/composite/sobjects/?ids="+ids+"&allOrNone="+strconv.FormatBool(allOrNone), *sf.auth, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processSalesforceResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}
	return nil
}
