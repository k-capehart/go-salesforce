package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/forcedotcom/go-soql"
	"github.com/mitchellh/mapstructure"
)

type Salesforce struct {
	auth *Auth
}

type QueryResponse struct {
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Records   []map[string]any `json:"records"`
}

type SObjectCollection struct {
	AllOrNone string           `json:"allOrNone"`
	Records   []map[string]any `json:"records"`
}

const (
	ApiVersion = "v60.0"
	JSONType   = "application/json"
	CSVType    = "text/csv"
)

func doRequest(method string, uri string, content string, auth Auth, body string) (*http.Response, error) {
	var reader *strings.Reader
	var req *http.Request
	var err error
	endpoint := auth.InstanceUrl + "/services/data/" + ApiVersion + uri

	if body != "" {
		reader = strings.NewReader(string(body))
		req, err = http.NewRequest(method, endpoint, reader)
	} else {
		req, err = http.NewRequest(method, endpoint, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", content)
	req.Header.Set("Accept", content)
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
			return nil, errors.New("issue decoding salesforce object, need a key value pair (custom struct or map)")
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
			return nil, errors.New("issue decoding salesforce object, need a key value pair (custom struct or map)")
		}
	}
	return recordMap, nil
}

func validateOfTypeSlice(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "slice" {
		return errors.New("expected a slice of salesforce objects")
	}
	return nil
}

func processSalesforceError(resp http.Response) error {
	var errorMessage string
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		errorMessage = "error processing failed http call"
	} else {
		errorMessage = string(resp.Status) + ": " + string(responseData)
		if resp.StatusCode == http.StatusNotFound {
			errorMessage = errorMessage + ".\nis there a typo in the request?"
		}
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
			if id, ok := val["id"]; ok {
				salesforceErrors = salesforceErrors + "{id: " + id.(string) + " message: " + val["errors"].([]any)[0].(map[string]any)["message"].(string) + "} "
			} else {
				salesforceErrors = salesforceErrors + "{message: " + val["errors"].([]any)[0].(map[string]any)["message"].(string) + "} "
			}
		}
	}
	if salesforceErrors != "" {
		salesforceErrors = strings.TrimSpace(salesforceErrors)
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
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	query = url.QueryEscape(query)
	resp, err := doRequest("GET", "/query/?q="+query, JSONType, *sf.auth, "")
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
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
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

	resp, err := doRequest("POST", "/sobjects/"+sObjectName, JSONType, *sf.auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObjectName+"/"+recordId, JSONType, *sf.auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) UpsertOne(sObjectName string, fieldName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	externalIdValue, ok := recordMap[fieldName].(string)
	if !ok || externalIdValue == "" {
		return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")
	delete(recordMap, fieldName)

	body, err := json.Marshal(recordMap)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/sobjects/"+sObjectName+"/"+fieldName+"/"+externalIdValue, JSONType, *sf.auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	resp, err := doRequest("DELETE", "/sobjects/"+sObjectName+"/"+recordId, JSONType, *sf.auth, "")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}
	return nil
}

func (sf *Salesforce) InsertCollection(sObjectName string, records any, allOrNone bool) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
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

	payload := SObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("POST", "/composite/sobjects/", JSONType, *sf.auth, string(body))
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
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
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
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
	}

	payload := SObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/composite/sobjects/", JSONType, *sf.auth, string(body))
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

func (sf *Salesforce) UpsertCollection(sObjectName string, fieldName string, records any, allOrNone bool) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
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
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
		}
	}

	payload := SObjectCollection{
		AllOrNone: strconv.FormatBool(allOrNone),
		Records:   recordMap,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	resp, err := doRequest("PATCH", "/composite/sobjects/"+sObjectName+"/"+fieldName, JSONType, *sf.auth, string(body))
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
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
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
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
		if i == len(recordMap)-1 {
			ids = ids + recordId
		} else {
			ids = ids + recordId + ","
		}
	}

	resp, err := doRequest("DELETE", "/composite/sobjects/?ids="+ids+"&allOrNone="+strconv.FormatBool(allOrNone), JSONType, *sf.auth, "")
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
