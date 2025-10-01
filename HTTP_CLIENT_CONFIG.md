# HTTP Client Configuration

## Overview

The Salesforce Go client now supports custom HTTP transport layer configuration, allowing you to:

1. **Provide a custom `http.RoundTripper`** - Lower-level control over HTTP request/response cycle
    - `http.RoundTripper` can be layered to allow for logging, observability, etc layers for each request
2. **Use default HTTP client** - Sensible defaults are provided when no custom configuration is specified

## Usage Examples

### Custom Round Tripper

```go
func main() {
    // Create a custom round tripper for fine-grained HTTP control
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

    creds := salesforce.Creds{
        // ... your credentials
    }

    // Initialize with custom round tripper
    sf, err := salesforce.Init(creds, 
        salesforce.WithRoundTripper(customRoundTripper),
        salesforce.WithAPIVersion("v64.0"),
    )
    if err != nil {
        log.Fatal(err)
    }
}
```

## Configuration Options

### HTTP Client Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithRoundTripper(rt http.RoundTripper)` | Set a custom round tripper | Default transport |

### Other Configuration Options

| Option | Description | Default |
|--------|-------------|---------|
| `WithAPIVersion(version string)` | Set Salesforce API version | v63.0 |
| `WithCompressionHeaders(enabled bool)` | Enable/disable compression | false |
| `WithBatchSizeMax(size int)` | Set max batch size for collections | 200 |
| `WithBulkBatchSizeMax(size int)` | Set max batch size for bulk operations | 10000 |

## Default HTTP Client Configuration

When no custom round tripper is provided, the library uses:

```go
&http.Client{
    Timeout: 120 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:       10,
        IdleConnTimeout:    30 * time.Second,
        DisableCompression: false,
    },
}
```

## Accessing Configuration

You can retrieve the current configuration using getter methods:

```go
sf, _ := salesforce.Init(creds)

// Get the configured HTTP client
client := sf.GetHTTPClient()

// Get other configuration values
apiVersion := sf.GetAPIVersion()
batchSizeMax := sf.GetBatchSizeMax()
bulkBatchSizeMax := sf.GetBulkBatchSizeMax()
compressionEnabled := sf.GetCompressionHeaders()
```

## Implementation Details

- The `doRequest` function now uses the configured HTTP client instead of `http.DefaultClient`
- The API version from configuration is used in all endpoint URLs
- Custom configurations are validated during initialization to prevent runtime errors

## Migration from Previous Versions

This change is fully backward compatible. Existing code will continue to work without any modifications, using the default HTTP client configuration.
