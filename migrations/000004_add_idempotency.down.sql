-- Rollback idempotency constraints

ALTER TABLE metrics DROP INDEX idx_metrics_unique_entry;
ALTER TABLE anomalies DROP INDEX idx_anomalies_unique_entry;
ALTER TABLE alarm_suggestions DROP INDEX idx_alarm_suggestions_unique;
