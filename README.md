# Preempt

Proactive alarm creation via anomaly detection from real-time metrics

## Overview

Preempt is a Go application that:
1. **Fetches real-time weather data** from the Open-Meteo API
2. **Stores metrics** in a SQLite database
3. **Detects anomalies** using statistical methods (Z-score based)
4. **Suggests alarms** based on patterns to prevent future issues

## Features

- **Real-time Data Collection**: Automatically fetches weather forecast data every 15 minutes
- **Anomaly Detection**: Identifies unusual metric values using Z-score analysis
- **Alarm Suggestions**: Proposes preventive alarm thresholds based on detected patterns
- **REST API**: Full-featured HTTP API for querying data and anomalies
- **SQLite Database**: Persistent storage of metrics, anomalies, and alarm suggestions

## Project Structure

```
.
├── cmd/
│   └── server/
│       └── main.go           # Application entry point
├── internal/
│   ├── api/
│   │   └── client.go         # Open-Meteo API client
│   ├── database/
│   │   └── db.go             # SQLite database layer
│   ├── detector/
│   │   ├── detector.go       # Anomaly detection algorithm
│   │   └── suggester.go      # Alarm suggestion engine
│   ├── models/
│   │   └── models.go         # Data structures
│   └── server/
│       └── server.go         # HTTP server and endpoints
├── go.mod                     # Go module definition
└── README.md
```

## Requirements

- Go 1.21+
- SQLite3

## Installation

1. Clone the repository:
```bash
cd /Users/adamnobunaga/projects/Preempt
```

2. Install dependencies:
```bash
go mod download
go mod tidy
```

3. Build the application:
```bash
go build -o preempt ./cmd/server
```

## Usage

### Running the Server

```bash
go run ./cmd/server
# or
./preempt
```

The server will start on `http://localhost:8080` and begin collecting data every 15 minutes.

### API Endpoints

#### Health Check
```bash
GET /health
```
Returns server status and current time.

#### Fetch Data (Manual)
```bash
POST /fetch
```
Manually triggers a data fetch from the Open-Meteo API, stores metrics, and detects anomalies.

**Response:**
```json
{
  "status": "success",
  "anomalies": 0,
  "timestamp": "2025-12-08T10:30:00Z",
  "forecast": {
    "temperature_2m": 15.2,
    "relative_humidity_2m": 65,
    "precipitation": 0.0,
    "wind_speed_10m": 12.5
  }
}
```

#### Get Metrics
```bash
GET /metrics?type=temperature_2m&hours=24
```

Query parameters:
- `type` (required): Metric type (`temperature_2m`, `relative_humidity_2m`, `precipitation`, `wind_speed_10m`)
- `hours` (optional, default: 24): Look back period in hours

**Response:**
```json
{
  "metric_type": "temperature_2m",
  "count": 96,
  "metrics": [
    {
      "id": 1,
      "timestamp": "2025-12-08T10:00:00Z",
      "metric_type": "temperature_2m",
      "value": 15.2
    }
  ]
}
```

#### Get Anomalies
```bash
GET /anomalies?limit=100
```

Query parameters:
- `limit` (optional, default: 100): Maximum number of anomalies to return

**Response:**
```json
{
  "count": 5,
  "anomalies": [
    {
      "id": 1,
      "timestamp": "2025-12-08T09:00:00Z",
      "metric_type": "wind_speed_10m",
      "value": 205.0,
      "z_score": 3.2,
      "severity": "high"
    }
  ]
}
```

#### Get Alarm Suggestions
```bash
GET /alarm-suggestions?limit=50
```

Query parameters:
- `limit` (optional, default: 50): Maximum number of suggestions to return

**Response:**
```json
{
  "count": 2,
  "suggestions": [
    {
      "id": 1,
      "metric_type": "temperature_2m",
      "threshold": 45.5,
      "operator": ">",
      "suggested_at": "2025-12-08T10:00:00Z",
      "confidence": 0.89,
      "description": "Temperature exceeding safe operational limits",
      "anomaly_count": 3
    }
  ]
}
```

#### Get Current Forecast
```bash
GET /current
```

Returns the current weather forecast data from the Open-Meteo API.

## How It Works

### 1. Data Collection
The application periodically fetches data from the Open-Meteo API for Tokyo (latitude: 35.6895, longitude: 139.6917) and stores:
- Temperature (2m height)
- Relative humidity
- Precipitation
- Wind speed

### 2. Anomaly Detection
The detector uses Z-score analysis combined with heuristic rules:
- **Temperature**: Flags values < -40°C or > 60°C
- **Humidity**: Flags 0% or 100% (invalid readings)
- **Precipitation**: Flags negative values
- **Wind Speed**: Flags > 200 km/h

Severity is classified as:
- **High**: Extreme values
- **Medium**: Significant deviations
- **Low**: Minor deviations

### 3. Alarm Suggestions
After detecting 3+ anomalies of the same type, the engine:
- Calculates mean and standard deviation
- Proposes a threshold (typically mean ± 2×stddev)
- Assigns a confidence score based on pattern consistency
- Generates a human-readable description

## Database Schema

### metrics table
```sql
CREATE TABLE metrics (
  id INTEGER PRIMARY KEY,
  timestamp DATETIME NOT NULL,
  metric_type TEXT NOT NULL,
  value REAL NOT NULL
);
```

### anomalies table
```sql
CREATE TABLE anomalies (
  id INTEGER PRIMARY KEY,
  timestamp DATETIME NOT NULL,
  metric_type TEXT NOT NULL,
  value REAL NOT NULL,
  z_score REAL NOT NULL,
  severity TEXT NOT NULL
);
```

### alarm_suggestions table
```sql
CREATE TABLE alarm_suggestions (
  id INTEGER PRIMARY KEY,
  metric_type TEXT NOT NULL,
  threshold REAL NOT NULL,
  operator TEXT NOT NULL,
  suggested_at DATETIME NOT NULL,
  confidence REAL NOT NULL,
  description TEXT NOT NULL,
  anomaly_count INTEGER NOT NULL
);
```

## Configuration

Currently, the application is configured for Tokyo, Japan. To monitor a different location, modify the coordinates in `cmd/server/main.go`:

```go
forecast, err := client.GetForecast(35.6895, 139.6917) // latitude, longitude
```

## Future Enhancements

- [ ] Time-series analysis with ARIMA models
- [ ] Machine learning for improved anomaly detection
- [ ] Multi-location support
- [ ] Alert notifications (email, Slack, etc.)
- [ ] Web dashboard for visualization
- [ ] Configuration file support
- [ ] Predictive alerting based on forecasts
- [ ] Custom thresholds per location

## License

MIT
