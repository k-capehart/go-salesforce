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
