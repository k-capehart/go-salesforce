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

func TestLoginPassword(t *testing.T) {
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

	auth, err := loginPassword(server.URL, "u", "p", "t", "key", "secret")
	if err != nil {
		t.Errorf("expected a successful login, got: %s", err.Error())
	}
	if auth.AccessToken != resp.AccessToken ||
		auth.InstanceUrl != resp.InstanceUrl ||
		auth.Id != resp.Id ||
		auth.IssuedAt != resp.IssuedAt ||
		auth.Signature != resp.Signature {

		t.Errorf("expected response to be unmarshalled into auth reference, got: %v", auth)
	}
}
