package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/models"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Load config
	config.Load("./config.yaml")
	cfg := config.Get()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	// Initialize database
	db, err := database.NewDB("myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true") // Adjust DSN as needed
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Consumer group and name
	consumerGroup := "weather_consumers"
	consumerName := "consumer-1"
	stream := cfg.Redis.Stream

	// Create consumer group if it doesn't exist
	err = redisClient.XGroupCreate(context.Background(), stream, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Fatalf("Failed to create consumer group: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signal
	go func() {
		<-quit
		log.Println("Shutting down store service...")
		cancel()
	}()

	log.Println("Store into db started, reading from Redis stream. Press Ctrl+C to stop...")

	// Read from stream in a loop
	for {
		msgs, err := redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    consumerGroup,
			Consumer: consumerName,
			Streams:  []string{stream, ">"},
			Count:    10,              // Process up to 10 messages at a time
			Block:    time.Second * 5, // Block for 5 seconds if no messages
		}).Result()

		if ctx.Err() != nil {
			// Context cancelled, exit gracefully
			break
		}

		if err != nil && err != redis.Nil {
			log.Printf("Error reading from Redis: %v", err)
			continue
		}

		for _, msg := range msgs {
			for _, m := range msg.Messages {
				// Check if shutdown requested
				if ctx.Err() != nil {
					log.Println("Store service stopped")
					return
				}

				// Unmarshal the data
				var payload struct {
					Location config.Location `json:"location"`
					Forecast json.RawMessage `json:"forecast"`
					Fields   []string        `json:"fields"`
					Type     string          `json:"type"`
				}

				err := json.Unmarshal([]byte(m.Values["data"].(string)), &payload)
				if err != nil {
					log.Printf("Failed to unmarshal message: %v", err)
					continue
				}

				// Convert to models.Forecast
				forecast := &models.Forecast{}
				if err := json.Unmarshal(payload.Forecast, forecast); err != nil {
					log.Printf("Failed to unmarshal forecast for %s: %v", payload.Location.Name, err)
					continue
				}

				// Store in DB
				isInitial := payload.Type == "historical"
				if err := db.StoreMetrics(forecast, payload.Location.Name, payload.Fields, isInitial); err != nil {
					log.Printf("Failed to store metrics for %s: %v", payload.Location.Name, err)
					continue
				}

				log.Printf("Stored %s data for %s (%.2f, %.2f)",
					payload.Type, payload.Location.Name,
					payload.Location.Latitude, payload.Location.Longitude)

				// Acknowledge the message
				redisClient.XAck(context.Background(), stream, consumerGroup, m.ID)
			}
		}
	}

	log.Println("Store service stopped")
}
