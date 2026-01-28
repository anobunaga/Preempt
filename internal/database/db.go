package database

import (
	"database/sql"
	"fmt"
	"log"
	"preempt/internal/metrics"
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
			location VARCHAR(255) NOT NULL DEFAULT '',
			timestamp DATETIME(6) NOT NULL,
			metric_type VARCHAR(100) NOT NULL,
			value DOUBLE NOT NULL,
			INDEX idx_metrics_timestamp (timestamp),
			INDEX idx_metrics_type (metric_type),
			INDEX idx_metrics_location (location)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS anomalies (
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
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS alarm_suggestions (
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
func (db *DB) StoreMetrics(forecast *models.Forecast, location string, fields []string, isInitial bool) error {
	if isInitial {
		return db.storeHourlyMetrics(forecast, location, fields)
	}
	return db.storeCurrentMetrics(forecast, location, fields)
}

func (db *DB) storeHourlyMetrics(forecast *models.Forecast, location string, fields []string) error {
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

			query := `INSERT INTO metrics (location, timestamp, metric_type, value) VALUES (?, ?, ?, ?)`
			queryStart := time.Now()
			_, err = db.conn.Exec(query, location, timestamp, fieldName, value)
			metrics.RecordDBQuery("INSERT", "metrics", time.Since(queryStart), err)
			if err != nil {
				return fmt.Errorf("failed to store hourly metric %s at %s: %w",
					fieldName, timestamps[i], err)
			}
		}
	}

	return nil
}

func (db *DB) storeCurrentMetrics(forecast *models.Forecast, location string, fields []string) error {
	defer func() {
		stats := db.conn.Stats()
		metrics.UpdateDBConnectionStats(stats.OpenConnections, stats.InUse, stats.Idle)
	}()

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

		query := `INSERT INTO metrics (location, timestamp, metric_type, value) VALUES (?, ?, ?, ?)`
		queryStart := time.Now()
		_, err := db.conn.Exec(query, location, now, fieldName, *value)
		metrics.RecordDBQuery("INSERT", "metrics", time.Since(queryStart), err)
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
	queryStart := time.Now()
	defer func() {
		stats := db.conn.Stats()
		metrics.UpdateDBConnectionStats(stats.OpenConnections, stats.InUse, stats.Idle)
	}()

	query := `INSERT INTO anomalies (location, timestamp, metric_type, value, z_score, severity) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, anomaly.Location, anomaly.Timestamp, anomaly.MetricType, anomaly.Value, anomaly.ZScore, anomaly.Severity)
	metrics.RecordDBQuery("INSERT", "anomalies", time.Since(queryStart), err)
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
	stmt, err := tx.Prepare(`INSERT INTO anomalies (location, timestamp, metric_type, value, z_score, severity) VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	// Insert each anomaly
	for _, anomaly := range anomalies {
		_, err = stmt.Exec(anomaly.Location, anomaly.Timestamp, anomaly.MetricType, anomaly.Value, anomaly.ZScore, anomaly.Severity)
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
	query := `INSERT INTO alarm_suggestions (location, metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count) 
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.conn.Exec(query, suggestion.Location, suggestion.MetricType, suggestion.Threshold, suggestion.Operator, suggestion.SuggestedAt,
		suggestion.Confidence, suggestion.Description, suggestion.AnomalyCount)
	return err
}

// GetMetrics retrieves metrics for a given time range, location, and metric types
// If metricTypes is empty or nil, returns all metric types for the location
func (db *DB) GetMetrics(location string, metricTypes []string, since time.Time) ([]models.Metric, error) {
	var query string
	var rows *sql.Rows
	var err error

	if len(metricTypes) == 1 {
		// Get single specific metric type
		query = `SELECT id, location, timestamp, metric_type, value FROM metrics WHERE location = ? AND metric_type = ? AND timestamp >= ? ORDER BY timestamp DESC`
		rows, err = db.conn.Query(query, location, metricTypes[0], since)
	} else {
		// Get multiple metric types using IN clause
		// Build placeholders: (?, ?, ?)
		placeholders := make([]string, len(metricTypes))
		for i := range placeholders {
			placeholders[i] = "?"
		}

		query = fmt.Sprintf(
			`SELECT id, location, timestamp, metric_type, value FROM metrics WHERE location = ? AND metric_type IN (%s) AND timestamp >= ? ORDER BY timestamp DESC`,
			strings.Join(placeholders, ","),
		)

		// Build args: [location, type1, type2, type3, since]
		args := make([]interface{}, len(metricTypes)+2)
		args[0] = location
		for i, mt := range metricTypes {
			args[i+1] = mt
		}
		args[len(metricTypes)+1] = since

		rows, err = db.conn.Query(query, args...)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metrics []models.Metric
	for rows.Next() {
		var m models.Metric
		if err := rows.Scan(&m.ID, &m.Location, &m.Timestamp, &m.MetricType, &m.Value); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}

	return metrics, rows.Err()
}

// GetAnomalies retrieves recent anomalies for a specific location
func (db *DB) GetAnomalies(location string, limit int) ([]models.Anomaly, error) {
	query := `SELECT id, location, timestamp, metric_type, value, z_score, severity FROM anomalies WHERE location = ? ORDER BY timestamp DESC LIMIT ?`
	rows, err := db.conn.Query(query, location, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var anomalies []models.Anomaly
	for rows.Next() {
		var a models.Anomaly
		if err := rows.Scan(&a.ID, &a.Location, &a.Timestamp, &a.MetricType, &a.Value, &a.ZScore, &a.Severity); err != nil {
			return nil, err
		}
		anomalies = append(anomalies, a)
	}

	return anomalies, rows.Err()
}

// GetAlarmSuggestions retrieves alarm suggestions for a specific location
func (db *DB) GetAlarmSuggestions(location string, limit int) ([]models.AlarmSuggestion, error) {
	query := `SELECT id, location, metric_type, threshold, operator, suggested_at, confidence, description, anomaly_count FROM alarm_suggestions WHERE location = ? ORDER BY confidence DESC, suggested_at DESC LIMIT ?`
	rows, err := db.conn.Query(query, location, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var suggestions []models.AlarmSuggestion
	for rows.Next() {
		var s models.AlarmSuggestion
		if err := rows.Scan(&s.ID, &s.Location, &s.MetricType, &s.Threshold, &s.Operator, &s.SuggestedAt, &s.Confidence, &s.Description, &s.AnomalyCount); err != nil {
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

// GetMetricStats returns statistical information about a metric for a specific location
func (db *DB) GetMetricStats(location string, metricType string, since time.Time) (mean, stdDev float64, count int, err error) {
	query := `
	SELECT 
		COUNT(*) as count,
		AVG(value) as mean,
		STDDEV_POP(value) as stddev
	FROM metrics 
	WHERE location = ? AND metric_type = ? AND timestamp >= ?
	`
	row := db.conn.QueryRow(query, location, metricType, since)
	err = row.Scan(&count, &mean, &stdDev)
	return
}

// GetLocationsWithData returns a set of all locations that have data in the database
func (db *DB) GetLocationsWithData() (map[string]bool, error) {
	query := `SELECT DISTINCT location FROM metrics`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations with data: %w", err)
	}
	defer rows.Close()

	locations := make(map[string]bool)
	for rows.Next() {
		var location string
		if err := rows.Scan(&location); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations[location] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}

// Location represents a location in the database
type Location struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// InsertLocation inserts a new location into the database
func (db *DB) InsertLocation(name string, latitude, longitude float64) error {
	query := `INSERT INTO locations (name, latitude, longitude) VALUES (?, ?, ?)`
	_, err := db.conn.Exec(query, name, latitude, longitude)
	if err != nil {
		// Check if it's a duplicate key error
		if strings.Contains(err.Error(), "Duplicate entry") {
			return fmt.Errorf("duplicate location")
		}
		return fmt.Errorf("failed to insert location: %w", err)
	}
	return nil
}

// GetAllLocations retrieves all locations from the database
func (db *DB) GetAllLocations() ([]Location, error) {
	query := `SELECT id, name, latitude, longitude FROM locations ORDER BY name`
	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query locations: %w", err)
	}
	defer rows.Close()

	var locations []Location
	for rows.Next() {
		var loc Location
		if err := rows.Scan(&loc.ID, &loc.Name, &loc.Latitude, &loc.Longitude); err != nil {
			return nil, fmt.Errorf("failed to scan location: %w", err)
		}
		locations = append(locations, loc)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating locations: %w", err)
	}

	return locations, nil
}

// GetLocationByName retrieves a specific location by name
func (db *DB) GetLocationByName(name string) (*Location, error) {
	query := `SELECT id, name, latitude, longitude FROM locations WHERE name = ? LIMIT 1`
	row := db.conn.QueryRow(query, name)

	var loc Location
	if err := row.Scan(&loc.ID, &loc.Name, &loc.Latitude, &loc.Longitude); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("location not found: %s", name)
		}
		return nil, fmt.Errorf("failed to scan location: %w", err)
	}

	return &loc, nil
}
