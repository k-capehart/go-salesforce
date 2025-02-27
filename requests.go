package salesforce

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type requestPayload struct {
	method   string
	uri      string
	content  string
	body     string
	retry    bool
	compress bool
}

func doRequest(auth *authentication, payload requestPayload) (*http.Response, error) {
	var reader io.Reader
	var req *http.Request
	var err error
	endpoint := auth.InstanceUrl + "/services/data/" + apiVersion + payload.uri

	if payload.body != "" {
		if payload.compress {
			reader, err = compress(payload.body)
			if err != nil {
				return nil, err
			}
		} else {
			reader = strings.NewReader(payload.body)
		}
		req, err = http.NewRequest(payload.method, endpoint, reader)
	} else {
		req, err = http.NewRequest(payload.method, endpoint, nil)
	}
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "go-salesforce")
	req.Header.Set("Content-Type", payload.content)
	req.Header.Set("Accept", payload.content)
	req.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	if payload.compress {
		req.Header.Set("Content-Encoding", "gzip") // compress request
		req.Header.Set("Accept-Encoding", "gzip")  // compress response
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		resp, err = processSalesforceError(*resp, auth, payload)
	}

	// salesforce does not guarantee that the response will be compressed
	if resp.Header.Get("Content-Encoding") == "gzip" {
		resp.Body, err = decompress(resp.Body)
	}

	return resp, err
}

func compress(body string) (io.Reader, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write([]byte(body)); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return &buf, nil
}

func decompress(body io.ReadCloser) (io.ReadCloser, error) {
	gzReader, err := gzip.NewReader(body)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(decompressed)), nil
}

func processSalesforceError(resp http.Response, auth *authentication, payload requestPayload) (*http.Response, error) {
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return &resp, err
	}
	var sfErrors []SalesforceErrorMessage
	err = json.Unmarshal(responseData, &sfErrors)
	if err != nil {
		return &resp, err
	}
	for _, sfError := range sfErrors {
		if sfError.ErrorCode == invalidSessionIdError && !payload.retry { // only attempt to refresh the session once
			err = refreshSession(auth)
			if err != nil {
				return &resp, err
			}
			newResp, err := doRequest(auth, requestPayload{payload.method, payload.uri, payload.content, payload.body, true, payload.compress})
			if err != nil {
				return &resp, err
			}
			return newResp, nil
		}
	}

	return &resp, errors.New(string(responseData))
}
