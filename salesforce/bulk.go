package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gocarina/gocsv"
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

type BulkJobData struct {
	Data string `json:"data"`
}

const (
	JobStateAborted        = "Aborted"
	JobStateUploadComplete = "UploadComplete"
)

func updateJobState(jobId string, state string, auth Auth) error {
	abortJob := BulkJob{State: state}
	abortBody, _ := json.Marshal(abortJob)
	_, abortErr := doRequest("PATCH", "/jobs/ingest/"+jobId, JSONType, auth, string(abortBody))
	if abortErr != nil {
		return abortErr
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
		updateJobState(bulkJob.Id, JobStateAborted, auth)
		return csvErr
	}

	resp, uploadDataErr := doRequest("PUT", "/jobs/ingest/"+bulkJob.Id+"/batches", CSVType, auth, csvContent)
	if uploadDataErr != nil {
		updateJobState(bulkJob.Id, JobStateAborted, auth)
		return uploadDataErr
	}
	if resp.StatusCode != http.StatusCreated {
		updateJobState(bulkJob.Id, JobStateAborted, auth)
		return processSalesforceError(*resp)
	}
	stateErr := updateJobState(bulkJob.Id, JobStateUploadComplete, auth)
	if stateErr != nil {
		return stateErr
	}

	return nil
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

	return nil
}
