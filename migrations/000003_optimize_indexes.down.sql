-- Rollback index optimizations
-- Remove composite indexes and revert to original single-column indexes

DROP INDEX IF EXISTS idx_anomalies_severity_timestamp ON anomalies;
DROP INDEX IF EXISTS idx_anomalies_location_timestamp ON anomalies;
DROP INDEX IF EXISTS idx_metrics_location_timestamp ON metrics;
DROP INDEX IF EXISTS idx_metrics_location_type_timestamp ON metrics;
