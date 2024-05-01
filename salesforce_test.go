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
)

func Test_doRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	auth := authorization{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	badserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	badauth := authorization{
		InstanceUrl: badserver.URL,
		AccessToken: "accesstokenvalue",
	}
	defer badserver.Close()

	type args struct {
		method  string
		uri     string
		content string
		auth    authorization
		body    string
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
				method:  http.MethodGet,
				uri:     "",
				content: jsonType,
				auth:    auth,
				body:    "",
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
				auth:    badauth,
				body:    "",
			},
			want:    http.StatusBadRequest,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doRequest(tt.args.method, tt.args.uri, tt.args.content, tt.args.auth, tt.args.body)
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
	sfauth := authorization{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	body, _ := json.Marshal(sfauth)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
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
			want:    &Salesforce{auth: &sfauth},
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
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Init() = %v, want %v", *got.auth, *tt.want.auth)
			}
		})
	}
}
