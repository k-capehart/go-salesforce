package salesforce

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/gocarina/gocsv"
	"k8s.io/apimachinery/pkg/util/wait"
)

type BulkJobCreationRequest struct {
	Object              string `json:"object"`
	Operation           string `json:"operation"`
	ExternalIdFieldName string `json:"externalIdFieldName"`
}

type BulkJob struct {
	Id    string `json:"id"`
	State string `json:"state"`
}

type BulkJobResults struct {
	Id                  string `json:"id"`
	State               string `json:"state"`
	NumberRecordsFailed int    `json:"numberRecordsFailed"`
	ErrorMessage        string `json:"errorMessage"`
}

type BulkJobData struct {
	Data string `json:"data"`
}

const (
	JobStateAborted        = "Aborted"
	JobStateUploadComplete = "UploadComplete"
	JobStateJobComplete    = "JobComplete"
	JobStateFailed         = "Failed"
)

func updateJobState(job BulkJob, state string, auth Auth) error {
	job.State = state
	body, _ := json.Marshal(job)
	resp, err := doRequest("PATCH", "/jobs/ingest/"+job.Id, JSONType, auth, string(body))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return processSalesforceError(*resp)
	}
	return nil
}

func createBulkJob(auth Auth, body []byte) (*BulkJob, error) {
	resp, err := doRequest("POST", "/jobs/ingest", JSONType, auth, string(body))
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

	bulkJob := &BulkJob{}
	jsonError := json.Unmarshal(respBody, bulkJob)
	if jsonError != nil {
		return nil, jsonError
	}

	return bulkJob, nil
}

func uploadJobData(auth Auth, records any, bulkJob BulkJob) error {
	sObjects := records
	csvContent, csvErr := gocsv.MarshalString(sObjects)
	if csvErr != nil {
		updateJobState(bulkJob, JobStateAborted, auth)
		return csvErr
	}

	resp, uploadDataErr := doRequest("PUT", "/jobs/ingest/"+bulkJob.Id+"/batches", CSVType, auth, csvContent)
	if uploadDataErr != nil {
		updateJobState(bulkJob, JobStateAborted, auth)
		return uploadDataErr
	}
	if resp.StatusCode != http.StatusCreated {
		updateJobState(bulkJob, JobStateAborted, auth)
		return processSalesforceError(*resp)
	}
	stateErr := updateJobState(bulkJob, JobStateUploadComplete, auth)
	if stateErr != nil {
		return stateErr
	}

	return nil
}

func getJobResults(auth Auth, bulkJobId string) (BulkJobResults, error) {
	resp, err := doRequest("GET", "/jobs/ingest/"+bulkJobId, JSONType, auth, "")
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

func waitForJobResult(auth Auth, bulkJobId string) error {
	err := wait.PollUntilContextTimeout(context.Background(), time.Second, time.Minute, false, func(context.Context) (bool, error) {
		bulkJob, reqErr := getJobResults(auth, bulkJobId)
		if reqErr != nil {
			return true, reqErr
		}
		if bulkJob.State == JobStateJobComplete || bulkJob.State == JobStateFailed {
			if bulkJob.ErrorMessage != "" {
				return true, errors.New(bulkJob.ErrorMessage)
			}
			if bulkJob.NumberRecordsFailed > 0 {
				failedRecords, getRecordsErr := getFailedRecords(auth, bulkJobId)
				if getRecordsErr != nil {
					return true, errors.New("failed to retrieve failed records from bulk operation")
				}
				return true, errors.New(failedRecords)
			}
			return true, nil
		}
		if bulkJob.State == JobStateAborted {
			return true, errors.New("bulk job aborted")
		}
		return false, nil
	})
	return err
}

func getFailedRecords(auth Auth, bulkJobId string) (string, error) {
	resp, err := doRequest("GET", "/jobs/ingest/"+bulkJobId+"/failedResults", JSONType, auth, "")
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

func (sf *Salesforce) InsertBulk(sObjectName string, records any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	job := BulkJobCreationRequest{
		Object:    sObjectName,
		Operation: "insert",
	}
	body, jsonError := json.Marshal(job)
	if jsonError != nil {
		return jsonError
	}

	bulkJob, err := createBulkJob(*sf.auth, body)
	if err != nil {
		return err
	}
	if bulkJob.Id == "" || bulkJob.State != "Open" {
		return errors.New("error creating bulk data job. Id does not exist or job closed prematurely")
	}

	uploadErr := uploadJobData(*sf.auth, records, *bulkJob)
	if uploadErr != nil {
		return uploadErr
	}

	pollErr := waitForJobResult(*sf.auth, bulkJob.Id)
	if pollErr != nil {
		return pollErr
	}

	return nil
}

func (sf *Salesforce) UpdateBulk(sObjectName string, records any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	job := BulkJobCreationRequest{
		Object:    sObjectName,
		Operation: "update",
	}
	body, jsonError := json.Marshal(job)
	if jsonError != nil {
		return jsonError
	}

	bulkJob, err := createBulkJob(*sf.auth, body)
	if err != nil {
		return err
	}
	if bulkJob.Id == "" || bulkJob.State != "Open" {
		return errors.New("error creating bulk data job. Id does not exist or job closed prematurely")
	}

	uploadErr := uploadJobData(*sf.auth, records, *bulkJob)
	if uploadErr != nil {
		return uploadErr
	}

	pollErr := waitForJobResult(*sf.auth, bulkJob.Id)
	if pollErr != nil {
		return pollErr
	}

	return nil
}

func (sf *Salesforce) UpsertBulk(sObjectName string, fieldName string, records any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	job := BulkJobCreationRequest{
		Object:              sObjectName,
		Operation:           "upsert",
		ExternalIdFieldName: fieldName,
	}
	body, jsonError := json.Marshal(job)
	if jsonError != nil {
		return jsonError
	}

	bulkJob, err := createBulkJob(*sf.auth, body)
	if err != nil {
		return err
	}
	if bulkJob.Id == "" || bulkJob.State != "Open" {
		return errors.New("error creating bulk data job. Id does not exist or job closed prematurely")
	}

	uploadErr := uploadJobData(*sf.auth, records, *bulkJob)
	if uploadErr != nil {
		return uploadErr
	}

	pollErr := waitForJobResult(*sf.auth, bulkJob.Id)
	if pollErr != nil {
		return pollErr
	}

	return nil
}

func (sf *Salesforce) DeleteBulk(sObjectName string, records any) error {
	authErr := validateAuth(*sf)
	if authErr != nil {
		return authErr
	}
	typErr := validateOfTypeSlice(records)
	if typErr != nil {
		return typErr
	}

	job := BulkJobCreationRequest{
		Object:    sObjectName,
		Operation: "delete",
	}
	body, jsonError := json.Marshal(job)
	if jsonError != nil {
		return jsonError
	}

	bulkJob, err := createBulkJob(*sf.auth, body)
	if err != nil {
		return err
	}
	if bulkJob.Id == "" || bulkJob.State != "Open" {
		return errors.New("error creating bulk data job. Id does not exist or job closed prematurely")
	}

	uploadErr := uploadJobData(*sf.auth, records, *bulkJob)
	if uploadErr != nil {
		return uploadErr
	}

	pollErr := waitForJobResult(*sf.auth, bulkJob.Id)
	if pollErr != nil {
		return pollErr
	}

	return nil
}
