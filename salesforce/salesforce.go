package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
)

type Salesforce struct {
	auth *Auth
}

type salesforceErrorMessage struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode"`
	Fields     []string `json:"fields"`
}

type salesforceError struct {
	Id      string                   `json:"id"`
	Errors  []salesforceErrorMessage `json:"errors"`
	Success bool                     `json:"success"`
}

const (
	apiVersion = "v60.0"
	jsonType   = "application/json"
	csvType    = "text/csv"
)

func doRequest(method string, uri string, content string, auth Auth, body string) (*http.Response, error) {
	var reader *strings.Reader
	var req *http.Request
	var err error
	endpoint := auth.InstanceUrl + "/services/data/" + apiVersion + uri

	if body != "" {
		reader = strings.NewReader(string(body))
		req, err = http.NewRequest(method, endpoint, reader)
	} else {
		req, err = http.NewRequest(method, endpoint, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", content)
	req.Header.Set("Accept", content)
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)

	return http.DefaultClient.Do(req)
}

func validateOfTypeSlice(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "slice" {
		return errors.New("expected a slice")
	}
	return nil
}

func validateOfTypeStruct(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "struct" {
		return errors.New("expected a custom struct type")
	}
	return nil
}

func processSalesforceError(resp http.Response) error {
	var errorMessage string
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	errorMessage = string(resp.Status) + ": " + string(responseData)
	if resp.StatusCode == http.StatusNotFound {
		errorMessage = errorMessage + ".\nis there a typo in the request?"
	}

	return errors.New(errorMessage)
}

func processSalesforceResponse(resp http.Response) error {
	sfErrors := []salesforceError{}
	var errorResponse error

	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	jsonError := json.Unmarshal(responseData, &sfErrors)
	if jsonError != nil {
		return jsonError
	}

	for _, sfError := range sfErrors {
		if !sfError.Success {
			for _, errorMessage := range sfError.Errors {
				newError := errorMessage.StatusCode + ": " + errorMessage.Message + " " + sfError.Id
				errorResponse = errors.Join(errorResponse, errors.New(newError))
			}
		}
	}
	return errorResponse
}

func Init(creds Creds) (*Salesforce, error) {
	var auth *Auth
	var err error
	if creds != (Creds{}) &&
		creds.Domain != "" && creds.Username != "" &&
		creds.Password != "" && creds.SecurityToken != "" &&
		creds.ConsumerKey != "" && creds.ConsumerSecret != "" {

		auth, err = loginPassword(
			creds.Domain,
			creds.Username,
			creds.Password,
			creds.SecurityToken,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	}

	if err != nil || auth == nil {
		return nil, errors.New("please refer to salesforce REST API developer guide for proper authentication: https://help.salesforce.com/s/articleView?id=sf.remoteaccess_oauth_flows.htm&type=5")
	}
	return &Salesforce{auth: auth}, nil
}

func (sf *Salesforce) DoRequest(method string, uri string, body []byte) (*http.Response, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return nil, authErr
	}

	resp, err := doRequest(method, uri, jsonType, *sf.auth, string(body))
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (sf *Salesforce) Query(query string, sObject any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	queryErr := performQuery(*sf.auth, query, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryStruct(soqlStruct any, sObject any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	queryErr := marshalQueryStruct(*sf.auth, soqlStruct, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) InsertOne(sObjectName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStruct(record)
	if typErr != nil {
		return typErr
	}

	dmlErr := doInsertOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStruct(record)
	if typErr != nil {
		return typErr
	}

	dmlErr := doUpdateOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpsertOne(sObjectName string, fieldName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStruct(record)
	if typErr != nil {
		return typErr
	}

	dmlErr := doUpsertOne(*sf.auth, sObjectName, fieldName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStruct(record)
	if typErr != nil {
		return typErr
	}

	dmlErr := doDeleteOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) InsertCollection(sObjectName string, records any, allOrNonePerBatch bool, batchSize int) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}
	if batchSize < 1 || batchSize > 200 {
		return errors.New("batch size must be: 1 <= n <= 200")
	}

	dmlErr := doInsertCollection(*sf.auth, sObjectName, records, allOrNonePerBatch, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpdateCollection(sObjectName string, records any, allOrNonePerBatch bool, batchSize int) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	dmlErr := doUpdateCollection(*sf.auth, sObjectName, records, allOrNonePerBatch, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, allOrNonePerBatch bool, batchSize int) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	dmlErr := doUpsertCollection(*sf.auth, sObjectName, externalIdFieldName, records, allOrNonePerBatch, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) DeleteCollection(sObjectName string, records any, allOrNonePerBatch bool, batchSize int) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	dmlErr := doDeleteCollection(*sf.auth, sObjectName, records, allOrNonePerBatch, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) InsertBulk(sObjectName string, records any, waitForResults bool) (string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return "", authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return "", typErr
	}

	jobId, bulkErr := doInsertBulk(*sf.auth, sObjectName, records, waitForResults)
	if bulkErr != nil {
		return "", bulkErr
	}

	return jobId, nil
}

func (sf *Salesforce) UpdateBulk(sObjectName string, records any, waitForResults bool) (string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return "", authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return "", typErr
	}

	jobId, bulkErr := doUpdateBulk(*sf.auth, sObjectName, records, waitForResults)
	if bulkErr != nil {
		return "", bulkErr
	}

	return jobId, nil
}

func (sf *Salesforce) UpsertBulk(sObjectName string, externalIdFieldName string, records any, waitForResults bool) (string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return "", authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return "", typErr
	}

	jobId, bulkErr := doUpsertBulk(*sf.auth, sObjectName, externalIdFieldName, records, waitForResults)
	if bulkErr != nil {
		return "", bulkErr
	}

	return jobId, nil
}

func (sf *Salesforce) DeleteBulk(sObjectName string, records any, waitForResults bool) (string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return "", authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return "", typErr
	}

	jobId, bulkErr := doDeleteBulk(*sf.auth, sObjectName, records, waitForResults)
	if bulkErr != nil {
		return "", bulkErr
	}

	return jobId, nil
}

func (sf *Salesforce) GetJobResults(bulkJobId string) (BulkJobResults, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return BulkJobResults{}, authErr
	}

	job, err := getJobResults(*sf.auth, bulkJobId)
	if err != nil {
		return BulkJobResults{}, err
	}

	return job, nil
}
