package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/forcedotcom/go-soql"
)

type Salesforce struct {
	auth *authentication
}

type SalesforceErrorMessage struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode"`
	Fields     []string `json:"fields"`
}

type SalesforceResult struct {
	Id        string                   `json:"id"`
	Errors    []SalesforceErrorMessage `json:"errors"`
	Success   bool                     `json:"success"`
	HasErrors bool                     `json:"hasErrors"`
}

const (
	apiVersion       = "v60.0"
	jsonType         = "application/json"
	csvType          = "text/csv"
	batchSizeMax     = 200
	bulkBatchSizeMax = 10000
)

func doRequest(method string, uri string, content string, auth authentication, body string, expectedStatus int) (*http.Response, error) {
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

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	if expectedStatus != 0 && resp.StatusCode != expectedStatus {
		return resp, processSalesforceError(*resp)
	}

	return resp, nil
}

func validateOfTypeSlice(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "slice" {
		return errors.New("expected a slice, got: " + t)
	}
	return nil
}

func validateOfTypeStructOrMap(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "struct" && t != "map" {
		return errors.New("expected a struct or map type, got: " + t)
	}
	return nil
}

func validateOfTypeStruct(data any) error {
	t := reflect.TypeOf(data).Kind().String()
	if t != "struct" {
		return errors.New("expected a go-soql struct, got: " + t)
	}
	return nil
}

func validateBatchSizeWithinRange(batchSize int, max int) error {
	if batchSize < 1 || batchSize > max {
		return errors.New("batch size = " + strconv.Itoa(batchSize) + " but must be 1 <= batchSize <= " + strconv.Itoa(max))
	}
	return nil
}

func validateGoSoql(sf Salesforce, record any) error {
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

func validateSingles(sf Salesforce, record any) error {
	authErr := validateAuth(sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeStructOrMap(record)
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

func validateBulk(sf Salesforce, records any, batchSize int, isFile bool) error {
	authErr := validateAuth(sf)
	if authErr != nil {
		return authErr
	}
	if !isFile {
		typErr := validateOfTypeSlice(records)
		if typErr != nil {
			return typErr
		}
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

func processSalesforceResponse(resp http.Response) (sfErrors []SalesforceResult, err error) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fmt.Println("Response Data: ", string(responseData))
	jsonError := json.Unmarshal(responseData, &sfErrors)
	if jsonError != nil {
		return nil, jsonError
	}

	return sfErrors, nil
}

func Init(creds Creds) (*Salesforce, error) {
	var auth *authentication
	var err error
	if creds != (Creds{}) && creds.Domain != "" && creds.ConsumerKey != "" && creds.ConsumerSecret != "" &&
		creds.Username != "" && creds.Password != "" && creds.SecurityToken != "" {

		auth, err = usernamePasswordFlow(
			creds.Domain,
			creds.Username,
			creds.Password,
			creds.SecurityToken,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	} else if creds != (Creds{}) && creds.Domain != "" && creds.ConsumerKey != "" && creds.ConsumerSecret != "" {
		auth, err = clientCredentialsFlow(
			creds.Domain,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	} else if creds != (Creds{}) && creds.AccessToken != "" {
		auth, err = setAccessToken(
			creds.Domain,
			creds.AccessToken,
		)
	}

	if err != nil {
		return nil, err
	} else if auth == nil || auth.AccessToken == "" {
		return nil, errors.New("unknown authentication error")
	}
	return &Salesforce{auth: auth}, nil
}

func (sf *Salesforce) DoRequest(method string, uri string, body []byte) (*http.Response, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return nil, authErr
	}

	resp, err := doRequest(method, uri, jsonType, *sf.auth, string(body), 0)
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
	validationErr := validateGoSoql(*sf, soqlStruct)
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

func (sf *Salesforce) InsertOne(sObjectName string, record any) (*SalesforceResult, error) {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return nil, validationErr
	}

	rec, dmlErr := doInsertOne(*sf.auth, sObjectName, record)
	if dmlErr != nil {
		return nil, dmlErr
	}

	return rec, nil
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

func (sf *Salesforce) InsertCollection(sObjectName string, records any, batchSize int) ([]SalesforceResult, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return nil, validationErr
	}

	salesforceError, dmlErr := doInsertCollection(*sf.auth, sObjectName, records, batchSize)
	return salesforceError, dmlErr
}

func (sf *Salesforce) UpdateCollection(sObjectName string, records any, batchSize int) ([]SalesforceResult, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return nil, validationErr
	}

	salesforceError, dmlErr := doUpdateCollection(*sf.auth, sObjectName, records, batchSize)
	return salesforceError, dmlErr
}

func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, batchSize int) ([]SalesforceResult, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return nil, validationErr
	}

	salesforceError, dmlErr := doUpsertCollection(*sf.auth, sObjectName, externalIdFieldName, records, batchSize)
	return salesforceError, dmlErr

}

func (sf *Salesforce) DeleteCollection(sObjectName string, records any, batchSize int) ([]SalesforceResult, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return nil, validationErr
	}

	salesforceError, dmlErr := doDeleteCollection(*sf.auth, sObjectName, records, batchSize)
	return salesforceError, dmlErr

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

func (sf *Salesforce) QueryBulkExport(query string, filePath string) error {
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

func (sf *Salesforce) QueryStructBulkExport(soqlStruct any, filePath string) error {
	validationErr := validateGoSoql(*sf, soqlStruct)
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
	validationErr := validateBulk(*sf, records, batchSize, false)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(*sf.auth, sObjectName, "", insertOperation, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) InsertBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(*sf.auth, sObjectName, "", insertOperation, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(*sf.auth, sObjectName, "", updateOperation, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(*sf.auth, sObjectName, "", updateOperation, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulk(sObjectName string, externalIdFieldName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(*sf.auth, sObjectName, externalIdFieldName, upsertOperation, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulkFile(sObjectName string, externalIdFieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(*sf.auth, sObjectName, externalIdFieldName, upsertOperation, filePath, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(*sf.auth, sObjectName, "", deleteOperation, records, batchSize, waitForResults)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(*sf.auth, sObjectName, "", deleteOperation, filePath, batchSize, waitForResults)
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
