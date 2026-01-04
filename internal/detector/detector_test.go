package detector

import (
	"math"
	"testing"
)

func TestNewAnomalyDetector(t *testing.T) {
	detector := &AnomalyDetector{
		zScoreThreshold: 2.0,
	}

	if detector.zScoreThreshold != 2.0 {
		t.Errorf("Expected zScoreThreshold to be 2.0, got %f", detector.zScoreThreshold)
	}
}

func TestCalculateZScore(t *testing.T) {
	tests := []struct {
		name   string
		value  float64
		mean   float64
		stdDev float64
		want   float64
	}{
		{
			name:   "value above mean",
			value:  100.0,
			mean:   50.0,
			stdDev: 25.0,
			want:   2.0,
		},
		{
			name:   "value below mean",
			value:  25.0,
			mean:   50.0,
			stdDev: 25.0,
			want:   -1.0,
		},
		{
			name:   "value equals mean",
			value:  50.0,
			mean:   50.0,
			stdDev: 25.0,
			want:   0.0,
		},
		{
			name:   "zero standard deviation",
			value:  50.0,
			mean:   50.0,
			stdDev: 0.0,
			want:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateZScore(tt.value, tt.mean, tt.stdDev)
			if got != tt.want {
				t.Errorf("CalculateZScore() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsOutlier(t *testing.T) {
	tests := []struct {
		name   string
		zScore float64
		want   bool
	}{
		{
			name:   "high positive outlier",
			zScore: 2.5,
			want:   true,
		},
		{
			name:   "high negative outlier",
			zScore: -2.5,
			want:   true,
		},
		{
			name:   "not an outlier - positive",
			zScore: 0.5,
			want:   false,
		},
		{
			name:   "not an outlier - negative",
			zScore: -0.5,
			want:   false,
		},
		{
			name:   "boundary case - exactly 1.0",
			zScore: 1.0,
			want:   false,
		},
		{
			name:   "boundary case - just over 1.0",
			zScore: 1.1,
			want:   true,
		},
		{
			name:   "zero",
			zScore: 0.0,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsOutlier(tt.zScore)
			if got != tt.want {
				t.Errorf("IsOutlier(%f) = %v, want %v", tt.zScore, got, tt.want)
			}
		})
	}
}

func TestCalculateSeverityFromZScore(t *testing.T) {
	tests := []struct {
		name   string
		zScore float64
		want   string
	}{
		{
			name:   "high severity - positive",
			zScore: 2.5,
			want:   "high",
		},
		{
			name:   "high severity - negative",
			zScore: -2.5,
			want:   "high",
		},
		{
			name:   "medium severity - positive",
			zScore: 1.8,
			want:   "medium",
		},
		{
			name:   "medium severity - negative",
			zScore: -1.8,
			want:   "medium",
		},
		{
			name:   "low severity",
			zScore: 1.0,
			want:   "low",
		},
		{
			name:   "boundary - exactly 2.0",
			zScore: 2.0,
			want:   "medium",
		},
		{
			name:   "boundary - exactly 1.5",
			zScore: 1.5,
			want:   "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSeverityFromZScore(tt.zScore)
			if got != tt.want {
				t.Errorf("calculateSeverityFromZScore(%f) = %v, want %v", tt.zScore, got, tt.want)
			}
		})
	}
}

func TestCalculateMean_Detector(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		want   float64
	}{
		{
			name:   "simple average",
			values: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
			want:   3.0,
		},
		{
			name:   "all same values",
			values: []float64{5.0, 5.0, 5.0},
			want:   5.0,
		},
		{
			name:   "negative values",
			values: []float64{-10.0, -5.0, 0.0, 5.0, 10.0},
			want:   0.0,
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

func TestCalculateStdDev_Detector(t *testing.T) {
	tests := []struct {
		name   string
		values []float64
		mean   float64
		want   float64
	}{
		{
			name:   "standard case",
			values: []float64{2.0, 4.0, 4.0, 4.0, 5.0, 5.0, 7.0, 9.0},
			mean:   5.0,
			want:   2.138,
		},
		{
			name:   "all same values",
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
		{
			name:   "empty slice",
			values: []float64{},
			mean:   0.0,
			want:   0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateStdDev(tt.values, tt.mean)
			if math.Abs(got-tt.want) > 0.01 { // Allow small rounding difference
				t.Errorf("calculateStdDev() = %v, want %v", got, tt.want)
			}
		})
	}
}
