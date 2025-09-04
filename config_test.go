package salesforce

import (
	"testing"
)

func TestWithCompressionHeaders(t *testing.T) {
	tests := []struct {
		name        string
		compression bool
		wantErr     bool
	}{
		{
			name:        "set_compression_headers_true",
			compression: true,
			wantErr:     false,
		},
		{
			name:        "set_compression_headers_false",
			compression: false,
			wantErr:     false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := configuration{}
			config.setDefaults()

			option := WithCompressionHeaders(tt.compression)
			err := option(&config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithCompressionHeaders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if config.compressionHeaders != tt.compression {
				t.Errorf(
					"WithCompressionHeaders() = %v, want %v",
					config.compressionHeaders,
					tt.compression,
				)
			}
		})
	}
}

func TestWithAPIVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantErr   bool
		wantValue string
	}{
		{
			name:      "valid_version",
			version:   "v58.0",
			wantErr:   false,
			wantValue: "v58.0",
		},
		{
			name:      "empty_version",
			version:   "",
			wantErr:   true,
			wantValue: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := configuration{}
			config.setDefaults()

			option := WithAPIVersion(tt.version)
			err := option(&config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithAPIVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config.apiVersion != tt.wantValue {
				t.Errorf("WithAPIVersion() = %v, want %v", config.apiVersion, tt.wantValue)
			}
		})
	}
}

func TestWithBatchSizeMax(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		wantErr   bool
		wantValue int
	}{
		{
			name:      "valid_size",
			size:      100,
			wantErr:   false,
			wantValue: 100,
		},
		{
			name:      "size_too_small",
			size:      0,
			wantErr:   true,
			wantValue: 0,
		},
		{
			name:      "size_too_large",
			size:      300,
			wantErr:   true,
			wantValue: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := configuration{}
			config.setDefaults()

			option := WithBatchSizeMax(tt.size)
			err := option(&config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithBatchSizeMax() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config.batchSizeMax != tt.wantValue {
				t.Errorf("WithBatchSizeMax() = %v, want %v", config.batchSizeMax, tt.wantValue)
			}
		})
	}
}

func TestWithBulkBatchSizeMax(t *testing.T) {
	tests := []struct {
		name      string
		size      int
		wantErr   bool
		wantValue int
	}{
		{
			name:      "valid_size",
			size:      5000,
			wantErr:   false,
			wantValue: 5000,
		},
		{
			name:      "size_too_small",
			size:      0,
			wantErr:   true,
			wantValue: 0,
		},
		{
			name:      "size_too_large",
			size:      15000,
			wantErr:   true,
			wantValue: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := configuration{}
			config.setDefaults()

			option := WithBulkBatchSizeMax(tt.size)
			err := option(&config)

			if (err != nil) != tt.wantErr {
				t.Errorf("WithBulkBatchSizeMax() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && config.bulkBatchSizeMax != tt.wantValue {
				t.Errorf(
					"WithBulkBatchSizeMax() = %v, want %v",
					config.bulkBatchSizeMax,
					tt.wantValue,
				)
			}
		})
	}
}

func TestConfigurationDefaults(t *testing.T) {
	config := configuration{}
	config.setDefaults()

	if config.compressionHeaders != false {
		t.Errorf(
			"Expected compressionHeaders default to be false, got %v",
			config.compressionHeaders,
		)
	}

	if config.apiVersion != apiVersion {
		t.Errorf("Expected apiVersion default to be %v, got %v", apiVersion, config.apiVersion)
	}

	if config.batchSizeMax != batchSizeMax {
		t.Errorf(
			"Expected batchSizeMax default to be %v, got %v",
			batchSizeMax,
			config.batchSizeMax,
		)
	}

	if config.bulkBatchSizeMax != bulkBatchSizeMax {
		t.Errorf(
			"Expected bulkBatchSizeMax default to be %v, got %v",
			bulkBatchSizeMax,
			config.bulkBatchSizeMax,
		)
	}
}
