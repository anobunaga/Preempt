package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	if s.mux == nil {
		t.Error("NewServer() mux should not be nil")
	}
}

func TestHandleHealth(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.handleHealth(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("handleHealth() status = %v, want %v", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("handleHealth() content-type = %v, want application/json", contentType)
	}

	var response map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("handleHealth() status in body = %v, want healthy", response["status"])
	}

	if response["time"] == "" {
		t.Error("handleHealth() time should not be empty")
	}
}

func TestHandleFetchCurrentWeather_InvalidMethod(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/fetch-current-weather", nil)
	w := httptest.NewRecorder()

	s.handleFetchCurrentWeather(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("handleFetchCurrentWeather() status = %v, want %v", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}

func TestHandleFetchCurrentWeather_InvalidJSON(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodPost, "/fetch-current-weather", bytes.NewBufferString("invalid json"))
	w := httptest.NewRecorder()

	s.handleFetchCurrentWeather(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("handleFetchCurrentWeather() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleFetchCurrentWeather_InvalidLatitude(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	tests := []struct {
		name     string
		latitude float64
	}{
		{"latitude too high", 91.0},
		{"latitude too low", -91.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := FetchRequest{
				Latitude:  tt.latitude,
				Longitude: 0.0,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/fetch-current-weather", bytes.NewBuffer(bodyBytes))
			w := httptest.NewRecorder()

			s.handleFetchCurrentWeather(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("handleFetchCurrentWeather() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleFetchCurrentWeather_InvalidLongitude(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	tests := []struct {
		name      string
		longitude float64
	}{
		{"longitude too high", 181.0},
		{"longitude too low", -181.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqBody := FetchRequest{
				Latitude:  0.0,
				Longitude: tt.longitude,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest(http.MethodPost, "/fetch-current-weather", bytes.NewBuffer(bodyBytes))
			w := httptest.NewRecorder()

			s.handleFetchCurrentWeather(w, req)

			resp := w.Result()
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("handleFetchCurrentWeather() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleMetrics_MissingLocation(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()

	s.handleMetrics(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("handleMetrics() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleAnomalies_MissingLocation(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/anomalies", nil)
	w := httptest.NewRecorder()

	s.handleAnomalies(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("handleAnomalies() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestHandleAlarmSuggestions_MissingLocation(t *testing.T) {
	s := &Server{
		mux: http.NewServeMux(),
	}

	req := httptest.NewRequest(http.MethodGet, "/alarm-suggestions", nil)
	w := httptest.NewRecorder()

	s.handleAlarmSuggestions(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("handleAlarmSuggestions() status = %v, want %v", resp.StatusCode, http.StatusBadRequest)
	}
}

func TestFetchRequest(t *testing.T) {
	req := FetchRequest{
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	if req.Latitude != 37.7749 {
		t.Errorf("FetchRequest.Latitude = %v, want %v", req.Latitude, 37.7749)
	}

	if req.Longitude != -122.4194 {
		t.Errorf("FetchRequest.Longitude = %v, want %v", req.Longitude, -122.4194)
	}
}

func TestFetchRequest_JSONMarshaling(t *testing.T) {
	req := FetchRequest{
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal FetchRequest: %v", err)
	}

	var decoded FetchRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FetchRequest: %v", err)
	}

	if decoded.Latitude != req.Latitude {
		t.Errorf("Decoded latitude = %v, want %v", decoded.Latitude, req.Latitude)
	}

	if decoded.Longitude != req.Longitude {
		t.Errorf("Decoded longitude = %v, want %v", decoded.Longitude, req.Longitude)
	}
}
