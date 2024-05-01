package salesforce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDoRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	auth := auth{
		InstanceUrl: server.URL,
		AccessToken: "accesstokenvalue",
	}
	defer server.Close()

	resp, err := doRequest(http.MethodGet, "", jsonType, auth, "")
	if err != nil {
		t.Errorf(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code: 200, got %d", resp.StatusCode)
	}
}

func TestValidateOfTypeSliceSuccess(t *testing.T) {
	data := []int{0}
	err := validateOfTypeSlice(data)
	if err != nil {
		t.Errorf("expected a successful validation, got error: %s", err.Error())
	}
}

func TestValidateOfTypeSliceFail(t *testing.T) {
	badData := 0
	err := validateOfTypeSlice(badData)
	if err == nil {
		t.Errorf("expected a validation error with integer type")
	}
}

func TestValidateOfTypeStructSuccess(t *testing.T) {
	type testStruct struct{}
	data := testStruct{}
	err := validateOfTypeStruct(data)
	if err != nil {
		t.Errorf("expected a successful validation, got error: %s", err.Error())
	}
}

func TestValidateOfTypeStructFail(t *testing.T) {
	badData := 0
	err := validateOfTypeStruct(badData)
	if err == nil {
		t.Errorf("expected a validation error with integer type")
	}
}

func TestValidateBatchSizeWithinRangeSuccess(t *testing.T) {
	err := validateBatchSizeWithinRange(1, 200)
	if err != nil {
		t.Errorf("expected a successful validation, got error: %s", err.Error())
	}
	err = validateBatchSizeWithinRange(200, 200)
	if err != nil {
		t.Errorf("expected a successful validation, got error: %s", err.Error())
	}
	err = validateBatchSizeWithinRange(100, 200)
	if err != nil {
		t.Errorf("expected a successful validation, got error: %s", err.Error())
	}
}

func TestValidateBatchSizeWithinRangeFail(t *testing.T) {
	err := validateBatchSizeWithinRange(0, 200)
	if err == nil {
		t.Errorf("expected a validation error for value smaller than range")
	}
	err = validateBatchSizeWithinRange(201, 200)
	if err == nil {
		t.Errorf("expected a validation error for value bigger than range")
	}
}

func TestProcessSalesforceError(t *testing.T) {
	body := io.NopCloser(strings.NewReader("error message"))
	resp := http.Response{
		Status:     "500",
		StatusCode: 500,
		Body:       body,
	}
	err := processSalesforceError(resp)
	if err == nil || err.Error() != string(resp.Status)+": "+"error message" {
		t.Errorf("expected to return an error message")
	}
}

func TestProcessSalesforceResponse(t *testing.T) {
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
	resp := http.Response{
		Status:     fmt.Sprint(http.StatusInternalServerError),
		StatusCode: http.StatusInternalServerError,
		Body:       body,
	}
	err := processSalesforceResponse(resp)
	if err == nil {
		t.Errorf("expected an error to be returned but got nothing")
	}
	if err.Error() != message[0].StatusCode+": "+message[0].Message+" "+exampleError[0].Id {
		t.Errorf("\nexpected: %s\nactual  : %s", message[0].StatusCode+": "+message[0].Message+" "+exampleError[0].Id, err.Error())
	}
}
