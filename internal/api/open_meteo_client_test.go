package api

import (
	"testing"
)

func TestNewOpenMeteoClient(t *testing.T) {
	client := NewOpenMeteoClient()
	if client == nil {
		t.Fatal("NewOpenMeteoClient() returned nil")
	}

	if client.client == nil {
		t.Error("OpenMeteoClient.client should not be nil")
	}
}

func TestBuildURL(t *testing.T) {
	client := NewOpenMeteoClient()

	tests := []struct {
		name   string
		params ForecastParams
		want   string
	}{
		{
			name: "basic current weather",
			params: ForecastParams{
				Latitude:      37.7749,
				Longitude:     -122.4194,
				CurrentFields: []string{"temperature_2m", "precipitation"},
			},
			want: "https://api.open-meteo.com/v1/forecast?latitude=37.7749&longitude=-122.4194&timezone=auto&temperature_unit=fahrenheit&forecast_days=0&current=temperature_2m,precipitation",
		},
		{
			name: "hourly data with past days",
			params: ForecastParams{
				Latitude:     37.7749,
				Longitude:    -122.4194,
				HourlyFields: []string{"temperature_2m"},
				PastDays:     7,
				ForecastDays: 0,
			},
			want: "https://api.open-meteo.com/v1/forecast?latitude=37.7749&longitude=-122.4194&timezone=auto&temperature_unit=fahrenheit&past_days=7&forecast_days=0&hourly=temperature_2m",
		},
		{
			name: "daily forecast",
			params: ForecastParams{
				Latitude:     40.7128,
				Longitude:    -74.0060,
				DailyFields:  []string{"temperature_2m_max", "temperature_2m_min"},
				ForecastDays: 7,
			},
			want: "https://api.open-meteo.com/v1/forecast?latitude=40.7128&longitude=-74.0060&timezone=auto&temperature_unit=fahrenheit&forecast_days=7&daily=temperature_2m_max,temperature_2m_min",
		},
		{
			name: "custom timezone and temperature unit",
			params: ForecastParams{
				Latitude:        51.5074,
				Longitude:       -0.1278,
				CurrentFields:   []string{"temperature_2m"},
				Timezone:        "Europe/London",
				TemperatureUnit: "celsius",
			},
			want: "https://api.open-meteo.com/v1/forecast?latitude=51.5074&longitude=-0.1278&timezone=Europe/London&temperature_unit=celsius&forecast_days=0&current=temperature_2m",
		},
		{
			name: "all field types",
			params: ForecastParams{
				Latitude:      37.7749,
				Longitude:     -122.4194,
				CurrentFields: []string{"temperature_2m"},
				HourlyFields:  []string{"precipitation"},
				DailyFields:   []string{"temperature_2m_max"},
			},
			want: "https://api.open-meteo.com/v1/forecast?latitude=37.7749&longitude=-122.4194&timezone=auto&temperature_unit=fahrenheit&forecast_days=0&current=temperature_2m&daily=temperature_2m_max&hourly=precipitation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.BuildURL(tt.params)
			if got != tt.want {
				t.Errorf("BuildURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCurrentWeather_NoFields(t *testing.T) {
	client := NewOpenMeteoClient()

	_, err := client.GetCurrentWeather(37.7749, -122.4194, []string{})
	if err == nil {
		t.Error("GetCurrentWeather() expected error for empty fields, got nil")
	}

	expectedMsg := "GetCurrentWeather: no weather fields provided"
	if err.Error() != expectedMsg {
		t.Errorf("GetCurrentWeather() error = %v, want %v", err.Error(), expectedMsg)
	}
}

func TestGetHistoricalHourlyData_NoFields(t *testing.T) {
	client := NewOpenMeteoClient()

	_, err := client.GetHistoricalHourlyData(37.7749, -122.4194, []string{}, 7)
	if err == nil {
		t.Error("GetHistoricalHourlyData() expected error for empty fields, got nil")
	}

	expectedMsg := "GetHistoricalHourlyData: no weather fields provided"
	if err.Error() != expectedMsg {
		t.Errorf("GetHistoricalHourlyData() error = %v, want %v", err.Error(), expectedMsg)
	}
}

func TestGetDailyForecast_NoFields(t *testing.T) {
	client := NewOpenMeteoClient()

	_, err := client.GetDailyForecast(37.7749, -122.4194, []string{})
	if err == nil {
		t.Error("GetDailyForecast() expected error for empty fields, got nil")
	}

	expectedMsg := "GetDailyWeather: no weather fields provided"
	if err.Error() != expectedMsg {
		t.Errorf("GetDailyForecast() error = %v, want %v", err.Error(), expectedMsg)
	}
}

func TestBuildURL_DefaultValues(t *testing.T) {
	client := NewOpenMeteoClient()

	params := ForecastParams{
		Latitude:      37.7749,
		Longitude:     -122.4194,
		CurrentFields: []string{"temperature_2m"},
	}

	url := client.BuildURL(params)

	if !contains(url, "timezone=auto") {
		t.Error("BuildURL() should include default timezone=auto")
	}

	if !contains(url, "temperature_unit=fahrenheit") {
		t.Error("BuildURL() should include default temperature_unit=fahrenheit")
	}
}

func TestBuildURL_NegativeCoordinates(t *testing.T) {
	client := NewOpenMeteoClient()

	params := ForecastParams{
		Latitude:      -33.8688,
		Longitude:     151.2093,
		CurrentFields: []string{"temperature_2m"},
	}

	url := client.BuildURL(params)

	if !contains(url, "latitude=-33.8688") {
		t.Error("BuildURL() should handle negative latitude")
	}

	if !contains(url, "longitude=151.2093") {
		t.Error("BuildURL() should handle positive longitude")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
