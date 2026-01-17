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
- **Multi-Location Monitoring** - Track 1000+ cities/locations simultaneously
- **Scalable Location Management** - Database-backed location storage with CSV bulk import
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
  seed/       # Location bulk import from CSV
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
migrations/   # Database schema migrations
  000001_initial_schema.up.sql
  000002_add_locations_table.up.sql
```

## Configuration

### Application Settings (config.yaml)

Only contains **application logic**, not infrastructure config:

```yaml
weather:
  monitored_fields: [temperature_2m, relative_humidity_2m, precipitation, wind_speed_10m, dew_point_2m]
```

### Infrastructure Configuration (Environment Variables)

Database and Redis are configured via environment variables in `docker-compose.yml`:

```yaml
environment:
  - DB_HOST=mysql
  - DB_PORT=3306
  - DB_USER=myapp
  - DB_PASSWORD=mypassword123
  - DB_NAME=preempt
  - REDIS_HOST=redis
  - REDIS_PORT=6379
```

**Production deployment:** Use AWS Secrets Manager or similar for sensitive values.

## Quick Start with Docker (Recommended)

Run the entire application with one command:

```bash
docker-compose up
```

The application will:
1. Start MySQL and Redis with health checks
2. Run database migrations (creates tables)
3. **Seed 892 locations automatically** from CSV
4. Start the API server (port 8080)
5. Start the collector (runs on startup + every 5 minutes via ofelia scheduler)
6. Start the store consumer (processes Redis stream continuously)
7. Start the ML trainer (Python service, processes ML jobs continuously)
8. Start the detector (runs on startup + every 10 minutes via ofelia scheduler)
9. Start the React frontend (port 3000)

**Access the application:**
- Frontend: http://localhost:3000
- API: http://localhost:8080
- Locations endpoint: http://localhost:8080/locations (returns 892 locations)

**View logs:**
```bash
# All services
docker compose logs -f

# Specific services
docker compose logs -f seed collector detector

# Check seeding progress
docker compose logs seed
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
CREATE DATABASE preempt;

# 3. Run migrations
make migrate-up

# 4. Seed locations
make seed-locations
# Should output: "Import complete! Successfully inserted 892 locations"

# 5. Install Go dependencies
go mod download

# 6. Install Python dependencies (for ML anomaly detection)
pip3 install -r requirements.txt

# 7. Install frontend dependencies
cd frontend && npm install && cd ..

# 8. Build services
make build

# 9. Configure environment
export DB_HOST=localhost
export DB_PORT=3306
export DB_USER=myapp
export DB_PASSWORD=mypassword123
export DB_NAME=preempt
export REDIS_HOST=localhost
export REDIS_PORT=6379
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

All data endpoints support the `location` query parameter.

**GET /locations** - List all available locations from database
```json
Response: {
  "locations": [
    {"id": 1, "name": "Tokyo", "latitude": 35.6762, "longitude": 139.6503},
    {"id": 2, "name": "Delhi", "latitude": 28.7041, "longitude": 77.1025},
    ...
  ],
  "count": 892
}
```

**GET /health** - Server health check

**GET /metrics?location={name}&type={metric}&hours={n}** - Query metrics
- `location`: required, city name (e.g., "Tokyo")
- `type`: optional, specific metric type
- `hours`: optional, default 24

**GET /anomalies?location={name}&limit={n}** - Get detected anomalies
- `location`: required
- `limit`: optional, default 100

**GET /alarm-suggestions?location={name}&limit={n}** - Get alarm suggestions
- `location`: required
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
1. Fetches metrics from MySQL (per location)
2. Publishes to Redis stream: ml_input (with job_id, location)
    ↓
ML Trainer Container (Python - train.py)
    ↓
3. Consumes from ml_input stream (consumer group)
4. Trains Isolation Forest per metric type + location
5. Detects anomalies with scores
6. Publishes to Redis stream: ml_output (with job_id)
    ↓
Detect Service (Go)
    ↓
7. Polls ml_output stream (matches job_id)
8. Stores anomalies to MySQL (with location)
```

**Heuristic Rules** (applied to both):
- Temperature: < -40°C or > 60°C
- Humidity: 0% or 100%
- Precipitation: negative values
- Wind Speed: > 200 km/h

Both methods run every 10 minutes across all locations, and results are combined. After detecting 3+ anomalies of the same type at a location, the system generates alarm threshold suggestions with confidence scores.

## Database Schema

Tables with location-based indexing:

**locations**: `id, name, latitude, longitude` (unique index on name)  
**metrics**: `id, timestamp, location, metric_type, value` (index on location, timestamp)  
**anomalies**: `id, timestamp, location, metric_type, value, z_score, severity` (index on location, timestamp)  
**alarm_suggestions**: `id, location, metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count` (index on location)

All indexes optimized for location-based queries.

## Migrations

Database schema is version-controlled using migrations:

```bash
# Apply all migrations
make migrate-up

# Rollback one migration
make migrate-down

# Check migration status
docker-compose run migrate -database "mysql://myapp:mypassword123@tcp(mysql:3306)/preempt" version
```

**Migration files:**
- `000001_initial_schema.up.sql` - Creates metrics, anomalies, alarm_suggestions tables
- `000002_add_locations_table.up.sql` - Creates locations table with unique constraint

## Utilities

**Makefile:**
```bash
make build            # Build all services
make clean            # Remove binaries
make test             # Run tests
make migrate-up       # Apply database migrations
make migrate-down     # Rollback last migration
make seed-locations   # Import locations from CSV
```

**Redis Monitoring:**
```bash
redis-cli XLEN weather_metrics              # Stream length
redis-cli XREVRANGE weather_metrics + - COUNT 10  # Recent messages
redis-cli XINFO GROUPS weather_metrics      # Consumer groups
```

**Database Queries:**
```sql
-- Check location count
SELECT COUNT(*) FROM locations;

-- Top 10 locations by metrics
SELECT location, COUNT(*) as metric_count 
FROM metrics 
GROUP BY location 
ORDER BY metric_count DESC 
LIMIT 10;

-- Anomalies by location
SELECT location, COUNT(*) as anomaly_count
FROM anomalies
GROUP BY location
ORDER BY anomaly_count DESC;
```

**Default credentials:** `myapp:mypassword123@preempt`

## Deployment

### Local Development
```bash
docker-compose up
```

## Future Enhancements

- **Location API endpoints** - Add/update/delete locations via REST API
- **Batch weather API calls** - Optimize for 10,000+ locations
- **TimescaleDB migration** - For better time-series performance at scale
- **WebSocket support** - Real-time frontend updates
- **Enhanced ML models** - LSTM/Prophet for time-series forecasting
- **Multi-metric correlation** - Detect anomalies across related metrics
- **Alert notifications** - Email, SMS, webhooks
- **Custom detection rules** - Per location/metric configuration
- **Authentication** - Multi-user support with role-based access
- **Geographic clustering** - Group nearby locations for efficient API batching