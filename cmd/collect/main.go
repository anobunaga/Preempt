package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

const (
	collectionInterval = 5 * time.Minute
	historicalDays     = 7
)

func main() {
	config.Load("./config.yaml")
	cfg := config.Get()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	db, err := database.NewDB("myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	client := api.NewOpenMeteoClient()

	// Iterate over all locations and fetch historical data in goroutines
	for _, location := range cfg.Locations {
		go func(loc config.Location) {
			log.Printf("Fetching historical data for %s", loc.Name)
			forecast, err := client.GetHistoricalHourlyData(loc.Latitude, loc.Longitude, cfg.Weather.MonitoredFields, historicalDays)
			if err != nil {
				log.Printf("Failed to fetch historical forecast for %s: %v", loc.Name, err)
				return
			}

			sendToRedis(redisClient, forecast, loc, cfg.Weather.MonitoredFields, "historical")
		}(location)
	}

	// Start periodic data collection
	go startDataCollection(client, redisClient, cfg)

	log.Println("Collector running. Press Ctrl+C to stop...")
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit

	log.Println("Shutting down collector...")
}

// startDataCollection periodically fetches data from the API (every 5 min)
func startDataCollection(client *api.OpenMeteoClient, redisClient *redis.Client, cfg *config.Config) {
	ticker := time.NewTicker(collectionInterval)
	defer ticker.Stop()

	for range ticker.C {
		log.Println("Fetching current data from Open-Meteo API")

		for _, location := range cfg.Locations {
			forecast, err := client.GetCurrentWeather(location.Latitude, location.Longitude, cfg.Weather.MonitoredFields)
			if err != nil {
				log.Printf("Failed to fetch forecast for %s: %v", location.Name, err)
				continue
			}

			sendToRedis(redisClient, forecast, location, cfg.Weather.MonitoredFields, "current")
		}
	}
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
		Stream: config.Get().Redis.Stream,
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
	if err != nil {
		log.Printf("Failed to publish to Redis for %s: %v", location.Name, err)
	} else {
		log.Printf("Published %s data for %s to Redis", dataType, location.Name)
	}
}
