package salesforce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"testing"
)

func Test_validateNumberOfSubrequests(t *testing.T) {
	type args struct {
		dataSize  int
		batchSize int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success_max",
			args: args{
				dataSize:  5000,
				batchSize: 200,
			},
			wantErr: false,
		},
		{
			name: "validation_success_min",
			args: args{
				dataSize:  1,
				batchSize: 200,
			},
			wantErr: false,
		},
		{
			name: "validation_fail_5001",
			args: args{
				dataSize:  5001,
				batchSize: 200,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateNumberOfSubrequests(tt.args.dataSize, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("validateNumberOfSubrequests() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_createCompositeRequestForCollection(t *testing.T) {
	recordMap := []map[string]any{
		{
			"Id":   "1234",
			"Name": "test account 1",
		},
		{
			"Id":   "5678",
			"Name": "test account 2",
		},
	}

	type args struct {
		method    string
		url       string
		allOrNone bool
		batchSize int
		recordMap []map[string]any
	}
	tests := []struct {
		name    string
		args    args
		want    compositeRequest
		wantErr bool
	}{
		{
			name: "create_multiple_requests",
			args: args{
				method:    http.MethodPatch,
				url:       "example.com",
				allOrNone: true,
				batchSize: 1,
				recordMap: recordMap,
			},
			want: compositeRequest{
				AllOrNone: true,
				CompositeRequest: []compositeSubRequest{
					{
						Body: sObjectCollection{
							AllOrNone: true,
							Records:   recordMap[:1],
						},
						Method:      http.MethodPatch,
						Url:         "example.com",
						ReferenceId: "refObj0",
					},
					{
						Body: sObjectCollection{
							AllOrNone: true,
							Records:   recordMap[1:],
						},
						Method:      http.MethodPatch,
						Url:         "example.com",
						ReferenceId: "refObj1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "create_one_request",
			args: args{
				method:    http.MethodPatch,
				url:       "example.com",
				allOrNone: true,
				batchSize: 1,
				recordMap: recordMap[1:],
			},
			want: compositeRequest{
				AllOrNone: true,
				CompositeRequest: []compositeSubRequest{
					{
						Body: sObjectCollection{
							AllOrNone: true,
							Records:   recordMap[1:],
						},
						Method:      http.MethodPatch,
						Url:         "example.com",
						ReferenceId: "refObj0",
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createCompositeRequestForCollection(tt.args.method, tt.args.url, tt.args.allOrNone, tt.args.batchSize, tt.args.recordMap)
			if (err != nil) != tt.wantErr {
				t.Errorf("createCompositeRequestForCollection() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createCompositeRequestForCollection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processCompositeResponse(t *testing.T) {
	message := []salesforceErrorMessage{{
		Message:    "example error",
		StatusCode: "500",
		Fields:     []string{"Name: bad name"},
	}}
	exampleError := []salesforceError{{
		Id:      "12345",
		Errors:  message,
		Success: false,
	}}
	compSubResults := []composteSubRequestResult{
		{
			Body:           exampleError,
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusBadRequest,
			ReferenceId:    "ref0",
		},
	}
	compResult := compositeRequestResult{
		CompositeResponse: compSubResults,
	}
	jsonBody, _ := json.Marshal(compResult)
	body := io.NopCloser(bytes.NewReader(jsonBody))
	httpresp := http.Response{
		Status:     fmt.Sprint(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
		Body:       body,
	}

	exampleErrorNoMessage := []salesforceError{{
		Id:      "12345",
		Errors:  []salesforceErrorMessage{},
		Success: false,
	}}
	compSubResultsNoMessage := []composteSubRequestResult{
		{
			Body:           exampleErrorNoMessage,
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusBadRequest,
			ReferenceId:    "ref0",
		},
	}
	compResultNoMessage := compositeRequestResult{
		CompositeResponse: compSubResultsNoMessage,
	}
	jsonBodyNoError, _ := json.Marshal(compResultNoMessage)
	bodyNoError := io.NopCloser(bytes.NewReader(jsonBodyNoError))
	httprespNoError := http.Response{
		Status:     fmt.Sprint(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
		Body:       bodyNoError,
	}

	type args struct {
		resp http.Response
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "process_500_error",
			args: args{
				resp: httpresp,
			},
			wantErr: true,
		},
		{
			name: "process_500_error_no_error_message",
			args: args{
				resp: httprespNoError,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := processCompositeResponse(tt.args.resp); (err != nil) != tt.wantErr {
				t.Errorf("processCompositeResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doCompositeRequest(t *testing.T) {
	compReqResultSuccess := compositeRequestResult{
		CompositeResponse: []composteSubRequestResult{{
			Body:           []salesforceError{{Success: true}},
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusOK,
			ReferenceId:    "sobject",
		}},
	}

	compReqResultFail := compositeRequestResult{
		CompositeResponse: []composteSubRequestResult{{
			Body: []salesforceError{{
				Success: false,
				Errors: []salesforceErrorMessage{{
					Message:    "error",
					StatusCode: "500",
				}},
			}},
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusBadRequest,
			ReferenceId:    "sobject",
		}},
	}

	server, sfAuth := setupTestServer(compReqResultSuccess, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer(compReqResultSuccess, http.StatusBadRequest)
	defer badReqServer.Close()

	sfErrorServer, sfErrorSfAuth := setupTestServer(compReqResultFail, http.StatusOK)
	defer sfErrorServer.Close()

	compReq := compositeRequest{
		AllOrNone: true,
		CompositeRequest: []compositeSubRequest{{
			Body: sObjectCollection{
				AllOrNone: true,
				Records:   []map[string]any{{"Id": "1234"}},
			},
			Method:      http.MethodPost,
			Url:         "endpoint/",
			ReferenceId: "sobject",
		}},
	}

	type args struct {
		auth    authentication
		compReq compositeRequest
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_request",
			args: args{
				auth:    sfAuth,
				compReq: compReq,
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:    badReqSfAuth,
				compReq: compReq,
			},
			wantErr: true,
		},
		{
			name: "salesforce_errors",
			args: args{
				auth:    sfErrorSfAuth,
				compReq: compReq,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doCompositeRequest(tt.args.auth, tt.args.compReq); (err != nil) != tt.wantErr {
				t.Errorf("doCompositeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doInsertComposite(t *testing.T) {
	type account struct {
		Name string
	}

	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert_composite",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Name: "test account 1",
					},
					{
						Name: "test account 2",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doInsertComposite(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.allOrNone, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doInsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpdateComposite(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}

	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_update_composite",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Id:   "1234",
						Name: "test account 1",
					},
					{
						Id:   "5678",
						Name: "test account 2",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doUpdateComposite(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.allOrNone, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doUpdateComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpsertComposite(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}

	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		fieldName   string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert_composite",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				records: []account{
					{
						ExternalId__c: "1234",
						Name:          "test account 1",
					},
					{
						ExternalId__c: "5678",
						Name:          "test account 2",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
		{
			name: "fail_no_external_id",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doUpsertComposite(tt.args.auth, tt.args.sObjectName, tt.args.fieldName, tt.args.records, tt.args.allOrNone, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doUpsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doDeleteComposite(t *testing.T) {
	type account struct {
		Id string
	}

	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete_composite_single_batch",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Id: "1234",
					},
					{
						Id: "5678",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: false,
		},
		{
			name: "successful_delete_composite_multi_batch",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Id: "1234",
					},
					{
						Id: "5678",
					},
				},
				batchSize: 1,
				allOrNone: true,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				records:     []account{{}},
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := doDeleteComposite(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.allOrNone, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doDeleteComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
