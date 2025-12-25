package main

import (
	"context"
	"encoding/json"
	"log"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"sync"

	"github.com/go-redis/redis/v8"
)

const (
	historicalDays = 7
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

	var wg sync.WaitGroup

	// Check each location and fetch historical data only for new locations
	for _, location := range cfg.Weather.Locations {
		wg.Add(1)
		go func(loc config.Location) {
			defer wg.Done()

			if !locationsWithData[loc.Name] {
				log.Printf("New location detected: %s - Fetching historical data", loc.Name)
				forecast, err := client.GetHistoricalHourlyData(loc.Latitude, loc.Longitude, cfg.Weather.MonitoredFields, historicalDays)
				if err != nil {
					log.Printf("Failed to fetch historical forecast for %s: %v", loc.Name, err)
					return
				}
				sendToRedis(redisClient, forecast, loc, cfg.Weather.MonitoredFields, "historical")
			} else {
				log.Printf("Fetching current weather data for: %s", loc.Name)
				weatherData, err := client.GetCurrentWeather(loc.Latitude, loc.Longitude, cfg.Weather.MonitoredFields)
				if err != nil {
					log.Printf("Failed to fetch current weather data for %s: %v", loc.Name, err)
					return
				}
				sendToRedis(redisClient, weatherData, loc, cfg.Weather.MonitoredFields, "current")
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
