package salesforce

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/afero"
)

func Test_createBulkJob(t *testing.T) {
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	badRespServer, badRespSfAuth := setupTestServer("1", http.StatusOK)
	defer badRespServer.Close()

	type account struct {
		Id   string
		Name string
	}
	acc := []account{
		{
			Id:   "123",
			Name: "test account 1",
		},
		{
			Id:   "456",
			Name: "test account 2",
		},
	}
	ingestBody, _ := json.Marshal(acc)

	queryJobReq := bulkQueryJobCreationRequest{
		Operation: queryJobType,
		Query:     "SELECT Id FROM Account",
	}
	queryBody, _ := json.Marshal(queryJobReq)

	type args struct {
		sf      *Salesforce
		jobType string
		body    []byte
	}
	tests := []struct {
		name    string
		args    args
		want    bulkJob
		wantErr bool
	}{
		{
			name: "create_bulk_ingest_job",
			args: args{
				sf:      buildSalesforceStruct(&sfAuth),
				jobType: ingestJobType,
				body:    ingestBody,
			},
			want:    job,
			wantErr: false,
		},
		{
			name: "create_bulk_query_job",
			args: args{
				sf:      buildSalesforceStruct(&sfAuth),
				jobType: queryJobType,
				body:    queryBody,
			},
			want:    job,
			wantErr: false,
		},
		{
			name: "bad_response",
			args: args{
				sf:      buildSalesforceStruct(&badRespSfAuth),
				jobType: queryJobType,
				body:    queryBody,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createBulkJob(tt.args.sf, tt.args.jobType, tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("createBulkJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createBulkJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getJobResults(t *testing.T) {
	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateOpen,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	server, sfAuth := setupTestServer(jobResults, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	badRespServer, badRespSfAuth := setupTestServer("1", http.StatusOK)
	defer badRespServer.Close()

	type args struct {
		sf        *Salesforce
		jobType   string
		bulkJobId string
	}
	tests := []struct {
		name    string
		args    args
		want    BulkJobResults
		wantErr bool
	}{
		{
			name: "get_job_results",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				jobType:   ingestJobType,
				bulkJobId: "1234",
			},
			want:    jobResults,
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:        buildSalesforceStruct(&badReqSfAuth),
				jobType:   ingestJobType,
				bulkJobId: "1234",
			},
			wantErr: true,
		},
		{
			name: "bad_response",
			args: args{
				sf:        buildSalesforceStruct(&badRespSfAuth),
				jobType:   ingestJobType,
				bulkJobId: "1234",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getJobResults(tt.args.sf, tt.args.jobType, tt.args.bulkJobId)
			if (err != nil) != tt.wantErr {
				t.Errorf("getJobResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getJobResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_isBulkJobDone(t *testing.T) {
	type args struct {
		bulkJob BulkJobResults
	}
	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "bulk_job_complete",
			args: args{
				bulkJob: BulkJobResults{
					Id:                  "1234",
					State:               jobStateJobComplete,
					NumberRecordsFailed: 0,
					ErrorMessage:        "",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "bulk_job_not_complete",
			args: args{
				bulkJob: BulkJobResults{
					Id:                  "1234",
					State:               jobStateOpen,
					NumberRecordsFailed: 0,
					ErrorMessage:        "",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "bulk_job_aborted",
			args: args{
				bulkJob: BulkJobResults{
					Id:                  "1234",
					State:               jobStateAborted,
					NumberRecordsFailed: 0,
					ErrorMessage:        "",
				},
			},
			want:    true,
			wantErr: true,
		},
		{
			name: "bulk_job_failed",
			args: args{
				bulkJob: BulkJobResults{
					Id:                  "1234",
					State:               jobStateFailed,
					NumberRecordsFailed: 1,
					ErrorMessage:        "example error",
				},
			},
			want:    true,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := isBulkJobDone(tt.args.bulkJob)
			if (err != nil) != tt.wantErr {
				t.Errorf("isBulkJobDone() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("isBulkJobDone() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getQueryJobResults(t *testing.T) {
	csvData := `"col"` + "\n" + `"row"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Sforce-Numberofrecords", "1")
		w.Header().Add("Sforce-Locator", "")
		if _, err := w.Write([]byte(csvData)); err != nil {
			t.Fatal(err.Error())
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		sf        *Salesforce
		bulkJobId string
		locator   string
	}
	tests := []struct {
		name    string
		args    args
		want    bulkJobQueryResults
		wantErr bool
	}{
		{
			name: "get_single_query_job_result",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "1234",
				locator:   "",
			},
			want: bulkJobQueryResults{
				NumberOfRecords: 1,
				Locator:         "",
				Data:            [][]string{{"col"}, {"row"}},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:        buildSalesforceStruct(&badSfAuth),
				bulkJobId: "1234",
				locator:   "",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getQueryJobResults(tt.args.sf, tt.args.bulkJobId, tt.args.locator)
			if (err != nil) != tt.wantErr {
				t.Errorf("getQueryJobResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getQueryJobResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_mapsToCSV(t *testing.T) {
	type args struct {
		maps []map[string]any
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "convert_map_to_csv_string",
			args: args{
				maps: []map[string]any{
					{
						"key": "val",
					},
					{
						"key": "val1",
					},
				},
			},
			want:    "key\nval\nval1\n",
			wantErr: false,
		},
		{
			name: "convert_map_to_csv_string_nil_val",
			args: args{
				maps: []map[string]any{
					{
						"key": nil,
					},
				},
			},
			want:    "key\n\n",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapsToCSV(tt.args.maps)
			if (err != nil) != tt.wantErr {
				t.Errorf("mapsToCSV() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("mapsToCSV() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_constructBulkJobRequest(t *testing.T) {
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	badJob := bulkJob{
		Id:    "1234",
		State: jobStateAborted,
	}
	badJobByte, _ := json.Marshal(badJob)
	badJobServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write(badJobByte); err != nil {
				t.Fatal(err.Error())
			}
		}),
	)
	badJobSfAuth := authentication{
		InstanceUrl: badJobServer.URL,
		AccessToken: "accesstokenvalue",
	}
	defer badJobServer.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	type args struct {
		sf          *Salesforce
		sObjectName string
		operation   string
		fieldName   string
	}
	tests := []struct {
		name    string
		args    args
		want    bulkJob
		wantErr bool
	}{
		{
			name: "construct_bulk_job_success",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				operation:   insertOperation,
				fieldName:   "",
			},
			want:    job,
			wantErr: false,
		},
		{
			name: "construct_bulk_job_fail",
			args: args{
				sf:          buildSalesforceStruct(&badJobSfAuth),
				sObjectName: "Account",
				operation:   insertOperation,
				fieldName:   "",
			},
			want:    badJob,
			wantErr: true,
		},
		{
			name: "bad_request",
			args: args{
				sf:          buildSalesforceStruct(&badReqSfAuth),
				sObjectName: "Account",
				operation:   insertOperation,
				fieldName:   "",
			},
			want:    bulkJob{},
			wantErr: true,
		},
		{
			name: "bad_response",
			args: args{
				sf:          buildSalesforceStruct(&authentication{}),
				sObjectName: "Account",
				operation:   insertOperation,
				fieldName:   "",
			},
			want:    bulkJob{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := constructBulkJobRequest(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.operation,
				tt.args.fieldName,
				"",
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("constructBulkJobRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("constructBulkJobRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doBulkJob(t *testing.T) {
	type account struct {
		ExternalId string
		Name       string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	jobBody, _ := json.Marshal(job)

	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateJobComplete,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	jobResultsBody, _ := json.Marshal(jobResults)

	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	waitingServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
				w.WriteHeader(http.StatusCreated)
			}
			if r.Method == http.MethodPost {
				if _, err := w.Write(jobBody); err != nil {
					panic(err.Error())
				}
			} else {
				if _, err := w.Write(jobResultsBody); err != nil {
					panic(err.Error())
				}
			}
		}),
	)
	waitingSfAuth := authentication{
		InstanceUrl: waitingServer.URL,
		AccessToken: "accesstokenvalue",
	}

	uploadFailServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				if _, err := w.Write(jobBody); err != nil {
					panic(err.Error())
				}
			}
		}),
	)
	uploadFailSfAuth := authentication{
		InstanceUrl: uploadFailServer.URL,
		AccessToken: "accesstokenvalue",
	}

	type args struct {
		sf             *Salesforce
		sObjectName    string
		fieldName      string
		operation      string
		records        any
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "bulk_insert_batch_size_200",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				fieldName:   "",
				operation:   insertOperation,
				records: []account{
					{
						Name: "test account 1",
					},
					{
						Name: "test account 2",
					},
				},
				batchSize:      200,
				waitForResults: false,
			},
			want:    []string{job.Id},
			wantErr: false,
		},
		{
			name: "bulk_upsert_batch_size_1",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				fieldName:   "externalId",
				operation:   upsertOperation,
				records: []account{
					{
						ExternalId: "acc1",
						Name:       "test account 1",
					},
					{
						ExternalId: "acc2",
						Name:       "test account 2",
					},
				},
				batchSize:      1,
				waitForResults: false,
			},
			want:    []string{job.Id, job.Id},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:          buildSalesforceStruct(&badReqSfAuth),
				sObjectName: "Account",
				fieldName:   "externalId",
				operation:   upsertOperation,
				records: []account{
					{
						ExternalId: "acc1",
						Name:       "test account 1",
					},
				},
				batchSize:      1,
				waitForResults: false,
			},
			wantErr: true,
		},
		{
			name: "bulk_insert_wait_for_results",
			args: args{
				sf:          buildSalesforceStruct(&waitingSfAuth),
				sObjectName: "Account",
				fieldName:   "",
				operation:   insertOperation,
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize:      200,
				waitForResults: true,
			},
			want:    []string{job.Id},
			wantErr: false,
		},
		{
			name: "bad_request_upload_fail",
			args: args{
				sf:          buildSalesforceStruct(&uploadFailSfAuth),
				sObjectName: "Account",
				fieldName:   "",
				operation:   insertOperation,
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize:      200,
				waitForResults: false,
			},
			want:    []string{job.Id},
			wantErr: true,
		},
		{
			name: "bad_data",
			args: args{
				sf:             buildSalesforceStruct(&sfAuth),
				sObjectName:    "Account",
				fieldName:      "",
				operation:      insertOperation,
				records:        1,
				batchSize:      200,
				waitForResults: false,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doBulkJob(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.fieldName,
				tt.args.operation,
				tt.args.records,
				tt.args.batchSize,
				tt.args.waitForResults,
				"",
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doBulkJob() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doBulkJob() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_waitForJobResultsAsync(t *testing.T) {
	jobResults := BulkJobResults{
		Id:    "1234",
		State: jobStateJobComplete,
	}
	server, sfAuth := setupTestServer(jobResults, http.StatusOK)
	defer server.Close()

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		sf        *Salesforce
		bulkJobId string
		jobType   string
		interval  time.Duration
		c         chan error
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "wait_for_ingest_result",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "1234",
				jobType:   ingestJobType,
				interval:  time.Nanosecond,
				c:         make(chan error),
			},
			wantErr: false,
		},
		{
			name: "wait_for_query_result",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "1234",
				jobType:   queryJobType,
				interval:  time.Nanosecond,
				c:         make(chan error),
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:        buildSalesforceStruct(&badSfAuth),
				bulkJobId: "",
				jobType:   queryJobType,
				interval:  time.Nanosecond,
				c:         make(chan error),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go waitForJobResultsAsync(
				tt.args.sf,
				tt.args.bulkJobId,
				tt.args.jobType,
				tt.args.interval,
				tt.args.c,
			)
			err := <-tt.args.c
			if (err != nil) != tt.wantErr {
				t.Errorf("waitForJobResult() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_waitForJobResults(t *testing.T) {
	jobResults := BulkJobResults{
		Id:    "1234",
		State: jobStateJobComplete,
	}
	server, sfAuth := setupTestServer(jobResults, http.StatusOK)
	defer server.Close()

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		sf        *Salesforce
		bulkJobId string
		jobType   string
		interval  time.Duration
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "wait_for_ingest_result",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "1234",
				jobType:   ingestJobType,
				interval:  time.Nanosecond,
			},
			wantErr: false,
		},
		{
			name: "wait_for_query_result",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "1234",
				jobType:   queryJobType,
				interval:  time.Nanosecond,
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:        buildSalesforceStruct(&badSfAuth),
				bulkJobId: "",
				jobType:   queryJobType,
				interval:  time.Nanosecond,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := waitForJobResults(
				tt.args.sf,
				tt.args.bulkJobId,
				tt.args.jobType,
				tt.args.interval,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("waitForQueryResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_collectQueryResults(t *testing.T) {
	csvData := `"col"` + "\n" + `"row"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "?locator=") {
			w.Header().Add("Sforce-Locator", "")
		} else {
			w.Header().Add("Sforce-Locator", "abc")
		}
		w.Header().Add("Sforce-Numberofrecords", "1")
		if _, err := w.Write([]byte(csvData)); err != nil {
			t.Fatal(err.Error())
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		sf        *Salesforce
		bulkJobId string
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "query_with_locator",
			args: args{
				sf:        buildSalesforceStruct(&sfAuth),
				bulkJobId: "123",
			},
			want:    [][]string{{"col"}, {"row"}, {"row"}},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:        buildSalesforceStruct(&badSfAuth),
				bulkJobId: "123",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := collectQueryResults(tt.args.sf, tt.args.bulkJobId)
			if (err != nil) != tt.wantErr {
				t.Errorf("collectQueryResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("collectQueryResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_uploadJobData(t *testing.T) {
	server, sfAuth := setupTestServer("", http.StatusOK)
	defer server.Close()

	badBatchReqServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}),
	)
	badBatchReqAuth := authentication{
		InstanceUrl: badBatchReqServer.URL,
		AccessToken: "accesstokenvalue",
	}
	defer badBatchReqServer.Close()

	badBatchReqAndJobStateServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}),
	)
	badBatchAndUpdateJobStateReqAuth := authentication{
		InstanceUrl: badBatchReqAndJobStateServer.URL,
		AccessToken: "accesstokenvalue",
	}
	defer badBatchReqAndJobStateServer.Close()

	badRequestServer, badRequestSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badRequestServer.Close()

	type args struct {
		sf      *Salesforce
		data    string
		bulkJob bulkJob
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "update_job_state_success",
			args: args{
				sf:      buildSalesforceStruct(&sfAuth),
				data:    "data",
				bulkJob: bulkJob{},
			},
			wantErr: false,
		},
		{
			name: "batch_req_fail",
			args: args{
				sf:      buildSalesforceStruct(&badBatchReqAuth),
				data:    "data",
				bulkJob: bulkJob{},
			},
			wantErr: true,
		},
		{
			name: "update_job_state_fail_aborted",
			args: args{
				sf:      buildSalesforceStruct(&badBatchAndUpdateJobStateReqAuth),
				data:    "data",
				bulkJob: bulkJob{},
			},
			wantErr: true,
		},
		{
			name: "update_job_state_fail_complete",
			args: args{
				sf:      buildSalesforceStruct(&badRequestSfAuth),
				data:    "data",
				bulkJob: bulkJob{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := uploadJobData(tt.args.sf, tt.args.data, tt.args.bulkJob); (err != nil) != tt.wantErr {
				t.Errorf("uploadJobData() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_readCSVFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("123"), 0o644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	type args struct {
		filePath string
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "read file successfully",
			args: args{
				filePath: "data/data.csv",
			},
			want:    [][]string{{"123"}},
			wantErr: false,
		},
		{
			name: "read file failure",
			args: args{
				filePath: "data/does_not_exist.csv",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readCSVFile(tt.args.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("readCSVFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readCSVFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_writeCSVFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system

	type args struct {
		filePath string
		data     [][]string
	}
	tests := []struct {
		name    string
		args    args
		want    [][]string
		wantErr bool
	}{
		{
			name: "write file successfully",
			args: args{
				filePath: "data/export.csv",
				data:     [][]string{{"123"}},
			},
			want:    [][]string{{"123"}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := writeCSVFile(tt.args.filePath, tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("writeCSVFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			got, err := readCSVFile("data/export.csv")
			if err != nil {
				t.Error(err.Error())
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("writeCSVFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_updateJobState(t *testing.T) {
	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		job   bulkJob
		state string
		sf    *Salesforce
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "bad_request",
			args: args{
				job:   bulkJob{},
				state: "",
				sf:    buildSalesforceStruct(&badSfAuth),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := updateJobState(tt.args.job, tt.args.state, tt.args.sf); (err != nil) != tt.wantErr {
				t.Errorf("updateJobState() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doBulkJobWithFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0o755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("header\nrow\nrow\n"), 0o644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	jobBody, _ := json.Marshal(job)

	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateJobComplete,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	jobResultsBody, _ := json.Marshal(jobResults)

	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	waitingServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
				w.WriteHeader(http.StatusCreated)
			}
			if r.Method == http.MethodPost {
				if _, err := w.Write(jobBody); err != nil {
					panic(err.Error())
				}
			} else {
				if _, err := w.Write(jobResultsBody); err != nil {
					panic(err.Error())
				}
			}
		}),
	)
	waitingSfAuth := authentication{
		InstanceUrl: waitingServer.URL,
		AccessToken: "accesstokenvalue",
	}

	uploadFailServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
				w.WriteHeader(http.StatusBadRequest)
			} else {
				if _, err := w.Write(jobBody); err != nil {
					panic(err.Error())
				}
			}
		}),
	)
	uploadFailSfAuth := authentication{
		InstanceUrl: uploadFailServer.URL,
		AccessToken: "accesstokenvalue",
	}

	type args struct {
		sf             *Salesforce
		sObjectName    string
		fieldName      string
		operation      string
		filePath       string
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "bulk_insert_batch_size_200",
			args: args{
				sf:             buildSalesforceStruct(&sfAuth),
				sObjectName:    "Account",
				fieldName:      "",
				operation:      insertOperation,
				filePath:       "data/data.csv",
				batchSize:      200,
				waitForResults: false,
			},
			want:    []string{job.Id},
			wantErr: false,
		},
		{
			name: "bulk_insert_batch_size_1",
			args: args{
				sf:             buildSalesforceStruct(&sfAuth),
				sObjectName:    "Account",
				fieldName:      "",
				operation:      insertOperation,
				filePath:       "data/data.csv",
				batchSize:      1,
				waitForResults: false,
			},
			want:    []string{job.Id, job.Id},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:             buildSalesforceStruct(&badReqSfAuth),
				sObjectName:    "Account",
				fieldName:      "externalId",
				operation:      upsertOperation,
				filePath:       "data/data.csv",
				batchSize:      1,
				waitForResults: false,
			},
			wantErr: true,
		},
		{
			name: "bulk_insert_wait_for_results",
			args: args{
				sf:             buildSalesforceStruct(&waitingSfAuth),
				sObjectName:    "Account",
				fieldName:      "",
				operation:      insertOperation,
				filePath:       "data/data.csv",
				batchSize:      200,
				waitForResults: true,
			},
			want:    []string{job.Id},
			wantErr: false,
		},
		{
			name: "bad_request_upload_fail",
			args: args{
				sf:             buildSalesforceStruct(&uploadFailSfAuth),
				sObjectName:    "Account",
				fieldName:      "",
				operation:      insertOperation,
				filePath:       "data/data.csv",
				batchSize:      200,
				waitForResults: false,
			},
			want:    []string{job.Id},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doBulkJobWithFile(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.fieldName,
				tt.args.operation,
				tt.args.filePath,
				tt.args.batchSize,
				tt.args.waitForResults,
				"",
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doBulkJobWithFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doBulkJobWithFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doQueryBulk(t *testing.T) {
	job := bulkJob{
		Id:    "1234",
		State: jobStateJobComplete,
	}
	jobCreationRespBody, _ := json.Marshal(job)
	badJob := bulkJob{
		Id:    "",
		State: jobStateJobComplete,
	}
	badJobCreationRespBody, _ := json.Marshal(badJob)

	badJobCreationServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-6:] == "/query" {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write(badJobCreationRespBody); err != nil {
					t.Fatal(err.Error())
				}
			}
		}),
	)
	defer badJobCreationServer.Close()
	badJobCreationSfAuth := authentication{
		InstanceUrl: badJobCreationServer.URL,
		AccessToken: "accesstokenvalue",
	}

	badResultsServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.RequestURI[len(r.RequestURI)-6:] == "/query" {
				w.WriteHeader(http.StatusOK)
				if _, err := w.Write(jobCreationRespBody); err != nil {
					t.Fatal(err.Error())
				}
			} else if r.RequestURI[len(r.RequestURI)-8:] == "/results" {
				w.WriteHeader(http.StatusBadRequest)
			}
		}),
	)
	defer badResultsServer.Close()
	badResultsSfAuth := authentication{
		InstanceUrl: badResultsServer.URL,
		AccessToken: "accesstokenvalue",
	}

	type args struct {
		sf       *Salesforce
		filePath string
		query    string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "bad_job_creation",
			args: args{
				sf:       buildSalesforceStruct(&badJobCreationSfAuth),
				filePath: "data/data.csv",
				query:    "SELECT Id FROM Account",
			},
			wantErr: true,
		},
		{
			name: "get_results_fail",
			args: args{
				sf:       buildSalesforceStruct(&badResultsSfAuth),
				filePath: "data/data.csv",
				query:    "SELECT Id FROM Account",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doQueryBulk(tt.args.sf, tt.args.filePath, tt.args.query); (err != nil) != tt.wantErr {
				t.Errorf("doQueryBulk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getJobRecordResults(t *testing.T) {
	csvData := `"name"` + "\n" + `"test account"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(csvData)); err != nil {
			t.Fatal(err.Error())
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badRequestServer, badRequestAuth := setupTestServer("", http.StatusBadRequest)
	defer badRequestServer.Close()

	successThenFailServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.RequestURI, successfulResults) {
				if _, err := w.Write([]byte(csvData)); err != nil {
					t.Fatal(err.Error())
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}),
	)
	successThenFailAuth := authentication{
		InstanceUrl: successThenFailServer.URL,
		AccessToken: "accesstokenvalue",
	}
	defer successThenFailServer.Close()

	type args struct {
		sf             *Salesforce
		bulkJobResults BulkJobResults
	}
	tests := []struct {
		name    string
		args    args
		want    BulkJobResults
		wantErr bool
	}{
		{
			name: "successful_get_job_record_results",
			args: args{
				sf:             buildSalesforceStruct(&sfAuth),
				bulkJobResults: BulkJobResults{Id: "1234"},
			},
			want: BulkJobResults{
				Id: "1234",
				FailedRecords: []map[string]any{{
					"name": "test account",
				}},
				SuccessfulRecords: []map[string]any{{
					"name": "test account",
				}},
			},
			wantErr: false,
		},
		{
			name: "failed_to_get_successful_records",
			args: args{
				sf:             buildSalesforceStruct(&badRequestAuth),
				bulkJobResults: BulkJobResults{Id: "1234"},
			},
			want:    BulkJobResults{Id: "1234"},
			wantErr: true,
		},
		{
			name: "failed_to_get_failed_records",
			args: args{
				sf:             buildSalesforceStruct(&successThenFailAuth),
				bulkJobResults: BulkJobResults{Id: "1234"},
			},
			want: BulkJobResults{
				Id: "1234",
				SuccessfulRecords: []map[string]any{{
					"name": "test account",
				}},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getJobRecordResults(tt.args.sf, tt.args.bulkJobResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("getJobRecordResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getJobRecordResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getBulkJobRecords(t *testing.T) {
	csvData := `"name"` + "\n" + `"test account"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(csvData)); err != nil {
			t.Fatal(err.Error())
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badReqServer, badReqAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	badDataServer := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte("name,type\ntest")); err != nil {
				t.Fatal(err.Error())
			}
		}),
	)
	badDataAuth := authentication{
		InstanceUrl: badDataServer.URL,
		AccessToken: "accesstokenvalue",
	}
	defer badDataServer.Close()

	type args struct {
		sf         *Salesforce
		bulkJobId  string
		resultType string
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]any
		wantErr bool
	}{
		{
			name: "successful_get_failed_job_records",
			args: args{
				sf:         buildSalesforceStruct(&sfAuth),
				bulkJobId:  "1234",
				resultType: failedResults,
			},
			want: []map[string]any{{
				"name": "test account",
			}},
			wantErr: false,
		},
		{
			name: "failed_bad_request",
			args: args{
				sf:         buildSalesforceStruct(&badReqAuth),
				bulkJobId:  "1234",
				resultType: failedResults,
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "failed_conversion",
			args: args{
				sf:         buildSalesforceStruct(&badDataAuth),
				bulkJobId:  "1234",
				resultType: failedResults,
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getBulkJobRecords(tt.args.sf, tt.args.bulkJobId, tt.args.resultType)
			if (err != nil) != tt.wantErr {
				t.Errorf("getBulkJobRecords() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getBulkJobRecords() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_csvToMap(t *testing.T) {
	type args struct {
		reader csv.Reader
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]any
		wantErr bool
	}{
		{
			name: "successful_csv_to_map_conversion",
			args: args{
				reader: *csv.NewReader(strings.NewReader("name\ntest account")),
			},
			want: []map[string]any{{
				"name": "test account",
			}},
			wantErr: false,
		},
		{
			name: "successful_empty_csv",
			args: args{
				reader: *csv.NewReader(strings.NewReader("")),
			},
			want:    nil,
			wantErr: false,
		},
		{
			name: "failed_conversion",
			args: args{
				reader: *csv.NewReader(strings.NewReader("name,type\nshop")),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := csvToMap(tt.args.reader)
			if (err != nil) != tt.wantErr {
				t.Errorf("csvToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("csvToMap() = %v, want %v", got, tt.want)
			}
		})
	}
}
