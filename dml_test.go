package salesforce

import (
	"testing"
)

func TestConvertToMap(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	acc := account{
		Id:   "1234",
		Name: "test account",
	}
	expected := map[string]any{
		"Id":   "1234",
		"Name": "test account",
	}

	actual, err := convertToMap(acc)
	if err != nil {
		t.Errorf("unexpected error while converting struct to map: %s", err.Error())
	}
	if actual["Id"] != expected["Id"] || actual["Name"] != expected["Name"] {
		t.Errorf("\nexpected: %v\nactual  : %v", expected, actual)
	}
}

func TestConvertToSliceOfMap(t *testing.T) {
	type account struct {
		Id   string
		Name string
	}
	acc := []account{
		{
			Id:   "1234",
			Name: "test account 1",
		},
		{
			Id:   "5678",
			Name: "test account 2",
		},
	}
	expected := []map[string]any{
		{
			"Id":   "1234",
			"Name": "test account 1",
		},
		{
			"Id":   "5678",
			"Name": "test account 2",
		},
	}

	actual, err := convertToSliceOfMaps(acc)
	if err != nil {
		t.Errorf("unexpected error while converting struct to map: %s", err.Error())
	}
	if actual[0]["Id"] != expected[0]["Id"] || actual[0]["Name"] != expected[0]["Name"] ||
		actual[1]["Id"] != expected[1]["Id"] || actual[1]["Name"] != expected[1]["Name"] {

		t.Errorf("\nexpected: %v\nactual  : %v", expected, actual)
	}
}
