package salesforce

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gocarina/gocsv"
)

type BulkJob struct {
	Object    string `json:"object"`
	Operation string `json:"operation"`
}

type BulkJobCreationResponse struct {
	Id    string `json:"id"`
	State string `json:"state"`
}

type BulkJobState struct {
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
	abortJob := BulkJobState{State: state}
	abortBody, _ := json.Marshal(abortJob)
	_, abortErr := doRequest("PATCH", "/jobs/ingest/"+jobId, JSONType, auth, string(abortBody))
	if abortErr != nil {
		return abortErr
	}
	return nil
}

func createBulkJob(auth Auth, body []byte) (*BulkJobCreationResponse, error) {
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

	bulkJob := &BulkJobCreationResponse{}
	jsonError := json.Unmarshal(respBody, bulkJob)
	if jsonError != nil {
		return nil, jsonError
	}

	return bulkJob, nil
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

	job := BulkJob{
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

	sObjects := records
	csvContent, csvErr := gocsv.MarshalString(sObjects)
	if csvErr != nil {
		updateJobState(bulkJob.Id, JobStateAborted, *sf.auth)
		return csvErr
	}

	resp, uploadDataErr := doRequest("PUT", "/jobs/ingest/"+bulkJob.Id+"/batches", CSVType, *sf.auth, csvContent)
	if uploadDataErr != nil {
		updateJobState(bulkJob.Id, JobStateAborted, *sf.auth)
		return err
	}
	if resp.StatusCode != http.StatusCreated {
		updateJobState(bulkJob.Id, JobStateAborted, *sf.auth)
		return processSalesforceError(*resp)
	}
	stateErr := updateJobState(bulkJob.Id, JobStateUploadComplete, *sf.auth)
	if stateErr != nil {
		return stateErr
	}

	return nil
}
