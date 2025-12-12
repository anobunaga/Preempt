package models

import "time"

// Forecast represents weather forecast data from Open-Meteo API
type Forecast struct {
	Latitude         float64      `json:"latitude"`
	Longitude        float64      `json:"longitude"`
	Timezone         string       `json:"timezone"`
	CurrentUnits     CurrentUnits `json:"current_units"`
	Current          Current      `json:"current"`
	HourlyUnits      HourlyUnits  `json:"hourly_units"`
	Hourly           Hourly       `json:"hourly"`
	DailyUnits       DailyUnits   `json:"daily_units"`
	Daily            Daily        `json:"daily"`
	GenerationTimeMs float64      `json:"generation_time_ms"`
}

type CurrentUnits struct {
	Time               string `json:"time"`
	Interval           string `json:"interval"`
	Temperature2m      string `json:"temperature_2m"`
	RelativeHumidity2m string `json:"relative_humidity_2m"`
	Precipitation      string `json:"precipitation"`
	WeatherCode        string `json:"weather_code"`
	WindSpeed10m       string `json:"wind_speed_10m"`
	DewPoint2m         string `json:"dew_point_2m"`
}

type Current struct {
	Time               string   `json:"time"`
	Interval           int      `json:"interval"`
	Temperature2m      *float64 `json:"temperature_2m"`
	RelativeHumidity2m *float64 `json:"relative_humidity_2m"`
	Precipitation      float64  `json:"precipitation"`
	WeatherCode        int      `json:"weather_code"`
	WindSpeed10m       float64  `json:"wind_speed_10m"`
	DewPoint2m         float64  `json:"dew_point_2m"`
}

type HourlyUnits struct {
	Time               string `json:"time"`
	Temperature2m      string `json:"temperature_2m"`
	RelativeHumidity2m string `json:"relative_humidity_2m"`
	Precipitation      string `json:"precipitation"`
	DewPoint2m         string `json:"dew_point_2m"`
}

type Hourly struct {
	Time               []string  `json:"time"`
	Temperature2m      []float64 `json:"temperature_2m"`
	RelativeHumidity2m []float64 `json:"relative_humidity_2m"`
	Precipitation      []float64 `json:"precipitation"`
	DewPoint2m         []float64 `json:"dew_point_2m"`
}

type DailyUnits struct {
	Time             string `json:"time"`
	WeatherCode      string `json:"weather_code"`
	Temperature2mMax string `json:"temperature_2m_max"`
	Temperature2mMin string `json:"temperature_2m_min"`
	PrecipitationSum string `json:"precipitation_sum"`
	WindSpeed10mMax  string `json:"wind_speed_10m_max"`
}

type Daily struct {
	Time             []string  `json:"time"`
	WeatherCode      []int     `json:"weather_code"`
	Temperature2mMax []float64 `json:"temperature_2m_max"`
	Temperature2mMin []float64 `json:"temperature_2m_min"`
	PrecipitationSum []float64 `json:"precipitation_sum"`
	WindSpeed10mMax  []float64 `json:"wind_speed_10m_max"`
}

// Metric represents a single stored metric
type Metric struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	MetricType string    `json:"metric_type"`
	Value      float64   `json:"value"`
}

// Anomaly represents a detected anomaly
type Anomaly struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	MetricType string    `json:"metric_type"`
	Value      float64   `json:"value"`
	ZScore     float64   `json:"z_score"`
	Severity   string    `json:"severity"` // "low", "medium", "high"
}

// AlarmSuggestion represents a suggested alarm rule
type AlarmSuggestion struct {
	ID           int64     `json:"id"`
	MetricType   string    `json:"metric_type"`
	Threshold    float64   `json:"threshold"`
	Operator     string    `json:"operator"` // ">", "<", "=="
	SuggestedAt  time.Time `json:"suggested_at"`
	Confidence   float64   `json:"confidence"` // 0-1
	Description  string    `json:"description"`
	AnomalyCount int       `json:"anomaly_count"`
}
