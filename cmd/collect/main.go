package main

import (
	"context"
	"encoding/json"
	"log"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"strings"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	historicalDays        = 7
	maxConcurrentRequests = 2 // Limit concurrent API requests
	maxRetries            = 3
)

func main() {
	config.Load("./config.yaml")
	cfg := config.Get()

	// Initialize Redis client
	redisCfg := config.GetRedisConfig()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	defer redisClient.Close()

	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	client := api.NewOpenMeteoClient()

	// Get all locations that already have data in the database
	locationsWithData, err := db.GetLocationsWithData()
	if err != nil {
		log.Fatalf("Failed to get locations with data: %v", err)
	}

	// Semaphore to limit concurrent API requests
	semaphore := make(chan struct{}, maxConcurrentRequests)
	var wg sync.WaitGroup

	// Check each location and fetch historical data only for new locations
	for _, location := range cfg.Weather.Locations {
		wg.Add(1)
		go func(loc config.Location) {
			defer wg.Done()

			// Acquire semaphore (blocks if max concurrent requests reached)
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Retry with exponential backoff
			for attempt := 0; attempt < maxRetries; attempt++ {
				var err error
				var success bool

				if !locationsWithData[loc.Name] {
					if attempt > 0 {
						log.Printf("Retry %d/%d: Fetching historical data for %s", attempt+1, maxRetries, loc.Name)
					} else {
						log.Printf("New location detected: %s - Fetching historical data", loc.Name)
					}
					forecast, fetchErr := client.GetHistoricalHourlyData(loc.Latitude, loc.Longitude, cfg.Weather.MonitoredFields, historicalDays)
					err = fetchErr
					if err == nil {
						sendToRedis(redisClient, forecast, loc, cfg.Weather.MonitoredFields, "historical")
						success = true
					}
				} else {
					if attempt > 0 {
						log.Printf("Retry %d/%d: Fetching current data for %s", attempt+1, maxRetries, loc.Name)
					} else {
						log.Printf("Fetching current weather data for: %s", loc.Name)
					}
					weatherData, fetchErr := client.GetCurrentWeather(loc.Latitude, loc.Longitude, cfg.Weather.MonitoredFields)
					err = fetchErr
					if err == nil {
						sendToRedis(redisClient, weatherData, loc, cfg.Weather.MonitoredFields, "current")
						success = true
					}
				}

				if success {
					return
				}

				// Check if rate limit error (429)
				isRateLimitError := strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too many")

				if isRateLimitError && attempt < maxRetries-1 {
					backoff := time.Duration(1<<uint(attempt)) * time.Second // 1s, 2s, 4s
					log.Printf("Rate limit error for %s, retrying in %v", loc.Name, backoff)
					time.Sleep(backoff)
					continue
				}

				log.Printf("Failed to fetch data for %s: %v", loc.Name, err)
				return
			}
		}(location)
	}

	wg.Wait()
	log.Printf("Data collection completed. Exiting")
}

// sendToRedis serializes the forecast data and publishes it to a Redis stream
func sendToRedis(redisClient *redis.Client, forecast interface{}, location config.Location, fields []string, dataType string) {
	// Serialize forecast and publish to Redis stream
	data, err := json.Marshal(map[string]interface{}{
		"location": location,
		"forecast": forecast,
		"fields":   fields,
		"type":     dataType,
	})
	if err != nil {
		log.Printf("Failed to serialize data for %s: %v", location.Name, err)
		return
	}

	err = redisClient.XAdd(context.Background(), &redis.XAddArgs{
		Stream: config.GetRedisConfig().Stream,
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
	if err != nil {
		log.Printf("Failed to publish to Redis for %s: %v", location.Name, err)
	} else {
		log.Printf("Published %s data for %s to Redis", dataType, location.Name)
	}
}
