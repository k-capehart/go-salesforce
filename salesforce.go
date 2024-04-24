package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/forcedotcom/go-soql"
)

type Salesforce struct {
	auth *auth
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
	apiVersion       = "v60.0"
	jsonType         = "application/json"
	csvType          = "text/csv"
	batchSizeMax     = 200
	bulkBatchSizeMax = 10000
)

func doRequest(method string, uri string, content string, auth auth, body string) (*http.Response, error) {
	var reader *strings.Reader
	var req *http.Request
	var err error
	endpoint := auth.InstanceUrl + "/services/data/" + apiVersion + uri

	if body != "" {
		reader = strings.NewReader(body)
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

func validateBatchSizeWithinRange(batchSize int, max int) error {
	if batchSize < 1 || batchSize > max {
		return errors.New("batch size = " + strconv.Itoa(batchSize) + " but must be 1 <= batchSize <= " + strconv.Itoa(max))
	}
	return nil
}

func validateSingles(sf Salesforce, record any) error {
	authErr := validateAuth(sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStruct(record)
	if typErr != nil {
		return typErr
	}
	return nil
}

func validateCollections(sf Salesforce, records any, batchSize int) error {
	authErr := validateAuth(sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}
	batchSizeErr := validateBatchSizeWithinRange(batchSize, batchSizeMax)
	if batchSizeErr != nil {
		return batchSizeErr
	}
	return nil
}

func validateBulk(sf Salesforce, records any, batchSize int) error {
	authErr := validateAuth(sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}
	batchSizeErr := validateBatchSizeWithinRange(batchSize, bulkBatchSizeMax)
	if batchSizeErr != nil {
		return batchSizeErr
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
	var auth *auth
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
	validationErr := validateSingles(*sf, soqlStruct)
	if validationErr != nil {
		return validationErr
	}

	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}
	queryErr := performQuery(*sf.auth, soqlQuery, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) InsertOne(sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doInsertOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpdateOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpsertOne(sObjectName string, externalIdFieldName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpsertOne(*sf.auth, sObjectName, externalIdFieldName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doDeleteOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) InsertCollection(sObjectName string, records any, batchSize int) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doInsertCollection(*sf.auth, sObjectName, records, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpdateCollection(sObjectName string, records any, batchSize int) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpdateCollection(*sf.auth, sObjectName, records, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, batchSize int) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpsertCollection(*sf.auth, sObjectName, externalIdFieldName, records, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) DeleteCollection(sObjectName string, records any, batchSize int) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doDeleteCollection(*sf.auth, sObjectName, records, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) InsertComposite(sObjectName string, records any, batchSize int, allOrNone bool) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doInsertComposite(*sf.auth, sObjectName, records, allOrNone, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpdateComposite(sObjectName string, records any, batchSize int, allOrNone bool) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpdateComposite(*sf.auth, sObjectName, records, allOrNone, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) UpsertComposite(sObjectName string, externalIdFieldName string, records any, batchSize int, allOrNone bool) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doUpsertComposite(*sf.auth, sObjectName, externalIdFieldName, records, allOrNone, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) DeleteComposite(sObjectName string, records any, batchSize int, allOrNone bool) error {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return validationErr
	}

	dmlErr := doDeleteComposite(*sf.auth, sObjectName, records, allOrNone, batchSize)
	if dmlErr != nil {
		return dmlErr
	}

	return nil
}

func (sf *Salesforce) QueryBulkExport(filePath string, query string) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	queryErr := doQueryBulk(*sf.auth, filePath, query)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryStructBulkExport(filePath string, soqlStruct any) error {
	validationErr := validateSingles(*sf, soqlStruct)
	if validationErr != nil {
		return validationErr
	}

	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}
	queryErr := doQueryBulk(*sf.auth, filePath, soqlQuery)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) InsertBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doInsertBulk(*sf.auth, sObjectName, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) InsertBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return []string{}, authErr
	}

	jobIds, bulkErr := doInsertBulkFile(*sf.auth, sObjectName, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doUpdateBulk(*sf.auth, sObjectName, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return []string{}, authErr
	}

	jobIds, bulkErr := doUpdateBulkFile(*sf.auth, sObjectName, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulk(sObjectName string, externalIdFieldName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doUpsertBulk(*sf.auth, sObjectName, externalIdFieldName, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulkFile(sObjectName string, externalIdFieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return []string{}, authErr
	}

	jobIds, bulkErr := doUpsertBulkFile(*sf.auth, sObjectName, externalIdFieldName, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doDeleteBulk(*sf.auth, sObjectName, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return []string{}, authErr
	}

	jobIds, bulkErr := doDeleteBulkFile(*sf.auth, sObjectName, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) GetJobResults(bulkJobId string) (BulkJobResults, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return BulkJobResults{}, authErr
	}

	job, err := getJobResults(*sf.auth, ingestJobType, bulkJobId)
	if err != nil {
		return BulkJobResults{}, err
	}

	return job, nil
}
