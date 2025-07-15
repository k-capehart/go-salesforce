package salesforce

import (
	"errors"
	"net/http"
	"time"
)

// configuration is now private to enforce functional configuration pattern
type configuration struct {
	compressionHeaders           bool // compress request and response if true to save bandwidth
	apiVersion                   string
	batchSizeMax                 int
	bulkBatchSizeMax             int
	httpClient                   *http.Client      // HTTP client (created internally)
	roundTripper                 http.RoundTripper // Custom round tripper
	shouldValidateAuthentication bool              // Validate session on client creation
	httpTimeout                  time.Duration     // HTTP client timeout
}

// setDefaults sets the default configuration values
func (c *configuration) setDefaults() {
	c.compressionHeaders = false
	c.shouldValidateAuthentication = true // Default to validating authentication
	c.apiVersion = apiVersion
	c.batchSizeMax = batchSizeMax
	c.bulkBatchSizeMax = bulkBatchSizeMax
	c.httpTimeout = 0    // Default to no timeout (can be set via WithHTTPTimeout option)
	c.roundTripper = nil // No custom round tripper by default
}

func (c *configuration) configureHttpClient() {
	// Set default HTTP client if none provided
	if c.roundTripper == nil {
		c.httpClient = &http.Client{
			Timeout: c.httpTimeout,
			Transport: &http.Transport{
				MaxIdleConns:       httpDefaultMaxIdleConnections,
				IdleConnTimeout:    httpDefaultIdleConnTimeout,
				DisableCompression: false,
			},
		}
	} else {
		// Use custom round tripper with configured timeout
		c.httpClient = &http.Client{
			Transport: c.roundTripper,
			Timeout:   c.httpTimeout,
		}
	}
}

// Option is a functional configuration option that can return an error
type Option func(*configuration) error

// WithCompressionHeaders sets whether to compress request and response headers
func WithCompressionHeaders(compression bool) Option {
	return func(c *configuration) error {
		c.compressionHeaders = compression
		return nil
	}
}

// WithAPIVersion sets the Salesforce API version to use
func WithAPIVersion(version string) Option {
	return func(c *configuration) error {
		if version == "" {
			return errors.New("API version cannot be empty")
		}
		c.apiVersion = version
		return nil
	}
}

// WithBatchSizeMax sets the maximum batch size for collections
func WithBatchSizeMax(size int) Option {
	return func(c *configuration) error {
		if size < 1 || size > 200 {
			return errors.New("batch size max must be between 1 and 200")
		}
		c.batchSizeMax = size
		return nil
	}
}

// WithBulkBatchSizeMax sets the maximum batch size for bulk operations
func WithBulkBatchSizeMax(size int) Option {
	return func(c *configuration) error {
		if size < 1 || size > 10000 {
			return errors.New("bulk batch size max must be between 1 and 10000")
		}
		c.bulkBatchSizeMax = size
		return nil
	}
}

// WithRoundTripper sets a custom round tripper for HTTP requests
func WithRoundTripper(rt http.RoundTripper) Option {
	return func(c *configuration) error {
		if rt == nil {
			return errors.New("round tripper cannot be nil")
		}
		c.roundTripper = rt
		return nil
	}
}

// WithHTTPTimeout sets the HTTP client timeout duration
func WithHTTPTimeout(timeout time.Duration) Option {
	return func(c *configuration) error {
		if timeout <= 0 {
			return errors.New("HTTP timeout must be greater than 0")
		}
		c.httpTimeout = timeout
		return nil
	}
}

// WithValidateAuthentication sets whether to validate the authentication session on client creation
func WithValidateAuthentication(validate bool) Option {
	return func(c *configuration) error {
		c.shouldValidateAuthentication = validate
		return nil
	}
}
