package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Database metrics
var (
	// DBQueriesTotal tracks the total number of database queries
	DBQueriesTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries executed",
		},
		[]string{"query_type", "table", "status"},
	)

	// DBQueryDuration tracks the duration of database queries
	DBQueryDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Duration of database queries in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"query_type", "table"},
	)

	// DBConnectionsOpen tracks the number of open database connections
	DBConnectionsOpen = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_open",
			Help: "Number of established connections both in use and idle",
		},
	)

	// DBConnectionsInUse tracks the number of connections currently in use
	DBConnectionsInUse = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_in_use",
			Help: "Number of connections currently in use",
		},
	)

	// DBConnectionsIdle tracks the number of idle connections
	DBConnectionsIdle = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_idle",
			Help: "Number of idle connections",
		},
	)

	// AppInfo provides static information about the application
	AppInfo = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "preempt_app_info",
			Help: "Application information (always 1)",
		},
	)

	// AppStartTime records when the application started
	AppStartTime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "preempt_app_start_time_seconds",
			Help: "Unix timestamp of when the application started",
		},
	)
)

func init() {
	// Set app info to 1 (always visible)
	AppInfo.Set(1)
	// Record app start time
	AppStartTime.SetToCurrentTime()
}

// RecordDBQuery records a database query execution
func RecordDBQuery(queryType, table string, duration time.Duration, err error) {
	status := "success"
	if err != nil {
		status = "error"
	}
	DBQueriesTotal.WithLabelValues(queryType, table, status).Inc()
	DBQueryDuration.WithLabelValues(queryType, table).Observe(duration.Seconds())
}

// UpdateDBConnectionStats updates database connection pool statistics
func UpdateDBConnectionStats(open, inUse, idle int) {
	DBConnectionsOpen.Set(float64(open))
	DBConnectionsInUse.Set(float64(inUse))
	DBConnectionsIdle.Set(float64(idle))
}
