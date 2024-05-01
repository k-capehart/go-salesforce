package salesforce

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

type bulkJobCreationRequest struct {
	Object              string `json:"object"`
	Operation           string `json:"operation"`
	ExternalIdFieldName string `json:"externalIdFieldName"`
}

type bulkQueryJobCreationRequest struct {
	Operation string `json:"operation"`
	Query     string `json:"query"`
}

type bulkJob struct {
	Id    string `json:"id"`
	State string `json:"state"`
}

type BulkJobResults struct {
	Id                  string `json:"id"`
	State               string `json:"state"`
	NumberRecordsFailed int    `json:"numberRecordsFailed"`
	ErrorMessage        string `json:"errorMessage"`
}

type bulkJobQueryResults struct {
	NumberOfRecords int        `json:"Sforce-Numberofrecords"`
	Locator         string     `json:"Sforce-Locator"`
	Data            [][]string `json:"data"`
}

const (
	jobStateAborted        = "Aborted"
	jobStateUploadComplete = "UploadComplete"
	jobStateJobComplete    = "JobComplete"
	jobStateFailed         = "Failed"
	jobStateOpen           = "Open"
	insertOperation        = "insert"
	updateOperation        = "update"
	upsertOperation        = "upsert"
	deleteOperation        = "delete"
	ingestJobType          = "ingest"
	queryJobType           = "query"
)

func updateJobState(job bulkJob, state string, auth authorization) error {
	job.State = state
	body, _ := json.Marshal(job)
	resp, err := doRequest(http.MethodPatch, "/jobs/ingest/"+job.Id, jsonType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	return nil
}

func createBulkJob(auth authorization, jobType string, body []byte) (bulkJob, error) {
	resp, err := doRequest(http.MethodPost, "/jobs/"+jobType, jsonType, auth, string(body))
	if err != nil {
		return bulkJob{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return bulkJob{}, processSalesforceError(*resp)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return bulkJob{}, readErr
	}

	newJob := &bulkJob{}
	jsonError := json.Unmarshal(respBody, newJob)
	if jsonError != nil {
		return bulkJob{}, jsonError
	}

	return *newJob, nil
}

func uploadJobData(auth authorization, data string, bulkJob bulkJob) error {
	resp, uploadDataErr := doRequest("PUT", "/jobs/ingest/"+bulkJob.Id+"/batches", csvType, auth, data)
	if uploadDataErr != nil {
		updateJobState(bulkJob, jobStateAborted, auth)
		return uploadDataErr
	}
	if resp.StatusCode != http.StatusCreated {
		updateJobState(bulkJob, jobStateAborted, auth)
		return processSalesforceError(*resp)
	}
	stateErr := updateJobState(bulkJob, jobStateUploadComplete, auth)
	if stateErr != nil {
		return stateErr
	}

	return nil
}

func getJobResults(auth authorization, jobType string, bulkJobId string) (BulkJobResults, error) {
	resp, err := doRequest(http.MethodGet, "/jobs/"+jobType+"/"+bulkJobId, jsonType, auth, "")
	if err != nil {
		return BulkJobResults{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return BulkJobResults{}, processSalesforceError(*resp)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return BulkJobResults{}, readErr
	}

	bulkJobResults := &BulkJobResults{}
	jsonError := json.Unmarshal(respBody, bulkJobResults)
	if jsonError != nil {
		return BulkJobResults{}, jsonError
	}

	return *bulkJobResults, nil
}

func waitForJobResult(auth authorization, bulkJobId string, c chan error) {
	err := wait.PollUntilContextTimeout(context.Background(), time.Second, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, ingestJobType, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		return isBulkJobDone(auth, bulkJob)
	})
	c <- err
}

func isBulkJobDone(auth authorization, bulkJob BulkJobResults) (bool, error) {
	if bulkJob.State == jobStateJobComplete || bulkJob.State == jobStateFailed {
		if bulkJob.ErrorMessage != "" {
			return true, errors.New(bulkJob.ErrorMessage)
		}
		if bulkJob.NumberRecordsFailed > 0 {
			failedRecords, getRecordsErr := getFailedRecords(auth, bulkJob.Id)
			if getRecordsErr != nil {
				return true, errors.New("unable to retrieve details about " + strconv.Itoa(bulkJob.NumberRecordsFailed) + " failed records from bulk operation")
			}
			return true, errors.New(failedRecords)
		}
		return true, nil
	}
	if bulkJob.State == jobStateAborted {
		return true, errors.New("bulk job aborted")
	}
	return false, nil
}

func getQueryJobResults(auth authorization, bulkJobId string, locator string) (bulkJobQueryResults, error) {
	uri := "/jobs/query/" + bulkJobId + "/results"
	if locator != "" {
		uri = uri + "/?locator=" + locator
	}
	resp, err := doRequest(http.MethodGet, uri, jsonType, auth, "")
	if err != nil {
		return bulkJobQueryResults{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return bulkJobQueryResults{}, processSalesforceError(*resp)
	}

	reader := csv.NewReader(resp.Body)
	records, readErr := reader.ReadAll()
	if readErr != nil {
		return bulkJobQueryResults{}, readErr
	}
	numberOfRecords, _ := strconv.Atoi(resp.Header["Sforce-Numberofrecords"][0])
	locator = ""
	if resp.Header["Sforce-Locator"][0] != "null" {
		locator = resp.Header["Sforce-Locator"][0]
	}

	queryResults := bulkJobQueryResults{
		NumberOfRecords: numberOfRecords,
		Locator:         locator,
		Data:            records,
	}

	return queryResults, nil
}

func waitForQueryResults(auth authorization, bulkJobId string) ([][]string, error) {
	err := wait.PollUntilContextTimeout(context.Background(), time.Second, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, queryJobType, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		return isBulkJobDone(auth, bulkJob)
	})
	if err != nil {
		return nil, err
	}

	queryResults, resultsErr := getQueryJobResults(auth, bulkJobId, "")
	if resultsErr != nil {
		return nil, resultsErr
	}
	records := queryResults.Data
	for queryResults.Locator != "" {
		queryResults, resultsErr = getQueryJobResults(auth, bulkJobId, queryResults.Locator)
		if resultsErr != nil {
			return nil, resultsErr
		}
		records = append(records, queryResults.Data[1:]...) // don't include headers in subsequent batches
	}

	return records, nil
}

func getFailedRecords(auth authorization, bulkJobId string) (string, error) {
	resp, err := doRequest(http.MethodGet, "/jobs/ingest/"+bulkJobId+"/failedResults", jsonType, auth, "")
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", processSalesforceError(*resp)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	return string(respBody), nil
}

func mapsToCSV(maps []map[string]any) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	var headers []string

	if len(maps) > 0 {
		headers = make([]string, 0, len(maps[0]))
		for header := range maps[0] {
			headers = append(headers, header)
		}
		err := w.Write(headers)
		if err != nil {
			return "", err
		}
	}

	for _, m := range maps {
		row := make([]string, 0, len(headers))
		for _, header := range headers {
			val := m[header]
			if val == nil {
				row = append(row, "")
			} else {
				row = append(row, fmt.Sprintf("%v", val))
			}
		}
		err := w.Write(row)
		if err != nil {
			return "", err
		}
	}

	w.Flush()
	err := w.Error()
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

func mapsToCSVSlices(maps []map[string]string) ([][]string, error) {
	var data [][]string
	var headers []string

	if len(maps) > 0 {
		headers = make([]string, 0, len(maps[0]))
		for header := range maps[0] {
			headers = append(headers, header)
		}
		data = append(data, headers)
	}

	for _, m := range maps {
		row := make([]string, 0, len(headers))
		for _, header := range headers {
			val := m[header]
			row = append(row, val)
		}
		data = append(data, row)
	}

	return data, nil
}

func csvToMap(reader csv.Reader) ([]map[string]string, error) {
	records, readErr := reader.ReadAll()
	if readErr != nil {
		return nil, readErr
	}

	keys := records[0]

	var recordMap []map[string]string
	for _, row := range records[1:] {
		record := make(map[string]string)
		for i, col := range row {
			record[keys[i]] = col
		}
		recordMap = append(recordMap, record)
	}

	return recordMap, nil
}

func readCSVFile(filePath string) ([][]string, error) {
	file, fileErr := os.Open(filePath)
	if fileErr != nil {
		return nil, fileErr
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, readErr := reader.ReadAll()
	if readErr != nil {
		return nil, readErr
	}

	return records, nil
}

func writeCSVFile(filePath string, data [][]string) error {
	file, fileErr := os.Create(filePath)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()
	writer.WriteAll(data)

	return nil
}

func constructBulkJobRequest(auth authorization, sObjectName string, operation string, fieldName string) (bulkJob, error) {
	jobReq := bulkJobCreationRequest{
		Object:              sObjectName,
		Operation:           operation,
		ExternalIdFieldName: fieldName,
	}
	body, jsonError := json.Marshal(jobReq)
	if jsonError != nil {
		return bulkJob{}, jsonError
	}

	job, jobCreationErr := createBulkJob(auth, ingestJobType, body)
	if jobCreationErr != nil {
		return bulkJob{}, jobCreationErr
	}
	if job.Id == "" || job.State != jobStateOpen {
		newErr := errors.New("error creating bulk data job: id does not exist or job closed prematurely")
		return job, newErr
	}

	return job, nil
}

func doBulkJob(auth authorization, sObjectName string, fieldName string, operation string, records any, batchSize int, waitForResults bool) ([]string, error) {
	recordMap, err := convertToSliceOfMaps(records)
	if err != nil {
		return []string{}, err
	}

	var jobErrors error
	var jobIds []string
	for len(recordMap) > 0 {
		var batch []map[string]any
		var remaining []map[string]any
		if len(recordMap) > batchSize {
			batch, remaining = recordMap[:batchSize], recordMap[batchSize:]
		} else {
			batch = recordMap
		}

		job, constructJobErr := constructBulkJobRequest(auth, sObjectName, operation, fieldName)
		if constructJobErr != nil {
			jobErrors = errors.Join(jobErrors, constructJobErr)
			break
		}
		jobIds = append(jobIds, job.Id)

		data, convertErr := mapsToCSV(batch)
		if convertErr != nil {
			jobErrors = errors.Join(jobErrors, err)
			break
		}

		uploadErr := uploadJobData(auth, data, job)
		if uploadErr != nil {
			jobErrors = uploadErr
			break
		}

		recordMap = remaining
	}

	if waitForResults {
		for _, id := range jobIds {
			c := make(chan error)
			go waitForJobResult(auth, id, c)
			jobError := <-c
			if jobError != nil {
				jobErrors = errors.Join(jobErrors, jobError)
			}
		}
	}

	return jobIds, jobErrors
}

func doBulkJobWithFile(auth authorization, sObjectName string, fieldName string, operation string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	var jobErrors error
	var jobIds []string

	records, readErr := readCSVFile(filePath)
	if readErr != nil {
		return jobIds, readErr
	}

	headers := records[0]
	records = records[1:]
	for len(records) > 0 {
		var batch [][]string
		var remaining [][]string
		if len(records) > batchSize {
			batch, remaining = records[:batchSize], records[batchSize:]
		} else {
			batch = records
		}

		job, constructJobErr := constructBulkJobRequest(auth, sObjectName, operation, fieldName)
		if constructJobErr != nil {
			jobErrors = errors.Join(jobErrors, constructJobErr)
			break
		}
		jobIds = append(jobIds, job.Id)

		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		batch = append([][]string{headers}, batch...)
		w.WriteAll(batch)
		w.Flush()
		writeErr := w.Error()
		if writeErr != nil {
			jobErrors = errors.Join(jobErrors, writeErr)
			break
		}

		uploadErr := uploadJobData(auth, buf.String(), job)
		if uploadErr != nil {
			jobErrors = errors.Join(jobErrors, uploadErr)
		}

		records = remaining
	}

	if waitForResults {
		for _, id := range jobIds {
			c := make(chan error)
			go waitForJobResult(auth, id, c)
			jobError := <-c
			if jobError != nil {
				jobErrors = errors.Join(jobErrors, jobError)
			}
		}
	}

	return jobIds, jobErrors
}

func doQueryBulk(auth authorization, filePath string, query string) error {
	queryJobReq := bulkQueryJobCreationRequest{
		Operation: queryJobType,
		Query:     query,
	}
	body, jsonErr := json.Marshal(queryJobReq)
	if jsonErr != nil {
		return jsonErr
	}

	job, jobCreationErr := createBulkJob(auth, queryJobType, body)
	if jobCreationErr != nil {
		return jobCreationErr
	}
	if job.Id == "" {
		newErr := errors.New("error creating bulk query job")
		return newErr
	}

	records, jobErr := waitForQueryResults(auth, job.Id)
	if jobErr != nil {
		return jobErr
	}
	writeErr := writeCSVFile(filePath, records)
	if writeErr != nil {
		return writeErr
	}

	return nil
}

func doInsertBulk(auth authorization, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", insertOperation, records, batchSize, waitForResults)
}

func doInsertBulkFile(auth authorization, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJobWithFile(auth, sObjectName, "", insertOperation, filePath, batchSize, waitForResults)
}

func doUpdateBulk(auth authorization, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", updateOperation, records, batchSize, waitForResults)
}

func doUpdateBulkFile(auth authorization, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJobWithFile(auth, sObjectName, "", updateOperation, filePath, batchSize, waitForResults)
}

func doUpsertBulk(auth authorization, sObjectName string, fieldName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, fieldName, upsertOperation, records, batchSize, waitForResults)
}

func doUpsertBulkFile(auth authorization, sObjectName string, fieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJobWithFile(auth, sObjectName, fieldName, upsertOperation, filePath, batchSize, waitForResults)
}

func doDeleteBulk(auth authorization, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", deleteOperation, records, batchSize, waitForResults)
}

func doDeleteBulkFile(auth authorization, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJobWithFile(auth, sObjectName, "", deleteOperation, filePath, batchSize, waitForResults)
}
