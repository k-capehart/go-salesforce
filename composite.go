package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
)

type compositeRequest struct {
	AllOrNone        bool                  `json:"allOrNone"`
	CompositeRequest []compositeSubRequest `json:"compositeRequest"`
}

type compositeSubRequest struct {
	Body        sObjectCollection `json:"body"`
	Method      string            `json:"method"`
	Url         string            `json:"url"`
	ReferenceId string            `json:"referenceId"`
}

type compositeRequestResult struct {
	CompositeResponse []compositeSubRequestResult `json:"compositeResponse"`
}

type compositeSubRequestResult struct {
	Body           []SalesforceResult `json:"body"`
	HttpHeaders    map[string]string  `json:"httpHeaders"`
	HttpStatusCode int                `json:"httpStatusCode"`
	ReferenceId    string             `json:"referenceId"`
}

func doCompositeRequest(auth authentication, compReq compositeRequest) (*SalesforceResults, error) {
	body, jsonErr := json.Marshal(compReq)
	if jsonErr != nil {
		return nil, jsonErr
	}
	resp, httpErr := doRequest(http.MethodPost, "/composite", jsonType, auth, string(body))
	if httpErr != nil {
		return nil, httpErr
	}
	results, salesforceErrors := processCompositeResponse(*resp, compReq.AllOrNone)
	if salesforceErrors != nil {
		return nil, salesforceErrors
	}
	return results, nil
}

func validateNumberOfSubrequests(dataSize int, batchSize int) error {
	numberOfBatches := int(math.Ceil(float64(float64(dataSize) / float64(batchSize))))
	if numberOfBatches > 25 {
		errorMessage := strconv.Itoa(numberOfBatches) + " subrequests exceed max of 25. max records = 25 * (batch size)"
		return errors.New(errorMessage)
	}
	return nil
}

func createCompositeRequestForCollection(method string, url string, allOrNone bool, batchSize int, recordMap []map[string]any) (compositeRequest, error) {
	validateErr := validateNumberOfSubrequests(len(recordMap), batchSize)
	if validateErr != nil {
		return compositeRequest{}, validateErr
	}

	var subReqs []compositeSubRequest
	batchNumber := 0

	for len(recordMap) > 0 {
		var batch, remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}

		payload := sObjectCollection{
			AllOrNone: allOrNone,
			Records:   batch,
		}

		subReq := compositeSubRequest{
			Body:        payload,
			Method:      method,
			Url:         url,
			ReferenceId: "refObj" + strconv.Itoa(batchNumber),
		}
		subReqs = append(subReqs, subReq)
		recordMap = remaining
		batchNumber++
	}

	return compositeRequest{
		AllOrNone:        allOrNone,
		CompositeRequest: subReqs,
	}, nil
}

func processCompositeResponse(resp http.Response, allOrNone bool) (*SalesforceResults, error) {
	compositeResults := compositeRequestResult{}
	results := SalesforceResults{}

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	jsonError := json.Unmarshal(responseData, &compositeResults)
	if jsonError != nil {
		return nil, jsonError
	}

	for _, subResult := range compositeResults.CompositeResponse {
		for _, result := range subResult.Body {
			if !result.Success {
				results.HasErrors = true
			}
		}
		results.Results = append(results.Results, subResult.Body...)
	}

	if results.HasErrors && allOrNone {
		fmt.Println("Records rolled back because not all records were valid and the request was using AllOrNone header")
	}

	return &results, nil
}

func doInsertComposite(auth authentication, sObjectName string, records any, allOrNone bool, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}

	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects"
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPost, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return nil, compositeErr
	}
	results, compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return nil, compositeReqErr
	}

	return results, nil
}

func doUpdateComposite(auth authentication, sObjectName string, records any, allOrNone bool, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return nil, errors.New("salesforce id not found in object data")
		}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects"
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPatch, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return nil, compositeErr
	}
	results, compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return nil, compositeReqErr
	}

	return results, nil
}

func doUpsertComposite(auth authentication, sObjectName string, fieldName string, records any, allOrNone bool, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return nil, errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
		}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects/" + sObjectName + "/" + fieldName
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPatch, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return nil, compositeErr
	}
	results, compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return nil, compositeReqErr
	}

	return results, nil
}

func doDeleteComposite(auth authentication, sObjectName string, records any, allOrNone bool, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}

	var subReqs []compositeSubRequest
	batchNumber := 0

	for len(recordMap) > 0 {
		var batch, remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}

		var ids string
		for i := 0; i < len(batch); i++ {
			recordId, ok := batch[i]["Id"].(string)
			if !ok || recordId == "" {
				return nil, errors.New("salesforce id not found in object data")
			}
			if i == len(batch)-1 {
				ids = ids + recordId
			} else {
				ids = ids + recordId + ","
			}
		}

		uri := "/services/data/" + apiVersion + "/composite/sobjects/?ids=" + ids + "&allOrNone=" + strconv.FormatBool(allOrNone)
		subReq := compositeSubRequest{
			Method:      http.MethodDelete,
			Url:         uri,
			ReferenceId: "refObj" + strconv.Itoa(batchNumber),
		}
		subReqs = append(subReqs, subReq)
		recordMap = remaining
		batchNumber++
	}

	compReq := compositeRequest{
		AllOrNone:        allOrNone,
		CompositeRequest: subReqs,
	}
	results, compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return nil, compositeReqErr
	}

	return results, nil
}
