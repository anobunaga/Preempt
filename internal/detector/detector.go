package detector

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"preempt/internal/config"
	"preempt/internal/database"
	"preempt/internal/models"
	"time"
)

// AnomalyDetector detects anomalies in metrics data
type AnomalyDetector struct {
	zScoreThreshold float64 // Standard deviations from mean to flag as anomaly
	cfg             *config.Config
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
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		zScoreThreshold: 2.0, // Flag values more than 2 std devs from mean
		cfg:             config.Get(),
	}
}

// DetectAnomalies detects anomalies by querying historical metrics from the database and using z score and ML model
func (ad *AnomalyDetector) DetectAnomalies(db *database.DB) ([]models.Anomaly, error) {

	stats_anomalies, err := ad.getStatsAnomalies(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies via stats method %s", err)
	}
	ml_anomalies, err := ad.getMLAnomalies(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get anomalies via machine learning model method %s", err)
	}

	//combine with stats z-score anomalies and return total list
	anomalies := append(stats_anomalies, ml_anomalies...)

	return anomalies, nil
}

func (ad *AnomalyDetector) getStatsAnomalies(db *database.DB) ([]models.Anomaly, error) {
	var anomalies []models.Anomaly
	now := time.Now()

	// Define metric types list
	metricTypes := ad.cfg.Weather.MonitoredFields

	// Get historical data for the last 7 days
	since := now.AddDate(0, 0, -7)
	metrics, err := db.GetMetrics(metricTypes, since)
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
	recentMetrics, err := db.GetMetrics(metricTypes, recentSince)
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

func (ad *AnomalyDetector) getMLAnomalies(db *database.DB) ([]models.Anomaly, error) {
	var anomalies []models.Anomaly
	// Export data to a temporary CSV file for Python to read
	tempFile, err := os.Create("metrics.csv")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name()) // Clean up
	defer tempFile.Close()
	fmt.Println("CSV file created at:", tempFile.Name())

	// Write CSV header
	if _, err := tempFile.WriteString("timestamp,metric_type,value\n"); err != nil {
		return nil, fmt.Errorf("failed to write header: %w", err)
	}

	// Get all metrics from the last 30 days
	metricTypes := ad.cfg.Weather.MonitoredFields
	since := time.Now().AddDate(0, 0, -30)
	metrics, err := db.GetMetrics(metricTypes, since) // Get all metric types
	if err != nil {
		return nil, fmt.Errorf("failed to get metrics: %w", err)
	}

	if len(metrics) < 10 {
		log.Printf("Not enough data for ML training (need at least 10, got %d)", len(metrics))
		return anomalies, nil // Return empty list, not an error
	}

	// Write metrics to CSV
	for _, m := range metrics {
		line := fmt.Sprintf("%s,%s,%.2f\n", m.Timestamp.Format(time.RFC3339), m.MetricType, m.Value)
		if _, err := tempFile.WriteString(line); err != nil {
			return nil, fmt.Errorf("failed to write metric: %w", err)
		}
	}
	tempFile.Close()

	// Run the Python script and capture output
	cmd := exec.Command("python3", "internal/ml/train.py", tempFile.Name())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run train.py: %w, output: %s", err, string(output))
	}

	log.Printf("ML model output: %s", string(output))

	// Parse the JSON output from Python
	var result MLAnomalyResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse ML output as JSON: %w, output: %s", err, string(output))
	}

	log.Printf("ML processed %d metric types and found %d total anomalies", result.ModelsSaved, result.TotalAnomaliesFound)
	log.Printf("Metrics processed: %v", result.MetricsProcessed)

	// Convert ML anomalies to our Anomaly model
	for _, mlAnomaly := range result.Anomalies {
		// Parse timestamp
		timestamp, err := time.Parse(time.RFC3339, mlAnomaly.Timestamp)
		if err != nil {
			log.Printf("Failed to parse timestamp %s: %v", mlAnomaly.Timestamp, err)
			continue
		}

		anomaly := models.Anomaly{
			Timestamp:  timestamp,
			MetricType: mlAnomaly.MetricType,
			Value:      mlAnomaly.Value,
			ZScore:     mlAnomaly.AnomalyScore, // Use anomaly score as Z-score equivalent
			Severity:   mlAnomaly.Severity,
		}
		anomalies = append(anomalies, anomaly)
	}

	return anomalies, nil
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
