# Preempt

Proactive alarm creation via anomaly detection from real-time metrics

## Overview

Preempt is a distributed weather monitoring system with a **microservices architecture**:

**Backend Services:**
1. **Collect** - Fetches weather data (7 days historical on startup, then every 5 minutes) and publishes to Redis
2. **Store** - Consumes Redis stream and persists to MySQL
3. **Detect** - Runs anomaly detection every 10 minutes using hybrid statistical + ML approach, generates alarm suggestions
4. **ML Trainer** - Python service that trains models and detects anomalies, communicates via Redis streams
5. **Server** - REST API for querying metrics, anomalies, and suggestions

**Frontend:**
- React dashboard with location selection, metric visualization, and anomaly display

**Data Pipeline:**
```
Open-Meteo API
      ↓
  Collect (every 5 min)
      ↓
Redis Stream (weather_metrics)
      ↓
   Store (continuous)
      ↓
    MySQL
      ↓
  Detect (every 5 min)
      ↓
      ├─────────────────┐
      │                 │
Statistical          Redis Stream (ml_input)
Z-Score                 ↓
Analysis          ML Trainer (Python, continuous)
      │                 ↓
      │           Redis Stream (ml_output)
      │                 │
      └─────────────────┘
              ↓
    Combine & Store Anomalies
              ↓
           MySQL
              ↓
      Server (REST API)
              ↓
          Frontend
```

## Features

- **Microservices Architecture** - Decoupled services via Redis streams
- **Containerized Deployment** - Docker Compose orchestration with auto-restart and health checks
- **Hybrid Anomaly Detection** - Statistical Z-score analysis + machine learning models
- **Independent ML Service** - Python ML trainer runs as separate container, communicates via Redis
- **Multi-Location Monitoring** - Track multiple cities/locations simultaneously
- **Real-time + Historical** - Bootstrap with 7 days history, then continuous 5-min updates
- **Alarm Suggestions** - Auto-generated thresholds from anomaly patterns
- **Location-based Filtering** - All data indexed and queryable by location
- **Modern Stack** - Go backend, Python ML, TypeScript + React frontend, MySQL + Redis

## Project Structure

```
cmd/
  collect/    # Data ingestion from Open-Meteo API
  store/      # Redis → MySQL persistence
  detect/     # Anomaly detection + alarm suggestions
  server/     # REST API server
frontend/
  src/        # React dashboard
internal/
  api/        # Open-Meteo client
  config/     # YAML config loader
  database/   # MySQL queries (location-aware)
  detector/   # Statistical + ML anomaly detection orchestration
  ml/         # Python ML service (train.py - runs as Docker container)
  models/     # Data structures
  server/     # HTTP handlers
```

## Configuration

Edit `config.yaml` to configure weather fields, Redis connection, and monitored locations:

```yaml
weather:
  monitored_fields: [temperature_2m, relative_humidity_2m, precipitation, wind_speed_10m, dew_point_2m]

redis:
  addr: "localhost:6379"
  stream: "weather_metrics"

locations:
  - name: "San Francisco"
    latitude: 37.7749
    longitude: -122.4194
  - name: "New York"
    latitude: 40.7128
    longitude: -74.0060
  # Add more as needed
```

Redis and database configurations are set via environment variables in `docker-compose.yml`.

## Quick Start with Docker (Recommended)

Run the entire application with one command:

```bash
docker-compose up
```

The application will:
- Start MySQL and Redis with health checks
- Initialize the database schema via `init.sql`
- Start the API server (port 8080)
- Start the collector (runs on startup + every 5 minutes via ofelia scheduler)
- Start the store consumer (processes Redis stream continuously)
- Start the ML trainer (Python service, processes ML jobs continuously)
- Start the detector (runs on startup + every 10 minutes via ofelia scheduler)
- Start the React frontend (port 3000)

**Access the application:**
- Frontend: http://localhost:3000
- API: http://localhost:8080

**View logs:**
```bash
# All services
docker compose logs -f

# Specific services
docker compose logs -f ml-trainer detector
```

**Stop everything:**
```bash
docker compose down
```

**Reset database and volumes:**
```bash
docker compose down -v
docker compose up -d
```

**Rebuild after code changes:**
```bash
docker compose build --no-cache
docker compose up -d
```
---

## Manual Setup (Development)

### Requirements
- Go 1.21+, MySQL 8.0+, Redis 6.0+, Node.js 18+, Python 3.11+

### Setup

```bash
# 1. Install and start Redis
brew install redis
brew services start redis or redis-server
redis-cli ping  # Should return PONG

# 2. Setup MySQL
mysql.server start
mysql -u root -p
use preempt; #switch to app DB

# 3. Install Go dependencies
go mod download

# 4. Install Python dependencies (for ML anomaly detection)
pip3 install -r requirements.txt

# 5. Install frontend dependencies
cd frontend && npm install && cd ..

# 6. Build services
make build

# 7. Configure
# Edit config.yaml with your locations and settings
```

### Run

Start each service in a separate terminal:

```bash
./collect   # Terminal 1
./store     # Terminal 2  
./detect    # Terminal 3 (automatically starts Python ML worker)
./server    # Terminal 4
cd frontend && npm run dev  # Terminal 5
redis-server # Terminal 6
```

**Note:** For development, you'll need to manually run `collect` and `detect` periodically, or use Docker Compose which handles scheduling automatically.

Access UI at `http://localhost:5173`

## API Reference

All data endpoints require `location` query parameter.

**GET /locations** - List available locations from config

**GET /health** - Server health check

**POST /fetch-current-weather?location={name}** - Manually trigger weather fetch
```json
Body: {"latitude": 37.7749, "longitude": -122.4194}
```

**GET /metrics?location={name}&type={metric}&hours={n}** - Query metrics
- `type`: optional, specific metric type
- `hours`: optional, default 24

**GET /anomalies?location={name}&limit={n}** - Get detected anomalies
- `limit`: optional, default 100

**GET /alarm-suggestions?location={name}&limit={n}** - Get alarm suggestions
- `limit`: optional, default 50

## Anomaly Detection

The system uses a **hybrid approach** combining two methods:

### 1. Statistical Analysis (Z-score)
- Calculates mean and standard deviation from 7 days of historical data
- Flags values > 2 standard deviations from mean
- Fast, interpretable, works well for Gaussian distributions

### 2. Machine Learning (Isolation Forest)
- Trains unsupervised model on historical patterns per metric type
- Detects complex, non-linear anomalies
- Communicates via Redis streams (`ml_input` → `ml_output`)
- Python ML trainer runs as independent Docker container
- Assigns anomaly scores and severity levels
- Models persisted in Docker volume `ml_models/`

**Communication Flow:**
```
Detect Service (Go)
    ↓
1. Fetches metrics from MySQL
2. Publishes to Redis stream: ml_input (with job_id)
    ↓
ML Trainer Container (Python - train.py)
    ↓
3. Consumes from ml_input stream (consumer group)
4. Trains Isolation Forest per metric type
5. Detects anomalies with scores
6. Publishes to Redis stream: ml_output (with job_id)
    ↓
Detect Service (Go)
    ↓
7. Polls ml_output stream (matches job_id)
8. Stores anomalies to MySQL
```

**Heuristic Rules** (applied to both):
- Temperature: < -40°C or > 60°C
- Humidity: 0% or 100%
- Precipitation: negative values
- Wind Speed: > 200 km/h

Both methods run every 10 minutes, and results are combined. After detecting 3+ anomalies of the same type, the system generates alarm threshold suggestions with confidence scores.

## Database Schema

Tables with location-based indexing:

**metrics**: `id, timestamp, location, metric_type, value`  
**anomalies**: `id, timestamp, location, metric_type, value, z_score, severity`  
**alarm_suggestions**: `id, location, metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count`

Indexes on `(location, timestamp)` and `(location, metric_type)` for efficient queries.

## Utilities

**Makefile:**
```bash
make build   # Build all services
make clean   # Remove binaries
make test    # Run tests
```

**Redis Monitoring:**
```bash
redis-cli XLEN weather_metrics              # Stream length
redis-cli XREVRANGE weather_metrics + - COUNT 10  # Recent messages
redis-cli XINFO GROUPS weather_metrics      # Consumer groups
```

**Database:** Default credentials - `myapp:mypassword123@weather_db`

## Future Enhancements

- WebSocket support for real-time frontend updates
- Enhanced ML models with LSTM/Prophet for time-series forecasting
- Multi-metric correlation analysis
- Alert notification system (email, SMS, webhooks)
- Custom detection rules per location/metric
- Authentication and multi-user support