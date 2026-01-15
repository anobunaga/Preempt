-- Initial schema migration
-- Creates tables for metrics, anomalies, and alarm suggestions

-- Metrics table
CREATE TABLE IF NOT EXISTS metrics (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    location VARCHAR(255) NOT NULL DEFAULT '',
    timestamp DATETIME(6) NOT NULL,
    metric_type VARCHAR(100) NOT NULL,
    value DOUBLE NOT NULL,
    INDEX idx_metrics_timestamp (timestamp),
    INDEX idx_metrics_type (metric_type),
    INDEX idx_metrics_location (location)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Anomalies table
CREATE TABLE IF NOT EXISTS anomalies (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    location VARCHAR(255) NOT NULL DEFAULT '',
    timestamp DATETIME(6) NOT NULL,
    metric_type VARCHAR(100) NOT NULL,
    value DOUBLE NOT NULL,
    z_score DOUBLE NOT NULL,
    severity VARCHAR(50) NOT NULL,
    INDEX idx_anomalies_timestamp (timestamp),
    INDEX idx_anomalies_type (metric_type),
    INDEX idx_anomalies_location (location)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Alarm suggestions table
CREATE TABLE IF NOT EXISTS alarm_suggestions (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    location VARCHAR(255) NOT NULL DEFAULT '',
    metric_type VARCHAR(100) NOT NULL,
    threshold DOUBLE NOT NULL,
    operator VARCHAR(10) NOT NULL,
    suggested_at DATETIME(6) NOT NULL,
    confidence DOUBLE NOT NULL,
    description TEXT NOT NULL,
    anomaly_count INT NOT NULL,
    INDEX idx_alarm_suggestions_location (location)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
