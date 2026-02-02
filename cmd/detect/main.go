package main

import (
	"log"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"
	"preempt/internal/models"
	"sync"
	"time"

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

// DetectionResult holds the results for a single location
type DetectionResult struct {
	Location       string
	Anomalies      []models.Anomaly
	Suggestions    []models.AlarmSuggestion
	Error          error
	ProcessingTime time.Duration
}

func runDetectionForAllLocations(db *database.DB, locations []database.Location, anomalyDetector *detector.AnomalyDetector, alarmSuggester *detector.AlarmSuggester) {
	startTime := time.Now()
	log.Printf("Running anomaly detection for %d locations with worker pool...", len(locations))

	// Configure worker pool - use 50 workers or fewer if less locations
	numWorkers := 50
	if len(locations) < 50 {
		numWorkers = len(locations)
	}

	// Create channels for job distribution and result collection
	jobs := make(chan database.Location, len(locations))
	results := make(chan DetectionResult, len(locations))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, db, jobs, results, anomalyDetector, alarmSuggester, &wg)
	}

	// Send all locations to job queue
	for _, location := range locations {
		jobs <- location
	}
	close(jobs)

	// Wait for all workers to finish, then close results channel
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and process results
	totalAnomalies := 0
	totalSuggestions := 0
	totalErrors := 0
	locationCount := 0

	for result := range results {
		locationCount++

		if result.Error != nil {
			log.Printf("[%d/%d] ❌ %s: %v (%.1fs)",
				locationCount, len(locations), result.Location, result.Error, result.ProcessingTime.Seconds())
			totalErrors++
			continue
		}

		if len(result.Anomalies) > 0 {
			// Store anomalies in database
			if err := db.StoreAnomalies(result.Anomalies); err != nil {
				log.Printf("[%d/%d] Failed to store anomalies for %s: %v",
					locationCount, len(locations), result.Location, err)
				totalErrors++
			} else {
				totalAnomalies += len(result.Anomalies)

				// Store alarm suggestions
				if len(result.Suggestions) > 0 {
					for _, suggestion := range result.Suggestions {
						if err := db.StoreAlarmSuggestion(&suggestion); err != nil {
							log.Printf("Failed to store alarm suggestion for %s: %v", result.Location, err)
						} else {
							totalSuggestions++
						}
					}
				}

				log.Printf("[%d/%d] ✓ %s: %d anomalies, %d suggestions (%.1fs)",
					locationCount, len(locations), result.Location,
					len(result.Anomalies), len(result.Suggestions), result.ProcessingTime.Seconds())
			}
		} else {
			log.Printf("[%d/%d] ✓ %s: no anomalies (%.1fs)",
				locationCount, len(locations), result.Location, result.ProcessingTime.Seconds())
		}
	}

	totalDuration := time.Since(startTime)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	log.Printf("Detection complete in %.1f minutes (%.1f seconds)", totalDuration.Minutes(), totalDuration.Seconds())
	log.Printf("  Locations: %d processed, %d errors", locationCount-totalErrors, totalErrors)
	log.Printf("  Anomalies: %d found", totalAnomalies)
	log.Printf("  Suggestions: %d generated", totalSuggestions)
	log.Printf("  Avg time/location: %.1fs", totalDuration.Seconds()/float64(locationCount))
	log.Printf("  Workers: %d", numWorkers)
	log.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
}

// worker processes locations from the jobs channel
func worker(id int, db *database.DB, jobs <-chan database.Location, results chan<- DetectionResult,
	anomalyDetector *detector.AnomalyDetector, alarmSuggester *detector.AlarmSuggester, wg *sync.WaitGroup) {
	defer wg.Done()

	for location := range jobs {
		startTime := time.Now()

		// Detect anomalies for this location
		anomalies, err := anomalyDetector.DetectAnomalies(db, location.Name)
		if err != nil {
			results <- DetectionResult{
				Location:       location.Name,
				Error:          err,
				ProcessingTime: time.Since(startTime),
			}
			continue
		}

		// Generate alarm suggestions if anomalies found
		var suggestions []models.AlarmSuggestion
		if len(anomalies) > 0 {
			suggestions = alarmSuggester.SuggestAlarms(anomalies, location.Name)
		}

		results <- DetectionResult{
			Location:       location.Name,
			Anomalies:      anomalies,
			Suggestions:    suggestions,
			ProcessingTime: time.Since(startTime),
		}
	}
}
