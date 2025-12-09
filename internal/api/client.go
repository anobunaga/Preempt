package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"preempt/internal/models"
)

const baseURL = "https://api.open-meteo.com/v1/forecast"

// OpenMeteoClient is a client for the Open-Meteo API
type OpenMeteoClient struct {
	client *http.Client
}

// NewOpenMeteoClient creates a new Open-Meteo API client
func NewOpenMeteoClient() *OpenMeteoClient {
	return &OpenMeteoClient{
		client: &http.Client{},
	}
}

// GetForecast fetches forecast data for the given coordinates
func (c *OpenMeteoClient) GetForecast(latitude, longitude float64) (*models.Forecast, error) {
	url := fmt.Sprintf("%s?latitude=%.4f&longitude=%.4f&hourly=temperature_2m,dew_point_2m&timezone=auto&temperature_unit=fahrenheit", baseURL, latitude, longitude)
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
