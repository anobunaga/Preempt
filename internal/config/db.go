package config

import (
	"fmt"
	"os"
)

// Returns the database connection string
// It checks for environment variables first, then falls back to a default
func GetDatabaseDSN() string {
	user := os.Getenv("DB_USER")
	password := os.Getenv("DB_PASSWORD")
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	database := os.Getenv("DB_NAME")

	if user != "" && password != "" && host != "" && port != "" && database != "" {
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true", user, password, host, port, database)
	}

	if dsn := os.Getenv("DATABASE_DSN"); dsn != "" {
		return dsn
	}

	return "myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true"
}
