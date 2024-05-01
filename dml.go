package salesforce

import (
	"encoding/json"
	"errors"
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

func doBatchedRequestsForCollection(auth auth, method string, url string, batchSize int, recordMap []map[string]any) error {
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

func doInsertOne(auth auth, sObjectName string, record any) error {
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

func doUpdateOne(auth auth, sObjectName string, record any) error {
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

func doUpsertOne(auth auth, sObjectName string, fieldName string, record any) error {
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

func doDeleteOne(auth auth, sObjectName string, record any) error {
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

func doInsertCollection(auth auth, sObjectName string, records any, batchSize int) error {
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

func doUpdateCollection(auth auth, sObjectName string, records any, batchSize int) error {
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

func doUpsertCollection(auth auth, sObjectName string, fieldName string, records any, batchSize int) error {
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

func doDeleteCollection(auth auth, sObjectName string, records any, batchSize int) error {
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
