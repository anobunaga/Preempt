package database

import (
	"database/sql"
	"fmt"
	"preempt/internal/models"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// DB represents the database connection
type DB struct {
	conn *sql.DB
}

// NewDB creates a new database connection and initializes the schema
// dsn format: "username:password@tcp(host:port)/dbname?parseTime=true"
// example: "user:pass@tcp(localhost:3306)/preempt?parseTime=true"
func NewDB(dsn string) (*DB, error) {
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Configure connection pool
	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &DB{conn: conn}

	// Initialize schema
	if err := db.initSchema(); err != nil {
		return nil, fmt.Errorf("failed to initialize schema: %w", err)
	}

	return db, nil
}

// initSchema creates the necessary tables
func (db *DB) initSchema() error {
	// MySQL doesn't support multiple statements in one Exec, so we need to split them
	statements := []string{
		`CREATE TABLE IF NOT EXISTS metrics (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			timestamp DATETIME(6) NOT NULL,
			metric_type VARCHAR(100) NOT NULL,
			value DOUBLE NOT NULL,
			INDEX idx_metrics_timestamp (timestamp),
			INDEX idx_metrics_type (metric_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS anomalies (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			timestamp DATETIME(6) NOT NULL,
			metric_type VARCHAR(100) NOT NULL,
			value DOUBLE NOT NULL,
			z_score DOUBLE NOT NULL,
			severity VARCHAR(50) NOT NULL,
			INDEX idx_anomalies_timestamp (timestamp),
			INDEX idx_anomalies_type (metric_type)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS alarm_suggestions (
			id BIGINT AUTO_INCREMENT PRIMARY KEY,
			metric_type VARCHAR(100) NOT NULL,
			threshold DOUBLE NOT NULL,
			operator VARCHAR(10) NOT NULL,
			suggested_at DATETIME(6) NOT NULL,
			confidence DOUBLE NOT NULL,
			description TEXT NOT NULL,
			anomaly_count INT NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, stmt := range statements {
		if _, err := db.conn.Exec(stmt); err != nil {
			return fmt.Errorf("failed to execute schema statement: %w", err)
		}
	}

	return nil
}

// StoreMetrics stores all current metrics from the forecast
func (db *DB) StoreMetrics(forecast *models.Forecast) error {
	now := time.Now()

	metrics := []struct {
		metricType string
		value      float64
	}{
		{"temperature_2m", forecast.Current.Temperature2m},
		{"dew_point_2m", forecast.Current.RelativeHumidity2m},
	}

	for _, m := range metrics {
		query := `INSERT INTO metrics (timestamp, metric_type, value) VALUES (?, ?, ?)`
		_, err := db.conn.Exec(query, now, m.metricType, m.value)
		if err != nil {
			return fmt.Errorf("failed to store metric %s: %w", m.metricType, err)
		}
	}

	return nil
}

// StoreAnomaly stores a detected anomaly
func (db *DB) StoreAnomaly(anomaly *models.Anomaly) error {
	query := `INSERT INTO anomalies (timestamp, metric_type, value, z_score, severity) VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, anomaly.Timestamp, anomaly.MetricType, anomaly.Value, anomaly.ZScore, anomaly.Severity)
	return err
}

// StoreAlarmSuggestion stores an alarm suggestion
func (db *DB) StoreAlarmSuggestion(suggestion *models.AlarmSuggestion) error {
	query := `INSERT INTO alarm_suggestions (metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, suggestion.MetricType, suggestion.Threshold, suggestion.Operator, suggestion.SuggestedAt,
		suggestion.Confidence, suggestion.Description, suggestion.AnomalyCount)
	return err
}

// GetMetrics retrieves metrics for a given time range and metric type
func (db *DB) GetMetrics(metricType string, since time.Time) ([]models.Metric, error) {
	query := `SELECT id, timestamp, metric_type, value FROM metrics WHERE metric_type = ? AND timestamp >= ? ORDER BY timestamp DESC`
	rows, err := db.conn.Query(query, metricType, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.Metric
	for rows.Next() {
		var m models.Metric
		if err := rows.Scan(&m.ID, &m.Timestamp, &m.MetricType, &m.Value); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// GetAnomalies retrieves recent anomalies
func (db *DB) GetAnomalies(limit int) ([]models.Anomaly, error) {
	query := `SELECT id, timestamp, metric_type, value, z_score, severity FROM anomalies ORDER BY timestamp DESC LIMIT ?`
	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []models.Anomaly
	for rows.Next() {
		var a models.Anomaly
		if err := rows.Scan(&a.ID, &a.Timestamp, &a.MetricType, &a.Value, &a.ZScore, &a.Severity); err != nil {
			return nil, err
		}
		anomalies = append(anomalies, a)
	}

	return anomalies, rows.Err()
}

// GetAlarmSuggestions retrieves alarm suggestions
func (db *DB) GetAlarmSuggestions(limit int) ([]models.AlarmSuggestion, error) {
	query := `SELECT id, metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count FROM alarm_suggestions ORDER BY confidence DESC, suggested_at DESC LIMIT ?`
	rows, err := db.conn.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []models.AlarmSuggestion
	for rows.Next() {
		var s models.AlarmSuggestion
		if err := rows.Scan(&s.ID, &s.MetricType, &s.Threshold, &s.Operator, &s.SuggestedAt, &s.Confidence, &s.Description, &s.AnomalyCount); err != nil {
			return nil, err
		}
		suggestions = append(suggestions, s)
	}

	return suggestions, rows.Err()
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// GetMetricStats returns statistical information about a metric
func (db *DB) GetMetricStats(metricType string, since time.Time) (mean, stdDev float64, count int, err error) {
	query := `
	SELECT 
		COUNT(*) as count,
		AVG(value) as mean,
		STDDEV_POP(value) as stddev
	FROM metrics 
	WHERE metric_type = ? AND timestamp >= ?
	`
	row := db.conn.QueryRow(query, metricType, since)
	err = row.Scan(&count, &mean, &stdDev)
	return
}
