package main

import (
	"encoding/csv"
	"io"
	"log"
	"os"
	"preempt/internal/config"
	"preempt/internal/database"
	"strconv"
)

func main() {
	// Load config for database connection
	config.Load("./config.yaml")

	// Initialize database
	db, err := database.NewDB(config.GetDatabaseDSN())
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	csvPath := "locations_seed.csv"
	file, err := os.Open(csvPath)
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read header row
	header, err := reader.Read()
	if err != nil {
		log.Fatalf("Failed to read CSV header: %v", err)
	}
	log.Printf("CSV Header: %v\n", header)

	// Read and insert all locations
	count := 0
	skipped := 0

	for {
		record, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatalf("Failed to read CSV record: %v", err)
		}

		if len(record) < 3 {
			log.Printf("Skipping invalid record: %v", record)
			skipped++
			continue
		}

		name := record[0]
		latitude, err := strconv.ParseFloat(record[1], 64)
		if err != nil {
			log.Printf("Skipping record with invalid latitude: %v", record)
			skipped++
			continue
		}

		longitude, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			log.Printf("Skipping record with invalid longitude: %v", record)
			skipped++
			continue
		}

		// Insert location into database
		err = db.InsertLocation(name, latitude, longitude)
		if err != nil {
			// Check if it's a duplicate error
			if err.Error() == "duplicate location" {
				log.Printf("Location already exists: %s", name)
				skipped++
			} else {
				log.Printf("Failed to insert location %s: %v", name, err)
				skipped++
			}
			continue
		}

		count++
		if count%100 == 0 {
			log.Printf("Inserted %d locations...", count)
		}
	}

	log.Printf("Import complete! Successfully inserted %d locations, skipped %d", count, skipped)
}
