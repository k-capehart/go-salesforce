package salesforce

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jszwec/csvutil"
)

type IteratorJob interface {
	Next() bool
	Error() error
	Decode(any) error
}

type bulkJobQueryIterator struct {
	NumberOfRecords int    `json:"Sforce-Numberofrecords"`
	Locator         string `json:"Sforce-Locator"`
	auth            *authentication
	uri             string
	err             error
	reader          io.ReadCloser
}

func newBulkJobQueryIterator(auth *authentication, bulkJobId string) (*bulkJobQueryIterator, error) {
	pollErr := waitForJobResults(auth, bulkJobId, queryJobType, (time.Second / 2))
	if pollErr != nil {
		return nil, pollErr
	}
	return &bulkJobQueryIterator{
		auth: auth,
		uri:  "/jobs/query/" + bulkJobId + "/results",
	}, nil
}

func (it *bulkJobQueryIterator) Next() bool {
	if it.reader != nil {
		it.err = it.reader.Close()
		if it.Locator == "" {
			return false
		}
	}
	uri := it.uri
	if it.Locator != "" {
		uri += "/?locator=" + it.Locator
	}
	resp, err := doRequest(it.auth, requestPayload{method: http.MethodGet, uri: uri, content: jsonType})
	if err != nil {
		it.err = err
		return false
	}
	it.reader = resp.Body

	it.NumberOfRecords, _ = strconv.Atoi(resp.Header["Sforce-Numberofrecords"][0])
	if resp.Header["Sforce-Locator"][0] != "null" {
		it.Locator = resp.Header["Sforce-Locator"][0]
	} else {
		it.Locator = ""
	}

	return true
}

func (it *bulkJobQueryIterator) Decode(val any) error {
	dec, err := csvutil.NewDecoder(csv.NewReader(it.reader))
	if err != nil {
		return fmt.Errorf("NewDecoder: %w", err)
	}

	if err := dec.Decode(val); err != nil && err != io.EOF {
		return fmt.Errorf("Decode: %w", err)
	}
	return nil
}

func (it *bulkJobQueryIterator) Error() error {
	return it.err
}
