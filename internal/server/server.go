package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"preempt/internal/api"
	"preempt/internal/database"
	"preempt/internal/detector"
)

// Server represents the HTTP server
type Server struct {
	db              *database.DB
	apiClient       *api.OpenMeteoClient
	anomalyDetector *detector.AnomalyDetector
	alarmSuggester  *detector.AlarmSuggester
	mux             *http.ServeMux
}

// NewServer creates a new HTTP server
func NewServer(db *database.DB, client *api.OpenMeteoClient, ad *detector.AnomalyDetector) *Server {
	s := &Server{
		db:              db,
		apiClient:       client,
		anomalyDetector: ad,
		alarmSuggester:  detector.NewAlarmSuggester(),
		mux:             http.NewServeMux(),
	}

	// Register routes
	s.mux.HandleFunc("/health", s.handleHealth)
	s.mux.HandleFunc("/fetch", s.handleFetch)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/anomalies", s.handleAnomalies)
	s.mux.HandleFunc("/alarm-suggestions", s.handleAlarmSuggestions)
	s.mux.HandleFunc("/current", s.handleCurrent)

	return s
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	return http.ListenAndServe(addr, s.mux)
}

// handleHealth returns the server health status
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().UTC().String(),
	})
}

// handleFetch manually triggers a data fetch
func (s *Server) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	forecast, err := s.apiClient.GetForecast(37.7749, -122.4194, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := s.db.StoreMetrics(forecast); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	anomalies := s.anomalyDetector.DetectAnomalies(forecast)
	for _, anomaly := range anomalies {
		s.db.StoreAnomaly(&anomaly)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "success",
		"anomalies": len(anomalies),
		"timestamp": time.Now(),
		"forecast":  forecast.Current,
	})
}

// handleMetrics returns stored metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metricType := r.URL.Query().Get("type")
	if metricType == "" {
		http.Error(w, "metric type required", http.StatusBadRequest)
		return
	}

	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil {
			hours = h
		}
	}

	since := time.Now().Add(time.Duration(-hours) * time.Hour)
	metrics, err := s.db.GetMetrics(metricType, since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"metric_type": metricType,
		"count":       len(metrics),
		"metrics":     metrics,
	})
}

// handleAnomalies returns detected anomalies
func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	anomalies, err := s.db.GetAnomalies(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":     len(anomalies),
		"anomalies": anomalies,
	})
}

// handleAlarmSuggestions returns alarm suggestions
func (s *Server) handleAlarmSuggestions(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	suggestions, err := s.db.GetAlarmSuggestions(limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"count":       len(suggestions),
		"suggestions": suggestions,
	})
}

// handleCurrent returns the current forecast
func (s *Server) handleCurrent(w http.ResponseWriter, r *http.Request) {
	forecast, err := s.apiClient.GetForecast(37.7749, -122.4194, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(forecast)
}
