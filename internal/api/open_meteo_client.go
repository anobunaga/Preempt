package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"preempt/internal/models"
	"strings"
)

const baseURL = "https://api.open-meteo.com/v1/forecast"

// OpenMeteoClient is a client for the Open-Meteo API
type OpenMeteoClient struct {
	client *http.Client
}

type ForecastParams struct {
	Latitude        float64
	Longitude       float64
	CurrentFields   []string
	HourlyFields    []string
	DailyFields     []string
	Timezone        string
	TemperatureUnit string
	PastDays        int // how many days in the past you want to get
	ForecastDays    int // how many days in the future you want to forecast
}

// NewOpenMeteoClient creates a new Open-Meteo API client
func NewOpenMeteoClient() *OpenMeteoClient {
	return &OpenMeteoClient{
		client: &http.Client{},
	}
}

// GetForecast fetches forecast data for the given coordinates, pull hourly on application initialization, otherwise just current metrics
func (c *OpenMeteoClient) GetForecast(forecastParams ForecastParams) (*models.Forecast, error) {
	url := c.BuildURL(forecastParams)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch forecast: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error: status %d, body: %s", resp.StatusCode, string(body))
	}

	var forecast models.Forecast
	if err := json.NewDecoder(resp.Body).Decode(&forecast); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &forecast, nil
}

// Builds URL for OpenMeteoClient request
func (c *OpenMeteoClient) BuildURL(forecastParams ForecastParams) string {
	if forecastParams.Timezone == "" {
		forecastParams.Timezone = "auto"
	}

	if forecastParams.TemperatureUnit == "" {
		forecastParams.TemperatureUnit = "fahrenheit"
	}

	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f&timezone=%s&temperature_unit=%s",
		baseURL, forecastParams.Latitude, forecastParams.Longitude, forecastParams.Timezone, forecastParams.TemperatureUnit)

	if forecastParams.PastDays > 0 {
		url += fmt.Sprintf("&past_days=%d", forecastParams.PastDays)
	}

	if forecastParams.ForecastDays >= 0 {
		url += fmt.Sprintf("&forecast_days=%d", forecastParams.ForecastDays)
	}

	if len(forecastParams.CurrentFields) > 0 {
		url += "&current=" + strings.Join(forecastParams.CurrentFields, ",")
	}

	if len(forecastParams.DailyFields) > 0 {
		url += "&daily=" + strings.Join(forecastParams.DailyFields, ",")
	}

	if len(forecastParams.HourlyFields) > 0 {
		url += "&hourly=" + strings.Join(forecastParams.HourlyFields, ",")
	}

	return url
}

func (c *OpenMeteoClient) GetCurrentWeather(lat, long float64, fields []string) (*models.Forecast, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("GetCurrentWeather: no weather fields provided")
	}

	forecastParams := ForecastParams{
		Latitude:      lat,
		Longitude:     long,
		CurrentFields: fields,
	}

	return c.GetForecast(forecastParams)
}

func (c *OpenMeteoClient) GetHistoricalHourlyData(lat, long float64, fields []string, pastDays int) (*models.Forecast, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("GetHistoricalHourlyData: no weather fields provided")
	}

	return c.GetForecast(ForecastParams{
		Latitude:     lat,
		Longitude:    long,
		HourlyFields: fields,
		PastDays:     pastDays,
		ForecastDays: 0,
	})
}

func (c *OpenMeteoClient) GetDailyForecast(lat, long float64, fields []string) (*models.Forecast, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("GetDailyWeather: no weather fields provided")
	}

	forecastParams := ForecastParams{
		Latitude:    lat,
		Longitude:   long,
		DailyFields: fields,
	}

	return c.GetForecast(forecastParams)
}
