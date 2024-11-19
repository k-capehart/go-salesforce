package salesforce

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type note struct {
	Description   string   `json:"description"`
	Fields        []string `json:"fields"`
	TableEnumOrId string   `json:"tableEnumOrId"`
}

type ExplainPlain struct {
	Cardinality          int64    `json:"cardinality"`
	Fields               []string `json:"fields"`
	LeadingOperationType string   `json:"leadingOperationType"`
	Notes                []note   `json:"notes"`
	RelativeCost         float64  `json:"relativeCost"`
	SObjectCardinality   int64
	SObjectType          string `json:"sobjectType"`
}

func performExplain(auth *authentication, query string) ([]ExplainPlain, error) {
	query = url.QueryEscape(query)

	explainURL := "/query/?explain=" + query
	resp, err := doRequest(auth, requestPayload{
		method:  http.MethodGet,
		uri:     explainURL,
		content: jsonType,
	})
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(resp.Body)
	var explainResp = struct {
		Plans []ExplainPlain `json:"plans"`
	}{}
	if queryResponseError := dec.Decode(&explainResp); queryResponseError != nil {
		return nil, queryResponseError
	}

	return explainResp.Plans, nil
}
