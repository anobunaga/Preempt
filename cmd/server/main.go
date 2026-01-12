package main

import (
	"log"
	"preempt/internal/api"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/detector"
	"preempt/internal/server"

	"github.com/go-redis/redis/v8"
)

func main() {
	// Load config
	if _, err := config.Load("./config.yaml"); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	cfg := config.Get()

	// Initialize database
	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer redisClient.Close()

	openMeteoClient := api.NewOpenMeteoClient()
	anomalyDetector := detector.NewAnomalyDetector(redisClient)

	srv := server.NewServer(db, openMeteoClient, anomalyDetector)

	log.Println("Server running on http://localhost:8080")

	if err := srv.Start(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
