package salesforce

import (
	"net/http"
	"reflect"
	"testing"
)

func Test_performQuery(t *testing.T) {
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
	server, _ := setupTestServer(resp, http.StatusOK)
	defer server.Close()

	type args struct {
		auth    authentication
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
			name: "query account",
			args: args{
				auth: authentication{
					InstanceUrl: server.URL,
					AccessToken: "accesstoken",
				},
				query:   "SELECT Id, Name FROM Account",
				sObject: []account{},
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
			if err := performQuery(tt.args.auth, tt.args.query, &tt.args.sObject); (err != nil) != tt.wantErr {
				t.Errorf("performQuery() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !reflect.DeepEqual(tt.args.sObject, tt.want) {
				t.Errorf("performQuery() = %v, want %v", tt.args.sObject, tt.want)
			}
		})
	}
}
