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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := processCompositeResponse(tt.args.resp); (err != nil) != tt.wantErr {
				t.Errorf("processCompositeResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
