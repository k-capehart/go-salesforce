package salesforce

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
)

func TestValidateNumberOfSubrequestsSuccess(t *testing.T) {
	err := validateNumberOfSubrequests(5000, 200)
	if err != nil {
		t.Errorf("unexpected validation error for composite req: %s", err.Error())
	}
}

func TestValidateNumberOfSubrequestsFail(t *testing.T) {
	err := validateNumberOfSubrequests(5001, 200)
	if err == nil {
		t.Errorf("expected a validation error for composite req")
	}
}

func TestCreateCompositeRequestForCollection(t *testing.T) {
	recordMap := []map[string]any{
		{
			"Id":   "1234",
			"Name": "test account 1",
		},
		{
			"Id":   "5678",
			"Name": "test account 2",
		},
	}

	actual, err := createCompositeRequestForCollection(http.MethodPatch, "example.com", true, 1, recordMap)
	if err != nil {
		t.Errorf("unexpected error while creating composite request: %s", err.Error())
	}
	actualRecordMap := append([]map[string]any{actual.CompositeRequest[0].Body.Records[0]}, actual.CompositeRequest[1].Body.Records[0])

	if actual.AllOrNone != true || actual.CompositeRequest[0].Body.AllOrNone != true || actual.CompositeRequest[1].Body.AllOrNone != true ||
		actual.CompositeRequest[0].Body.Records[0]["Id"] != recordMap[0]["Id"] ||
		actual.CompositeRequest[0].Body.Records[0]["Name"] != recordMap[0]["Name"] ||
		actual.CompositeRequest[1].Body.Records[0]["Id"] != recordMap[1]["Id"] ||
		actual.CompositeRequest[1].Body.Records[0]["Name"] != recordMap[1]["Name"] {
		t.Errorf("\nexpected: %v\nactual  : %v", recordMap, actualRecordMap)
	}
}

func TestProcessCompositeResponse(t *testing.T) {
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
	compSubResults := []composteSubRequestResult{
		{
			Body:           exampleError,
			HttpHeaders:    map[string]string{},
			HttpStatusCode: http.StatusBadRequest,
			ReferenceId:    "ref0",
		},
	}
	compResult := compositeRequestResult{
		CompositeResponse: compSubResults,
	}
	jsonBody, _ := json.Marshal(compResult)
	body := io.NopCloser(bytes.NewReader(jsonBody))
	resp := http.Response{
		Status:     fmt.Sprint(http.StatusBadRequest),
		StatusCode: http.StatusBadRequest,
		Body:       body,
	}
	err := processCompositeResponse(resp)
	if err == nil {
		t.Errorf("expected an error to be returned but got nothing")
	}
	if err.Error() != message[0].StatusCode+": "+message[0].Message+" "+exampleError[0].Id {
		t.Errorf("\nexpected: %s\nactual  : %s", message[0].StatusCode+": "+message[0].Message+" "+exampleError[0].Id, err.Error())
	}
}
