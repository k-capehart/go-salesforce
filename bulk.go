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
	"strconv"
	"time"

	"github.com/spf13/afero"
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
	SuccessfulRecords   []map[string]any
	FailedRecords       []map[string]any
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
	failedResults          = "failedResults"
	successfulResults      = "successfulResults"
)

var appFs = afero.NewOsFs() // afero.Fs type is a wrapper around os functions, allowing us to mock it in tests

func updateJobState(job bulkJob, state string, auth authentication) error {
	job.State = state
	body, _ := json.Marshal(job)
	_, err := doRequest(http.MethodPatch, "/jobs/ingest/"+job.Id, jsonType, auth, string(body))
	if err != nil {
		return err
	}

	return nil
}

func createBulkJob(auth authentication, jobType string, body []byte) (bulkJob, error) {
	resp, err := doRequest(http.MethodPost, "/jobs/"+jobType, jsonType, auth, string(body))
	if err != nil {
		return bulkJob{}, err
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

func uploadJobData(auth authentication, data string, bulkJob bulkJob) error {
	_, uploadDataErr := doRequest("PUT", "/jobs/ingest/"+bulkJob.Id+"/batches", csvType, auth, data)
	if uploadDataErr != nil {
		if err := updateJobState(bulkJob, jobStateAborted, auth); err != nil {
			return err
		}
		return uploadDataErr
	}
	stateErr := updateJobState(bulkJob, jobStateUploadComplete, auth)
	if stateErr != nil {
		return stateErr
	}

	return nil
}

func getJobResults(auth authentication, jobType string, bulkJobId string) (BulkJobResults, error) {
	resp, err := doRequest(http.MethodGet, "/jobs/"+jobType+"/"+bulkJobId, jsonType, auth, "")
	if err != nil {
		return BulkJobResults{}, err
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

func getJobRecordResults(auth authentication, bulkJobResults BulkJobResults) (BulkJobResults, error) {
	successfulRecords, err := getBulkJobRecords(auth, bulkJobResults.Id, successfulResults)
	if err != nil {
		fmt.Println("failed to get SuccessfulRecords")
		return bulkJobResults, err
	}
	bulkJobResults.SuccessfulRecords = successfulRecords
	failedRecords, err := getBulkJobRecords(auth, bulkJobResults.Id, failedResults)
	if err != nil {
		fmt.Println("failed to get FailedRecords")
		return bulkJobResults, err
	}
	bulkJobResults.FailedRecords = failedRecords
	return bulkJobResults, err
}

func getBulkJobRecords(auth authentication, bulkJobId string, resultType string) ([]map[string]any, error) {
	resp, err := doRequest(http.MethodGet, "/jobs/ingest/"+bulkJobId+"/"+resultType, jsonType, auth, "")
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(resp.Body)
	results, err := csvToMap(*reader)
	if err != nil {
		return nil, err
	}

	return results, nil
}

func waitForJobResultsAsync(auth authentication, bulkJobId string, jobType string, interval time.Duration, c chan error) {
	err := wait.PollUntilContextTimeout(context.Background(), interval, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, jobType, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		return isBulkJobDone(bulkJob)
	})
	c <- err
}

func waitForJobResults(auth authentication, bulkJobId string, jobType string, interval time.Duration) error {
	err := wait.PollUntilContextTimeout(context.Background(), interval, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, jobType, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		return isBulkJobDone(bulkJob)
	})
	return err
}

func isBulkJobDone(bulkJob BulkJobResults) (bool, error) {
	if bulkJob.State == jobStateJobComplete || bulkJob.State == jobStateFailed {
		if bulkJob.ErrorMessage != "" {
			return true, errors.New(bulkJob.ErrorMessage)
		}
		return true, nil
	}
	if bulkJob.State == jobStateAborted {
		return true, errors.New("bulk job aborted")
	}
	return false, nil
}

func getQueryJobResults(auth authentication, bulkJobId string, locator string) (bulkJobQueryResults, error) {
	uri := "/jobs/query/" + bulkJobId + "/results"
	if locator != "" {
		uri = uri + "/?locator=" + locator
	}
	resp, err := doRequest(http.MethodGet, uri, jsonType, auth, "")
	if err != nil {
		return bulkJobQueryResults{}, err
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

func collectQueryResults(auth authentication, bulkJobId string) ([][]string, error) {
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

func csvToMap(reader csv.Reader) ([]map[string]any, error) {
	records, readErr := reader.ReadAll()
	if readErr != nil {
		return nil, readErr
	}
	if len(records) > 0 {
		headers := records[0]
		var recordMap []map[string]any
		for _, row := range records[1:] {
			record := make(map[string]any)
			for i, col := range row {
				record[headers[i]] = col
			}
			recordMap = append(recordMap, record)
		}
		return recordMap, nil
	}
	return nil, nil
}

func readCSVFile(filePath string) ([][]string, error) {
	file, fileErr := appFs.Open(filePath)
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
	file, fileErr := appFs.Create(filePath)
	if fileErr != nil {
		return fileErr
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	if writer == nil {
		return errors.New("error writing csv file")
	}
	defer writer.Flush()
	if err := writer.WriteAll(data); err != nil {
		return errors.New("error writing csv file")
	}

	return nil
}

func constructBulkJobRequest(auth authentication, sObjectName string, operation string, fieldName string) (bulkJob, error) {
	jobReq := bulkJobCreationRequest{
		Object:              sObjectName,
		Operation:           operation,
		ExternalIdFieldName: fieldName,
	}
	body, _ := json.Marshal(jobReq)

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

func doBulkJob(auth authentication, sObjectName string, fieldName string, operation string, records any, batchSize int, waitForResults bool) ([]string, error) {
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
		recordMap = remaining

		job, constructJobErr := constructBulkJobRequest(auth, sObjectName, operation, fieldName)
		if constructJobErr != nil {
			return jobIds, constructJobErr
		}
		jobIds = append(jobIds, job.Id)

		data, convertErr := mapsToCSV(batch)
		if convertErr != nil {
			return jobIds, convertErr
		}

		uploadErr := uploadJobData(auth, data, job)
		if uploadErr != nil {
			return jobIds, uploadErr
		}
	}

	if waitForResults {
		c := make(chan error, len(jobIds))
		for _, id := range jobIds {
			go waitForJobResultsAsync(auth, id, ingestJobType, (time.Second / 2), c)
		}
		jobErrors = <-c
	}

	return jobIds, jobErrors
}

func doBulkJobWithFile(auth authentication, sObjectName string, fieldName string, operation string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
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
		records = remaining

		job, constructJobErr := constructBulkJobRequest(auth, sObjectName, operation, fieldName)
		if constructJobErr != nil {
			jobErrors = errors.Join(jobErrors, constructJobErr)
			break
		}
		jobIds = append(jobIds, job.Id)

		var buf bytes.Buffer
		w := csv.NewWriter(&buf)
		batch = append([][]string{headers}, batch...)
		if err := w.WriteAll(batch); err != nil {
			jobErrors = errors.Join(jobErrors, err)
			break
		}
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
	}

	if waitForResults {
		c := make(chan error, len(jobIds))
		for _, id := range jobIds {
			go waitForJobResultsAsync(auth, id, ingestJobType, (time.Second / 2), c)
		}
		jobErrors = <-c
	}

	return jobIds, jobErrors
}

func doQueryBulk(auth authentication, filePath string, query string) error {
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

	pollErr := waitForJobResults(auth, job.Id, queryJobType, (time.Second / 2))
	if pollErr != nil {
		return pollErr
	}
	records, reqErr := collectQueryResults(auth, job.Id)
	if reqErr != nil {
		return reqErr
	}
	writeErr := writeCSVFile(filePath, records)
	if writeErr != nil {
		return writeErr
	}

	return nil
}
