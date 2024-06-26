package salesforce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func setupTestServer(body any, status int) (*httptest.Server, authentication) {
	respBody, _ := json.Marshal(body)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI[len(r.RequestURI)-8:] == "/batches" {
			w.WriteHeader(http.StatusCreated)
		} else {
			w.WriteHeader(status)
			if body != "" {
				if _, err := w.Write(respBody); err != nil {
					panic(err.Error())
				}
			}
		}
	}))

	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	return server, sfAuth
}

func Test_doRequest(t *testing.T) {
	server, sfAuth := setupTestServer("", http.StatusOK)
	defer server.Close()
	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	type args struct {
		method         string
		uri            string
		content        string
		auth           authentication
		body           string
		expectedStatus int
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "make_generic_http_call_ok",
			args: args{
				method:         http.MethodGet,
				uri:            "",
				content:        jsonType,
				auth:           sfAuth,
				body:           "",
				expectedStatus: http.StatusOK,
			},
			want:    http.StatusOK,
			wantErr: false,
		},
		{
			name: "make_generic_http_call_bad_request",
			args: args{
				method:  http.MethodGet,
				uri:     "",
				content: jsonType,
				auth:    badSfAuth,
				body:    "",
			},
			want:    http.StatusBadRequest,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doRequest(tt.args.method, tt.args.uri, tt.args.content, tt.args.auth, tt.args.body, tt.args.expectedStatus)
			if (err != nil) != tt.wantErr {
				t.Errorf("doRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.StatusCode, tt.want) {
				t.Errorf("doRequest() = %v, want %v", got.StatusCode, tt.want)
			}
		})
	}
}

func Test_validateOfTypeSlice(t *testing.T) {
	type args struct {
		data any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success",
			args: args{
				data: []int{0},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			args: args{
				data: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateOfTypeSlice(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("validateOfTypeSlice() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateOfTypeStruct(t *testing.T) {
	type testStruct struct{}
	type args struct {
		data any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success",
			args: args{
				data: testStruct{},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			args: args{
				data: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateOfTypeStruct(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("validateOfTypeStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateOfTypeStructOrMap(t *testing.T) {
	type testStruct struct{}
	type args struct {
		data any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success_struct",
			args: args{
				data: testStruct{},
			},
			wantErr: false,
		},
		{
			name: "validation_success_struct",
			args: args{
				data: map[string]string{},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			args: args{
				data: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateOfTypeStructOrMap(tt.args.data); (err != nil) != tt.wantErr {
				t.Errorf("validateOfTypeStructOrMap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateBatchSizeWithinRange(t *testing.T) {
	type args struct {
		batchSize int
		max       int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success_min",
			args: args{
				batchSize: 1,
				max:       200,
			},
			wantErr: false,
		},
		{
			name: "validation_success_max",
			args: args{
				batchSize: 200,
				max:       200,
			},
			wantErr: false,
		},
		{
			name: "validation_fail_0",
			args: args{
				batchSize: 0,
				max:       200,
			},
			wantErr: true,
		},
		{
			name: "validation_fail_201",
			args: args{
				batchSize: 201,
				max:       200,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBatchSizeWithinRange(tt.args.batchSize, tt.args.max); (err != nil) != tt.wantErr {
				t.Errorf("validateBatchSizeWithinRange() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_processSalesforceError(t *testing.T) {
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
				resp: http.Response{
					Status:     "500",
					StatusCode: 500,
					Body:       io.NopCloser(strings.NewReader("error message")),
				},
			},
			want:    "500: error message",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := processSalesforceError(tt.args.resp)
			if err != nil != tt.wantErr {
				t.Errorf("processSalesforceError() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(err.Error(), tt.want) {
				t.Errorf("processSalesforceError() = %v, want %v", err.Error(), tt.want)
			}
		})
	}
}

func Test_processSalesforceResponse(t *testing.T) {
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
			err := processSalesforceResponse(tt.args.resp)
			if err != nil != tt.wantErr {
				t.Errorf("processSalesforceResponse() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(err.Error(), tt.want) {
				t.Errorf("processSalesforceResponse() = %v, want %v", err.Error(), tt.want)
			}
		})
	}
}

func TestInit(t *testing.T) {
	sfAuth := authentication{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	server, _ := setupTestServer(sfAuth, http.StatusOK)
	defer server.Close()

	type args struct {
		creds Creds
	}
	tests := []struct {
		name    string
		args    args
		want    *Salesforce
		wantErr bool
	}{
		{
			name:    "authentication_failure",
			args:    args{Creds{}},
			want:    nil,
			wantErr: true,
		},
		{
			name: "authentication_username_password",
			args: args{creds: Creds{
				Domain:         server.URL,
				Username:       "u",
				Password:       "p",
				SecurityToken:  "t",
				ConsumerKey:    "key",
				ConsumerSecret: "secret",
			}},
			want:    &Salesforce{auth: &sfAuth, AccessToken: sfAuth.AccessToken},
			wantErr: false,
		},
		{
			name: "authentication_client_credentials",
			args: args{creds: Creds{
				Domain:         server.URL,
				ConsumerKey:    "key",
				ConsumerSecret: "secret",
			}},
			want:    &Salesforce{auth: &sfAuth, AccessToken: sfAuth.AccessToken},
			wantErr: false,
		},
		{
			name: "authentication_access_token",
			args: args{creds: Creds{
				Domain:      server.URL,
				AccessToken: "1234",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Init(tt.args.creds)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.want != nil && !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Init() = %v, want %v", *got.auth, *tt.want.auth)
			}
		})
	}
}

func Test_validateSingles(t *testing.T) {
	type account struct{}

	type args struct {
		sf     Salesforce
		record any
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				record: account{},
			},
			wantErr: false,
		},
		{
			name: "validation_fail_auth",
			args: args{
				sf:     Salesforce{},
				record: account{},
			},
			wantErr: true,
		},
		{
			name: "validation_fail_type",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				record: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateSingles(tt.args.sf, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("validateSingles() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateCollections(t *testing.T) {
	type account struct{}

	type args struct {
		sf        Salesforce
		records   any
		batchSize int
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   []account{},
				batchSize: 200,
			},
			wantErr: false,
		},
		{
			name: "validation_fail_auth",
			args: args{
				sf:        Salesforce{},
				records:   []account{},
				batchSize: 200,
			},
			wantErr: true,
		},
		{
			name: "validation_fail_type",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   0,
				batchSize: 200,
			},
			wantErr: true,
		},
		{
			name: "validation_fail_batch_size",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   []account{},
				batchSize: 0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateCollections(tt.args.sf, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("validateCollections() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_validateBulk(t *testing.T) {
	type account struct{}

	type args struct {
		sf        Salesforce
		records   any
		batchSize int
		isFile    bool
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "validation_success",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   []account{},
				batchSize: 10000,
				isFile:    false,
			},
			wantErr: false,
		},
		{
			name: "validation_fail_auth",
			args: args{
				sf:        Salesforce{},
				records:   []account{},
				batchSize: 10000,
				isFile:    false,
			},
			wantErr: true,
		},
		{
			name: "validation_fail_type",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   0,
				batchSize: 10000,
				isFile:    false,
			},
			wantErr: true,
		},
		{
			name: "validation_fail_batch_size",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   []account{},
				batchSize: 0,
				isFile:    false,
			},
			wantErr: true,
		},
		{
			name: "validation_success_file",
			args: args{
				sf: Salesforce{auth: &authentication{
					AccessToken: "1234",
				}},
				records:   nil,
				batchSize: 2000,
				isFile:    true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateBulk(tt.args.sf, tt.args.records, tt.args.batchSize, tt.args.isFile); (err != nil) != tt.wantErr {
				t.Errorf("validateBulk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_DoRequest(t *testing.T) {
	server, sfAuth := setupTestServer("response_body", http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		method string
		uri    string
		body   []byte
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *http.Response
		wantErr bool
	}{
		{
			name: "successful_request",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				method: http.MethodGet,
				uri:    "/request",
				body:   []byte("example"),
			},
			want: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("\"response_body\"")),
			},
			wantErr: false,
		},
		{
			name: "validation_fail_auth",
			fields: fields{
				auth: nil,
			},
			args: args{
				method: http.MethodGet,
				uri:    "/request",
				body:   []byte("example"),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.DoRequest(tt.args.method, tt.args.uri, tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DoRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != nil {
				gotBody, _ := io.ReadAll(got.Body)
				wantBody, _ := io.ReadAll(tt.want.Body)
				if got.StatusCode != tt.want.StatusCode || string(gotBody) != string(wantBody) {
					t.Errorf("Salesforce.DoRequest() = %v %v, want %v %v", got.StatusCode, string(gotBody), tt.want.StatusCode, string(wantBody))
				}
			} else if !tt.wantErr {
				t.Error("Salesforce.DoRequest() did not return a response")
			}
		})
	}
}

func TestSalesforce_Query(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	resp := queryResponse{
		TotalSize: 1,
		Done:      true,
		Records: []map[string]any{{
			"Id":   "123abc",
			"Name": "test account",
		}},
	}
	server, sfAuth := setupTestServer(resp, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		query   string
		sObject *[]account
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []account
		wantErr bool
	}{
		{
			name: "validation_fail",
			fields: fields{
				auth: nil,
			},
			args: args{
				query:   "SELECT Id FROM Account",
				sObject: &[]account{},
			},
			want:    []account{},
			wantErr: true,
		},
		{
			name: "successful_query",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				query:   "SELECT Id FROM Account",
				sObject: &[]account{},
			},
			want: []account{{
				Id:   "123abc",
				Name: "test account",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.Query(tt.args.query, tt.args.sObject); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.Query() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.sObject, &tt.want) {
				t.Errorf("Salesforce.Query() = %v, want %v", tt.args.sObject, tt.want)
			}
		})
	}
}

func TestSalesforce_QueryStruct(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	resp := queryResponse{
		TotalSize: 1,
		Done:      true,
		Records: []map[string]any{{
			"Id":   "123abc",
			"Name": "test account",
		}},
	}
	server, sfAuth := setupTestServer(resp, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		soqlStruct any
		sObject    any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []account
		wantErr bool
	}{
		{
			name: "validation_fail",
			fields: fields{
				auth: nil,
			},
			args: args{
				soqlStruct: account{},
				sObject:    &[]account{},
			},
			want:    []account{},
			wantErr: true,
		},
		{
			name: "successful_query",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				soqlStruct: account{},
				sObject:    &[]account{},
			},
			want: []account{{
				Id:   "123abc",
				Name: "test account",
			}},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.QueryStruct(tt.args.soqlStruct, tt.args.sObject); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.QueryStruct() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.sObject, &tt.want) {
				t.Errorf("Salesforce.QueryStruct() = %v, want %v", tt.args.sObject, tt.want)
			}
		})
	}
}

func TestSalesforce_InsertOne(t *testing.T) {
	type account struct {
		Name string
	}

	server, sfAuth := setupTestServer("", http.StatusCreated)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record: account{
					Name: "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record:      0,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.InsertOne(tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.InsertOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpdateOne(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	server, sfAuth := setupTestServer("", http.StatusNoContent)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_update",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record: account{
					Id:   "1234",
					Name: "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record:      0,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record: account{
					Name: "test account",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpdateOne(tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpdateOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpsertOne(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}
	server, sfAuth := setupTestServer("", http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName         string
		externalIdFieldName string
		record              any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				record: account{
					ExternalId__c: "1234",
					Name:          "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				record:              0,
			},
			wantErr: true,
		},
		{
			name: "fail_no_external_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				record: account{
					Name: "test account",
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpsertOne(tt.args.sObjectName, tt.args.externalIdFieldName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpsertOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_DeleteOne(t *testing.T) {
	type account struct {
		Id string
	}
	server, sfAuth := setupTestServer("", http.StatusNoContent)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		record      any
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record: account{
					Id: "1234",
				},
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record:      0,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				record:      account{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.DeleteOne(tt.args.sObjectName, tt.args.record); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DeleteOne() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_InsertCollection(t *testing.T) {
	type account struct {
		Name string
	}
	server, sfAuth := setupTestServer([]salesforceError{{Success: true}}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]salesforceError{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: account{
					Name: "test account 1",
				},
				batchSize: 200,
			},
			wantErr: true,
		},
		{
			name: "bad_request",
			fields: fields{
				auth: &badReqSfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: []account{
					{
						Name: "test account 1",
					},
				},
				batchSize: 200,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.InsertCollection(tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.InsertCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpdateCollection(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	server, sfAuth := setupTestServer([]salesforceError{{Success: true}}, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_update",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   200,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpdateCollection(tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpdateCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpsertCollection(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}
	server, sfAuth := setupTestServer([]salesforceError{{Success: true}}, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName         string
		externalIdFieldName string
		records             any
		batchSize           int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				records:             0,
				batchSize:           200,
			},
			wantErr: true,
		},
		{
			name: "fail_no_external_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
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
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpsertCollection(tt.args.sObjectName, tt.args.externalIdFieldName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpsertCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_DeleteCollection(t *testing.T) {
	type account struct {
		Id string
	}
	server, sfAuth := setupTestServer([]salesforceError{{Success: true}}, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   200,
			},
			wantErr: true,
		},
		{
			name: "fail_no_id",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     []account{{}, {}},
				batchSize:   200,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.DeleteCollection(tt.args.sObjectName, tt.args.records, tt.args.batchSize); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DeleteCollection() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_InsertComposite(t *testing.T) {
	type account struct {
		Name string
	}
	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]salesforceError{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
		allOrNone   bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_insert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: account{
					Name: "test account 1",
				},
				batchSize: 200,
				allOrNone: true,
			},
			wantErr: true,
		},
		{
			name: "bad_request",
			fields: fields{
				auth: &badReqSfAuth,
			},
			args: args{
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
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.InsertComposite(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.allOrNone); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.InsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpdateComposite(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]salesforceError{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
		allOrNone   bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_update",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   200,
				allOrNone:   true,
			},
			wantErr: true,
		},
		{
			name: "bad_request",
			fields: fields{
				auth: &badReqSfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: []account{
					{
						Id:   "1234",
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
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpdateComposite(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.allOrNone); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpdateComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_UpsertComposite(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}
	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]salesforceError{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName         string
		externalIdFieldName string
		records             any
		batchSize           int
		allOrNone           bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_upsert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				records:             0,
				batchSize:           200,
				allOrNone:           true,
			},
			wantErr: true,
		},
		{
			name: "bad_request",
			fields: fields{
				auth: &badReqSfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				records: []account{
					{
						ExternalId__c: "1234",
						Name:          "test account 1",
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
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.UpsertComposite(tt.args.sObjectName, tt.args.externalIdFieldName, tt.args.records, tt.args.batchSize, tt.args.allOrNone); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpsertComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_DeleteComposite(t *testing.T) {
	type account struct {
		Id string
	}
	server, sfAuth := setupTestServer(compositeRequestResult{}, http.StatusOK)
	defer server.Close()

	badReqServer, badReqSfAuth := setupTestServer([]salesforceError{{Success: false}}, http.StatusBadRequest)
	defer badReqServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName string
		records     any
		batchSize   int
		allOrNone   bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "successful_delete",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   200,
			},
			wantErr: true,
		},
		{
			name: "bad_req",
			fields: fields{
				auth: &badReqSfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: []account{{
					Id: "1234",
				}},
				batchSize: 200,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.DeleteComposite(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.allOrNone); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DeleteComposite() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_InsertBulk(t *testing.T) {
	type account struct {
		Name string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		records        any
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "successful_insert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: []account{
					{
						Name: "test account 1",
					},
					{
						Name: "test account 2",
					},
				},
				batchSize: 2000,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: account{
					Name: "test account 1",
				},
				batchSize: 2000,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.InsertBulk(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.InsertBulk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.InsertBulk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_UpdateBulk(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		records        any
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "successful_update",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
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
				batchSize: 2000,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   2000,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.UpdateBulk(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpdateBulk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.UpdateBulk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_UpsertBulk(t *testing.T) {
	type account struct {
		ExternalId__c string
		Name          string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName         string
		externalIdFieldName string
		records             any
		batchSize           int
		waitForResults      bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "successful_upsert",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
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
				batchSize: 2000,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				records:             0,
				batchSize:           2000,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.UpsertBulk(tt.args.sObjectName, tt.args.externalIdFieldName, tt.args.records, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpsertBulk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.UpsertBulk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_DeleteBulk(t *testing.T) {
	type account struct {
		Id string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		records        any
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "successful_delete",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records: []account{
					{
						Id: "1234",
					},
					{
						Id: "5678",
					},
				},
				batchSize: 2000,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName: "Account",
				records:     0,
				batchSize:   2000,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.DeleteBulk(tt.args.sObjectName, tt.args.records, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DeleteBulk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.DeleteBulk() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_GetJobResults(t *testing.T) {
	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateOpen,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	server, sfAuth := setupTestServer(jobResults, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		bulkJobId string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    BulkJobResults
		wantErr bool
	}{
		{
			name: "get_job_results",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				bulkJobId: "1234",
			},
			want:    jobResults,
			wantErr: false,
		},
		{
			name: "validation_fail",
			fields: fields{
				auth: nil,
			},
			args: args{
				bulkJobId: "1234",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.GetJobResults(tt.args.bulkJobId)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.GetJobResults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.GetJobResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_InsertBulkFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("header\nrow"), 0644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		filePath       string
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "insert bulk data successfully",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      2000,
				waitForResults: false,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      10001,
				waitForResults: false,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.InsertBulkFile(tt.args.sObjectName, tt.args.filePath, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.InsertBulkFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.InsertBulkFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_UpdateBulkFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("header\nrow"), 0644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		filePath       string
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "update bulk data successfully",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      2000,
				waitForResults: false,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      10001,
				waitForResults: false,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.UpdateBulkFile(tt.args.sObjectName, tt.args.filePath, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpdateBulkFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.UpdateBulkFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_UpsertBulkFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("header\nrow"), 0644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName         string
		externalIdFieldName string
		filePath            string
		batchSize           int
		waitForResults      bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "upsert bulk data successfully",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				filePath:            "data/data.csv",
				batchSize:           2000,
				waitForResults:      false,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:         "Account",
				externalIdFieldName: "ExternalId__c",
				filePath:            "data/data.csv",
				batchSize:           10001,
				waitForResults:      false,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.UpsertBulkFile(tt.args.sObjectName, tt.args.externalIdFieldName, tt.args.filePath, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.UpsertBulkFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.UpsertBulkFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_DeleteBulkFile(t *testing.T) {
	appFs = afero.NewMemMapFs() // replace appFs with mocked file system
	if err := appFs.MkdirAll("data", 0755); err != nil {
		t.Fatalf("error creating directory in virtual file system")
	}
	if err := afero.WriteFile(appFs, "data/data.csv", []byte("header\nrow"), 0644); err != nil {
		t.Fatalf("error creating file in virtual file system")
	}

	job := bulkJob{
		Id:    "1234",
		State: jobStateOpen,
	}
	server, sfAuth := setupTestServer(job, http.StatusOK)
	defer server.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		sObjectName    string
		filePath       string
		batchSize      int
		waitForResults bool
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "delete bulk data successfully",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      2000,
				waitForResults: false,
			},
			want:    []string{"1234"},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				auth: &sfAuth,
			},
			args: args{
				sObjectName:    "Account",
				filePath:       "data/data.csv",
				batchSize:      10001,
				waitForResults: false,
			},
			want:    []string{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			got, err := sf.DeleteBulkFile(tt.args.sObjectName, tt.args.filePath, tt.args.batchSize, tt.args.waitForResults)
			if (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.DeleteBulkFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Salesforce.DeleteBulkFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSalesforce_QueryBulkExport(t *testing.T) {
	job := bulkJob{
		Id:    "1234",
		State: jobStateJobComplete,
	}
	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateJobComplete,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	jobCreationRespBody, _ := json.Marshal(job)
	jobResultsRespBody, _ := json.Marshal(jobResults)
	csvData := `"col"` + "\n" + `"row"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI[len(r.RequestURI)-6:] == "/query" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(jobCreationRespBody); err != nil {
				t.Fatalf(err.Error())
			}
		} else if r.RequestURI[len(r.RequestURI)-5:] == "/1234" {
			if _, err := w.Write(jobResultsRespBody); err != nil {
				t.Fatalf(err.Error())
			}
		} else if r.RequestURI[len(r.RequestURI)-8:] == "/results" {
			w.Header().Add("Sforce-Locator", "")
			w.Header().Add("Sforce-Numberofrecords", "1")
			if _, err := w.Write([]byte(csvData)); err != nil {
				t.Fatalf(err.Error())
			}
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badServer, badAuth := setupTestServer(job, http.StatusBadRequest)
	defer badServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		query    string
		filePath string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "export data successfully",
			fields: fields{
				&sfAuth,
			},
			args: args{
				query:    "SELECT Id FROM Account",
				filePath: "data/export.csv",
			},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				&badAuth,
			},
			args: args{
				query:    "SELECT Id FROM Account",
				filePath: "data/export.csv",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.QueryBulkExport(tt.args.query, tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.QueryBulkExport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSalesforce_QueryStructBulkExport(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	job := bulkJob{
		Id:    "1234",
		State: jobStateJobComplete,
	}
	jobResults := BulkJobResults{
		Id:                  "1234",
		State:               jobStateJobComplete,
		NumberRecordsFailed: 0,
		ErrorMessage:        "",
	}
	jobCreationRespBody, _ := json.Marshal(job)
	jobResultsRespBody, _ := json.Marshal(jobResults)
	csvData := `"col"` + "\n" + `"row"`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RequestURI[len(r.RequestURI)-6:] == "/query" {
			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(jobCreationRespBody); err != nil {
				t.Fatalf(err.Error())
			}
		} else if r.RequestURI[len(r.RequestURI)-5:] == "/1234" {
			if _, err := w.Write(jobResultsRespBody); err != nil {
				t.Fatalf(err.Error())
			}
		} else if r.RequestURI[len(r.RequestURI)-8:] == "/results" {
			w.Header().Add("Sforce-Locator", "")
			w.Header().Add("Sforce-Numberofrecords", "1")
			if _, err := w.Write([]byte(csvData)); err != nil {
				t.Fatalf(err.Error())
			}
		}
	}))
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badServer, badAuth := setupTestServer(job, http.StatusBadRequest)
	defer badServer.Close()

	type fields struct {
		auth *authentication
	}
	type args struct {
		soqlStruct any
		filePath   string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "export data successfully",
			fields: fields{
				&sfAuth,
			},
			args: args{
				soqlStruct: account{},
				filePath:   "data/export.csv",
			},
			wantErr: false,
		},
		{
			name: "validation error",
			fields: fields{
				&badAuth,
			},
			args: args{
				soqlStruct: account{},
				filePath:   "data/export.csv",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &Salesforce{
				auth: tt.fields.auth,
			}
			if err := sf.QueryStructBulkExport(tt.args.soqlStruct, tt.args.filePath); (err != nil) != tt.wantErr {
				t.Errorf("Salesforce.QueryStructBulkExport() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
