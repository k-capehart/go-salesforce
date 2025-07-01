package salesforce

import (
	"testing"
)

func TestConfiguration_SetDefaults(t *testing.T) {
	expectedDefaults := Configuration{
		CompressionHeaders: false,
	}

	tests := []struct {
		name   string
		config *Configuration
		want   Configuration
	}{
		{
			name:   "set_defaults",
			config: &Configuration{},
			want:   expectedDefaults,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.config.SetDefaults()
			if *tt.config != tt.want {
				t.Errorf("Configuration.SetDefaults() = %v, want %v", *tt.config, expectedDefaults)
			}
		})
	}
}

func TestConfiguration_SetCompressionHeaders(t *testing.T) {
	type fields struct {
		CompressionHeaders bool
	}
	type args struct {
		compression bool
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "set_compression_headers_true",
			fields: fields{
				CompressionHeaders: false,
			},
			args: args{
				compression: true,
			},
			want: true,
		},
		{
			name: "set_compression_headers_false",
			fields: fields{
				CompressionHeaders: true,
			},
			args: args{
				compression: false,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Configuration{
				CompressionHeaders: tt.fields.CompressionHeaders,
			}
			c.SetCompressionHeaders(tt.args.compression)
			if c.CompressionHeaders != tt.want {
				t.Errorf(
					"Configuration.SetCompressionHeaders() = %v, want %v",
					c.CompressionHeaders,
					tt.want,
				)
			}
		})
	}
}
