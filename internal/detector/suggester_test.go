package detector

import (
	"math"
	"preempt/internal/models"
	"testing"
	"time"
)

func TestNewAlarmSuggester(t *testing.T) {
	suggester := NewAlarmSuggester()

	if suggester == nil {
		t.Fatal("NewAlarmSuggester() returned nil")
	}

	if suggester.minAnomaliesForSuggestion != 3 {
		t.Errorf("Expected minAnomaliesForSuggestion to be 3, got %d", suggester.minAnomaliesForSuggestion)
	}
}

func TestSuggestAlarms_NoAnomalies(t *testing.T) {
	suggester := NewAlarmSuggester()
	suggestions := suggester.SuggestAlarms([]models.Anomaly{}, "TestLocation")

	if suggestions != nil {
		t.Error("Expected nil suggestions for empty anomalies, got non-nil")
	}
}

func TestSuggestAlarms_InsufficientAnomalies(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      100.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      105.0,
			Timestamp:  time.Now(),
		},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 0 {
		t.Errorf("Expected 0 suggestions for 2 anomalies (need 3+), got %d", len(suggestions))
	}
}

func TestSuggestAlarms_HighTemperature(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      95.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      98.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      100.0,
			Timestamp:  time.Now(),
		},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 1 {
		t.Fatalf("Expected 1 suggestion, got %d", len(suggestions))
	}

	suggestion := suggestions[0]
	if suggestion.MetricType != "temperature_2m" {
		t.Errorf("Expected metric_type 'temperature_2m', got '%s'", suggestion.MetricType)
	}

	if suggestion.Operator != ">" {
		t.Errorf("Expected operator '>', got '%s'", suggestion.Operator)
	}

	if suggestion.AnomalyCount != 3 {
		t.Errorf("Expected anomaly_count 3, got %d", suggestion.AnomalyCount)
	}
}

func TestSuggestAlarms_LowTemperature(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      -10.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      -15.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "temperature_2m",
			Value:      -12.0,
			Timestamp:  time.Now(),
		},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 1 {
		t.Fatalf("Expected 1 suggestion, got %d", len(suggestions))
	}

	suggestion := suggestions[0]
	if suggestion.Operator != "<" {
		t.Errorf("Expected operator '<', got '%s'", suggestion.Operator)
	}
}

func TestSuggestAlarms_HighHumidity(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		{
			Location:   "TestLocation",
			MetricType: "relative_humidity_2m",
			Value:      85.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "relative_humidity_2m",
			Value:      88.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "relative_humidity_2m",
			Value:      90.0,
			Timestamp:  time.Now(),
		},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 1 {
		t.Fatalf("Expected 1 suggestion, got %d", len(suggestions))
	}

	suggestion := suggestions[0]
	if suggestion.MetricType != "relative_humidity_2m" {
		t.Errorf("Expected metric_type 'relative_humidity_2m', got '%s'", suggestion.MetricType)
	}
}

func TestSuggestAlarms_Precipitation(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		{
			Location:   "TestLocation",
			MetricType: "precipitation",
			Value:      50.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "precipitation",
			Value:      55.0,
			Timestamp:  time.Now(),
		},
		{
			Location:   "TestLocation",
			MetricType: "precipitation",
			Value:      60.0,
			Timestamp:  time.Now(),
		},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 1 {
		t.Fatalf("Expected 1 suggestion, got %d", len(suggestions))
	}

	suggestion := suggestions[0]
	if suggestion.Operator != ">" {
		t.Errorf("Expected operator '>', got '%s'", suggestion.Operator)
	}
}

func TestSuggestAlarms_MultipleMetricTypes(t *testing.T) {
	suggester := NewAlarmSuggester()

	anomalies := []models.Anomaly{
		// 3 temperature anomalies
		{MetricType: "temperature_2m", Value: 95.0, Location: "TestLocation", Timestamp: time.Now()},
		{MetricType: "temperature_2m", Value: 98.0, Location: "TestLocation", Timestamp: time.Now()},
		{MetricType: "temperature_2m", Value: 100.0, Location: "TestLocation", Timestamp: time.Now()},
		// 3 wind speed anomalies
		{MetricType: "wind_speed_10m", Value: 50.0, Location: "TestLocation", Timestamp: time.Now()},
		{MetricType: "wind_speed_10m", Value: 55.0, Location: "TestLocation", Timestamp: time.Now()},
		{MetricType: "wind_speed_10m", Value: 60.0, Location: "TestLocation", Timestamp: time.Now()},
	}

	suggestions := suggester.SuggestAlarms(anomalies, "TestLocation")

	if len(suggestions) != 2 {
		t.Errorf("Expected 2 suggestions (one per metric type), got %d", len(suggestions))
	}
}

func TestCalculateConfidence(t *testing.T) {
	suggester := NewAlarmSuggester()

	tests := []struct {
		name      string
		values    []float64
		threshold float64
		operator  string
		want      float64
	}{
		{
			name:      "all values trigger - greater than",
			values:    []float64{100.0, 105.0, 110.0},
			threshold: 90.0,
			operator:  ">",
			want:      1.0,
		},
		{
			name:      "half values trigger - greater than",
			values:    []float64{100.0, 80.0, 90.0, 110.0},
			threshold: 95.0,
			operator:  ">",
			want:      0.5,
		},
		{
			name:      "no values trigger - greater than",
			values:    []float64{50.0, 60.0, 70.0},
			threshold: 100.0,
			operator:  ">",
			want:      0.0,
		},
		{
			name:      "all values trigger - less than",
			values:    []float64{10.0, 15.0, 20.0},
			threshold: 30.0,
			operator:  "<",
			want:      1.0,
		},
		{
			name:      "empty values",
			values:    []float64{},
			threshold: 100.0,
			operator:  ">",
			want:      0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := suggester.calculateConfidence(tt.values, tt.threshold, tt.operator)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("calculateConfidence() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateMean_Suggester(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{
			name:   "simple average",
			values: []float64{10.0, 20.0, 30.0},
			want:   20.0,
		},
		{
			name:   "empty slice",
			values: []float64{},
			want:   0.0,
		},
		{
			name:   "single value",
			values: []float64{42.0},
			want:   42.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateMean(tt.values)
			if math.Abs(got-tt.want) > 0.0001 {
				t.Errorf("calculateMean() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateStdDev_Suggester(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		mean   float64
		want   float64
	}{
		{
			name:   "standard deviation",
			values: []float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0},
			mean:   5.0,
			want:   2.138,
		},
		{
			name:   "no variation",
			values: []float64{5.0, 5.0, 5.0},
			mean:   5.0,
			want:   0.0,
		},
		{
			name:   "single value",
			values: []float64{42.0},
			mean:   42.0,
			want:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateStdDev(tt.values, tt.mean)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("calculateStdDev() = %v, want %v", got, tt.want)
			}
		})
	}
}
