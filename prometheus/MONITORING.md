# Preempt Monitoring Guide

Prometheus metrics and Grafana dashboards for the Preempt weather anomaly detection system.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Available Metrics](#available-metrics)
3. [Troubleshooting](#troubleshooting)
4. [Prometheus Queries](#prometheus-queries)
5. [Grafana Setup](#grafana-setup)

## Quick Start

```bash
# Start all services
docker-compose up -d

# Generate database activity (required for metrics)
docker-compose exec api /app/bin/seed

# View metrics
curl http://localhost:8080/prometheus | grep db_
curl http://localhost:8081/prometheus | grep db_  # Store service

# Access UIs
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3001 (admin/admin123)
```

**Services with metrics:**
- API (port 8080) - Read queries
- Store (port 8081) - Write queries

---

## Troubleshooting No Metrics

### Problem: "No data" in Prometheus/Grafana

This is the most common issue! Follow these debugging steps in order:

#### Step 1: Check API is Running

```bash
docker-compose ps api

# Should show:
# Name              State   Ports
# preempt-api       Up      0.0.0.0:8080->8080/tcp
```

If not running:
```bash
docker-compose logs api
# Look for errors
docker-compose up -d api
```

#### Step 2: Verify Metrics Endpoint Works

```bash
curl http://localhost:8080/prometheus

# You should see LOTS of output including:
# db_queries_total{...} 0
# db_connections_open 0
# go_goroutines 10
# process_cpu_seconds_total 1.2
```

**If you get "connection refused":**
- API isn't running: `docker-compose up -d api`
- Wrong port: check `docker-compose ps`

**If you see metrics but all db_ metrics are 0:**
- ✅ Setup is working!
- ❌ No database activity yet → Continue to Step 3

**If you don't see any db_ metrics at all:**
- Check logs: `docker-compose logs api | grep -i error`
- Verify imports in code (metrics package should be imported)

#### Step 3: Generate Database Activity

**THIS IS THE KEY STEP!** Metrics only appear when database queries happen:

```bash
# Run migrations first
docker-compose exec api /app/bin/migrate

# Check metrics again
curl http://localhost:8080/prometheus | grep db_queries_total

# Should now show:
# db_queries_total{query_type="INSERT",table="schema_migrations",status="success"} 3

# Seed database for more metrics (892 locations)
docker-compose exec api /app/bin/seed

# Check again - should show lots of activity
curl http://localhost:8080/prometheus | grep db_queries_total

# Should show:
# db_queries_total{query_type="INSERT",table="locations",status="success"} 892
```

**Generate more activity:**
```bash
# Make API requests to trigger SELECT queries
curl http://localhost:8080/locations
curl http://localhost:8080/metrics/latest?location_id=1
curl http://localhost:8080/anomalies?location_id=1

# Check metrics again
curl http://localhost:8080/prometheus | grep db_queries_total
```

#### Step 4: Check Prometheus is Scraping

```bash
# Check Prometheus logs
docker-compose logs prometheus | tail -20

# Look for errors like:
# "context deadline exceeded" = Can't reach API
# "connection refused" = API not running

# Check if Prometheus can reach API from inside Docker network
docker-compose exec prometheus wget -qO- http://api:8080/prometheus | head -20

# Should show metrics output
```

**In Prometheus UI:**
1. Go to http://localhost:9090/targets
2. Find the "api" target
3. Check **State** column:
   - ✅ **UP** (green) = Working
   - ❌ **DOWN** (red) = Click "show more" to see error
4. Check **Last Scrape** - should be recent (< 15s ago)
## Troubleshooting

**No metrics appearing?**

1. **Check services are running:**
   ```bash
   docker-compose ps api store prometheus
   ```

2. **Generate database activity (required!):**
   ```bash
   docker-compose exec api /app/bin/seed
   curl http://localhost:8080/locations
   ```

3. **Check Prometheus targets:** http://localhost:9090/targets (should show "UP")

4. **Verify endpoints:**
   ```bash
   curl http://localhost:8080/prometheus | grep db_
   curl http://localhost:8081/prometheus | grep db_
   ```

**Common Issues:**

| Issue | Solution |
|-------|----------|
| CounterVec/Histogram not visible | No queries executed yet - run seed |
| Targets show DOWN | Check `docker-compose logs prometheus` |
| "No data" in Grafana | Check time range, wait 15s for scrape |
| Store metrics missing | Rebuild: `docker-compose build store && docker-compose restart store` |Prometheus Queries (PromQL)

### Basic Queries

```promql
# Show all database metrics
{__name__=~"db_.*"}

# Total queries executed
db_queries_total

# Queries per second (last 5 minutes)
rate(db_queries_total[5m])

# Failed queries only
db_queries_total{status="error"}

# INSERT queries to locations table
db_queries_total{query_type="INSERT", table="locations"}

# All SELECT queries across all tables
db_queries_total{query_type="SELECT"}
```

### Query Performance

```promql
# Average query duration (last 5 minutes)
rate(db_query_duration_seconds_sum[5m]) 
/ 
rate(db_query_duration_seconds_count[5m])

# Average query duration per table
rate(db_query_duration_seconds_sum[5m]) 
/ 
rate(db_query_duration_seconds_count[5m])
by (table)

# 95th percentile query latency
histogram_quantile(0.95, 
  rate(db_query_duration_seconds_bucket[5m])
)

# 99th percentile query latency
histogram_quantile(0.99, 
  rate(db_query_duration_seconds_bucket[5m])
)

# Median (50th percentile) query latency
histogram_quantile(0.50, 
  rate(db_query_duration_seconds_bucket[5m])
)

# Queries slower than 100ms
sum(rate(db_query_duration_seconds_bucket{le="0.1"}[5m])) by (table, query_type)

# Percentage of queries faster than 100ms
(
  sum(rate(db_query_duration_seconds_bucket{le="0.1"}[5m]))
  /
  sum(rate(db_query_duration_seconds_count[5m]))
) * 100
```

### Connection Pool Analysis

```promql
# Connection pool utilization (%)
(db_connections_in_use / db_connections_open) * 100

## Available Metrics

### Database Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `db_queries_total` | Counter | query_type, table, status | Total queries executed |
| `db_query_duration_seconds` | Histogram | query_type, table | Query execution time |
| `db_connections_open` | Gauge | - | Total open connections |
| `db_connections_in_use` | Gauge | - | Active connections |
| `db_connections_idle` | Gauge | - | Idle connections |

### Application Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `preempt_app_info` | Gauge | Always 1 (test metric) |
| `preempt_app_start_time_seconds` | Gauge | App start timestamp |

**Important: Histogram Metrics**

Histograms create 3 time series automatically:
- `db_query_duration_seconds_bucket{le="..."}` - Observations per bucket
- `db_query_duration_seconds_sum` - Total sum of all durations
- `db_query_duration_seconds_count` - Total count of observations

Search for `db_query_duration_seconds_bucket`, not just `db_query_duration_seconds`.*Query:**
  ```promql
  sum(rate(db_queries_total[5m])) by (query_type)
  ```
- **Legend:** `{{query_type}}`
- **Unit:** queries/sec (ops/sec)
- **Y-axis min:** 0

**Step-by-step:**
1. Add new panel
2. Enter query above
3. Panel options → Title: "Database Queries Per Second"
4. Standard options → Unit: "ops/sec"
5. Legend → Mode: "List"
6. Legend → Values: "Last", "Max"
7. Apply

### Panel 2: Query Duration (p95)

**Settings:**
- **Title:** Query Duration (95th Percentile)
- **Visualization:** Time series
- **Query:**
  ```promql
  histogram_quantile(0.95, 
    rate(db_query_duration_seconds_bucket[5m])
  ) by (table, query_type)
  ```
- **Legend:** `{{table}} - {{query_type}}`
- **Unit:** seconds (s)
- **Thresholds:**
  - Green: 0 - 0.1s
  - Yellow: 0.1s - 0.5s
  - Red: > 0.5s

**Step-by-step:**
1. Add new panel
2. Enter query above
3. Panel options → Title: "Query Duration (95th Percentile)"
4. Standard options → Unit: "s"
5. Standard options → Min: 0
6. Thresholds:
   - Base: 0 (green)
   - Add: 0.1 (yellow)
   - Add: 0.5 (red)
7. Graph styles → Fill opacity: 10
8. Apply

### Panel 3: Connection Pool Usage

**Settings:**
- **Title:** Connection Pool Utilization
- **Visualization:** Gauge
- **Query:**
  ```promql
  db_connections_in_use
  ```
- **Unit:** Connections
- **Max:** Query result from `db_connections_open`
- **Thresholds:**
  - Green: 0 - 70%
  - Yellow: 70% - 90%
  - Red: > 90%

**Step-by-step:**
1. Add new panel
2. Change visualization to "Gauge"
3. Enter query: `db_connections_in_use`
4. Panel options → Title: "Connection Pool Utilization"
5. Standard options → Unit: "short"
**Example Panels:**

| Panel | Visualization | Query | Unit |
|-------|--------------|-------|------|
| Queries/sec | Time series | `sum(rate(db_queries_total[5m])) by (query_type)` | ops/sec |
| p95 Latency | Time series | `histogram_quantile(0.95, rate(db_query_duration_seconds_bucket[5m]))` | seconds |
| Pool Usage | Gauge | `db_connections_in_use` | connections |
| Error Rate | Stat | `sum(rate(db_queries_total{status="error"}[5m]))` | ops/sec |
| Total Queries | Stat | `sum(increase(db_queries_total[24h]))` | short |"type": "timeseries",
        "title": "Query Rate by Table",
        "targets": [
          {
## Grafana Setup

**Quick Setup:**
1. Go to http://localhost:3001 (admin/admin123)
2. Click "+" → "Dashboard" → "Add new panel"
3. Select "Prometheus" datasource
4. Enter query, set title/unit/visualization
5. Click "Apply" then "Save""**
3. Paste JSON
4. Click **"Load"**
5. Select **"Prometheus"** datasource
6. Click **"Import"**

---

## Production Setup

### Retention & Storage

**Current development setup:**
```yaml
# prometheus/prometheus.yml
command:
  - '--storage.tsdb.retention.time=30d'
```

**For production:**
```yaml
# docker-compose.yml - prometheus service
command:
  - '--config.file=/etc/prometheus/prometheus.yml'
  - '--storage.tsdb.path=/prometheus'
  - '--storage.tsdb.retention.time=90d'  # 90 days
  - '--storage.tsdb.retention.size=50GB'  # Max 50GB
  - '--web.enable-lifecycle'  # Allow hot reload
  - '--web.enable-admin-api'  # Enable admin API
```

**Estimate storage needs:**
- ~1-2KB per sample
- If collecting 1000 metrics at 15s interval:
  - Per hour: 1000 × 4 samples × 2KB = 8MB
  - Per day: 8MB × 24 = 192MB
  - Per 90 days: 192MB × 90 = ~17GB

### High Availability

**Multiple Prometheus instances for redundancy:**

```yaml
# docker-compose.yml
prometheus-1:
  image: prom/prometheus:latest
  volumes:
    - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
    - prometheus_data_1:/prometheus
  # ... rest of config ...

prometheus-2:
  image: prom/prometheus:latest
  volumes:
    - ./prometheus/prometheus.yml:/etc/prometheus/prometheus.yml
    - prometheus_data_2:/prometheus
  # ... same config ...
```

**Import Full Dashboard:**

**Create alert rules file:**

`prometheus/alerts.yml`:
```yaml
groups:
  - name: database_alerts
    interval: 30s
    rules:
      # Alert if query error rate > 1%
      - alert: HighDatabaseErrorRate
        expr: |
          (
            sum(rate(db_queries_total{status="error"}[5m])) 
            / 
            sum(rate(db_queries_total[5m]))
          ) > 0.01
        for: 5m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "High database error rate detected"
          description: "{{ $value | humanizePercentage }} of queries are failing"

      # Alert if p95 latency > 500ms
      - alert: SlowDatabaseQueries
        expr: |
          histogram_quantile(0.95, 
            rate(db_query_duration_seconds_bucket[5m])
          ) > 0.5
        for: 10m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "Database queries are slow"
          description: "95th percentile latency is {{ $value }}s"

      # Alert if connection pool is exhausted
      - alert: DatabaseConnectionPoolExhausted
        expr: db_connections_idle == 0
        for: 2m
        labels:
          severity: critical
          component: database
        annotations:
          summary: "Database connection pool is exhausted"
          description: "No idle connections available. In use: {{ $value }}"

      # Alert if connection pool utilization > 90%
      - alert: HighConnectionPoolUtilization
        expr: |
          (db_connections_in_use / db_connections_open) > 0.9
        for: 5m
        labels:
          severity: warning
          component: database
        annotations:
          summary: "Connection pool is highly utilized"
          description: "{{ $value | humanizePercentage }} of connections are in use"

      # Alert if database is down
      - alert: DatabaseDown
        expr: up{job="api"} == 0
        for: 1m
        labels:
          severity: critical
          component: database
        annotations:
          summary: "API service is down"
          description: "Cannot scrape metrics from API"
```

**Update Prometheus config:**

`prometheus/prometheus.yml`:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - /etc/prometheus/alerts.yml

alerting:
  alertmanagers:
    - static_configs:
        - targets: ['alertmanager:9093']

scrape_configs:
  # ... existing config ...
```

**Add Alertmanager:**

`docker-compose.yml`:
```yaml
alertmanager:
  image: prom/alertmanager:latest
  container_name: preempt-alertmanager
  ports:
    - "9093:9093"
  volumes:
    - ./prometheus/alertmanager.yml:/etc/alertmanager/alertmanager.yml
    - alertmanager_data:/alertmanager
  command:
    - '--config.file=/etc/alertmanager/alertmanager.yml'
    - '--storage.path=/alertmanager'
  networks:
    - preempt-network
  restart: unless-stopped
```

`prometheus/alertmanager.yml`:
```yaml
global:
  resolve_timeout: 5m

route:
  group_by: ['alertname', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  receiver: 'email'

receivers:
  - name: 'email'
    email_configs:
      - to: 'alerts@example.com'
        from: 'prometheus@example.com'
        smarthost: 'smtp.gmail.com:587'
        auth_username: 'your-email@gmail.com'
        auth_password: 'your-app-password'
        
  - name: 'slack'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

### Security

**Enable basic authentication:**

`prometheus/web-config.yml`:
```yaml
basic_auth_users:
  admin: $2y$10$hashed_password_here  # Use htpasswd to generate
```

Generate password hash:
```bash
htpasswd -nBC 10 admin
# Enter password when prompted
# Copy the hash to web-config.yml
```

**Update docker-compose:**
```yaml
prometheus:
  command:
    - '--config.file=/etc/prometheus/prometheus.yml'
    - '--web.config.file=/etc/prometheus/web-config.yml'
  volumes:
    - ./prometheus/web-config.yml:/etc/prometheus/web-config.yml
```

**TLS encryption:**

`prometheus/web-config.yml`:
```yaml
tls_server_config:
  cert_file: /etc/prometheus/cert.pem
  key_file: /etc/prometheus/key.pem
```

Generate self-signed cert:
```bash
openssl req -new -newkey rsa:2048 -days 365 -nodes -x509 \
  -keyout prometheus/key.pem -out prometheus/cert.pem
```

### Backup & Recovery

**Prometheus snapshots:**

```bash
# Enable admin API in Prometheus config first
# Then create snapshot
curl -X POST http://localhost:9090/api/v1/admin/tsdb/snapshot

# Response includes snapshot name
# Snapshot saved to: /prometheus/snapshots/20260127T120000Z-0123456789abcdef

# Copy to backup location
docker cp preempt-prometheus:/prometheus/snapshots /backups/prometheus/

# Backup to S3
aws s3 sync /prometheus/snapshots/ s3://my-bucket/prometheus-backups/
```

**Restore from snapshot:**

```bash
# Stop Prometheus
docker-compose stop prometheus

# Remove old data
docker volume rm preempt_prometheus_data

# Restore snapshot
docker cp /backups/prometheus/20260127T120000Z-0123456789abcdef \
  preempt-prometheus:/prometheus/

# Start Prometheus
docker-compose up -d prometheus
```

**Grafana dashboard backup:**

```bash
# Export all dashboards
for uid in $(curl -s -u admin:admin123 http://localhost:3001/api/search | jq -r '.[].uid'); do
  curl -s -u admin:admin123 http://localhost:3001/api/dashboards/uid/$uid | \
    jq '.dashboard' > "dashboard-$uid.json"
done

# Backup to git
git add dashboards/*.json
git commit -m "Backup Grafana dashboards"
git push
```

**Automated backup script:**

`backup-monitoring.sh`:
```bash
#!/bin/bash

BACKUP_DIR="/backups/monitoring/$(date +%Y%m%d)"
mkdir -p $BACKUP_DIR

# Backup Prometheus snapshot
curl -X POST http://localhost:9090/api/v1/admin/tsdb/snapshot
SNAPSHOT=$(docker exec preempt-prometheus ls -t /prometheus/snapshots | head -1)
docker cp preempt-prometheus:/prometheus/snapshots/$SNAPSHOT $BACKUP_DIR/

# Backup Grafana dashboards
mkdir -p $BACKUP_DIR/grafana
for uid in $(curl -s -u admin:admin123 http://localhost:3001/api/search | jq -r '.[].uid'); do
  curl -s -u admin:admin123 http://localhost:3001/api/dashboards/uid/$uid | \
    jq '.dashboard' > "$BACKUP_DIR/grafana/dashboard-$uid.json"
done

# Backup config files
cp -r prometheus/ $BACKUP_DIR/
cp -r grafana/ $BACKUP_DIR/
cp docker-compose.yml $BACKUP_DIR/

# Upload to S3
aws s3 sync $BACKUP_DIR s3://my-bucket/monitoring-backups/$(date +%Y%m%d)/

# Clean old backups (keep 30 days)
find /backups/monitoring -type d -mtime +30 -exec rm -rf {} +

echo "Backup completed: $BACKUP_DIR"
```

Add to cron:
```bash
# Run daily at 2 AM
0 2 * * * /path/to/backup-monitoring.sh
```

### Monitoring Best Practices

1. **Set SLOs (Service Level Objectives):**
   - 95% of queries complete in < 100ms
   - Error rate < 0.1%
   - Connection pool utilization < 80%

2. **Create dashboards for different audiences:**
   - **Executive:** High-level metrics, uptime, error rates
   - **Engineering:** Detailed performance, latency percentiles
   - **On-call:** Alert status, recent incidents

3. **Alert on symptoms, not causes:**
   - ✅ Good: "User requests are slow (p95 > 500ms)"
   - ❌ Bad: "CPU usage is high"

4. **Reduce alert fatigue:**
   - Set appropriate thresholds
   - Use `for:` duration to avoid flapping
   - Group related alerts

5. **Regular reviews:**
   - Weekly: Check dashboard accuracy
   - Monthly: Review alert thresholds
   - Quarterly: Capacity planning from trends

---

## Next Steps

Once you have database metrics working:

1. **Add HTTP metrics** - Track API endpoint performance
   - Request rate by endpoint
   - Response times (p50, p95, p99)
   - Status code distribution

2. **Add Redis metrics** - Monitor cache performance
   - Cache hit/miss rates
   - Stream message rates
   - Connection pool stats

3. **Add business metrics** - Track application KPIs
   - Anomalies detected per hour
   - Weather API quota usage
   - ML model training duration

4. **Set up custom alerts** - Get notified of issues
   - Error rate spikes
   - Performance degradation
   - Resource exhaustion

5. **Create dashboard templates** - Standardize visualizations
   - Service overview
   - Performance details
   - Error tracking

**Adding new metrics is easy!**

Edit `internal/metrics/metrics.go`:
```go
var MyNewCounter = promauto.NewCounter(
    prometheus.CounterOpts{
        Name: "my_new_metric_total",
        Help: "Description of metric",
    },
)
```

Use it anywhere:
```go
metrics.MyNewCounter.Inc()
```

It automatically appears at `/prometheus`! 🎉

---

## Resources

- **Prometheus Documentation:** https://prometheus.io/docs/
- **PromQL Tutorial:** https://prometheus.io/docs/prometheus/latest/querying/basics/
- **Grafana Docs:** https://grafana.com/docs/grafana/latest/
- **Histogram Best Practices:** https://prometheus.io/docs/practices/histograms/
- **PromQL Examples:** https://prometheus.io/docs/prometheus/latest/querying/examples/
- **Grafana Dashboard Library:** https://grafana.com/grafana/dashboards/
