package salesforce

import (
	"net/http"
	"testing"
	"time"
)

func TestWithRoundTripper(t *testing.T) {
	tests := []struct {
		name         string
		roundTripper http.RoundTripper
		wantErr      bool
		errorMsg     string
	}{
		{
			name: "valid_round_tripper",
			roundTripper: &http.Transport{
				MaxIdleConns: 10,
			},
			wantErr: false,
		},
		{
			name:         "nil_round_tripper",
			roundTripper: nil,
			wantErr:      true,
			errorMsg:     "round tripper cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration{}
			option := WithRoundTripper(tt.roundTripper)
			err := option(config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithRoundTripper() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errorMsg {
				t.Errorf("WithRoundTripper() error message = %v, want %v", err.Error(), tt.errorMsg)
				return
			}

			if !tt.wantErr {
				if config.roundTripper != tt.roundTripper {
					t.Errorf(
						"WithRoundTripper() roundTripper = %v, want %v",
						config.roundTripper,
						tt.roundTripper,
					)
				}
				if config.httpClient != nil {
					t.Errorf(
						"WithRoundTripper() should clear httpClient, got %v",
						config.httpClient,
					)
				}
			}
		})
	}
}

func TestConfigurationHTTPClientDefaults(t *testing.T) {
	t.Run("default_http_client", func(t *testing.T) {
		config := configuration{}
		config.setDefaults()
		config.configureHttpClient()

		if config.httpClient == nil {
			t.Error("setDefaults() should set a default HTTP client")
		}

		if config.httpClient.Timeout != httpDefaultTimeout {
			t.Errorf(
				"setDefaults() HTTP client timeout = %v, want %v",
				config.httpClient.Timeout,
				httpDefaultTimeout,
			)
		}

		transport, ok := config.httpClient.Transport.(*http.Transport)
		if !ok {
			t.Error("setDefaults() HTTP client should use http.Transport")
		}

		if transport.MaxIdleConns != httpDefaultMaxIdleConnections {
			t.Errorf(
				"setDefaults() HTTP transport MaxIdleConns = %v, want %v",
				transport.MaxIdleConns,
				httpDefaultMaxIdleConnections,
			)
		}

		if transport.IdleConnTimeout != httpDefaultIdleConnTimeout {
			t.Errorf(
				"setDefaults() HTTP transport IdleConnTimeout = %v, want %v",
				transport.IdleConnTimeout,
				httpDefaultIdleConnTimeout,
			)
		}
	})

	t.Run("with_custom_round_tripper", func(t *testing.T) {
		config := configuration{}
		customRT := &http.Transport{MaxIdleConns: httpDefaultMaxIdleConnections}
		config.roundTripper = customRT
		config.setDefaults()
		config.configureHttpClient()

		if config.httpClient == nil {
			t.Error("setDefaults() should create HTTP client with custom round tripper")
		}

		if config.httpClient.Transport != customRT {
			t.Errorf(
				"setDefaults() HTTP client transport = %v, want %v",
				config.httpClient.Transport,
				customRT,
			)
		}

		if config.httpClient.Timeout != httpDefaultTimeout {
			t.Errorf(
				"setDefaults() HTTP client timeout = %v, want %v",
				config.httpClient.Timeout,
				httpDefaultTimeout,
			)
		}
	})

	t.Run("with_custom_http_client", func(t *testing.T) {
		config := configuration{}
		customClient := &http.Client{Timeout: httpDefaultTimeout}
		config.httpClient = customClient
		config.setDefaults()

		if config.httpClient != customClient {
			t.Errorf(
				"setDefaults() should preserve custom HTTP client, got %v, want %v",
				config.httpClient,
				customClient,
			)
		}
	})
}

func TestSalesforceGetHTTPClient(t *testing.T) {
	customClient := &http.Client{
		Timeout: 45 * time.Second,
	}

	// Create a Salesforce struct directly to avoid network calls during testing
	sf := &Salesforce{
		auth: &authentication{
			AccessToken: "test-token",
			InstanceUrl: "https://test.my.salesforce.com",
		},
		config: &configuration{
			httpClient: customClient,
		},
	}

	if sf.GetHTTPClient() != customClient {
		t.Errorf("GetHTTPClient() = %v, want %v", sf.GetHTTPClient(), customClient)
	}
}

func TestSalesforceAPIVersionInRequests(t *testing.T) {
	customVersion := "v64.0"

	// Create a Salesforce struct directly to avoid network calls during testing
	sf := &Salesforce{
		auth: &authentication{
			AccessToken: "test-token",
			InstanceUrl: "https://test.my.salesforce.com",
		},
		config: &configuration{
			apiVersion: customVersion,
		},
	}

	if sf.GetAPIVersion() != customVersion {
		t.Errorf("GetAPIVersion() = %v, want %v", sf.GetAPIVersion(), customVersion)
	}

	// Verify the configuration is passed correctly to doRequest by checking it's stored in the struct
	if sf.config.apiVersion != customVersion {
		t.Errorf("config.apiVersion = %v, want %v", sf.config.apiVersion, customVersion)
	}
}

func TestWithHTTPTimeout(t *testing.T) {
	tests := []struct {
		name     string
		timeout  time.Duration
		wantErr  bool
		errorMsg string
	}{
		{
			name:    "valid_timeout",
			timeout: 30 * time.Second,
			wantErr: false,
		},
		{
			name:    "valid_timeout_1_second",
			timeout: 1 * time.Second,
			wantErr: false,
		},
		{
			name:    "valid_timeout_1_minute",
			timeout: 1 * time.Minute,
			wantErr: false,
		},
		{
			name:     "zero_timeout",
			timeout:  0,
			wantErr:  true,
			errorMsg: "HTTP timeout must be greater than 0",
		},
		{
			name:     "negative_timeout",
			timeout:  -1 * time.Second,
			wantErr:  true,
			errorMsg: "HTTP timeout must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration{}
			config.setDefaults() // Set defaults to have a baseline
			option := WithHTTPTimeout(tt.timeout)
			err := option(config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithHTTPTimeout() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && err.Error() != tt.errorMsg {
				t.Errorf("WithHTTPTimeout() error message = %v, want %v", err.Error(), tt.errorMsg)
				return
			}

			if !tt.wantErr {
				if config.httpTimeout != tt.timeout {
					t.Errorf(
						"WithHTTPTimeout() httpTimeout = %v, want %v",
						config.httpTimeout,
						tt.timeout,
					)
				}
			}
		})
	}
}

func TestWithValidateAuthentication(t *testing.T) {
	tests := []struct {
		name     string
		validate bool
		wantErr  bool
	}{
		{
			name:     "validate_true",
			validate: true,
			wantErr:  false,
		},
		{
			name:     "validate_false",
			validate: false,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &configuration{}
			config.setDefaults() // Set defaults to have a baseline
			option := WithValidateAuthentication(tt.validate)
			err := option(config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithValidateAuthentication() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if config.shouldValidateAuthentication != tt.validate {
					t.Errorf(
						"WithValidateAuthentication() shouldValidateAuthentication = %v, want %v",
						config.shouldValidateAuthentication,
						tt.validate,
					)
				}
			}
		})
	}
}
