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
			got, err := createCompositeRequestForCollection(
				tt.args.method,
				tt.args.url,
				tt.args.allOrNone,
				tt.args.batchSize,
				tt.args.recordMap,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"createCompositeRequestForCollection() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("createCompositeRequestForCollection() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processCompositeResponse(t *testing.T) {
	message := []SalesforceErrorMessage{{
		Message:    "example error",
		StatusCode: "500",
		Fields:     []string{"Name: bad name"},
	}}
	exampleError := []SalesforceResult{{
		Id:      "12345",
		Errors:  message,
		Success: false,
	}}
	compSubResults := []compositeSubRequestResult{
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
	httpResp := http.Response{
		Status:     fmt.Sprint(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
		Body:       body,
	}

	exampleErrorNoMessage := []SalesforceResult{{
		Id:      "12345",
		Errors:  []SalesforceErrorMessage{},
		Success: false,
	}}
	compSubResultsNoMessage := []compositeSubRequestResult{
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
	httpRespNoError := http.Response{
		Status:     fmt.Sprint(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
		Body:       bodyNoError,
	}

	type args struct {
		resp      http.Response
		allOrNone bool
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "process_500_error",
			args: args{
				resp: httpResp,
			},
			want: SalesforceResults{
				Results:             exampleError,
				HasSalesforceErrors: true,
			},
			wantErr: false,
		},
		{
			name: "process_500_error_no_error_message",
			args: args{
				resp: httpRespNoError,
			},
			want: SalesforceResults{
				Results:             exampleErrorNoMessage,
				HasSalesforceErrors: true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processCompositeResponse(tt.args.resp, tt.args.allOrNone)
			if (err != nil) != tt.wantErr {
				t.Errorf("processCompositeResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("processCompositeResponse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doCompositeRequest(t *testing.T) {
	compReqResultSuccess := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body:           []SalesforceResult{{Success: true}},
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusOK,
			ReferenceId:    "sobject",
		}},
	}

	sfResultsFail := SalesforceResults{
		Results: []SalesforceResult{{
			Success: false,
			Errors: []SalesforceErrorMessage{{
				Message:    "error",
				StatusCode: "500",
			}},
		}},
		HasSalesforceErrors: true,
	}

	compReqResultFail := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body:           sfResultsFail.Results,
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
		sf      *Salesforce
		compReq compositeRequest
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "successful_request",
			args: args{
				sf:      buildSalesforceStruct(&sfAuth),
				compReq: compReq,
			},
			want: SalesforceResults{
				Results:             []SalesforceResult{{Success: true}},
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				sf:      buildSalesforceStruct(&badReqSfAuth),
				compReq: compReq,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
		{
			name: "salesforce_errors",
			args: args{
				sf:      buildSalesforceStruct(&sfErrorSfAuth),
				compReq: compReq,
			},
			want:    sfResultsFail,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doCompositeRequest(tt.args.sf, tt.args.compReq)
			if (err != nil) != tt.wantErr {
				t.Errorf("doCompositeRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doCompositeRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doInsertComposite(t *testing.T) {
	type account struct {
		Name string
	}

	compResult := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body: []SalesforceResult{{
				Id:      "1234",
				Errors:  []SalesforceErrorMessage{},
				Success: true,
			}},
		}},
	}

	server, sfAuth := setupTestServer(compResult, http.StatusOK)
	defer server.Close()

	type args struct {
		sf          *Salesforce
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "successful_insert_composite",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want: SalesforceResults{
				Results:             compResult.CompositeResponse[0].Body,
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doInsertComposite(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.records,
				tt.args.allOrNone,
				tt.args.batchSize,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doInsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doInsertComposite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doUpdateComposite(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}

	compResult := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body: []SalesforceResult{{
				Id:      "1234",
				Errors:  []SalesforceErrorMessage{},
				Success: true,
			}},
		}},
	}

	server, sfAuth := setupTestServer(compResult, http.StatusOK)
	defer server.Close()

	type args struct {
		sf          *Salesforce
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "successful_update_composite",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want: SalesforceResults{
				Results:             compResult.CompositeResponse[0].Body,
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize: 200,
				allOrNone: true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doUpdateComposite(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.records,
				tt.args.allOrNone,
				tt.args.batchSize,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doUpdateComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doUpdateComposite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doUpsertComposite(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}

	compResult := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body: []SalesforceResult{{
				Id:      "1234",
				Errors:  []SalesforceErrorMessage{},
				Success: true,
			}},
		}},
	}

	server, sfAuth := setupTestServer(compResult, http.StatusOK)
	defer server.Close()

	type args struct {
		sf          *Salesforce
		sObjectName string
		fieldName   string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "successful_upsert_composite",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want: SalesforceResults{
				Results:             compResult.CompositeResponse[0].Body,
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
		{
			name: "fail_no_external_id",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want:    SalesforceResults{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doUpsertComposite(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.fieldName,
				tt.args.records,
				tt.args.allOrNone,
				tt.args.batchSize,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doUpsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doUpsertComposite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_doDeleteComposite(t *testing.T) {
	type account struct {
		Id string
	}

	compResult := compositeRequestResult{
		CompositeResponse: []compositeSubRequestResult{{
			Body: []SalesforceResult{{
				Id:      "1234",
				Errors:  []SalesforceErrorMessage{},
				Success: true,
			}},
		}},
	}

	server, sfAuth := setupTestServer(compResult, http.StatusOK)
	defer server.Close()

	type args struct {
		sf          *Salesforce
		sObjectName string
		records     any
		allOrNone   bool
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		want    SalesforceResults
		wantErr bool
	}{
		{
			name: "successful_delete_composite_single_batch",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want: SalesforceResults{
				Results:             compResult.CompositeResponse[0].Body,
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "successful_delete_composite_multi_batch",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
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
			want: SalesforceResults{
				Results:             compResult.CompositeResponse[0].Body,
				HasSalesforceErrors: false,
			},
			wantErr: false,
		},
		{
			name: "bad_data",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				records:     "1",
				batchSize:   200,
				allOrNone:   true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			args: args{
				sf:          buildSalesforceStruct(&sfAuth),
				sObjectName: "Account",
				records:     []account{{}},
				batchSize:   200,
				allOrNone:   true,
			},
			want:    SalesforceResults{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doDeleteComposite(
				tt.args.sf,
				tt.args.sObjectName,
				tt.args.records,
				tt.args.allOrNone,
				tt.args.batchSize,
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("doDeleteComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("doDeleteComposite() = %v, want %v", got, tt.want)
			}
		})
	}
}
