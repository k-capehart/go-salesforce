package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/k-capehart/go-salesforce/v2"
)

func main() {
	// Example 1: Using a custom transport with timeout and TLS configuration

	creds := salesforce.Creds{
		Domain:         "your-domain.my.salesforce.com",
		Username:       "your-username",
		Password:       "your-password",
		SecurityToken:  "your-security-token",
		ConsumerKey:    "your-consumer-key",
		ConsumerSecret: "your-consumer-secret",
	}

	// Initialize Salesforce client with custom HTTP client
	sf, err := salesforce.Init(creds,
		salesforce.WithRoundTripper(&http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false, // Set to true if you need to skip SSL verification
			},
			MaxIdleConns:       20,
			IdleConnTimeout:    90 * time.Second,
			DisableCompression: false,
		}),
	)
	if err != nil {
		log.Fatal("Failed to initialize Salesforce client:", err)
	}

	fmt.Printf("Salesforce client initialized with custom HTTP client\n")
	fmt.Printf("HTTP Client timeout: %v\n", sf.GetHTTPClient().Timeout)
	fmt.Printf("API Version: %s\n", sf.GetAPIVersion())

	// Example 2: Using a custom round tripper
	customRoundTripper := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 5,
	}

	sf2, err := salesforce.Init(creds,
		salesforce.WithRoundTripper(customRoundTripper),
		salesforce.WithAPIVersion("v64.0"),
	)
	if err != nil {
		log.Fatal("Failed to initialize Salesforce client with round tripper:", err)
	}

	fmt.Printf("Salesforce client initialized with custom round tripper\n")
	fmt.Printf("API Version: %s\n", sf2.GetAPIVersion())

	// Example 3: Using default configuration
	sf3, err := salesforce.Init(creds)
	if err != nil {
		log.Fatal("Failed to initialize Salesforce client with defaults:", err)
	}

	fmt.Printf("Salesforce client initialized with default configuration\n")
	fmt.Printf("HTTP Client timeout: %v\n", sf3.GetHTTPClient().Timeout)
	fmt.Printf("Compression headers enabled: %v\n", sf3.GetCompressionHeaders())

	// Example of combining multiple configuration options
	sf4, err := salesforce.Init(creds,
		salesforce.WithRoundTripper(http.DefaultTransport),
		salesforce.WithCompressionHeaders(true),
		salesforce.WithAPIVersion("v65.0"),
		salesforce.WithBatchSizeMax(150),
		salesforce.WithBulkBatchSizeMax(5000),
	)
	if err != nil {
		log.Fatal("Failed to initialize Salesforce client with multiple options:", err)
	}

	fmt.Printf("Salesforce client initialized with multiple configuration options\n")
	fmt.Printf("API Version: %s\n", sf4.GetAPIVersion())
	fmt.Printf("Batch Size Max: %d\n", sf4.GetBatchSizeMax())
	fmt.Printf("Bulk Batch Size Max: %d\n", sf4.GetBulkBatchSizeMax())
	fmt.Printf("Compression headers enabled: %v\n", sf4.GetCompressionHeaders())
}
