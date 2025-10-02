package main

import (
	"fmt"
	"log"

	"github.com/k-capehart/go-salesforce/v2"
)

func main() {
	// Example of the new functional configuration approach

	// Basic usage with default configuration (same as before for backwards compatibility)
	creds := salesforce.Creds{
		Domain:         "your-domain.my.salesforce.com",
		Username:       "your-username",
		Password:       "your-password",
		SecurityToken:  "your-security-token",
		ConsumerKey:    "your-consumer-key",
		ConsumerSecret: "your-consumer-secret",
	}

	// Initialize with default configuration
	sf, err := salesforce.Init(creds)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Default API Version: %s\n", sf.GetAPIVersion())
	fmt.Printf("Default Batch Size Max: %d\n", sf.GetBatchSizeMax())
	fmt.Printf("Default Bulk Batch Size Max: %d\n", sf.GetBulkBatchSizeMax())
	fmt.Printf("Compression Headers: %v\n", sf.GetCompressionHeaders())
	fmt.Printf("Auth Flow: %s\n", sf.GetAuthFlow())

	// Example with custom configuration using functional options
	sfCustom, err := salesforce.Init(creds,
		salesforce.WithAPIVersion("v58.0"),
		salesforce.WithBatchSizeMax(150),
		salesforce.WithBulkBatchSizeMax(8000),
		salesforce.WithCompressionHeaders(true),
	)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("\nCustom API Version: %s\n", sfCustom.GetAPIVersion())
	fmt.Printf("Custom Batch Size Max: %d\n", sfCustom.GetBatchSizeMax())
	fmt.Printf("Custom Bulk Batch Size Max: %d\n", sfCustom.GetBulkBatchSizeMax())
	fmt.Printf("Custom Compression Headers: %v\n", sfCustom.GetCompressionHeaders())
	fmt.Printf("Auth Flow: %s\n", sfCustom.GetAuthFlow())

	// Example with error handling in functional options
	_, err = salesforce.Init(creds,
		salesforce.WithAPIVersion(""), // This will cause an error
	)
	if err != nil {
		fmt.Printf("\nExpected error: %s\n", err)
	}

	// Example with invalid batch size
	_, err = salesforce.Init(creds,
		salesforce.WithBatchSizeMax(300), // This will cause an error (max is 200)
	)
	if err != nil {
		fmt.Printf("Expected error: %s\n", err)
	}
}
