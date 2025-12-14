package main

import (
	"log"
	"os"
	"os/signal"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"
	"syscall"
	"time"
)

const (
	collectionInterval = 5 * time.Minute
	historicalDays     = 7
)

func main() {
	config.Load("./config.yaml")
	db, err := database.NewDB("myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	client := api.NewOpenMeteoClient()
	ad := detector.NewAnomalyDetector()

	// San Francisco coordinates: 37.7749° N, -122.4194° W - change this later to allow users to choose location
	latitude := 37.7749
	longitude := -122.4194

	log.Printf("Fetching weather data for San Francisco")

	forecast, err := client.GetHistoricalHourlyData(latitude, longitude, config.Get().Weather.MonitoredFields, historicalDays)

	if err != nil {
		log.Fatalf("Failed to fetch forecast: %v", err)
	}

	if err := db.StoreMetrics(forecast, config.Get().Weather.MonitoredFields, true); err != nil {
		log.Fatalf("Failed to store metrics: %v", err)
	}

	anomalies, err := ad.DetectAnomalies(db)
	if err != nil {
		log.Fatalf("Failed to detect anomalies: %v", err)
	}

	if err := db.StoreAnomalies(anomalies); err != nil {
		log.Fatalf("Failed to store anomalies: %v", err)
	}

	go startDataCollection(db, client, ad, latitude, longitude)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Collector running. Press Ctrl+C to stop...")
	<-quit

	log.Println("Shutting down collector...")
}

// startDataCollection periodically fetches data from the API (every 5 min)
func startDataCollection(db *database.DB, client *api.OpenMeteoClient, detector *detector.AnomalyDetector, lat, long float64) {
	ticker := time.NewTicker(collectionInterval)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Fetching data from Open-Meteo API via go routine")
		forecast, err := client.GetCurrentWeather(lat, long, config.Get().Weather.MonitoredFields)
		if err != nil {
			log.Printf("Failed to fetch forecast: %v", err)
			continue
		}

		if err := db.StoreMetrics(forecast, config.Get().Weather.MonitoredFields, false); err != nil {
			log.Printf("Failed to store metrics: %v", err)
			continue
		}
	}
}
