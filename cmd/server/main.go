package main

import (
	"fmt"
	"log"
	"preempt/internal/api"
	"preempt/internal/database"
	"preempt/internal/detector"
	"time"
)

func main() {
	/*
		// Initialize database
		db, err := database.NewDB("metrics.db")
		if err != nil {
			log.Fatalf("Failed to initialize database: %v", err)
		}
		defer db.Close()
	*/
	// Initialize API client
	client := api.NewOpenMeteoClient()

	// San Francisco coordinates: 37.7749° N, 122.4194° W
	latitude := 37.7749
	longitude := -122.4194

	log.Printf("Fetching weather data for San Francisco (%.4f, %.4f)...", latitude, longitude)

	forecast, err := client.GetForecast(latitude, longitude)
	if err != nil {
		log.Fatalf("Failed to fetch forecast: %v", err)
	}

	// Print temperature metrics
	fmt.Println("\n=== Temperature Data for San Francisco ===")
	fmt.Printf("Hourly Temperature: %.1f°F\n", forecast.Hourly.Temperature2m)
	fmt.Printf("Hourly dew point: %.1f°F\n", forecast.Hourly.DewPoint2m)

	/*
		// Initialize anomaly detector
		anomalyDetector := detector.NewAnomalyDetector()

		// Create HTTP server
		httpServer := server.NewServer(db, client, anomalyDetector)

		// Start background data collection
		go startDataCollection(db, client, anomalyDetector)

		// Start HTTP server
		log.Println("Starting server on :8080")
		if err := httpServer.Start(":8080"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	*/
}

// startDataCollection periodically fetches data from the API
func startDataCollection(db *database.DB, client *api.OpenMeteoClient, detector *detector.AnomalyDetector) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Fetching data from Open-Meteo API...")
		forecast, err := client.GetForecast(35.6895, 139.6917)
		if err != nil {
			log.Printf("Failed to fetch forecast: %v", err)
			continue
		}

		// Store in database
		if err := db.StoreMetrics(forecast); err != nil {
			log.Printf("Failed to store metrics: %v", err)
			continue
		}

		// Detect anomalies
		anomalies := detector.DetectAnomalies(forecast)
		if len(anomalies) > 0 {
			log.Printf("Detected %d anomalies", len(anomalies))
			for _, anomaly := range anomalies {
				log.Printf("  - %s: %v", anomaly.MetricType, anomaly.Value)
			}
		}
	}
}
