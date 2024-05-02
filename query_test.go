package salesforce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func setupSuite(t *testing.T, records []map[string]any) *httptest.Server {
	resp := queryResponse{
		TotalSize: 1,
		Done:      true,
		Records:   records,
	}
	body, _ := json.Marshal(resp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(body)
	}))
	return server
}

func Test_performQuery(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	acc := []map[string]any{{
		"Id":   "123abc",
		"Name": "test account",
	}}
	server := setupSuite(t, acc)
	defer server.Close()

	type args struct {
		auth    authentication
		query   string
		sObject []account
	}
	tests := []struct {
		name    string
		args    args
		want    account
		wantErr bool
	}{
		{
			name: "query account",
			args: args{
				auth: authentication{
					InstanceUrl: server.URL,
					AccessToken: "accesstoken",
				},
				query:   "SELECT Id, Name FROM Account",
				sObject: []account{},
			},
			want: account{
				Id:   "123abc",
				Name: "test account",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := performQuery(tt.args.auth, tt.args.query, &tt.args.sObject); (err != nil) != tt.wantErr {
				t.Errorf("performQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.sObject[0], tt.want) {
				t.Errorf("performQuery() = %v, want %v", tt.args.sObject, tt.want)
			}
		})
	}
}

func TestQueryStruct(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	acc := []map[string]any{{
		"Id":   "123abc",
		"Name": "test account",
	}}
	server := setupSuite(t, acc)
	sf := Salesforce{auth: &authentication{
		InstanceUrl: server.URL,
		AccessToken: "123",
	}}
	defer server.Close()

	soqlStruct := account{}
	result := []account{}
	err := sf.QueryStruct(soqlStruct, &result)
	if err != nil {
		t.Errorf("unexpected error during query: %s", err.Error())
	}
	if result[0].Id != acc[0]["Id"] || result[0].Name != acc[0]["Name"] {
		t.Errorf("\nexpected: %v\nactual  : %v", acc, result)
	}
}
