# Preempt

Proactive alarm creation via anomaly detection from real-time metrics

## Overview

Preempt is a Go application with a **separated architecture** consisting of two independent services:

1. **Weather Collector** (`cmd/weather_collector`): Background service that:
   - Loads 7 days of historical hourly data on startup (bootstrap phase)
   - Continuously fetches current weather data every 5 minutes
   - Stores metrics in MySQL database
   - Detects anomalies and suggests alarm thresholds

2. **API Server** (`cmd/server`): HTTP REST API that:
   - Serves queries for metrics, anomalies, and alarm suggestions
   - Provides endpoint to manually fetch current weather data
   - Offers health check and system status endpoints

Both services operate on weather data from San Francisco (currently hardcoded, will support multiple locations later).

## Features

- **Separated Services**: Independent data collector and API server for better scalability
- **YAML Configuration**: Easy configuration of monitored weather fields
- **Historical Bootstrap**: Automatically loads past 7 days of hourly data on startup
- **Real-time Monitoring**: Continuous weather data collection every 5 minutes
- **Flexible API Client**: Configurable field selection for Open-Meteo API requests
- **Anomaly Detection**: Hybrid approach using Z-score statistics and heuristic rules
- **Alarm Suggestions**: Automatic threshold generation based on historical patterns
- **REST API**: Full-featured HTTP API for querying data and anomalies
- **MySQL Database**: Persistent storage of metrics, anomalies, and alarm suggestions

## Project Structure

```
.
├── cmd/
│   ├── weather_collector/
│   │   └── main.go           # Background data collection service
│   └── server/
│       └── main.go           # HTTP API server
├── internal/
│   ├── api/
│   │   └── open_meteo_client.go  # Open-Meteo API client (flexible field selection)
│   ├── config/
│   │   └── config.go         # YAML configuration management
│   ├── database/
│   │   └── db.go             # MySQL database layer
│   ├── detector/
│   │   ├── detector.go       # Anomaly detection algorithm
│   │   └── suggester.go      # Alarm suggestion engine
│   ├── ml/
│   │   ├── train.py          # Machine learning model training
│   │   └── infer.py          # ML model inference
│   ├── models/
│   │   └── models.go         # Data structures
│   └── server/
│       └── server.go         # HTTP request handlers
├── config.yaml               # Application configuration file
├── go.mod
├── go.sum
└── README.md
```

## Configuration

The application uses a YAML configuration file (`config.yaml`) to manage monitored weather fields:

```yaml
weather:
  monitored_fields:
    - temperature_2m
    - relative_humidity_2m
    - precipitation
    - wind_speed_10m
    - dew_point_2m
```

These fields are used by:
- Weather collector to determine what data to fetch and store
- API server health endpoint to report what fields are being monitored
- Database operations for storing and querying metrics

## Requirements

- Go 1.19+
- MySQL v1.6.0

## Installation

1. Clone the repository

2. Install dependencies:
```bash
go mod download
go mod tidy
```

3. Set up MySQL database:
```bash
# Start MySQL (if not running)
mysql.server start

# Create database and tables
mysql -u root -p
```

**Default database credentials:**
- Username: `myapp`
- Password: `mypassword123`
- Database: `weather_db`

4. Configure monitored fields in `config.yaml`:
```yaml
weather:
  monitored_fields:
    - temperature_2m
    - relative_humidity_2m
    - precipitation
    - wind_speed_10m
    - dew_point_2m
```

5. Build both services:
```bash
# Build weather collector
go build -o weather_collector ./cmd/weather_collector

# Build API server
go build -o api_server ./cmd/server
```

## Usage

### Running the Weather Collector

The weather collector is a background service that continuously collects weather data:

```bash
go run ./cmd/weather_collector
# or
./weather_collector
```

**What it does:**
1. **Bootstrap Phase**: Loads past 7 days of hourly historical data using Open-Meteo Archive API
2. **Monitoring Phase**: Fetches current weather every 5 minutes using Open-Meteo Forecast API
3. **Detection**: Runs anomaly detection on all collected metrics
4. **Suggestions**: Generates alarm threshold suggestions based on detected anomalies

**Expected output:**
```
2025/12/08 10:00:00 Starting weather data collector...
2025/12/08 10:00:00 Configuration loaded successfully
2025/12/08 10:00:00 Monitored fields: [temperature_2m relative_humidity_2m precipitation wind_speed_10m dew_point_2m]
2025/12/08 10:00:05 Bootstrap: Fetched 168 hours of historical data
2025/12/08 10:00:05 Bootstrap complete. Starting real-time monitoring...
2025/12/08 10:00:05 Monitoring goroutine started (fetch interval: 5 minutes)
```

### Running the API Server

The API server provides HTTP endpoints for querying weather data:

```bash
go run ./cmd/server/main.go
# or
./api_server
```

The server will start on `http://localhost:8080`.

**Expected output:**
```
2025/12/08 10:05:00 Server starting on :8080
```

### API Endpoints

#### Health Check
```bash
GET /health
```

Returns server status and list of monitored weather fields from configuration.

**Response:**
```json
{
    "status": "healthy",
    "time": "2025-12-14 07:10:55.354398 +0000 UTC"
}
```

#### Fetch Current Weather (Manual)
```bash
POST /fetch-current-weather
Content-Type: application/json

{
  "latitude": 37.7749,
  "longitude": -122.4194
}
```

Manually triggers a fetch of current weather data from Open-Meteo API for specified coordinates.

**Response:**
```json
{
    "anomalies": 0,
    "forecast": {
        "time": "2025-12-13T23:00",
        "interval": 900,
        "temperature_2m": 47.3,
        "relative_humidity_2m": 96,
        "precipitation": 0,
        "weather_code": 0,
        "wind_speed_10m": 7.2,
        "dew_point_2m": 46.2
    },
    "status": "success",
    "timestamp": "2025-12-13T23:11:12.073126-08:00"
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

## How It Works

### 1. Data Collection
The application periodically fetches data from the Open-Meteo API for San Francisco and stores:
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

Currently, the application is configured for San Francisco. To monitor a different location, modify the coordinates in `cmd/server/main.go`:

```go
forecast, err := client.GetForecast(37.7749, -122.4194) // latitude, longitude
```

## Future Enhancements

- Machine learning for improved anomaly detection
- Multi-location support
- Add support for other API metrics (will need kafka to scale)
- Frontend for visualization