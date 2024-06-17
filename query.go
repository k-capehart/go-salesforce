package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mitchellh/mapstructure"
)

type queryResponse struct {
	TotalSize      int              `json:"totalSize"`
	Done           bool             `json:"done"`
	NextRecordsUrl string           `json:"nextRecordsUrl"`
	Records        []map[string]any `json:"records"`
}

func performQuery(auth authentication, query string, sObject any) error {
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

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}

		tempQueryResp := &queryResponse{}
		if err = json.Unmarshal(respBody, &tempQueryResp); err != nil {
			return err
		}

		queryResp.TotalSize = queryResp.TotalSize + tempQueryResp.TotalSize
		queryResp.Records = append(queryResp.Records, tempQueryResp.Records...)
		queryResp.Done = tempQueryResp.Done
		if !tempQueryResp.Done && tempQueryResp.NextRecordsUrl != "" {
			queryResp.NextRecordsUrl = strings.TrimPrefix(tempQueryResp.NextRecordsUrl, "/services/data/"+apiVersion)
		}
	}

	if err := mapstructure.Decode(queryResp.Records, sObject); err != nil {
		return err
	}

	return nil
}
