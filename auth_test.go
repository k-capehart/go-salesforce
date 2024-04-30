package salesforce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateAuthSuccess(t *testing.T) {
	sf := Salesforce{auth: &auth{}}
	err := validateAuth(sf)
	if err != nil {
		t.Errorf("expected a successful validation for salesforce auth, got: %s", err.Error())
	}
}

func TestValidateAuthFail(t *testing.T) {
	sf := Salesforce{}
	err := validateAuth(sf)
	if err == nil {
		t.Errorf("expected a validation error for salesforce auth")
	}
}

func TestInitUsernamePasswordSuccess(t *testing.T) {
	resp := auth{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	body, _ := json.Marshal(resp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	sf, err := Init(Creds{
		Domain:         server.URL,
		Username:       "u",
		Password:       "p",
		SecurityToken:  "t",
		ConsumerKey:    "key",
		ConsumerSecret: "secret",
	})
	if err != nil {
		t.Errorf("unexpected error during salesforce login: %s", err.Error())
	}
	if sf.auth.AccessToken != resp.AccessToken ||
		sf.auth.InstanceUrl != resp.InstanceUrl ||
		sf.auth.Id != resp.Id ||
		sf.auth.IssuedAt != resp.IssuedAt ||
		sf.auth.Signature != resp.Signature {

		t.Errorf("expected response to be unmarshalled into auth reference, got: %v", sf.auth)
	}
}

func TestInitUsernamePasswordFail(t *testing.T) {
	resp := auth{
		AccessToken: "1234",
		InstanceUrl: "example.com",
		Id:          "123abc",
		IssuedAt:    "01/01/1970",
		Signature:   "signed",
	}
	body, _ := json.Marshal(resp)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer server.Close()

	_, err := Init(Creds{})
	if err == nil {
		t.Errorf("expected an error during login when no credentials are given")
	}
}
