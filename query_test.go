package salesforce

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	return server
}

func TestQuery(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	acc := []map[string]any{{
		"Id":   "123abc",
		"Name": "test account",
	}}
	server := setupSuite(t, acc)
	sf := Salesforce{auth: &auth{
		InstanceUrl: server.URL,
		AccessToken: "123",
	}}
	defer server.Close()

	result := []account{}
	err := sf.Query("SELECT Id, Name FROM Account", &result)
	if err != nil {
		t.Errorf("unexpected error during query: %s", err.Error())
	}
	if result[0].Id != acc[0]["Id"] || result[0].Name != acc[0]["Name"] {
		t.Errorf("\nexpected: %v\nactual  : %v", acc, result)
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
	sf := Salesforce{auth: &auth{
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
