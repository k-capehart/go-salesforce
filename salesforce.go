package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"time"

	"github.com/forcedotcom/go-soql"
)

type Salesforce struct {
	auth     *authentication
	config   *configuration
	AuthFlow AuthFlowType
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
	apiVersion                    = "v63.0"
	jsonType                      = "application/json"
	csvType                       = "text/csv"
	batchSizeMax                  = 200
	bulkBatchSizeMax              = 10000
	invalidSessionIdError         = "INVALID_SESSION_ID"
	httpDefaultMaxIdleConnections = 10
	httpDefaultIdleConnTimeout    = time.Duration(30 * time.Second)
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
		return errors.New(
			"batch size = " + strconv.Itoa(
				batchSize,
			) + " but must be 1 <= batchSize <= " + strconv.Itoa(
				max,
			),
		)
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
	batchSizeErr := validateBatchSizeWithinRange(batchSize, sf.config.batchSizeMax)
	if batchSizeErr != nil {
		return batchSizeErr
	}
	return nil
}

func validateBulk(
	sf Salesforce,
	records any,
	batchSize int,
	isFile bool,
	sObjectName string,
	assignmentRuleId string,
) error {
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
	batchSizeErr := validateBatchSizeWithinRange(batchSize, sf.config.bulkBatchSizeMax)
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

func Init(creds Creds, options ...Option) (*Salesforce, error) {
	var auth *authentication
	var err error
	var authFlow AuthFlowType

	// Initialize configuration with defaults
	config := &configuration{}
	config.setDefaults()

	// Apply functional options
	for _, option := range options {
		if err := option(config); err != nil {
			return nil, fmt.Errorf("configuration error: %w", err)
		}
	}

	config.configureHttpClient()

	if creds == (Creds{}) {
		return nil, errors.New("creds is empty")
	}

	// Determine authentication flow and authenticate
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
		authFlow = AuthFlowUsernamePassword
	} else if creds.Domain != "" && creds.ConsumerKey != "" && creds.ConsumerSecret != "" {
		auth, err = clientCredentialsFlow(
			creds.Domain,
			creds.ConsumerKey,
			creds.ConsumerSecret,
		)
		authFlow = AuthFlowClientCredentials
	} else if creds.AccessToken != "" {
		auth, err = config.getAccessTokenAuthentication(
			context.Background(),
			creds.Domain,
			creds.AccessToken,
		)
		authFlow = AuthFlowAccessToken
	} else if creds.Domain != "" && creds.Username != "" &&
		creds.ConsumerKey != "" && creds.ConsumerRSAPem != "" {
		auth, err = jwtFlow(
			creds.Domain,
			creds.Username,
			creds.ConsumerKey,
			creds.ConsumerRSAPem,
			JwtExpirationTime,
		)
		authFlow = AuthFlowJWT
	}

	if err != nil {
		return nil, err
	} else if auth == nil || auth.AccessToken == "" {
		return nil, errors.New("unknown authentication error")
	}
	auth.creds = creds

	return &Salesforce{
		auth:     auth,
		config:   config,
		AuthFlow: authFlow,
	}, nil
}

func (sf *Salesforce) DoRequest(
	ctx context.Context,
	method string,
	uri string,
	body []byte,
	opts ...RequestOption,
) (*http.Response, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return nil, authErr
	}

	resp, err := doRequest(ctx, sf.auth, sf.config, requestPayload{
		method:   method,
		uri:      uri,
		content:  jsonType,
		body:     string(body),
		options:  opts,
		compress: sf.config.compressionHeaders,
	})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (sf *Salesforce) Query(ctx context.Context, query string, sObject any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}

	queryErr := sf.performQuery(ctx, query, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryStruct(ctx context.Context, soqlStruct any, sObject any) error {
	validationErr := validateGoSoql(*sf, soqlStruct)
	if validationErr != nil {
		return validationErr
	}

	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}
	queryErr := sf.performQuery(ctx, soqlQuery, sObject)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) InsertOne(
	ctx context.Context,
	sObjectName string,
	record any,
) (SalesforceResult, error) {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return SalesforceResult{}, validationErr
	}

	return sf.doInsertOne(ctx, sObjectName, record)
}

func (sf *Salesforce) UpdateOne(ctx context.Context, sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	return sf.doUpdateOne(ctx, sObjectName, record)
}

func (sf *Salesforce) UpsertOne(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	record any,
) (SalesforceResult, error) {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return SalesforceResult{}, validationErr
	}

	return sf.doUpsertOne(ctx, sObjectName, externalIdFieldName, record)
}

func (sf *Salesforce) DeleteOne(ctx context.Context, sObjectName string, record any) error {
	validationErr := validateSingles(*sf, record)
	if validationErr != nil {
		return validationErr
	}

	return sf.doDeleteOne(ctx, sObjectName, record)
}

func (sf *Salesforce) InsertCollection(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doInsertCollection(ctx, sObjectName, records, batchSize)
}

func (sf *Salesforce) UpdateCollection(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doUpdateCollection(ctx, sObjectName, records, batchSize)
}

func (sf *Salesforce) UpsertCollection(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doUpsertCollection(ctx, sObjectName, externalIdFieldName, records, batchSize)
}

func (sf *Salesforce) DeleteCollection(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doDeleteCollection(ctx, sObjectName, records, batchSize)
}

func (sf *Salesforce) InsertComposite(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	allOrNone bool,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doInsertComposite(ctx, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) UpdateComposite(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	allOrNone bool,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doUpdateComposite(ctx, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) UpsertComposite(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	records any,
	batchSize int,
	allOrNone bool,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doUpsertComposite(
		ctx,
		sObjectName,
		externalIdFieldName,
		records,
		allOrNone,
		batchSize,
	)
}

func (sf *Salesforce) DeleteComposite(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	allOrNone bool,
) (SalesforceResults, error) {
	validationErr := validateCollections(*sf, records, batchSize)
	if validationErr != nil {
		return SalesforceResults{}, validationErr
	}

	return sf.doDeleteComposite(ctx, sObjectName, records, allOrNone, batchSize)
}

func (sf *Salesforce) QueryBulkExport(ctx context.Context, query string, filePath string) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	queryErr := sf.doQueryBulk(ctx, filePath, query)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryStructBulkExport(
	ctx context.Context,
	soqlStruct any,
	filePath string,
) error {
	validationErr := validateGoSoql(*sf, soqlStruct)
	if validationErr != nil {
		return validationErr
	}

	soqlQuery, err := soql.Marshal(soqlStruct)
	if err != nil {
		return err
	}
	queryErr := sf.doQueryBulk(ctx, filePath, soqlQuery)
	if queryErr != nil {
		return queryErr
	}

	return nil
}

func (sf *Salesforce) QueryBulkIterator(ctx context.Context, query string) (IteratorJob, error) {
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

	job, jobCreationErr := sf.createBulkJob(ctx, queryJobType, body)
	if jobCreationErr != nil {
		return nil, jobCreationErr
	}
	if job.Id == "" {
		newErr := errors.New("error creating bulk query job")
		return nil, newErr
	}
	return sf.newBulkJobQueryIterator(ctx, job.Id)
}

func (sf *Salesforce) InsertBulk(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.InsertBulkAssign(ctx, sObjectName, records, batchSize, waitForResults, "")
}

func (sf *Salesforce) InsertBulkAssign(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJob(
		ctx,
		sObjectName,
		"",
		insertOperation,
		records,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) InsertBulkFile(
	ctx context.Context,
	sObjectName string,
	filePath string,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.InsertBulkFileAssign(ctx, sObjectName, filePath, batchSize, waitForResults, "")
}

func (sf *Salesforce) InsertBulkFileAssign(
	ctx context.Context,
	sObjectName string,
	filePath string,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJobWithFile(
		ctx,
		sObjectName,
		"",
		insertOperation,
		filePath,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulk(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.UpdateBulkAssign(ctx, sObjectName, records, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpdateBulkAssign(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJob(
		ctx,
		sObjectName,
		"",
		updateOperation,
		records,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpdateBulkFile(
	ctx context.Context,
	sObjectName string,
	filePath string,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.UpdateBulkFileAssign(ctx, sObjectName, filePath, batchSize, waitForResults, "")
}

func (sf *Salesforce) UpdateBulkFileAssign(
	ctx context.Context,
	sObjectName string,
	filePath string,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJobWithFile(
		ctx,
		sObjectName,
		"",
		updateOperation,
		filePath,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulk(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	records any,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.UpsertBulkAssign(
		ctx,
		sObjectName,
		externalIdFieldName,
		records,
		batchSize,
		waitForResults,
		"",
	)
}

func (sf *Salesforce) UpsertBulkAssign(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	records any,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJob(
		ctx,
		sObjectName,
		externalIdFieldName,
		upsertOperation,
		records,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) UpsertBulkFile(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	filePath string,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	return sf.UpsertBulkFileAssign(
		ctx,
		sObjectName,
		externalIdFieldName,
		filePath,
		batchSize,
		waitForResults,
		"",
	)
}

func (sf *Salesforce) UpsertBulkFileAssign(
	ctx context.Context,
	sObjectName string,
	externalIdFieldName string,
	filePath string,
	batchSize int,
	waitForResults bool,
	assignmentRuleId string,
) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, assignmentRuleId)
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJobWithFile(
		ctx,
		sObjectName,
		externalIdFieldName,
		upsertOperation,
		filePath,
		batchSize,
		waitForResults,
		assignmentRuleId,
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulk(
	ctx context.Context,
	sObjectName string,
	records any,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	validationErr := validateBulk(*sf, records, batchSize, false, sObjectName, "")
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJob(
		ctx,
		sObjectName,
		"",
		deleteOperation,
		records,
		batchSize,
		waitForResults,
		"",
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) DeleteBulkFile(
	ctx context.Context,
	sObjectName string,
	filePath string,
	batchSize int,
	waitForResults bool,
) ([]string, error) {
	validationErr := validateBulk(*sf, nil, batchSize, true, sObjectName, "")
	if validationErr != nil {
		return []string{}, validationErr
	}

	jobIds, bulkErr := sf.doBulkJobWithFile(
		ctx,
		sObjectName,
		"",
		deleteOperation,
		filePath,
		batchSize,
		waitForResults,
		"",
	)
	if bulkErr != nil {
		return []string{}, bulkErr
	}

	return jobIds, nil
}

func (sf *Salesforce) GetJobResults(ctx context.Context, bulkJobId string) (BulkJobResults, error) {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return BulkJobResults{}, authErr
	}

	job, err := sf.getJobResults(ctx, ingestJobType, bulkJobId)
	if err != nil {
		return BulkJobResults{}, err
	}

	if job.State == jobStateJobComplete {
		job, err = sf.getJobRecordResults(ctx, job)
		if err != nil {
			return job, err
		}
	}

	return job, nil
}

// GetAuthFlow returns the authentication flow type used
func (sf *Salesforce) GetAuthFlow() AuthFlowType {
	return sf.AuthFlow
}

// GetAPIVersion returns the configured API version
func (sf *Salesforce) GetAPIVersion() string {
	return sf.config.apiVersion
}

// GetBatchSizeMax returns the configured maximum batch size for collections
func (sf *Salesforce) GetBatchSizeMax() int {
	return sf.config.batchSizeMax
}

// GetBulkBatchSizeMax returns the configured maximum batch size for bulk operations
func (sf *Salesforce) GetBulkBatchSizeMax() int {
	return sf.config.bulkBatchSizeMax
}

// GetCompressionHeaders returns whether compression headers are enabled
func (sf *Salesforce) GetCompressionHeaders() bool {
	return sf.config.compressionHeaders
}

// GetHTTPClient returns the configured HTTP client
func (sf *Salesforce) GetHTTPClient() *http.Client {
	return sf.config.httpClient
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
