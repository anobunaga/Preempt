package server

import (
	"encoding/json"
	"net/http"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"
	"strconv"
	"time"
)

type FetchRequest struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

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
	s.mux.HandleFunc("/locations", s.handleLocations)
	s.mux.HandleFunc("/metrics", s.handleMetrics)
	s.mux.HandleFunc("/anomalies", s.handleAnomalies)
	s.mux.HandleFunc("/alarm-suggestions", s.handleAlarmSuggestions)

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

// handleLocations returns available locations from database
func (s *Server) handleLocations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	locations, err := s.db.GetAllLocations()
	if err != nil {
		http.Error(w, "Failed to fetch locations: "+err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"locations": locations,
		"count":     len(locations),
	})
}

// handleMetrics returns stored metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if location == "" {
		http.Error(w, "location parameter is required", http.StatusBadRequest)
		return
	}

	metricType := r.URL.Query().Get("type")
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil {
			hours = h
		}
	}

	since := time.Now().Add(-time.Duration(hours) * time.Hour)

	// If no type specified, return all metrics
	if metricType == "" {
		cfg := config.Get()
		allMetrics := make(map[string]interface{})

		for _, field := range cfg.Weather.MonitoredFields {
			metrics, err := s.db.GetMetrics(location, []string{field}, since)
			if err != nil {
				continue
			}
			allMetrics[field] = map[string]interface{}{
				"count": len(metrics),
				"data":  metrics,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"location": location,
			"hours":    hours,
			"metrics":  allMetrics,
		})
		return
	}

	// Get specific metric type
	metrics, err := s.db.GetMetrics(location, []string{metricType}, since)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"location":    location,
		"metric_type": metricType,
		"hours":       hours,
		"count":       len(metrics),
		"data":        metrics,
	})
}

// handleAnomalies returns detected anomalies
func (s *Server) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if location == "" {
		http.Error(w, "location parameter is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 100
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	anomalies, err := s.db.GetAnomalies(location, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"location":  location,
		"count":     len(anomalies),
		"anomalies": anomalies,
	})
}

// handleAlarmSuggestions returns alarm suggestions
func (s *Server) handleAlarmSuggestions(w http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if location == "" {
		http.Error(w, "location parameter is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	suggestions, err := s.db.GetAlarmSuggestions(location, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"location":    location,
		"count":       len(suggestions),
		"suggestions": suggestions,
	})
}
