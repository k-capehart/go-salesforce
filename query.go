package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/forcedotcom/go-soql"
	"github.com/mitchellh/mapstructure"
)

type queryResponse struct {
	TotalSize      int              `json:"totalSize"`
	Done           bool             `json:"done"`
	NextRecordsUrl string           `json:"nextRecordsUrl"`
	Records        []map[string]any `json:"records"`
}

func performQuery(auth auth, query string, sObject any) error {
	query = url.QueryEscape(query)
	queryResp := &queryResponse{
		Done:           false,
		NextRecordsUrl: "/query/?q=" + query,
	}

	for !queryResp.Done {
		resp, err := doRequest(http.MethodGet, queryResp.NextRecordsUrl, jsonType, auth, "")
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK {
			return processSalesforceError(*resp)
		}

		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}

		tempQueryResp := &queryResponse{}
		queryResponseError := json.Unmarshal(respBody, &tempQueryResp)
		if queryResponseError != nil {
			return err
		}

		queryResp.TotalSize = queryResp.TotalSize + tempQueryResp.TotalSize
		queryResp.Records = append(queryResp.Records, tempQueryResp.Records...)
		queryResp.Done = tempQueryResp.Done
		if !tempQueryResp.Done && tempQueryResp.NextRecordsUrl != "" {
			queryResp.NextRecordsUrl = strings.TrimPrefix(tempQueryResp.NextRecordsUrl, "/services/data/"+apiVersion)
		}
	}

	sObjectError := mapstructure.Decode(queryResp.Records, sObject)
	if sObjectError != nil {
		return sObjectError
	}

	return nil
}

func marshalQueryStruct(auth auth, soqlStruct any, sObject any) error {
	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}

	performQuery(auth, soqlQuery, sObject)

	return nil
}
