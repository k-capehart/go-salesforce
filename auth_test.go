package salesforce

import (
	"net/http"
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

func Test_usernamePasswordFlow(t *testing.T) {
	auth := authentication{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
		grantType:   grantTypeUsernamePassword,
	}
	server, _ := setupTestServer(auth, http.StatusOK)
	defer server.Close()

	badServer, _ := setupTestServer(auth, http.StatusForbidden)
	defer badServer.Close()

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
				domain:         badServer.URL,
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
			got, err := usernamePasswordFlow(tt.args.domain, tt.args.username, tt.args.password, tt.args.securityToken, tt.args.consumerKey, tt.args.consumerSecret)
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

func Test_clientCredentialsFlow(t *testing.T) {
	auth := authentication{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
		grantType:   grantTypeClientCredentials,
	}
	server, _ := setupTestServer(auth, http.StatusOK)
	defer server.Close()

	badServer, _ := setupTestServer(auth, http.StatusForbidden)
	defer badServer.Close()

	type args struct {
		domain         string
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
				consumerKey:    "key",
				consumerSecret: "secret",
			},
			want:    &auth,
			wantErr: false,
		},
		{
			name: "authentication_fail",
			args: args{
				domain:         badServer.URL,
				consumerKey:    "key",
				consumerSecret: "secret",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := clientCredentialsFlow(tt.args.domain, tt.args.consumerKey, tt.args.consumerSecret)
			if (err != nil) != tt.wantErr {
				t.Errorf("clientCredentialsFlow() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("clientCredentialsFlow() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setAccessToken(t *testing.T) {
	auth := authentication{
		InstanceUrl: "example.com",
		AccessToken: "1234",
	}
	server, _ := setupTestServer(auth, http.StatusOK)
	defer server.Close()

	badServer, _ := setupTestServer(auth, http.StatusForbidden)
	defer badServer.Close()

	type args struct {
		domain      string
		accessToken string
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
				domain:      server.URL,
				accessToken: "1234",
			},
			want:    &auth,
			wantErr: false,
		},
		{
			name: "authentication_fail_http_error",
			args: args{
				domain:      badServer.URL,
				accessToken: "1234",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "authentication_fail_no_token",
			args: args{
				domain:      server.URL,
				accessToken: "",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := setAccessToken(tt.args.domain, tt.args.accessToken)
			if (err != nil) != tt.wantErr {
				t.Errorf("setAccessToken() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if (tt.want == nil && !reflect.DeepEqual(got, tt.want)) ||
				(tt.want != nil && !reflect.DeepEqual(got.AccessToken, tt.want.AccessToken)) {
				t.Errorf("setAccessToken() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_refreshSession(t *testing.T) {
	refreshedAuth := authentication{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	serverClientCredentials, sfAuthClientCredentials := setupTestServer(refreshedAuth, http.StatusOK)
	defer serverClientCredentials.Close()
	sfAuthClientCredentials.grantType = grantTypeClientCredentials

	serverUserNamePassword, sfAuthUserNamePassword := setupTestServer(refreshedAuth, http.StatusOK)
	defer serverUserNamePassword.Close()
	sfAuthUserNamePassword.grantType = grantTypeUsernamePassword

	serverNoGrantType, sfAuthNoGrantType := setupTestServer(refreshedAuth, http.StatusOK)
	defer serverNoGrantType.Close()

	serverBadRequest, sfAuthBadRequest := setupTestServer("", http.StatusBadGateway)
	defer serverBadRequest.Close()
	sfAuthBadRequest.grantType = grantTypeClientCredentials

	serverNoRefresh, sfAuthNoRefresh := setupTestServer("", http.StatusOK)
	defer serverNoRefresh.Close()
	sfAuthNoRefresh.grantType = grantTypeClientCredentials

	type args struct {
		auth *authentication
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name:    "refresh_client_credentials",
			args:    args{auth: &sfAuthClientCredentials},
			wantErr: false,
		},
		{
			name:    "refresh_username_password",
			args:    args{auth: &sfAuthUserNamePassword},
			wantErr: false,
		},
		{
			name:    "error_no_grant_type",
			args:    args{auth: &sfAuthNoGrantType},
			wantErr: true,
		},
		{
			name:    "error_bad_request",
			args:    args{auth: &sfAuthBadRequest},
			wantErr: true,
		},
		{
			name:    "no_refresh",
			args:    args{auth: &sfAuthNoRefresh},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := refreshSession(tt.args.auth); (err != nil) != tt.wantErr {
				t.Errorf("refreshSession() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
