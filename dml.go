package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-viper/mapstructure/v2"
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

func doBatchedRequestsForCollection(
	sf *Salesforce,
	method string,
	url string,
	batchSize int,
	recordMap []map[string]any,
) (SalesforceResults, error) {
	results := []SalesforceResult{}

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
			return SalesforceResults{Results: results}, err
		}

		resp, err := doRequest(sf.auth, sf.config, requestPayload{
			method:   method,
			uri:      url,
			content:  jsonType,
			body:     string(body),
			compress: sf.config.compressionHeaders,
		})
		if err != nil {
			return SalesforceResults{Results: results}, err
		}
		currentResults, err := processSalesforceResponse(*resp)
		if err != nil {
			return SalesforceResults{Results: results}, err
		}

		results = append(results, currentResults...)
	}

	for _, result := range results {
		if !result.Success {
			return SalesforceResults{Results: results, HasSalesforceErrors: true}, nil
		}
	}

	return SalesforceResults{Results: results}, nil
}

func decodeResponseBody(response *http.Response) (value SalesforceResult, err error) {
	defer response.Body.Close()
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&value)
	return value, err
}

func doInsertOne(sf *Salesforce, sObjectName string, record any) (SalesforceResult, error) {
	recordMap, err := convertToMap(record)
	if err != nil {
		return SalesforceResult{}, err
	}
	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")

	body, err := json.Marshal(recordMap)
	if err != nil {
		return SalesforceResult{}, err
	}

	resp, err := doRequest(sf.auth, sf.config, requestPayload{
		method:   http.MethodPost,
		uri:      "/sobjects/" + sObjectName,
		content:  jsonType,
		body:     string(body),
		compress: sf.config.compressionHeaders,
	})
	if err != nil {
		return SalesforceResult{}, err
	}

	data, err := decodeResponseBody(resp)
	if err != nil {
		fmt.Println("Error decoding: ", err)
		return SalesforceResult{}, err
	}

	return data, nil
}

func doUpdateOne(sf *Salesforce, sObjectName string, record any) error {
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

	_, err = doRequest(sf.auth, sf.config, requestPayload{
		method:   http.MethodPatch,
		uri:      "/sobjects/" + sObjectName + "/" + recordId,
		content:  jsonType,
		body:     string(body),
		compress: sf.config.compressionHeaders,
	})
	if err != nil {
		return err
	}

	return nil
}

func doUpsertOne(
	sf *Salesforce,
	sObjectName string,
	fieldName string,
	record any,
) (SalesforceResult, error) {
	recordMap, err := convertToMap(record)
	if err != nil {
		return SalesforceResult{}, err
	}

	externalIdValue, ok := recordMap[fieldName].(string)
	if !ok || externalIdValue == "" {
		return SalesforceResult{}, fmt.Errorf(
			"salesforce externalId: %s not found in %s data. make sure to append custom fields with '__c'",
			fieldName,
			sObjectName,
		)
	}

	recordMap["attributes"] = map[string]string{"type": sObjectName}
	delete(recordMap, "Id")
	delete(recordMap, fieldName)

	body, err := json.Marshal(recordMap)
	if err != nil {
		return SalesforceResult{}, err
	}

	resp, err := doRequest(sf.auth, sf.config, requestPayload{
		method:   http.MethodPatch,
		uri:      "/sobjects/" + sObjectName + "/" + fieldName + "/" + externalIdValue,
		content:  jsonType,
		body:     string(body),
		compress: sf.config.compressionHeaders,
	})
	if err != nil {
		return SalesforceResult{}, err
	}

	data, err := decodeResponseBody(resp)
	if err != nil {
		fmt.Println("Error decoding: ", err)
		return SalesforceResult{}, err
	}

	return data, nil
}

func doDeleteOne(sf *Salesforce, sObjectName string, record any) error {
	recordMap, err := convertToMap(record)
	if err != nil {
		return err
	}

	recordId, ok := recordMap["Id"].(string)
	if !ok || recordId == "" {
		return errors.New("salesforce id not found in object data")
	}

	_, err = doRequest(sf.auth, sf.config, requestPayload{
		method:   http.MethodDelete,
		uri:      "/sobjects/" + sObjectName + "/" + recordId,
		content:  jsonType,
		compress: sf.config.compressionHeaders,
	})
	if err != nil {
		return err
	}

	return nil
}

func doInsertCollection(
	sf *Salesforce,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return SalesforceResults{}, err
	}
	for i := range recordMap {
		delete(recordMap[i], "Id")
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
	}

	return doBatchedRequestsForCollection(
		sf,
		http.MethodPost,
		"/composite/sobjects/",
		batchSize,
		recordMap,
	)
}

func doUpdateCollection(
	sf *Salesforce,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return SalesforceResults{}, err
	}
	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		recordId, ok := recordMap[i]["Id"].(string)
		if !ok || recordId == "" {
			return SalesforceResults{}, errors.New("salesforce id not found in object data")
		}
	}

	return doBatchedRequestsForCollection(
		sf,
		http.MethodPatch,
		"/composite/sobjects/",
		batchSize,
		recordMap,
	)
}

func doUpsertCollection(
	sf *Salesforce,
	sObjectName string,
	fieldName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return SalesforceResults{}, err
	}
	for i := range recordMap {
		recordMap[i]["attributes"] = map[string]string{"type": sObjectName}
		externalIdValue, ok := recordMap[i][fieldName].(string)
		if !ok || externalIdValue == "" {
			return SalesforceResults{}, fmt.Errorf(
				"salesforce externalId: %s not found in %s data. make sure to append custom fields with '__c'",
				fieldName,
				sObjectName,
			)
		}
	}

	uri := "/composite/sobjects/" + sObjectName + "/" + fieldName
	return doBatchedRequestsForCollection(sf, http.MethodPatch, uri, batchSize, recordMap)
}

func doDeleteCollection(
	sf *Salesforce,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return SalesforceResults{}, err
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
				return SalesforceResults{}, errors.New("salesforce id not found in object data")
			}
			if i == len(batch)-1 {
				ids = ids + recordId
			} else {
				ids = ids + recordId + ","
			}
		}
		batchedIds = append(batchedIds, ids)
	}

	results := []SalesforceResult{}

	for i := range batchedIds {
		resp, err := doRequest(sf.auth, sf.config, requestPayload{
			method:   http.MethodDelete,
			uri:      "/composite/sobjects/?ids=" + batchedIds[i] + "&allOrNone=false",
			content:  jsonType,
			compress: sf.config.compressionHeaders,
		})
		if err != nil {
			return SalesforceResults{Results: results}, err
		}
		currentResults, err := processSalesforceResponse(*resp)
		if err != nil {
			return SalesforceResults{Results: results}, err
		}

		results = append(results, currentResults...)
	}

	for _, result := range results {
		if !result.Success {
			return SalesforceResults{Results: results, HasSalesforceErrors: true}, nil
		}
	}

	return SalesforceResults{Results: results}, nil
}
