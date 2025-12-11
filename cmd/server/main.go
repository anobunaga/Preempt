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

	// Initialize database
	db, err := database.NewDB("myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize API client
	client := api.NewOpenMeteoClient()

	// San Francisco coordinates: 37.7749° N, -122.4194° W - change this later to allow users to choose location
	latitude := 37.7749
	longitude := -122.4194

	log.Printf("Fetching weather data for San Francisco (%.4f, %.4f)...", latitude, longitude)

	forecast, err := client.GetForecast(latitude, longitude)
	if err != nil {
		log.Fatalf("Failed to fetch forecast: %v", err)
	}

	// Store metrics in database
	if err := db.StoreMetrics(forecast); err != nil {
		log.Fatalf("Failed to store metrics: %v", err)
	}

	// Detect anomalies
	ad := detector.NewAnomalyDetector()
	//forecast.Hourly.Temperature2m[0] = 100.0 // Inject an anomaly for testing
	anomalies, err := ad.DetectAnomaliesInDataPoints(forecast.Hourly.Temperature2m)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Print results
	fmt.Println("=== Anomaly Detection Results for temperature only ===")
	if len(anomalies) == 0 {
		fmt.Println("No anomalies detected!")
	} else {
		fmt.Printf("Found %d anomalies:\n", len(anomalies))
		for _, a := range anomalies {
			fmt.Printf("  Index: %d | Value: %.2f | Z-Score: %.2f | Severity: %s\n",
				a.Index, a.Value, a.ZScore, a.Severity)
		}
	}
	// Start background data collection
	//go startDataCollection(db, client, ad, latitude, longitude)
	/*

		// Create HTTP server
		httpServer := server.NewServer(db, client, anomalyDetector)

		// Start HTTP server
		log.Println("Starting server on :8080")
		if err := httpServer.Start(":8080"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	*/
}

// startDataCollection periodically fetches data from the API
func startDataCollection(db *database.DB, client *api.OpenMeteoClient, detector *detector.AnomalyDetector, lat float64, long float64) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Fetching data from Open-Meteo API...")
		forecast, err := client.GetForecast(lat, long)
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
