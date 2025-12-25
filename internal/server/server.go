package server

import (
	"encoding/json"
	"log"
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
	s.mux.HandleFunc("/fetch-current-weather", s.handleFetchCurrentWeather)
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

// handleLocations returns available locations from config
func (s *Server) handleLocations(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	config.Load("./config.yaml")
	cfg := config.Get()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"locations": cfg.Weather.Locations,
	})
}

// handleFetch manually triggers a data fetch
func (s *Server) handleFetchCurrentWeather(w http.ResponseWriter, r *http.Request) {
	location := r.URL.Query().Get("location")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FetchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.Latitude < -90 || req.Latitude > 90 {
		http.Error(w, "Latitude must be between -90 and 90", http.StatusBadRequest)
		return
	}

	if req.Longitude < -180 || req.Longitude > 180 {
		http.Error(w, "Longitude must be between -180 and 180", http.StatusBadRequest)
		return
	}

	forecast, err := s.apiClient.GetCurrentWeather(req.Latitude, req.Longitude, config.Get().Weather.MonitoredFields)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// can be asynchronous
	log.Println("location: ", location)
	if err := s.db.StoreMetrics(forecast, location, config.Get().Weather.MonitoredFields, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// can be asynchronous
	anomalies, err := s.anomalyDetector.DetectAnomalies(s.db, location)
	if err != nil {
		log.Fatalf("Failed to fetch anomalies: %v", err)
	}
	for _, anomaly := range anomalies {
		if err := s.db.StoreAnomaly(&anomaly); err != nil {
			continue
		}
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
