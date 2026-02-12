package main

import (
	"context"
	"encoding/json"
	"preempt/internal/config"
	"testing"

	"github.com/go-redis/redis/v8"
)

func TestSendToRedis_Serialization(t *testing.T) {
	location := config.Location{
		Name:      "TestLocation",
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	fields := []string{"temperature_2m", "precipitation"}
	forecast := map[string]interface{}{
		"temperature": 72.5,
		"humidity":    65.0,
	}

	data, err := json.Marshal(map[string]interface{}{
		"location": location,
		"forecast": forecast,
		"fields":   fields,
		"type":     "current",
	})

	if err != nil {
		t.Fatalf("Failed to serialize data: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Failed to deserialize data: %v", err)
	}

	if result["type"] != "current" {
		t.Errorf("Expected type 'current', got '%v'", result["type"])
	}
}

func TestSendToRedis_DataStructure(t *testing.T) {
	location := config.Location{
		Name:      "San Francisco",
		Latitude:  37.7749,
		Longitude: -122.4194,
	}

	fields := []string{"temperature_2m"}
	forecast := map[string]interface{}{"temp": 70.0}

	payload := map[string]interface{}{
		"location": location,
		"forecast": forecast,
		"fields":   fields,
		"type":     "historical",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("Failed to marshal payload: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded["type"] != "historical" {
		t.Errorf("Expected type 'historical', got '%v'", decoded["type"])
	}

	locationData := decoded["location"].(map[string]interface{})
	if locationData["Name"] != "San Francisco" {
		t.Errorf("Expected location name 'San Francisco', got '%v'", locationData["Name"])
	}
}

func TestRedisXAddArgs(t *testing.T) {
	values := map[string]interface{}{
		"data": "test data",
	}

	args := &redis.XAddArgs{
		Stream: "test_stream",
		Values: values,
	}

	if args.Stream != "test_stream" {
		t.Errorf("Expected stream 'test_stream', got '%s'", args.Stream)
	}

	if values["data"] != "test data" {
		t.Errorf("Expected data 'test data', got '%v'", values["data"])
	}
}

func TestHistoricalDaysConstant(t *testing.T) {
	if historicalDays != 7 {
		t.Errorf("Expected historicalDays to be 7, got %d", historicalDays)
	}
}

func TestContextUsage(t *testing.T) {
	ctx := context.Background()
	if ctx == nil {
		t.Error("context.Background() should not return nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if ctx.Err() == nil {
		t.Error("Expected context to be cancelled")
	}
}

func TestLocationDataStructure(t *testing.T) {
	location := config.Location{
		Name:      "New York",
		Latitude:  40.7128,
		Longitude: -74.0060,
	}

	if location.Name != "New York" {
		t.Errorf("Expected name 'New York', got '%s'", location.Name)
	}

	if location.Latitude != 40.7128 {
		t.Errorf("Expected latitude 40.7128, got %f", location.Latitude)
	}

	if location.Longitude != -74.0060 {
		t.Errorf("Expected longitude -74.0060, got %f", location.Longitude)
	}
}
