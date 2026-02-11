-- Optimize database indexes for concurrent detector queries
-- Add composite indexes to eliminate full table scans and improve query performance

-- Composite index for detector queries (location + metric_type + timestamp)
-- Optimizes: WHERE location = ? AND metric_type IN (...) AND timestamp >= ?
-- This is the primary index used by detector's GetMetrics() calls
CREATE INDEX idx_metrics_location_type_timestamp 
ON metrics(location, metric_type, timestamp);

-- Composite index for location + timestamp queries (fallback)
-- Optimizes: WHERE location = ? AND timestamp >= ?
CREATE INDEX idx_metrics_location_timestamp 
ON metrics(location, timestamp);

-- Composite index for anomalies queries
-- Optimizes API endpoint: GET /api/anomalies?location=X&since=Y
CREATE INDEX idx_anomalies_location_timestamp 
ON anomalies(location, timestamp);

-- Composite index for anomalies with severity filtering
-- Optimizes: WHERE severity = ? AND timestamp >= ?
CREATE INDEX idx_anomalies_severity_timestamp 
ON anomalies(severity, timestamp);
