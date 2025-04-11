package salesforce

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"

	"github.com/forcedotcom/go-soql"
)

type Salesforce struct {
	auth   *authentication
	Config Configuration
}

type SalesforceErrorMessage struct {
	Message    string   `json:"message"`
	StatusCode string   `json:"statusCode"`
	Fields     []string `json:"fields"`
	ErrorCode  string   `json:"errorCode"`
}

type SalesforceResult struct {
	Id      string                   `json:"id"`
	Errors  []SalesforceErrorMessage `json:"errors"`
	Success bool                     `json:"success"`
}

type SalesforceResults struct {
	Results             []SalesforceResult
	HasSalesforceErrors bool
}

const (
	apiVersion            = "v62.0"
	jsonType              = "application/json"
	csvType               = "text/csv"
	batchSizeMax          = 200
	bulkBatchSizeMax      = 10000
	invalidSessionIdError = "INVALID_SESSION_ID"
)

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

func validateBulk(sf Salesforce, records any, batchSize int, isFile bool, sObjectName string, assignmentRuleId string) error {
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
	if assignmentRuleId != "" {
		sAssignError := validateObjectWithAssignmentRuleId(sObjectName)
		if sAssignError != nil {
			return sAssignError
		}
	}
	return nil
}

func validateObjectWithAssignmentRuleId(sObjectName string) error {
	if !slices.Contains([]string{"Lead", "Case"}, sObjectName) {
		return fmt.Errorf("InsertBulkAssign doesn't support this type of object: %s", sObjectName)
	}
	return nil
}

func Init(creds Creds) (*Salesforce, error) {
	var auth *authentication
	var err error
	if creds == (Creds{}) {
		return nil, errors.New("creds is empty")
	}
	if creds.Domain != "" && creds.ConsumerKey != "" && creds.ConsumerSecret != "" &&
		creds.Username != "" && creds.Password != "" && creds.SecurityToken != "" {
		auth, err = usernamePasswordFlow(
			creds.Domain,
			creds.Username,
			creds.Password,
			creds.SecurityToken,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	} else if creds.Domain != "" && creds.ConsumerKey != "" && creds.ConsumerSecret != "" {
		auth, err = clientCredentialsFlow(
			creds.Domain,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
	} else if creds.AccessToken != "" {
		auth, err = setAccessToken(
			creds.Domain,
			creds.AccessToken,
		)
	} else if creds.Domain != "" && creds.Username != "" &&
		creds.ConsumerKey != "" && creds.ConsumerRSAPem != "" {
		auth, err = jwtFlow(
			creds.Domain,
			creds.Username,
			creds.ConsumerKey,
			creds.ConsumerRSAPem,
			JwtExpirationTime,
		)
	}

	if err != nil {
		return nil, err
	} else if auth == nil || auth.AccessToken == "" {
		return nil, errors.New("unknown authentication error")
	}
	auth.creds = creds
	config := Configuration{}
	config.SetDefaults()
	return &Salesforce{auth: auth, Config: config}, nil
}

func (sf *Salesforce) DoRequest(method string, uri string, body []byte) (*http.Response, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return nil, authErr
	}

	resp, err := doRequest(sf.auth, requestPayload{
		method:   method,
		uri:      uri,
		content:  jsonType,
		body:     string(body),
		compress: sf.Config.CompressionHeaders,
	})
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

	queryErr := performQuery(sf, query, sObject)
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
	queryErr := performQuery(sf, soqlQuery, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) InsertOne(sObjectName string, record any) (SalesforceResult, error) {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return SalesforceResult{}, validationErr
	}

	return doInsertOne(sf, sObjectName, record)
}

func (sf *Salesforce) UpdateOne(sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	return doUpdateOne(sf, sObjectName, record)
}

func (sf *Salesforce) UpsertOne(sObjectName string, externalIdFieldName string, record any) (SalesforceResult, error) {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return SalesforceResult{}, validationErr
	}

	return doUpsertOne(sf, sObjectName, externalIdFieldName, record)
}

func (sf *Salesforce) DeleteOne(sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	return doDeleteOne(sf, sObjectName, record)
}

func (sf *Salesforce) InsertCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doInsertCollection(sf, sObjectName, records, batchSize)
}

func (sf *Salesforce) UpdateCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doUpdateCollection(sf, sObjectName, records, batchSize)
}

func (sf *Salesforce) UpsertCollection(sObjectName string, externalIdFieldName string, records any, batchSize int) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doUpsertCollection(sf, sObjectName, externalIdFieldName, records, batchSize)
}

func (sf *Salesforce) DeleteCollection(sObjectName string, records any, batchSize int) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doDeleteCollection(sf, sObjectName, records, batchSize)
}

func (sf *Salesforce) InsertComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doInsertComposite(sf, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) UpdateComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doUpdateComposite(sf, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) UpsertComposite(sObjectName string, externalIdFieldName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doUpsertComposite(sf, sObjectName, externalIdFieldName, records, allOrNone, batchSize)
}

func (sf *Salesforce) DeleteComposite(sObjectName string, records any, batchSize int, allOrNone bool) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return doDeleteComposite(sf, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) QueryBulkExport(query string, filePath string) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	queryErr := doQueryBulk(sf, filePath, query)
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
	queryErr := doQueryBulk(sf, filePath, soqlQuery)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryBulkIterator(query string) (IteratorJob, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return nil, authErr
	}
	queryJobReq := bulkQueryJobCreationRequest{
		Operation: queryJobType,
		Query:     query,
	}
	body, jsonErr := json.Marshal(queryJobReq)
	if jsonErr != nil {
		return nil, jsonErr
	}

	job, jobCreationErr := createBulkJob(sf, queryJobType, body)
	if jobCreationErr != nil {
		return nil, jobCreationErr
	}
	if job.Id == "" {
		newErr := errors.New("error creating bulk query job")
		return nil, newErr
	}
	return newBulkJobQueryIterator(sf, job.Id)
}

func (sf *Salesforce) InsertBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return sf.InsertBulkAssign(sObjectName, records, batchSize, waitForResults, "")
}

func (sf *Salesforce) InsertBulkAssign(sObjectName string, records any, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(sf, sObjectName, "", insertOperation, records, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) InsertBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return sf.InsertBulkFileAssign(sObjectName, filePath, batchSize, waitForResults, "")
}

func (sf *Salesforce) InsertBulkFileAssign(sObjectName string, filePath string, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(sf, sObjectName, "", insertOperation, filePath, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return sf.UpdateBulkAssign(sObjectName, records, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpdateBulkAssign(sObjectName string, records any, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(sf, sObjectName, "", updateOperation, records, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return sf.UpdateBulkFileAssign(sObjectName, filePath, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpdateBulkFileAssign(sObjectName string, filePath string, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(sf, sObjectName, "", updateOperation, filePath, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulk(sObjectName string, externalIdFieldName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return sf.UpsertBulkAssign(sObjectName, externalIdFieldName, records, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpsertBulkAssign(sObjectName string, externalIdFieldName string, records any, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(sf, sObjectName, externalIdFieldName, upsertOperation, records, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulkFile(sObjectName string, externalIdFieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return sf.UpsertBulkFileAssign(sObjectName, externalIdFieldName, filePath, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpsertBulkFileAssign(sObjectName string, externalIdFieldName string, filePath string, batchSize int, waitForResults bool, assignmentRuleId string) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(sf, sObjectName, externalIdFieldName, upsertOperation, filePath, batchSize, waitForResults, assignmentRuleId)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulk(sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, "")
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJob(sf, sObjectName, "", deleteOperation, records, batchSize, waitForResults, "")
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulkFile(sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, "")
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := doBulkJobWithFile(sf, sObjectName, "", deleteOperation, filePath, batchSize, waitForResults, "")
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

	job, err := getJobResults(sf, ingestJobType, bulkJobId)
	if err != nil {
		return BulkJobResults{}, err
	}

	if job.State == jobStateJobComplete {
		job, err = getJobRecordResults(sf, job)
		if err != nil {
			return job, err
		}
	}

	return job, nil
}

func (sf *Salesforce) GetAccessToken() string {
	if sf.auth == nil {
		return ""
	}
	return sf.auth.AccessToken
}

func (sf *Salesforce) GetInstanceUrl() string {
	if sf.auth == nil {
		return ""
	}
	return sf.auth.InstanceUrl
}
