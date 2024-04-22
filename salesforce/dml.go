package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/mitchellh/mapstructure"
)

type sObjectCollection struct {
	AllOrNone bool             `json:"allOrNone"`
	Records   []map[string]any `json:"records"`
}

type compositeRequest struct {
	AllOrNone        bool                  `json:"allOrNone"`
	CompositeRequest []compositeSubRequest `json:"compositeRequest"`
}

type compositeSubRequest struct {
	Body        any    `json:"body"`
	Method      string `json:"method"`
	Url         string `json:"url"`
	ReferenceId string `json:"referenceId"`
}

type compositeRequestResult struct {
	CompositeResponse []composteSubRequestResult `json:"compositeResponse"`
}

type composteSubRequestResult struct {
	Body           []salesforceError `json:"body"`
	HttpHeaders    map[string]string `json:"httpHeaders"`
	HttpStatusCode int               `json:"httpStatusCode"`
	ReferenceId    string            `json:"referenceId"`
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

func doCompositeRequest(auth Auth, compReq compositeRequest) error {
	body, jsonErr := json.Marshal(compReq)
	if jsonErr != nil {
		return jsonErr
	}
	resp, httpErr := doRequest(http.MethodPost, "/composite", jsonType, auth, string(body))
	if httpErr != nil {
		return httpErr
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	salesforceErrors := processCompositeResponse(*resp)
	if salesforceErrors != nil {
		return salesforceErrors
	}
	return nil
}

func doBatchedRequestsForCollection(auth Auth, method string, url string, batchSize int, recordMap []map[string]any) error {
	var dmlErrors error

	for len(recordMap) > 0 {
		var batch, remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}

		payload := sObjectCollection{
			AllOrNone: false,
			Records:   batch,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			dmlErrors = errors.Join(dmlErrors, err)
		}

		resp, err := doRequest(method, url, jsonType, auth, string(body))
		if err != nil {
			dmlErrors = errors.Join(dmlErrors, err)
		}

		if resp.StatusCode != http.StatusOK {
			dmlErrors = errors.Join(dmlErrors, processSalesforceError(*resp))
		}
		salesforceErrors := processSalesforceResponse(*resp)
		if salesforceErrors != nil {
			dmlErrors = errors.Join(dmlErrors, salesforceErrors)
		}
		recordMap = remaining
	}

	return dmlErrors
}

func createCompositeRequestForCollection(method string, url string, allOrNone bool, batchSize int, recordMap []map[string]any) (compositeRequest, error) {
	var subReqs []compositeSubRequest
	batchNumber := 0

	if len(recordMap)/batchSize > 25 {
		errorMessage := "compsite requests cannot have more than 25 subrequests. number of subrequests = (number of records) / (batch size)"
		return compositeRequest{}, errors.New(errorMessage)
	}

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

func processCompositeResponse(resp http.Response) error {
	compositeResults := compositeRequestResult{}
	var errorResponse error

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	jsonError := json.Unmarshal(responseData, &compositeResults)
	if jsonError != nil {
		return jsonError
	}

	for _, subResult := range compositeResults.CompositeResponse {
		for _, sfError := range subResult.Body {
			if !sfError.Success {
				if len(sfError.Errors) > 0 {
					for _, errorMessage := range sfError.Errors {
						newError := errorMessage.StatusCode + ": " + errorMessage.Message + " " + sfError.Id
						errorResponse = errors.Join(errorResponse, errors.New(newError))
					}
				} else {
					newError := "an unknown error occurred: " + strconv.Itoa(subResult.HttpStatusCode)
					errorResponse = errors.Join(errorResponse, errors.New(newError))
				}
			}
		}
	}

	return errorResponse
}

func doInsertOne(auth Auth, sObjectName string, record any) error {
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

	resp, err := doRequest(http.MethodPost, "/sobjects/"+sObjectName, jsonType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		return processSalesforceError(*resp)
	}

	return nil
}

func doUpdateOne(auth Auth, sObjectName string, record any) error {
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

	resp, err := doRequest(http.MethodPatch, "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}

	return nil
}

func doUpsertOne(auth Auth, sObjectName string, fieldName string, record any) error {
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

	resp, err := doRequest(http.MethodPatch, "/sobjects/"+sObjectName+"/"+fieldName+"/"+externalIdValue, jsonType, auth, string(body))
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}

	return nil
}

func doDeleteOne(auth Auth, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	resp, err := doRequest(http.MethodDelete, "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, "")
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusNoContent {
		return processSalesforceError(*resp)
	}

	return nil
}

func doInsertCollection(auth Auth, sObjectName string, records any, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}
	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	return doBatchedRequestsForCollection(auth, http.MethodPost, "/composite/sobjects/", batchSize, recordMap)
}

func doUpdateCollection(auth Auth, sObjectName string, records any, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}
	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
	}

	return doBatchedRequestsForCollection(auth, http.MethodPatch, "/composite/sobjects/", batchSize, recordMap)
}

func doUpsertCollection(auth Auth, sObjectName string, fieldName string, records any, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}
	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
		}
	}

	uri := "/composite/sobjects/" + sObjectName + "/" + fieldName
	return doBatchedRequestsForCollection(auth, http.MethodPatch, uri, batchSize, recordMap)

}

func doDeleteCollection(auth Auth, sObjectName string, records any, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	var dmlErrors error

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
				return errors.New("salesforce id not found in object data")
			}
			if i == len(batch)-1 {
				ids = ids + recordId
			} else {
				ids = ids + recordId + ","
			}
		}

		resp, err := doRequest(http.MethodDelete, "/composite/sobjects/?ids="+ids+"&allOrNone=false", jsonType, auth, "")
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

		recordMap = remaining
	}

	return dmlErrors
}

func doInsertComposite(auth Auth, sObjectName string, records any, allOrNone bool, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects"
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPost, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return compositeErr
	}
	compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return compositeReqErr
	}

	return nil
}

func doUpdateComposite(auth Auth, sObjectName string, records any, allOrNone bool, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return errors.New("salesforce id not found in object data")
		}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects"
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPatch, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return compositeErr
	}
	compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return compositeReqErr
	}

	return nil
}

func doUpsertComposite(auth Auth, sObjectName string, fieldName string, records any, allOrNone bool, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
	}

	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return errors.New("salesforce externalId: " + fieldName + " not found in " + sObjectName + " data. make sure to append custom fields with '__c'")
		}
	}

	uri := "/services/data/" + apiVersion + "/composite/sobjects/" + sObjectName + "/" + fieldName
	compReq, compositeErr := createCompositeRequestForCollection(http.MethodPatch, uri, allOrNone, batchSize, recordMap)
	if compositeErr != nil {
		return compositeErr
	}
	compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return compositeReqErr
	}

	return nil
}

func doDeleteComposite(auth Auth, sObjectName string, records any, allOrNone bool, batchSize int) error {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return err
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
				return errors.New("salesforce id not found in object data")
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
	compositeReqErr := doCompositeRequest(auth, compReq)
	if compositeReqErr != nil {
		return compositeReqErr
	}

	return nil
}
