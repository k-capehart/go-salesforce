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
)

func updateJobState(job bulkJob, state string, auth auth) error {
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

func createBulkJob(auth auth, body []byte) (*bulkJob, error) {
	resp, err := doRequest(http.MethodPost, "/jobs/ingest", jsonType, auth, string(body))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, processSalesforceError(*resp)
	}

	respBody, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}

	bulkJob := &bulkJob{}
	jsonError := json.Unmarshal(respBody, bulkJob)
	if jsonError != nil {
		return nil, jsonError
	}

	return bulkJob, nil
}

func uploadJobData(auth auth, data string, bulkJob bulkJob) error {
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

func getJobResults(auth auth, bulkJobId string) (BulkJobResults, error) {
	resp, err := doRequest(http.MethodGet, "/jobs/ingest/"+bulkJobId, jsonType, auth, "")
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

func waitForJobResult(auth auth, bulkJobId string, c chan error) {
	err := wait.PollUntilContextTimeout(context.Background(), time.Second, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		return processJobResults(auth, bulkJob)
	})
	c <- err
}

func processJobResults(auth auth, bulkJob BulkJobResults) (bool, error) {
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

func getFailedRecords(auth auth, bulkJobId string) (string, error) {
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

func readCSVFile(filePath string) ([]map[string]string, error) {
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

func doBulkJob(auth auth, sObjectName string, fieldName string, operation string, records any, batchSize int, waitForResults bool) ([]string, error) {
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

		jobReq := bulkJobCreationRequest{
			Object:              sObjectName,
			Operation:           operation,
			ExternalIdFieldName: fieldName,
		}
		body, jsonError := json.Marshal(jobReq)
		if jsonError != nil {
			jobErrors = errors.Join(jobErrors, jsonError)
			break
		}

		job, jobCreationErr := createBulkJob(auth, body)
		if jobCreationErr != nil {
			jobErrors = errors.Join(jobErrors, jobCreationErr)
			break
		}
		if job.Id == "" || job.State != jobStateOpen {
			newErr := errors.New("error creating bulk data job: id does not exist or job closed prematurely")
			jobErrors = errors.Join(jobErrors, newErr)
			break
		}
		jobIds = append(jobIds, job.Id)

		data, convertErr := mapsToCSV(batch)
		if convertErr != nil {
			jobErrors = errors.Join(jobErrors, err)
			break
		}

		uploadErr := uploadJobData(auth, data, *job)
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

func doInsertBulk(auth auth, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", insertOperation, records, batchSize, waitForResults)
}

func doInsertBulkFile(auth auth, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	data, err := readCSVFile(filePath)
	if err != nil {
		return []string{}, err
	}
	return doBulkJob(auth, sObjectName, "", insertOperation, data, batchSize, waitForResults)
}

func doUpdateBulk(auth auth, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", updateOperation, records, batchSize, waitForResults)
}

func doUpdateBulkFile(auth auth, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	data, err := readCSVFile(filePath)
	if err != nil {
		return []string{}, err
	}
	return doBulkJob(auth, sObjectName, "", updateOperation, data, batchSize, waitForResults)
}

func doUpsertBulk(auth auth, sObjectName string, fieldName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, fieldName, upsertOperation, records, batchSize, waitForResults)
}

func doUpsertBulkFile(auth auth, sObjectName string, fieldName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	data, err := readCSVFile(filePath)
	if err != nil {
		return []string{}, err
	}
	return doBulkJob(auth, sObjectName, fieldName, upsertOperation, data, batchSize, waitForResults)
}

func doDeleteBulk(auth auth, sObjectName string, records any, batchSize int, waitForResults bool) ([]string, error) {
	return doBulkJob(auth, sObjectName, "", deleteOperation, records, batchSize, waitForResults)
}

func doDeleteBulkFile(auth auth, sObjectName string, filePath string, batchSize int, waitForResults bool) ([]string, error) {
	data, err := readCSVFile(filePath)
	if err != nil {
		return []string{}, err
	}
	return doBulkJob(auth, sObjectName, "", deleteOperation, data, batchSize, waitForResults)
}
