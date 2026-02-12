package config

import (
	"os"
	"sync"
	"testing"
)

func TestLoad(t *testing.T) {
	tempConfig := `weather:
  monitored_fields:
    - temperature_2m
    - relative_humidity_2m
    - precipitation
  locations:
    - name: "San Francisco"
      latitude: 37.7749
      longitude: -122.4194
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  stream: "weather_metrics"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(tempConfig)); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}
	tmpFile.Close()

	instance = nil
	once = *new(sync.Once)

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	if len(cfg.Weather.MonitoredFields) != 3 {
		t.Errorf("Expected 3 monitored fields, got %d", len(cfg.Weather.MonitoredFields))
	}

	if len(cfg.Weather.Locations) != 1 {
		t.Errorf("Expected 1 location, got %d", len(cfg.Weather.Locations))
	}

	if cfg.Weather.Locations[0].Name != "San Francisco" {
		t.Errorf("Expected location name 'San Francisco', got '%s'", cfg.Weather.Locations[0].Name)
	}

	if cfg.Redis.Addr != "localhost:6379" {
		t.Errorf("Expected Redis addr 'localhost:6379', got '%s'", cfg.Redis.Addr)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Write([]byte("invalid: [yaml: content"))
	tmpFile.Close()

	instance = nil
	once = *new(sync.Once)

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("Expected error for invalid YAML, got nil")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	instance = nil
	once = *new(sync.Once)

	_, err := Load("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
}

func TestLoad_EmptyMonitoredFields(t *testing.T) {
	tempConfig := `weather:
  monitored_fields: []
  locations:
    - name: "Test"
      latitude: 0.0
      longitude: 0.0
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  stream: "weather_metrics"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Write([]byte(tempConfig))
	tmpFile.Close()

	instance = nil
	once = *new(sync.Once)

	_, err = Load(tmpFile.Name())
	if err == nil {
		t.Error("Expected validation error for empty monitored_fields, got nil")
	}
}

func TestGet(t *testing.T) {
	tempConfig := `weather:
  monitored_fields:
    - temperature_2m
  locations:
    - name: "Test"
      latitude: 0.0
      longitude: 0.0
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  stream: "weather_metrics"
`
	tmpFile, err := os.CreateTemp("", "config-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.Write([]byte(tempConfig))
	tmpFile.Close()

	instance = nil
	once = *new(sync.Once)

	_, err = Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() returned nil")
	}

	if len(cfg.Weather.MonitoredFields) != 1 {
		t.Errorf("Expected 1 monitored field, got %d", len(cfg.Weather.MonitoredFields))
	}
}

func TestGet_Panic(t *testing.T) {
	instance = nil
	once = *new(sync.Once)

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected Get() to panic when config not loaded")
		}
	}()

	Get()
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Weather: struct {
					MonitoredFields []string   `yaml:"monitored_fields"`
					Locations       []Location `yaml:"locations"`
				}{
					MonitoredFields: []string{"temperature_2m"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty monitored fields",
			config: &Config{
				Weather: struct {
					MonitoredFields []string   `yaml:"monitored_fields"`
					Locations       []Location `yaml:"locations"`
				}{
					MonitoredFields: []string{},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
