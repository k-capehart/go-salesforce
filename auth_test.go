package salesforce

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func Test_validateAuth(t *testing.T) {
	type args struct {
		sf Salesforce
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
			},
			wantErr: false,
		},
		{
			name: "validation_fail",
			args: args{
				sf: Salesforce{},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateAuth(tt.args.sf); (err != nil) != tt.wantErr {
				t.Errorf("validateAuth() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_loginPassword(t *testing.T) {
	auth := authentication{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	server, _ := setupTestServer(auth, http.StatusOK)
	defer server.Close()

	badserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer badserver.Close()

	type args struct {
		domain         string
		username       string
		password       string
		securityToken  string
		consumerKey    string
		consumerSecret string
	}
	tests := []struct {
		name    string
		args    args
		want    *authentication
		wantErr bool
	}{
		{
			name: "authentication_success",
			args: args{
				domain:         server.URL,
				username:       "u",
				password:       "p",
				securityToken:  "t",
				consumerKey:    "key",
				consumerSecret: "secret",
			},
			want:    &auth,
			wantErr: false,
		},
		{
			name: "authentication_fail",
			args: args{
				domain:         badserver.URL,
				username:       "u",
				password:       "p",
				securityToken:  "t",
				consumerKey:    "key",
				consumerSecret: "secret",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loginPassword(tt.args.domain, tt.args.username, tt.args.password, tt.args.securityToken, tt.args.consumerKey, tt.args.consumerSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("loginPassword() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("loginPassword() = %v, want %v", *got, *tt.want)
			}
		})
	}
}
