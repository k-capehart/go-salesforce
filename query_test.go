package salesforce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func Test_performQuery(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}

	queryMoreResp := queryResponse{
		TotalSize:      1,
		Done:           false,
		NextRecordsUrl: "/queryMore",
		Records: []map[string]any{{
			"Id":   "123abc",
			"Name": "test account",
		}},
	}
	queryMoreRespBody, _ := json.Marshal(queryMoreResp)

	queryDoneResp := queryResponse{
		TotalSize:      1,
		Done:           true,
		NextRecordsUrl: "",
		Records: []map[string]any{{
			"Id":   "123abc",
			"Name": "test account",
		}},
	}
	queryDoneRespBody, _ := json.Marshal(queryDoneResp)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.RequestURI, "/query/") {
			if _, err := w.Write(queryMoreRespBody); err != nil {
				panic(err.Error())
			}
		} else {
			if _, err := w.Write(queryDoneRespBody); err != nil {
				panic(err.Error())
			}
		}
	}))
	defer server.Close()
	sfAuth := authentication{
		InstanceUrl: server.URL,
		AccessToken: "accesstoken",
	}

	badServer, badSfAuth := setupTestServer("", http.StatusBadRequest)
	defer badServer.Close()

	badRespServer, badRespSfAuth := setupTestServer("1", http.StatusOK)
	defer badRespServer.Close()

	type args struct {
		sf      *Salesforce
		query   string
		sObject []account
	}
	tests := []struct {
		name    string
		args    args
		want    []account
		wantErr bool
	}{
		{
			name: "query_account",
			args: args{
				sf:      buildSalesforceStruct(&sfAuth),
				query:   "SELECT Id, Name FROM Account",
				sObject: []account{},
			},
			want: []account{
				{
					Id:   "123abc",
					Name: "test account",
				},
				{
					Id:   "123abc",
					Name: "test account",
				},
			},
			wantErr: false,
		},
		{
			name: "http_error",
			args: args{
				sf:      buildSalesforceStruct(&badSfAuth),
				query:   "SELECT Id, Name FROM Account",
				sObject: []account{},
			},
			want:    []account{},
			wantErr: true,
		},
		{
			name: "bad_response",
			args: args{
				sf:      buildSalesforceStruct(&badRespSfAuth),
				query:   "SELECT Id FROM Account",
				sObject: []account{},
			},
			want:    []account{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.args.sf.performQuery(t.Context(), tt.args.query, &tt.args.sObject); (err != nil) != tt.wantErr {
				t.Errorf("performQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.sObject, tt.want) {
				t.Errorf("performQuery() = %v, want %v", tt.args.sObject, tt.want)
			}
		})
	}
}
