package main

import (
	"log"
	"os"
	"os/signal"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"
	"syscall"
	"time"
)

const (
	detectionInterval = 10 * time.Minute // Run anomaly detection every 10 minutes
)

func main() {
	// Load config
	config.Load("./config.yaml")
	cfg := config.Get()

	// Initialize database
	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize anomaly detector and alarm suggester
	anomalyDetector := detector.NewAnomalyDetector()
	alarmSuggester := detector.NewAlarmSuggester()

	log.Println("Detector started, running anomaly detection every 10 minutes...")

	// Run detection immediately on start
	runDetectionForAllLocations(db, cfg, anomalyDetector, alarmSuggester)

	// Run detection periodically
	ticker := time.NewTicker(detectionInterval)
	defer ticker.Stop()

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			runDetectionForAllLocations(db, cfg, anomalyDetector, alarmSuggester)
		case <-quit:
			log.Println("Shutting down detector...")
			return
		}
	}
}

func runDetectionForAllLocations(db *database.DB, cfg *config.Config, anomalyDetector *detector.AnomalyDetector, alarmSuggester *detector.AlarmSuggester) {
	log.Println("Running anomaly detection for all locations...")

	totalAnomalies := 0
	totalSuggestions := 0

	for _, location := range cfg.Weather.Locations {
		log.Printf("Detecting anomalies for %s", location.Name)

		// Detect anomalies for this location
		anomalies, err := anomalyDetector.DetectAnomalies(db, location.Name)
		if err != nil {
			log.Printf("Failed to detect anomalies for %s: %v", location.Name, err)
			continue
		}

		if len(anomalies) > 0 {
			// Store anomalies in database
			if err := db.StoreAnomalies(anomalies); err != nil {
				log.Printf("Failed to store anomalies for %s: %v", location.Name, err)
			} else {
				log.Printf("✓ Stored %d anomalies for %s", len(anomalies), location.Name)
				totalAnomalies += len(anomalies)
			}

			// Generate alarm suggestions based on anomalies
			suggestions := alarmSuggester.SuggestAlarms(anomalies, location.Name)
			if len(suggestions) > 0 {
				for _, suggestion := range suggestions {
					if err := db.StoreAlarmSuggestion(&suggestion); err != nil {
						log.Printf("Failed to store alarm suggestion for %s: %v", location.Name, err)
					} else {
						log.Printf("✓ Stored alarm suggestion for %s - %s (confidence: %.2f)",
							location.Name, suggestion.MetricType, suggestion.Confidence)
						totalSuggestions++
					}
				}
			}
		} else {
			log.Printf("No anomalies detected for %s", location.Name)
		}
	}

	log.Printf("Detection complete: %d total anomalies found, %d alarm suggestions generated",
		totalAnomalies, totalSuggestions)
}
