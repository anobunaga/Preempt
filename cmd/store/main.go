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
	"preempt/internal/wal"
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

	// Initialize WAL
	walPath := os.Getenv("WAL_PATH")
	if walPath == "" {
		walPath = "/app/wal/store.wal"
	}
	
	walInstance, err := wal.NewWAL(walPath)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer walInstance.Close()
	
	log.Printf("WAL initialized at %s", walPath)

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

	// Process any pending WAL entries from previous crash
	if err := processPendingWAL(walInstance, db, redisClient, stream, consumerGroup); err != nil {
		log.Printf("Warning: Error processing pending WAL entries: %v", err)
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

	// Start pending message cleanup goroutine
	go cleanupPendingMessages(ctx, redisClient, walInstance, db, stream, consumerGroup, consumerName)

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

				// Step 1: Write to WAL FIRST (durability!)
				if err := walInstance.Write(m.ID, []byte(m.Values["data"].(string))); err != nil {
					log.Printf("Failed to write to WAL for message %s: %v", m.ID, err)
					continue // Will retry via pending cleanup
				}

				// Step 2: Unmarshal the data
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
					log.Printf("Failed to unmarshal message %s: %v", m.ID, err)
					// Permanent error - ACK anyway to avoid reprocessing
					redisClient.XAck(ctx, stream, consumerGroup, m.ID)
					walInstance.Remove(m.ID)
					continue
				}

				// Step 3: Convert to models.Forecast
				forecast := &models.Forecast{}
				if err := json.Unmarshal(payload.Forecast, forecast); err != nil {
					log.Printf("Failed to unmarshal forecast for %s: %v", payload.Location.Name, err)
					// Permanent error - ACK anyway
					redisClient.XAck(ctx, stream, consumerGroup, m.ID)
					walInstance.Remove(m.ID)
					continue
				}

				// Step 4: Store in DB (idempotent - safe to retry)
				isInitial := payload.Type == "historical"
				if err := db.StoreMetrics(forecast, payload.Location.Name, payload.Fields, isInitial); err != nil {
					log.Printf("Failed to store metrics for %s: %v", payload.Location.Name, err)
					// Don't ACK - will retry via pending cleanup
					continue
				}

				log.Printf("Stored %s data for %s (%.2f, %.2f)",
					payload.Type, payload.Location.Name,
					payload.Location.Latitude, payload.Location.Longitude)

				// Step 5: Remove from WAL (committed)
				if err := walInstance.Remove(m.ID); err != nil {
					log.Printf("Failed to remove from WAL: %v", err)
					// Continue anyway - will be cleaned up during compaction
				}

				// Step 6: ACK message (ONLY after successful DB write)
				if err := redisClient.XAck(ctx, stream, consumerGroup, m.ID).Err(); err != nil {
					log.Printf("Failed to ACK message %s: %v", m.ID, err)
					// Message processed but not ACKed - pending cleanup will find it
				}

				// Trim stream to prevent unbounded growth
				redisClient.XTrimMaxLen(ctx, stream, 1000).Err()
			}
		}
	}

	log.Println("Store service stopped")
}

// processPendingWAL replays uncommitted WAL entries on startup (crash recovery)
func processPendingWAL(walInstance *wal.WAL, db *database.DB, redisClient *redis.Client, stream, consumerGroup string) error {
	entries, err := walInstance.GetPending()
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		log.Println("No pending WAL entries to process")
		return nil
	}

	log.Printf("⚠️  Processing %d pending WAL entries from previous run (crash recovery)...", len(entries))

	successCount := 0
	for _, entry := range entries {
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

		if err := json.Unmarshal(entry.Payload, &payload); err != nil {
			log.Printf("Failed to unmarshal WAL entry %s: %v", entry.MessageID, err)
			walInstance.Remove(entry.MessageID) // Remove corrupted entry
			continue
		}

		forecast := &models.Forecast{}
		if err := json.Unmarshal(payload.Forecast, forecast); err != nil {
			log.Printf("Failed to unmarshal forecast from WAL entry %s: %v", entry.MessageID, err)
			walInstance.Remove(entry.MessageID)
			continue
		}

		// Store in DB (idempotent - safe to retry)
		isInitial := payload.Type == "historical"
		if err := db.StoreMetrics(forecast, payload.Location.Name, payload.Fields, isInitial); err != nil {
			log.Printf("Failed to store metrics from WAL entry %s: %v", entry.MessageID, err)
			continue // Leave in WAL, will retry later
		}

		// Remove from WAL
		walInstance.Remove(entry.MessageID)

		// ACK in Redis
		if err := redisClient.XAck(context.Background(), stream, consumerGroup, entry.MessageID).Err(); err != nil {
			log.Printf("Failed to ACK WAL entry %s: %v", entry.MessageID, err)
		}

		successCount++
	}

	log.Printf("✓ WAL replay complete: %d/%d entries processed successfully", successCount, len(entries))
	return nil
}

// cleanupPendingMessages monitors and reclaims stuck messages in Redis stream
func cleanupPendingMessages(ctx context.Context, redisClient *redis.Client, walInstance *wal.WAL, 
	db *database.DB, stream, consumerGroup, consumerName string) {
	
	ticker := time.NewTicker(60 * time.Second) // Check every minute
	defer ticker.Stop()

	log.Println("Started pending message cleanup monitor")

	for {
		select {
		case <-ctx.Done():
			log.Println("Pending message cleanup stopped")
			return
		case <-ticker.C:
			// Get pending messages older than 5 minutes
			pending, err := redisClient.XPendingExt(ctx, &redis.XPendingExtArgs{
				Stream: stream,
				Group:  consumerGroup,
				Start:  "-",
				End:    "+",
				Count:  100, // Process up to 100 stuck messages per cycle
			}).Result()

			if err != nil {
				log.Printf("Error fetching pending messages: %v", err)
				continue
			}

			// Filter messages stuck for >5 minutes
			var stuckMessages []string
			for _, msg := range pending {
				if msg.Idle > 5*time.Minute {
					stuckMessages = append(stuckMessages, msg.ID)
				}
			}

			if len(stuckMessages) == 0 {
				continue
			}

			log.Printf("⚠️  Found %d stuck messages (>5 min idle), reclaiming...", len(stuckMessages))

			// Claim messages for reprocessing
			for _, msgID := range stuckMessages {
				claimed, err := redisClient.XClaim(ctx, &redis.XClaimArgs{
					Stream:   stream,
					Group:    consumerGroup,
					Consumer: consumerName,
					MinIdle:  5 * time.Minute,
					Messages: []string{msgID},
				}).Result()

				if err != nil {
					log.Printf("Error claiming message %s: %v", msgID, err)
					continue
				}

				if len(claimed) == 0 {
					continue
				}

				msg := claimed[0]
				log.Printf("Retrying stuck message: %s", msg.ID)

				// Write to WAL if not already there
				walInstance.Write(msg.ID, []byte(msg.Values["data"].(string)))

				// Parse and process
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

				if err := json.Unmarshal([]byte(msg.Values["data"].(string)), &payload); err != nil {
					log.Printf("Failed to unmarshal reclaimed message %s: %v", msg.ID, err)
					redisClient.XAck(ctx, stream, consumerGroup, msg.ID).Err() // ACK to remove from pending
					walInstance.Remove(msg.ID)
					continue
				}

				forecast := &models.Forecast{}
				if err := json.Unmarshal(payload.Forecast, forecast); err != nil {
					log.Printf("Failed to unmarshal forecast from reclaimed message %s: %v", msg.ID, err)
					redisClient.XAck(ctx, stream, consumerGroup, msg.ID).Err()
					walInstance.Remove(msg.ID)
					continue
				}

				// Store in DB (idempotent)
				isInitial := payload.Type == "historical"
				if err := db.StoreMetrics(forecast, payload.Location.Name, payload.Fields, isInitial); err != nil {
					log.Printf("Failed to store reclaimed message %s: %v", msg.ID, err)
					continue // Will retry in next cycle
				}

				// Success - clean up
				walInstance.Remove(msg.ID)
				if err := redisClient.XAck(ctx, stream, consumerGroup, msg.ID).Err(); err != nil {
					log.Printf("Failed to ACK recovered message %s: %v", msg.ID, err)
				}
				log.Printf("✓ Recovered stuck message: %s", msg.ID)
			}
		}
	}
}

