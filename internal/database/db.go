package database

import (
	"database/sql"
	"fmt"
	"log"
	"preempt/internal/models"
	"strings"
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
func (db *DB) StoreMetrics(forecast *models.Forecast, fields []string, isInitial bool) error {
	if isInitial {
		return db.storeHourlyMetrics(forecast, fields)
	}
	return db.storeCurrentMetrics(forecast, fields)
}

func (db *DB) storeHourlyMetrics(forecast *models.Forecast, fields []string) error {
	if len(forecast.Hourly.Time) == 0 {
		return fmt.Errorf("no hourly data in forecast")
	}

	timestamps := forecast.Hourly.Time

	fieldData := map[string][]float64{
		"temperature_2m":       forecast.Hourly.Temperature2m,
		"relative_humidity_2m": forecast.Hourly.RelativeHumidity2m,
		"precipitation":        forecast.Hourly.Precipitation,
		"wind_speed_10m":       forecast.Hourly.WindSpeed10m,
		"dew_point_2m":         forecast.Hourly.DewPoint2m,
	}

	for _, fieldName := range fields {
		values, exists := fieldData[fieldName]
		if !exists {
			log.Printf("Warning: field %s not found in hourly data", fieldName)
			continue
		}

		if len(values) == 0 {
			log.Printf("Skipping %s - no hourly data", fieldName)
			continue
		}

		if len(values) != len(timestamps) {
			log.Printf("Warning: %s has %d values but %d timestamps",
				fieldName, len(values), len(timestamps))
			continue
		}

		for i, value := range values {
			timestamp, err := time.Parse("2006-01-02T15:04", timestamps[i])
			if err != nil {
				log.Printf("Failed to parse timestamp %s: %v", timestamps[i], err)
				continue
			}

			query := `INSERT INTO metrics (timestamp, metric_type, value) VALUES (?, ?, ?)`
			_, err = db.conn.Exec(query, timestamp, fieldName, value)
			if err != nil {
				return fmt.Errorf("failed to store hourly metric %s at %s: %w",
					fieldName, timestamps[i], err)
			}
		}
	}

	return nil
}

func (db *DB) storeCurrentMetrics(forecast *models.Forecast, fields []string) error {
	now := time.Now()

	fieldData := map[string]*float64{
		"temperature_2m":       forecast.Current.Temperature2m,
		"relative_humidity_2m": forecast.Current.RelativeHumidity2m,
		"precipitation":        forecast.Current.Precipitation,
		"wind_speed_10m":       forecast.Current.WindSpeed10m,
		"dew_point_2m":         forecast.Current.DewPoint2m,
	}

	storedCount := 0
	for _, fieldName := range fields {
		value, exists := fieldData[fieldName]
		if !exists {
			log.Printf("Warning: field %s not found in current data", fieldName)
			continue
		}

		if value == nil {
			log.Printf("Skipping %s - no current data", fieldName)
			continue
		}

		query := `INSERT INTO metrics (timestamp, metric_type, value) VALUES (?, ?, ?)`
		_, err := db.conn.Exec(query, now, fieldName, *value)
		if err != nil {
			return fmt.Errorf("failed to store current metric %s: %w", fieldName, err)
		}
		storedCount++
	}

	log.Printf("✓ Stored %d current metrics", storedCount)
	return nil
}

// StoreAnomaly stores a detected anomaly
func (db *DB) StoreAnomaly(anomaly *models.Anomaly) error {
	query := `INSERT INTO anomalies (timestamp, metric_type, value, z_score, severity) VALUES (?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, anomaly.Timestamp, anomaly.MetricType, anomaly.Value, anomaly.ZScore, anomaly.Severity)
	return err
}

func (db *DB) StoreAnomalies(anomalies []models.Anomaly) error {
	if len(anomalies) == 0 {
		log.Printf("No anomalies")
		return nil // Nothing to store
	}

	// Begin transaction for batch insert
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Will be ignored if committed

	// Prepare statement
	stmt, err := tx.Prepare(`INSERT INTO anomalies (timestamp, metric_type, value, z_score, severity) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert each anomaly
	for _, anomaly := range anomalies {
		_, err = stmt.Exec(anomaly.Timestamp, anomaly.MetricType, anomaly.Value, anomaly.ZScore, anomaly.Severity)
		if err != nil {
			return fmt.Errorf("failed to insert anomaly for %s at %s: %w", anomaly.MetricType, anomaly.Timestamp, err)
		}
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("✓ Stored %d anomalies", len(anomalies))
	return nil
}

// StoreAlarmSuggestion stores an alarm suggestion
func (db *DB) StoreAlarmSuggestion(suggestion *models.AlarmSuggestion) error {
	query := `INSERT INTO alarm_suggestions (metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count) 
	          VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, suggestion.MetricType, suggestion.Threshold, suggestion.Operator, suggestion.SuggestedAt,
		suggestion.Confidence, suggestion.Description, suggestion.AnomalyCount)
	return err
}

// GetMetrics retrieves metrics for a given time range and metric types
// If metricTypes is empty or nil, returns all metric types
func (db *DB) GetMetrics(metricTypes []string, since time.Time) ([]models.Metric, error) {
	var query string
	var rows *sql.Rows
	var err error

	if len(metricTypes) == 1 {
		// Get single specific metric type
		query = `SELECT id, timestamp, metric_type, value FROM metrics WHERE metric_type = ? AND timestamp >= ? ORDER BY timestamp DESC`
		rows, err = db.conn.Query(query, metricTypes[0], since)
	} else {
		// Get multiple metric types using IN clause
		// Build placeholders: (?, ?, ?)
		placeholders := make([]string, len(metricTypes))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		query = fmt.Sprintf(
			`SELECT id, timestamp, metric_type, value FROM metrics WHERE metric_type IN (%s) AND timestamp >= ? ORDER BY timestamp DESC`,
			strings.Join(placeholders, ","),
		)

		// Build args: [type1, type2, type3, since]
		args := make([]interface{}, len(metricTypes)+1)
		for i, mt := range metricTypes {
			args[i] = mt
		}
		args[len(metricTypes)] = since

		rows, err = db.conn.Query(query, args...)
	}

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
