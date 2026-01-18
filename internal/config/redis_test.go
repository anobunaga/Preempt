package config

import (
	"os"
	"testing"
)

func TestGetRedisConfig_FromEnvVars(t *testing.T) {
	origAddr := os.Getenv("REDIS_ADDR")
	origPassword := os.Getenv("REDIS_PASSWORD")
	origDB := os.Getenv("REDIS_DB")
	origStream := os.Getenv("REDIS_STREAM")

	defer func() {
		os.Setenv("REDIS_ADDR", origAddr)
		os.Setenv("REDIS_PASSWORD", origPassword)
		os.Setenv("REDIS_DB", origDB)
		os.Setenv("REDIS_STREAM", origStream)
	}()

	os.Setenv("REDIS_ADDR", "testhost:6380")
	os.Setenv("REDIS_PASSWORD", "testpassword")
	os.Setenv("REDIS_DB", "5")
	os.Setenv("REDIS_STREAM", "test_stream")

	cfg := GetRedisConfig()

	if cfg.Addr != "testhost:6380" {
		t.Errorf("GetRedisConfig().Addr = %v, want %v", cfg.Addr, "testhost:6380")
	}

	if cfg.Password != "testpassword" {
		t.Errorf("GetRedisConfig().Password = %v, want %v", cfg.Password, "testpassword")
	}

	if cfg.DB != 5 {
		t.Errorf("GetRedisConfig().DB = %v, want %v", cfg.DB, 5)
	}

	if cfg.Stream != "test_stream" {
		t.Errorf("GetRedisConfig().Stream = %v, want %v", cfg.Stream, "test_stream")
	}
}

func TestGetRedisConfig_Defaults(t *testing.T) {
	os.Unsetenv("REDIS_ADDR")
	os.Unsetenv("REDIS_PASSWORD")
	os.Unsetenv("REDIS_DB")
	os.Unsetenv("REDIS_STREAM")

	cfg := GetRedisConfig()

	if cfg.Addr != "localhost:6379" {
		t.Errorf("GetRedisConfig().Addr = %v, want %v", cfg.Addr, "localhost:6379")
	}

	if cfg.Password != "" {
		t.Errorf("GetRedisConfig().Password = %v, want empty string", cfg.Password)
	}

	if cfg.DB != 0 {
		t.Errorf("GetRedisConfig().DB = %v, want %v", cfg.DB, 0)
	}

	if cfg.Stream != "weather_metrics" {
		t.Errorf("GetRedisConfig().Stream = %v, want %v", cfg.Stream, "weather_metrics")
	}
}

func TestGetRedisConfig_InvalidDB(t *testing.T) {
	origDB := os.Getenv("REDIS_DB")
	defer os.Setenv("REDIS_DB", origDB)

	os.Setenv("REDIS_DB", "invalid")

	cfg := GetRedisConfig()

	if cfg.DB != 0 {
		t.Errorf("GetRedisConfig().DB = %v, want %v (default on parse error)", cfg.DB, 0)
	}
}

func TestGetEnv(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		want         string
	}{
		{
			name:         "env var set",
			key:          "TEST_KEY",
			defaultValue: "default",
			envValue:     "custom",
			want:         "custom",
		},
		{
			name:         "env var not set",
			key:          "TEST_KEY_NOT_SET",
			defaultValue: "default",
			envValue:     "",
			want:         "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			} else {
				os.Unsetenv(tt.key)
			}

			got := getEnv(tt.key, tt.defaultValue)
			if got != tt.want {
				t.Errorf("getEnv() = %v, want %v", got, tt.want)
			}
		})
	}
}
