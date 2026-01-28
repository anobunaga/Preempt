package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"preempt/internal/config"
	"preempt/internal/database"
	_ "preempt/internal/metrics"
	"preempt/internal/models"
	"syscall"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// Load config
	config.Load("./config.yaml")

	// Initialize Redis client from environment variables
	redisCfg := config.GetRedisConfig()
	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	defer redisClient.Close()

	// Initialize database
	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Consumer group and name
	consumerGroup := "weather_consumers"
	consumerName := "consumer-1"
	stream := redisCfg.Stream

	log.Printf("Connecting to Redis at %s", redisCfg.Addr)

	// Test Redis connection with retry
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		ctx := context.Background()
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Redis connection attempt %d/%d failed: %v", i+1, maxRetries, err)
			time.Sleep(time.Second * 2)
			continue
		}
		log.Println("Successfully connected to Redis")
		break
	}

	// Create consumer group if it doesn't exist
	err = redisClient.XGroupCreateMkStream(context.Background(), stream, consumerGroup, "0").Err()
	if err != nil && err.Error() != "BUSYGROUP Consumer Group name already exists" {
		log.Fatalf("Failed to create consumer group: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start metrics endpoint on port 8081
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/prometheus", promhttp.Handler())
		log.Println("Store metrics endpoint started on :8081/prometheus")
		if err := http.ListenAndServe(":8081", mux); err != nil {
			log.Printf("Metrics endpoint error: %v", err)
		}
	}()

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
					Location struct {
						Name      string  `json:"name"`
						Latitude  float64 `json:"latitude"`
						Longitude float64 `json:"longitude"`
					} `json:"location"`
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

				// Trim weather_metrics stream to prevent unbounded growth (keep last 1000 messages)
				redisClient.XTrimMaxLen(context.Background(), stream, 1000).Err()
			}
		}
	}

	log.Println("Store service stopped")
}
