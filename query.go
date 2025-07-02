package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

type queryResponse struct {
	TotalSize      int              `json:"totalSize"`
	Done           bool             `json:"done"`
	NextRecordsUrl string           `json:"nextRecordsUrl"`
	Records        []map[string]any `json:"records"`
}

func performQuery(sf *Salesforce, query string, sObject any) error {
	query = url.QueryEscape(query)
	queryResp := &queryResponse{
		Done:           false,
		NextRecordsUrl: "/query/?q=" + query,
	}

	for !queryResp.Done {
		resp, err := doRequest(sf.auth, sf.config, requestPayload{
			method:   http.MethodGet,
			uri:      queryResp.NextRecordsUrl,
			content:  jsonType,
			compress: sf.config.compressionHeaders,
		})
		if err != nil {
			return err
		}

		respBody, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return readErr
		}

		tempQueryResp := &queryResponse{}
		queryResponseError := json.Unmarshal(respBody, &tempQueryResp)
		if queryResponseError != nil {
			return queryResponseError
		}

		queryResp.TotalSize = queryResp.TotalSize + tempQueryResp.TotalSize
		queryResp.Records = append(queryResp.Records, tempQueryResp.Records...)
		queryResp.Done = tempQueryResp.Done
		if !tempQueryResp.Done && tempQueryResp.NextRecordsUrl != "" {
			queryResp.NextRecordsUrl = strings.TrimPrefix(
				tempQueryResp.NextRecordsUrl,
				"/services/data/"+apiVersion,
			)
		}
	}

	sObjectError := mapstructure.Decode(queryResp.Records, sObject)
	if sObjectError != nil {
		return sObjectError
	}

	return nil
}
