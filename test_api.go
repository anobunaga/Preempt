package main

import (
	"encoding/json"
	"fmt"
	"preempt/internal/api"
)

func main() {
	// Create API client
	client := api.NewOpenMeteoClient()

	// Test initial fetch (hourly data for 7 days)
	fmt.Println("=== Testing Initial Fetch (Hourly Data) ===")
	forecast, err := client.GetForecast(37.7749, -122.4194, true)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// Pretty print the response
	jsonData, _ := json.MarshalIndent(forecast, "", "  ")
	fmt.Println(string(jsonData))

	fmt.Println("\n=== Hourly Data Summary ===")
	fmt.Printf("Number of hourly temperatures: %d\n", len(forecast.Hourly.Temperature2m))
	fmt.Printf("Number of hourly humidity readings: %d\n", len(forecast.Hourly.RelativeHumidity2m))
	fmt.Printf("First temperature: %.1f°F\n", forecast.Hourly.Temperature2m[0])
	fmt.Printf("First humidity: %.1f%%\n", forecast.Hourly.RelativeHumidity2m[0])

	// Test current fetch
	fmt.Println("\n=== Testing Current Fetch ===")
	currentForecast, err := client.GetForecast(37.7749, -122.4194, false)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	// fmt.Printf("Current Temperature: %.1f°F\n", *currentForecast.Current.Temperature2m)
	// fmt.Printf("Current Humidity: %.1f%%\n", *currentForecast.Current.RelativeHumidity2m)
	fmt.Printf("Current temp data: \n")
	jsonData, _ = json.MarshalIndent(currentForecast, "", "  ")
	fmt.Println(string(jsonData))
}
