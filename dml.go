package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/mitchellh/mapstructure"
)

type sObjectCollection struct {
	AllOrNone bool             `json:"allOrNone"`
	Records   []map[string]any `json:"records"`
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

func processSalesforceResponse(resp http.Response) ([]SalesforceResult, error) {
	results := []SalesforceResult{}
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	jsonError := json.Unmarshal(responseData, &results)
	if jsonError != nil {
		return nil, jsonError
	}

	return results, nil
}

func doBatchedRequestsForCollection(auth authentication, method string, url string, batchSize int, recordMap []map[string]any) (*SalesforceResults, error) {
	var results = SalesforceResults{}

	for len(recordMap) > 0 {
		var batch, remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}
		recordMap = remaining

		payload := sObjectCollection{
			AllOrNone: false,
			Records:   batch,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			return &results, err
		}

		resp, err := doRequest(method, url, jsonType, auth, string(body))
		if err != nil {
			return &results, err
		}
		currentResults, err := processSalesforceResponse(*resp)
		if err != nil {
			return &results, err
		}

		results.Results = append(results.Results, currentResults...)
	}

	for _, result := range results.Results {
		if !result.Success {
			results.HasErrors = true
			return &results, nil
		}
	}

	return &results, nil
}

func decodeResponseBody(response *http.Response) (value SalesforceResult, err error) {
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&value)
	return value, err
}

func doInsertOne(auth authentication, sObjectName string, record any) (*SalesforceResult, error) {
	recordMap, err := convertToMap(record)
	if err != nil {
		return nil, err
	}
	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return nil, err
	}

	resp, err := doRequest(http.MethodPost, "/sobjects/"+sObjectName, jsonType, auth, string(body))
	if err != nil {
		return nil, err
	}

	data, err := decodeResponseBody(resp)
	if err != nil {
		fmt.Println("Error decoding: ", err)
		return nil, err
	}

	return &data, nil
}

func doUpdateOne(auth authentication, sObjectName string, record any) error {
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

	_, err = doRequest(http.MethodPatch, "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, string(body))
	if err != nil {
		return err
	}

	return nil
}

func doUpsertOne(auth authentication, sObjectName string, fieldName string, record any) (*SalesforceResult, error) {
	recordMap, err := convertToMap(record)
	if err != nil {
		return nil, err
	}

	externalIdValue, ok := recordMap[fieldName].(string)
	if !ok || externalIdValue == "" {
		return nil, fmt.Errorf("salesforce externalId: %s not found in %s data. make sure to append custom fields with '__c'", fieldName, sObjectName)
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")
	delete(recordMap, fieldName)

	body, err := json.Marshal(recordMap)
	if err != nil {
		return nil, err
	}

	resp, err := doRequest(http.MethodPatch, "/sobjects/"+sObjectName+"/"+fieldName+"/"+externalIdValue, jsonType, auth, string(body))
	if err != nil {
		return nil, err
	}

	data, err := decodeResponseBody(resp)
	if err != nil {
		fmt.Println("Error decoding: ", err)
		return nil, err
	}

	return &data, nil
}

func doDeleteOne(auth authentication, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	_, err = doRequest(http.MethodDelete, "/sobjects/"+sObjectName+"/"+recordId, jsonType, auth, "")
	if err != nil {
		return err
	}

	return nil
}

func doInsertCollection(auth authentication, sObjectName string, records any, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}
	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	return doBatchedRequestsForCollection(auth, http.MethodPost, "/composite/sobjects/", batchSize, recordMap)
}

func doUpdateCollection(auth authentication, sObjectName string, records any, batchSize int) (*SalesforceResults, error) {
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

	return doBatchedRequestsForCollection(auth, http.MethodPatch, "/composite/sobjects/", batchSize, recordMap)
}

func doUpsertCollection(auth authentication, sObjectName string, fieldName string, records any, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}
	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return nil, fmt.Errorf("salesforce externalId: %s not found in %s data. make sure to append custom fields with '__c'", fieldName, sObjectName)
		}
	}

	uri := "/composite/sobjects/" + sObjectName + "/" + fieldName
	return doBatchedRequestsForCollection(auth, http.MethodPatch, uri, batchSize, recordMap)

}

func doDeleteCollection(auth authentication, sObjectName string, records any, batchSize int) (*SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return nil, err
	}

	// we want to verify that ids are present before we start deleting
	batchedIds := []string{}
	for len(recordMap) > 0 {
		var batch, remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}
		recordMap = remaining

		var ids string
		for i := range batch {
			batch[i]["attributes"] = map[string]string{"type": sObjectName}
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
		batchedIds = append(batchedIds, ids)
	}

	var results = SalesforceResults{}

	for i := range batchedIds {
		resp, err := doRequest(http.MethodDelete, "/composite/sobjects/?ids="+batchedIds[i]+"&allOrNone=false", jsonType, auth, "")
		if err != nil {
			return &results, err
		}
		currentResults, err := processSalesforceResponse(*resp)
		if err != nil {
			return &results, err
		}

		results.Results = append(results.Results, currentResults...)
	}

	for _, result := range results.Results {
		if !result.Success {
			results.HasErrors = true
			break
		}
	}

	return &results, nil
}
