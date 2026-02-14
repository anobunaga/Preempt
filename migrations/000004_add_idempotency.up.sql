-- Add unique constraints for idempotent inserts
-- This prevents duplicate entries when messages are reprocessed from WAL or Redis pending queue

-- Metrics table: (location, metric_type, timestamp) must be unique
-- Ensures same weather reading at same time for same location can't be inserted twice
ALTER TABLE metrics 
ADD UNIQUE INDEX idx_metrics_unique_entry (location, metric_type, timestamp);

-- Anomalies table: (location, metric_type, timestamp) must be unique
-- Prevents duplicate anomaly detection results
ALTER TABLE anomalies
ADD UNIQUE INDEX idx_anomalies_unique_entry (location, metric_type, timestamp);

-- Alarm suggestions: (location, metric_type, suggested_at) must be unique
-- Prevents duplicate alarm suggestions for same metric at same time
ALTER TABLE alarm_suggestions
ADD UNIQUE INDEX idx_alarm_suggestions_unique (location, metric_type, suggested_at);
