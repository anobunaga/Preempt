package detector

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/models"
	"time"

	"github.com/go-redis/redis/v8"
)

// AnomalyDetector detects anomalies in metrics data
type AnomalyDetector struct {
	zScoreThreshold float64 // Standard deviations from mean to flag as anomaly
	cfg             *config.Config
	redisClient     *redis.Client
}

// MLAnomalyResult represents the JSON output from the Python ML script
type MLAnomalyResult struct {
	ModelsSaved         int             `json:"models_saved"`
	TotalAnomaliesFound int             `json:"total_anomalies_found"`
	Anomalies           []MLAnomalyData `json:"anomalies"`
	MetricsProcessed    []string        `json:"metrics_processed"`
}

type MLAnomalyData struct {
	Timestamp    string  `json:"timestamp"`
	MetricType   string  `json:"metric_type"`
	Value        float64 `json:"value"`
	AnomalyScore float64 `json:"anomaly_score"`
	Severity     string  `json:"severity"`
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector(redisClient *redis.Client) *AnomalyDetector {
	return &AnomalyDetector{
		zScoreThreshold: 2.0, // Flag values more than 2 std devs from mean
		cfg:             config.Get(),
		redisClient:     redisClient,
	}
}

// DetectAnomalies detects anomalies by querying historical metrics from the database and using z score and ML model
func (ad *AnomalyDetector) DetectAnomalies(db *database.DB, location string) ([]models.Anomaly, error) {

	stats_anomalies, err := ad.getStatsAnomalies(db, location)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies via stats method %s", err)
	}
	ml_anomalies, err := ad.getMLAnomalies(db, location)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies via machine learning model method %s", err)
	}

	//combine with stats z-score anomalies and return total list
	anomalies := append(stats_anomalies, ml_anomalies...)

	return anomalies, nil
}

func (ad *AnomalyDetector) getStatsAnomalies(db *database.DB, location string) ([]models.Anomaly, error) {
	var anomalies []models.Anomaly
	now := time.Now()

	// Define metric types list
	metricTypes := ad.cfg.Weather.MonitoredFields

	// Get historical data for the last 7 days
	since := now.AddDate(0, 0, -7)
	metrics, err := db.GetMetrics(location, metricTypes, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics %w", err)
	}

	// Group metrics by type
	metricsByType := make(map[string][]models.Metric)
	for _, m := range metrics {
		metricsByType[m.MetricType] = append(metricsByType[m.MetricType], m)
	}

	// Get recent metrics (last 24 hours) - single query
	recentSince := now.Add(-24 * time.Hour)
	recentMetrics, err := db.GetMetrics(location, metricTypes, recentSince)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent metrics: %w", err)
	}

	// Group recent metrics by type
	recentByType := make(map[string][]models.Metric)
	for _, m := range recentMetrics {
		recentByType[m.MetricType] = append(recentByType[m.MetricType], m)
	}

	// Process each metric type independently
	for _, metricType := range metricTypes {
		metrics := metricsByType[metricType]

		if len(metrics) < 3 {
			log.Printf("Warning: not enough data for %s (%d samples)", metricType, len(metrics))
			continue // Not enough data for statistical analysis
		}

		// Extract values for THIS metric type
		var values []float64
		for _, m := range metrics {
			values = append(values, m.Value)
		}

		// Calculate mean and std dev for THIS metric type
		mean := calculateMean(values)
		stdDev := calculateStdDev(values, mean)

		log.Printf("  %s: mean=%.2f, stdDev=%.2f, samples=%d", metricType, mean, stdDev, len(values))

		if stdDev == 0 {
			log.Printf("  %s: no variation in data, skipping", metricType)
			continue // No variation, no anomalies
		}

		// Get recent metrics for THIS metric type
		recentForType := recentByType[metricType]

		// Check each recent metric against THIS metric type's statistics from past 7 days
		anomalyCount := 0
		for _, m := range recentForType {
			zScore := CalculateZScore(m.Value, mean, stdDev)
			if IsOutlier(zScore) {
				severity := calculateSeverityFromZScore(zScore)
				anomalies = append(anomalies, models.Anomaly{
					Location:   location,
					Timestamp:  m.Timestamp,
					MetricType: metricType,
					Value:      m.Value,
					ZScore:     zScore,
					Severity:   severity,
				})
				anomalyCount++
			}
		}

		log.Printf("  %s: found %d anomalies", metricType, anomalyCount)
	}

	return anomalies, nil
}

func (ad *AnomalyDetector) getMLAnomalies(db *database.DB, location string) ([]models.Anomaly, error) {
	var anomalies []models.Anomaly
	ctx := context.Background()

	// Get all metrics from the last 30 days
	metricTypes := ad.cfg.Weather.MonitoredFields
	since := time.Now().AddDate(0, 0, -30)
	metrics, err := db.GetMetrics(location, metricTypes, since)
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	if len(metrics) < 10 {
		log.Printf("Not enough data for ML training (need at least 10, got %d)", len(metrics))
		return anomalies, nil
	}

	// Convert metrics to JSON format for Redis
	type MetricData struct {
		Timestamp  string  `json:"timestamp"`
		MetricType string  `json:"metric_type"`
		Value      float64 `json:"value"`
	}

	var metricsData []MetricData
	for _, m := range metrics {
		metricsData = append(metricsData, MetricData{
			Timestamp:  m.Timestamp.Format(time.RFC3339),
			MetricType: m.MetricType,
			Value:      m.Value,
		})
	}

	// Create unique job ID
	jobID := fmt.Sprintf("%s_%d", location, time.Now().Unix())

	// Get current position in ml_output stream before publishing job
	lastID := "0-0"
	lastMessages, err := ad.redisClient.XRevRangeN(ctx, "ml_output", "+", "-", 1).Result()
	if err == nil && len(lastMessages) > 0 {
		lastID = lastMessages[0].ID
	}

	// Publish metrics to Redis stream for ML processing
	payload := map[string]interface{}{
		"location": location,
		"metrics":  metricsData,
		"job_id":   jobID,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Send to ML input stream
	err = ad.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: "ml_input",
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
	if err != nil {
		return nil, fmt.Errorf("failed to publish to Redis ML stream: %w", err)
	}

	log.Printf("Published %d metrics to ML input stream for location %s (job_id: %s)", len(metricsData), location, jobID)

	// Wait for ML results (with timeout)
	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("timeout waiting for ML results for job %s", jobID)
		case <-ticker.C:
			// Read messages published after we sent the job
			messages, err := ad.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{"ml_output", lastID},
				Count:   10,
				Block:   0,
			}).Result()

			if err != nil {
				log.Printf("Error reading from ml_output: %v", err)
				continue
			}

			if len(messages) == 0 {
				continue
			}

			// Look for our job results
			foundJobIDs := []string{}
			for _, message := range messages {
				for _, msg := range message.Messages {
					dataStr, ok := msg.Values["data"].(string)
					if !ok {
						log.Printf("Warning: message has no 'data' field")
						continue
					}

					var result struct {
						JobID               string          `json:"job_id"`
						Location            string          `json:"location"`
						ModelsSaved         int             `json:"models_saved"`
						TotalAnomaliesFound int             `json:"total_anomalies_found"`
						Anomalies           []MLAnomalyData `json:"anomalies"`
						MetricsProcessed    []string        `json:"metrics_processed"`
					}

					if err := json.Unmarshal([]byte(dataStr), &result); err != nil {
						log.Printf("Failed to parse ML result: %v", err)
						continue
					}

					foundJobIDs = append(foundJobIDs, result.JobID)

					// Check if this is our job
					if result.JobID == jobID {
						log.Printf("✓ Found matching job %s!", jobID)
						log.Printf("ML processed %d metric types and found %d total anomalies for %s",
							result.ModelsSaved, result.TotalAnomaliesFound, location)
						log.Printf("Metrics processed: %v", result.MetricsProcessed)

						// Convert ML anomalies to our Anomaly model
						for _, mlAnomaly := range result.Anomalies {
							timestamp, err := time.Parse(time.RFC3339, mlAnomaly.Timestamp)
							if err != nil {
								log.Printf("Failed to parse timestamp %s: %v", mlAnomaly.Timestamp, err)
								continue
							}

							anomaly := models.Anomaly{
								Location:   location,
								Timestamp:  timestamp,
								MetricType: mlAnomaly.MetricType,
								Value:      mlAnomaly.Value,
								ZScore:     mlAnomaly.AnomalyScore,
								Severity:   mlAnomaly.Severity,
							}
							anomalies = append(anomalies, anomaly)
						}

						// Trim streams to prevent unbounded growth (keep last 500 messages)
						ad.redisClient.XTrimMaxLen(ctx, "ml_input", 500).Err()
						ad.redisClient.XTrimMaxLen(ctx, "ml_output", 500).Err()

						return anomalies, nil
					}
				}
			}

			// Log all job_ids we found (for debugging)
			if len(foundJobIDs) > 0 && len(foundJobIDs) <= 10 {
				log.Printf("Job %s not found. Found job_ids: %v", jobID, foundJobIDs)
			} else if len(foundJobIDs) > 10 {
				log.Printf("Job %s not found. Checked %d jobs (showing first 10): %v", jobID, len(foundJobIDs), foundJobIDs[:10])
			}
		}
	}
}

// calculateSeverityFromZScore determines severity based on Z-score
func calculateSeverityFromZScore(zScore float64) string {
	absZScore := math.Abs(zScore)
	if absZScore > 2.0 {
		return "high"
	} else if absZScore > 1.5 {
		return "medium"
	}
	return "low"
}

// CalculateZScore calculates the Z-score for a value given mean and standard deviation
func CalculateZScore(value, mean, stdDev float64) float64 {
	if stdDev == 0 {
		return 0
	}
	return (value - mean) / stdDev
}

// IsOutlier checks if a Z-score indicates an outlier (> 2 std devs from mean)
func IsOutlier(zScore float64) bool {
	return math.Abs(zScore) > 1.0
}
