package detector

import (
	"math"
	"preempt/internal/models"
	"time"
)

// AlarmSuggester suggests alarms based on detected anomalies
type AlarmSuggester struct {
	minAnomaliesForSuggestion int
}

// NewAlarmSuggester creates a new alarm suggester
func NewAlarmSuggester() *AlarmSuggester {
	return &AlarmSuggester{
		minAnomaliesForSuggestion: 3, // Suggest after 3 similar anomalies
	}
}

// SuggestAlarms analyzes anomalies and suggests alarms to prevent future issues
func (as *AlarmSuggester) SuggestAlarms(anomalies []models.Anomaly) []models.AlarmSuggestion {
	if len(anomalies) == 0 {
		return nil
	}

	// Group anomalies by metric type
	anomaliesByType := make(map[string][]models.Anomaly)
	for _, a := range anomalies {
		anomaliesByType[a.MetricType] = append(anomaliesByType[a.MetricType], a)
	}

	var suggestions []models.AlarmSuggestion

	for metricType, typeAnomalies := range anomaliesByType {
		if len(typeAnomalies) >= as.minAnomaliesForSuggestion {
			suggestion := as.generateSuggestion(metricType, typeAnomalies)
			if suggestion != nil {
				suggestions = append(suggestions, *suggestion)
			}
		}
	}

	return suggestions
}

// generateSuggestion creates an alarm suggestion for a metric with repeated anomalies
func (as *AlarmSuggester) generateSuggestion(metricType string, anomalies []models.Anomaly) *models.AlarmSuggestion {
	if len(anomalies) == 0 {
		return nil
	}

	// Calculate statistics
	values := make([]float64, len(anomalies))
	maxValue := math.Inf(-1)
	minValue := math.Inf(1)

	for i, a := range anomalies {
		values[i] = a.Value
		if a.Value > maxValue {
			maxValue = a.Value
		}
		if a.Value < minValue {
			minValue = a.Value
		}
	}

	mean := calculateMean(values)
	stdDev := calculateStdDev(values, mean)

	// Suggest threshold based on anomaly pattern
	var threshold float64
	var operator string
	var description string

	switch metricType {
	case "temperature_2m":
		if mean > 30 {
			// High temperatures - suggest upper threshold
			threshold = mean + (2 * stdDev)
			operator = ">"
			description = "Temperature exceeding safe operational limits"
		} else if mean < 0 {
			// Low temperatures - suggest lower threshold
			threshold = mean - (2 * stdDev)
			operator = "<"
			description = "Temperature dropping below safe operational limits"
		}

	case "relative_humidity_2m":
		if mean > 80 {
			threshold = mean + stdDev
			operator = ">"
			description = "Humidity levels becoming excessive"
		} else if mean < 20 {
			threshold = mean - stdDev
			operator = "<"
			description = "Humidity levels dropping dangerously low"
		}

	case "precipitation":
		threshold = mean + (2 * stdDev)
		operator = ">"
		description = "Precipitation exceeding normal levels"

	case "wind_speed_10m":
		threshold = mean + (2 * stdDev)
		operator = ">"
		description = "Wind speed reaching dangerous levels"

	default:
		return nil
	}

	// Calculate confidence based on consistency of anomalies
	confidence := as.calculateConfidence(values, threshold, operator)

	return &models.AlarmSuggestion{
		MetricType:   metricType,
		Threshold:    threshold,
		Operator:     operator,
		SuggestedAt:  time.Now(),
		Confidence:   confidence,
		Description:  description,
		AnomalyCount: len(anomalies),
	}
}

// calculateConfidence calculates how confident we are in the alarm threshold
func (as *AlarmSuggester) calculateConfidence(values []float64, threshold float64, operator string) float64 {
	if len(values) == 0 {
		return 0
	}

	// Count how many values would trigger the alarm
	triggeredCount := 0
	for _, v := range values {
		if operator == ">" && v > threshold {
			triggeredCount++
		} else if operator == "<" && v < threshold {
			triggeredCount++
		}
	}

	// Confidence is the ratio of triggered values (0 to 1)
	return float64(triggeredCount) / float64(len(values))
}

// calculateMean calculates the mean of values
func calculateMean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev calculates the standard deviation of values
func calculateStdDev(values []float64, mean float64) float64 {
	if len(values) <= 1 {
		return 0
	}
	variance := 0.0
	for _, v := range values {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(len(values) - 1)
	return math.Sqrt(variance)
}
