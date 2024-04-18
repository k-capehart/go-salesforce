package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"

	"github.com/forcedotcom/go-soql"
	"github.com/mitchellh/mapstructure"
)

type queryResponse struct {
	TotalSize int              `json:"totalSize"`
	Done      bool             `json:"done"`
	Records   []map[string]any `json:"records"`
}

func performQuery(auth Auth, query string, sObject any) error {
	query = url.QueryEscape(query)
	resp, err := doRequest(http.MethodGet, "/query/?q="+query, jsonType, auth, "")
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

	queryResponse := &queryResponse{}
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

func marshalQueryStruct(auth Auth, soqlStruct any, sObject any) error {
	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}

	performQuery(auth, soqlQuery, sObject)

	return nil
}
