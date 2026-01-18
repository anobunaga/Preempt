package main

import (
	"log"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Load config
	config.Load("./config.yaml")

	// Initialize database
	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Get all locations from database
	locations, err := db.GetAllLocations()
	if err != nil {
		log.Fatalf("Failed to get locations from database: %v", err)
	}

	if len(locations) == 0 {
		log.Fatalf("No locations found in database. Please run the seed script first.")
	}

	log.Printf("Found %d locations in database", len(locations))

	// Initialize Redis client from environment variables
	redisCfg := config.GetRedisConfig()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	defer redisClient.Close()

	// Initialize anomaly detector with Redis client and alarm suggester
	anomalyDetector := detector.NewAnomalyDetector(redisClient)
	alarmSuggester := detector.NewAlarmSuggester()

	log.Println("Running anomaly detection for all locations...")

	// Run detection once (ofelia will handle scheduling)
	runDetectionForAllLocations(db, locations, anomalyDetector, alarmSuggester)

	log.Println("Detection run completed successfully")
}

func runDetectionForAllLocations(db *database.DB, locations []database.Location, anomalyDetector *detector.AnomalyDetector, alarmSuggester *detector.AlarmSuggester) {
	log.Println("Running anomaly detection for all locations...")

	totalAnomalies := 0
	totalSuggestions := 0

	for _, location := range locations {
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
