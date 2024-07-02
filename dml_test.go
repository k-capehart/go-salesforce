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

func Test_convertToMap(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	type args struct {
		obj any
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]any
		wantErr bool
	}{
		{
			name: "convert_account_to_map",
			args: args{obj: account{
				Id:   "1234",
				Name: "test account",
			}},
			want: map[string]any{
				"Id":   "1234",
				"Name": "test account",
			},
			wantErr: false,
		},
		{
			name:    "convert_fail",
			args:    args{obj: 1},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToMap(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToMap() = %v, want %v", got, tt.want)
			}
		})
	}

}

func Test_convertToSliceOfMaps(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	type args struct {
		obj any
	}
	tests := []struct {
		name    string
		args    args
		want    []map[string]any
		wantErr bool
	}{
		{
			name: "convert_account_slice_to_slice_of_maps",
			args: args{obj: []account{
				{
					Id:   "1234",
					Name: "test account 1",
				},
				{
					Id:   "5678",
					Name: "test account 2",
				},
			}},
			want: []map[string]any{
				{
					"Id":   "1234",
					"Name": "test account 1",
				},
				{
					"Id":   "5678",
					"Name": "test account 2",
				},
			},
			wantErr: false,
		},
		{
			name:    "convert_fail",
			args:    args{obj: 1},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertToSliceOfMaps(tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToSliceOfMaps() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertToSliceOfMaps() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_processSalesforceResponse(t *testing.T) {
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
	jsonBody, _ := json.Marshal(exampleError)
	body := io.NopCloser(bytes.NewReader(jsonBody))
	httpResp := http.Response{
		Status:     fmt.Sprint(http.StatusInternalServerError),
		StatusCode: http.StatusInternalServerError,
		Body:       body,
	}
	type args struct {
		resp http.Response
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "process_500_error",
			args: args{
				resp: httpResp,
			},
			want:    "500: example error 12345",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := processSalesforceResponse(tt.args.resp)
			if err != nil != tt.wantErr {
				t.Errorf("processSalesforceResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(err.Error(), tt.want) {
				t.Errorf("processSalesforceResponse() = %v, want %v", err.Error(), tt.want)
			}
		})
	}
}

func Test_doBatchedRequestsForCollection(t *testing.T) {
	server, sfAuth := setupTestServer([]SalesforceResult{{Success: true}}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]SalesforceResult{}, http.StatusBadRequest)
	defer badReqServer.Close()

	sfError := []SalesforceResult{{
		Id: "1234",
		Errors: []SalesforceErrorMessage{{
			Message:    "error",
			StatusCode: "400",
		}},
		Success: false,
	}}

	sfErrorServer, sfErrorSfAuth := setupTestServer(sfError, http.StatusOK)
	defer sfErrorServer.Close()

	type args struct {
		auth      authentication
		method    string
		url       string
		batchSize int
		recordMap []map[string]any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "single_record",
			args: args{
				auth:      sfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 200,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple_batches",
			args: args{
				auth:      sfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 1,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
					{
						"Name": "test record 2",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:      badReqSfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 1,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "salesforce_error",
			args: args{
				auth:      sfErrorSfAuth,
				method:    http.MethodPost,
				url:       "",
				batchSize: 1,
				recordMap: []map[string]any{
					{
						"Name": "test record 1",
					},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doBatchedRequestsForCollection(tt.args.auth, tt.args.method, tt.args.url, tt.args.batchSize, tt.args.recordMap); (err != nil) != tt.wantErr {
				t.Errorf("doBatchedRequestsForCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doInsertOne(t *testing.T) {
	type account struct {
		Name string
	}

	server, sfAuth := setupTestServer("", http.StatusCreated)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	type args struct {
		auth        authentication
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record: account{
					Name: "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:        badReqSfAuth,
				sObjectName: "Account",
				record: account{
					Name: "test account",
				},
			},
			wantErr: true,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record:      "1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doInsertOne(tt.args.auth, tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("doInsertOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpdateOne(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}

	server, sfAuth := setupTestServer("", http.StatusNoContent)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	type args struct {
		auth        authentication
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_update",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record: account{
					Id:   "1234",
					Name: "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:        badReqSfAuth,
				sObjectName: "Account",
				record: account{
					Id:   "1234",
					Name: "test account",
				},
			},
			wantErr: true,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record:      "1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doUpdateOne(tt.args.auth, tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("doUpdateOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpsertOne(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}

	server, sfAuth := setupTestServer("", http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	type args struct {
		auth        authentication
		sObjectName string
		fieldName   string
		record      any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				record: account{
					ExternalId__c: "1234",
					Name:          "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:        badReqSfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				record: account{
					ExternalId__c: "1234",
					Name:          "test account",
				},
			},
			wantErr: true,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				fieldName:   "ExternalId__c",
				record:      "1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doUpsertOne(tt.args.auth, tt.args.sObjectName, tt.args.fieldName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("doUpsertOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doDeleteOne(t *testing.T) {
	type account struct {
		Id string
	}

	server, sfAuth := setupTestServer("", http.StatusNoContent)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badReqServer.Close()

	type args struct {
		auth        authentication
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record: account{
					Id: "1234",
				},
			},
			wantErr: false,
		},
		{
			name: "bad_request",
			args: args{
				auth:        badReqSfAuth,
				sObjectName: "Account",
				record: account{
					Id: "1234",
				},
			},
			wantErr: true,
		},
		{
			name: "bad_data",
			args: args{
				auth:        sfAuth,
				sObjectName: "Account",
				record:      "1",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doDeleteOne(tt.args.auth, tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("doDeleteOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doInsertCollection(t *testing.T) {
	type account struct {
		Name string
	}

	server, sfAuth := setupTestServer([]SalesforceResult{{Success: true}}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert_collection",
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
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doInsertCollection(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doInsertCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpdateCollection(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}

	server, sfAuth := setupTestServer([]SalesforceResult{{Success: true}}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_update_collection",
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
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doUpdateCollection(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doUpdateCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doUpsertCollection(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}

	server, sfAuth := setupTestServer([]SalesforceResult{{Success: true}}, http.StatusOK)
	defer server.Close()

	type args struct {
		auth        authentication
		sObjectName string
		fieldName   string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert_collection",
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
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doUpsertCollection(tt.args.auth, tt.args.sObjectName, tt.args.fieldName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doUpsertCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_doDeleteCollection(t *testing.T) {
	type account struct {
		Id string
	}

	server, sfAuth := setupTestServer([]SalesforceResult{{Success: true}}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]SalesforceResult{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	sfErrorServer, sfErrorSfAuth := setupTestServer(
		[]SalesforceResult{{
			Success: false,
			Errors: []SalesforceErrorMessage{{
				Message:    "error",
				StatusCode: "500",
			}},
		}},
		http.StatusOK,
	)
	defer sfErrorServer.Close()

	type args struct {
		auth        authentication
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete_collection_single_batch",
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
			},
			wantErr: false,
		},
		{
			name: "successful_delete_collection_multi_batch",
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
			},
			wantErr: true,
		},
		{
			name: "bad_request",
			args: args{
				auth:        badReqSfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Id: "1234",
					},
				},
				batchSize: 1,
			},
			wantErr: true,
		},
		{
			name: "salesforce_errors",
			args: args{
				auth:        sfErrorSfAuth,
				sObjectName: "Account",
				records: []account{
					{
						Id: "1234",
					},
				},
				batchSize: 1,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := doDeleteCollection(tt.args.auth, tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("doDeleteCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
