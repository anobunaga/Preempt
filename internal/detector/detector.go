package detector

import (
	"fmt"
	"math"
	"preempt/internal/models"
	"time"
)

// AnomalyDetector detects anomalies in metrics data
type AnomalyDetector struct {
	zScoreThreshold float64 // Standard deviations from mean to flag as anomaly
}

// DataPointAnomaly represents an anomaly found in a list of data points
type DataPointAnomaly struct {
	Index    int     // Position in the original data slice
	Value    float64 // The anomalous value
	ZScore   float64 // How many standard deviations from mean
	Severity string  // low, medium, high
}

// NewAnomalyDetector creates a new anomaly detector
func NewAnomalyDetector() *AnomalyDetector {
	return &AnomalyDetector{
		zScoreThreshold: 2.0, // Flag values more than 2 std devs from mean
	}
}

// DetectAnomaliesInDataPoints detects anomalies in a list of numbers using Z-score method
func (ad *AnomalyDetector) DetectAnomaliesInDataPoints(forecast *models.Forecast) ([]DataPointAnomaly, error) {
	//Modify this later to include relative humidity
	dataPoints := forecast.Hourly.Temperature2m
	if len(dataPoints) < 3 {
		return nil, fmt.Errorf("need at least 3 data points for anomaly detection, got %d", len(dataPoints))
	}

	// Calculate mean
	mean := calculateMean(dataPoints)

	// Calculate standard deviation
	stdDev := calculateStdDev(dataPoints, mean)

	// If stdDev is 0, all values are the same - no anomalies
	if stdDev == 0 {
		return []DataPointAnomaly{}, nil
	}

	var anomalies []DataPointAnomaly

	// Check each data point
	for i, value := range dataPoints {
		zScore := CalculateZScore(value, mean, stdDev)

		// If Z-score exceeds threshold, it's an anomaly
		if IsOutlier(zScore) {
			severity := calculateSeverityFromZScore(zScore)
			anomalies = append(anomalies, DataPointAnomaly{
				Index:    i,
				Value:    value,
				ZScore:   zScore,
				Severity: severity,
			})
		}
	}

	return anomalies, nil
}

// DetectAnomalies detects anomalies in the current forecast data
func (ad *AnomalyDetector) DetectAnomalies(forecast *models.Forecast) []models.Anomaly {
	var anomalies []models.Anomaly
	now := time.Now()

	// Check current metrics
	metrics := []struct {
		metricType string
		value      *float64
	}{
		{"temperature_2m", forecast.Current.Temperature2m},
		{"relative_humidity_2m", forecast.Current.RelativeHumidity2m},
	}

	// For now, we use simple statistical detection
	// In a real system, these would be compared against historical data
	for _, m := range metrics {
		// Simple heuristic-based anomaly detection
		if ad.isAnomalous(m.metricType, *m.value) {
			severity := ad.calculateSeverity(*m.value)
			anomalies = append(anomalies, models.Anomaly{
				Timestamp:  now,
				MetricType: m.metricType,
				Value:      *m.value,
				ZScore:     0, // Would be calculated with historical data
				Severity:   severity,
			})
		}
	}

	return anomalies
}

// isAnomalous checks if a metric value is anomalous using heuristics
func (ad *AnomalyDetector) isAnomalous(metricType string, value float64) bool {
	switch metricType {
	case "temperature_2m":
		// Flag extreme temperatures (< -40 or > 60 Celsius)
		return value < -40 || value > 60
	case "relative_humidity_2m":
		// Flag if humidity is 0 or 100 (invalid readings)
		return value <= 0 || value >= 100
	case "precipitation":
		// Flag if precipitation is negative (impossible)
		return value < 0
	case "wind_speed_10m":
		// Flag if wind speed is > 200 km/h (hurricane force)
		return value > 200
	default:
		return false
	}
}

// calculateSeverity determines the severity of an anomaly
func (ad *AnomalyDetector) calculateSeverity(value float64) string {
	absValue := math.Abs(value)
	if absValue > 10 {
		return "high"
	} else if absValue > 5 {
		return "medium"
	}
	return "low"
}

// calculateSeverityFromZScore determines severity based on Z-score
func calculateSeverityFromZScore(zScore float64) string {
	absZScore := math.Abs(zScore)
	if absZScore > 3.0 {
		return "high"
	} else if absZScore > 2.5 {
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
	return math.Abs(zScore) > 2.0
}
