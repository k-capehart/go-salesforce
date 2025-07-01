package salesforce

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func Test_doRequest(t *testing.T) {
	server, sfAuth := setupTestServer("", http.StatusOK)
	defer server.Close()

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	recordArrayResp := "{\"records\":[{\"Id\":\"123abc\"}]}"
	serverWith300Resp, authWith300Resp := setupTestServer(
		recordArrayResp,
		http.StatusMultipleChoices,
	)
	defer serverWith300Resp.Close()

	compressedServer, sfAuthCompressed := setupTestServer("test", http.StatusOK)
	defer compressedServer.Close()

	type args struct {
		auth    *authentication
		payload requestPayload
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
				auth: &sfAuth,
				payload: requestPayload{
					method:  http.MethodGet,
					uri:     "",
					content: jsonType,
					body:    "",
				},
			},
			want:    http.StatusOK,
			wantErr: false,
		},
		{
			name: "make_generic_http_call_bad_request",
			args: args{
				auth: &badSfAuth,
				payload: requestPayload{
					method:  http.MethodGet,
					uri:     "",
					content: jsonType,
					body:    "",
				},
			},
			want:    http.StatusBadRequest,
			wantErr: true,
		},
		{
			name: "handle_multiple_records_with_same_externalId_statusCode_300",
			args: args{
				auth: &authWith300Resp,
				payload: requestPayload{
					method:  http.MethodGet,
					uri:     "/sobjects/Contact/ContactExternalId__c/Avng1",
					content: jsonType,
					body:    "",
				},
			},
			want:    http.StatusMultipleChoices,
			wantErr: false,
		},
		{
			name: "compression_headers",
			args: args{
				auth: &sfAuthCompressed,
				payload: requestPayload{
					method:   http.MethodGet,
					uri:      "",
					content:  jsonType,
					body:     "test",
					compress: true,
				},
			},
			want:    http.StatusOK,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := doRequest(tt.args.auth, tt.args.payload)
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

func Test_compression(t *testing.T) {
	compressedResp, _ := compress("testRecord1")

	type args struct {
		body io.ReadCloser
	}
	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "decompress_gzip",
			args: args{
				body: io.NopCloser(compressedResp),
			},
			want:    []byte("testRecord1"),
			wantErr: false,
		},
		{
			name: "decompress_invalid",
			args: args{
				body: io.NopCloser(strings.NewReader("invalid data")),
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := decompress(tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("decompress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			var readGot []byte
			if got != nil {
				readGot, err = io.ReadAll(got)
				if (err != nil) != tt.wantErr {
					t.Errorf("decompress() error = %v, wantErr %v", err, tt.wantErr)
					return
				}
			} else {
				readGot = nil
			}
			if !reflect.DeepEqual(readGot, tt.want) {
				t.Errorf("decompress() = %v, want %v", readGot, tt.want)
			}
		})
	}
}

func Test_processSalesforceError(t *testing.T) {
	body, _ := json.Marshal([]SalesforceErrorMessage{{
		Message:    "error message",
		StatusCode: strconv.Itoa(http.StatusInternalServerError),
		Fields:     []string{},
		ErrorCode:  strconv.Itoa(http.StatusInternalServerError),
	}})
	exampleResp := http.Response{
		Status:     "500",
		StatusCode: 500,
		Body:       io.NopCloser(strings.NewReader(string(body))),
	}

	badServer, badSfAuth := setupTestServer(body, http.StatusInternalServerError)
	defer badServer.Close()

	bodyInvalidSession, _ := json.Marshal([]SalesforceErrorMessage{{
		Message:    "error message",
		StatusCode: strconv.Itoa(http.StatusInternalServerError),
		Fields:     []string{},
		ErrorCode:  invalidSessionIdError,
	}})
	reqPayload := requestPayload{
		method:  http.MethodGet,
		uri:     "",
		content: jsonType,
		body:    "",
	}

	serverRefreshed, sfAuthRefreshed := setupTestServer("", http.StatusOK)
	defer serverRefreshed.Close()
	serverInvalidSession, sfAuthInvalidSession := setupTestServer(sfAuthRefreshed, http.StatusOK)
	defer serverInvalidSession.Close()
	sfAuthInvalidSession.grantType = grantTypeClientCredentials

	serverRefreshFail, sfAuthRefreshFail := setupTestServer("", http.StatusBadRequest)
	defer serverRefreshFail.Close()
	sfAuthRefreshFail.grantType = grantTypeClientCredentials

	serverRetryFail := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.RequestURI, "/oauth2/token") {
				body, err := json.Marshal(badSfAuth)
				if err != nil {
					panic(err)
				}
				if _, err := w.Write(body); err != nil {
					panic(err)
				}
			} else {
				w.WriteHeader(http.StatusBadRequest)
			}
		}),
	)
	defer serverRetryFail.Close()
	sfAuthRetryFail := authentication{
		InstanceUrl: serverRetryFail.URL,
		AccessToken: "1234",
		grantType:   grantTypeClientCredentials,
	}

	type args struct {
		resp    http.Response
		auth    *authentication
		payload requestPayload
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "process_500_error",
			args: args{
				resp:    exampleResp,
				auth:    &badSfAuth,
				payload: reqPayload,
			},
			want:    exampleResp.StatusCode,
			wantErr: true,
		},
		{
			name: "process_invalid_session",
			args: args{
				resp: http.Response{
					Status:     "400",
					StatusCode: 400,
					Body:       io.NopCloser(strings.NewReader(string(bodyInvalidSession))),
				},
				auth:    &sfAuthInvalidSession,
				payload: reqPayload,
			},
			want:    http.StatusOK,
			wantErr: false,
		},
		{
			name: "fail_to_refresh",
			args: args{
				resp: http.Response{
					Status:     "400",
					StatusCode: 400,
					Body:       io.NopCloser(strings.NewReader(string(bodyInvalidSession))),
				},
				auth:    &sfAuthRefreshFail,
				payload: reqPayload,
			},
			want:    400,
			wantErr: true,
		},
		{
			name: "fail_to_retry_request",
			args: args{
				resp: http.Response{
					Status:     "400",
					StatusCode: 400,
					Body:       io.NopCloser(strings.NewReader(string(bodyInvalidSession))),
				},
				auth:    &sfAuthRetryFail,
				payload: reqPayload,
			},
			want:    400,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processSalesforceError(tt.args.resp, tt.args.auth, tt.args.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("processSalesforceError() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.StatusCode, tt.want) {
				t.Errorf("processSalesforceError() = %v, want %v", got.StatusCode, tt.want)
			}
		})
	}
}
