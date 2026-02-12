package config

import (
	"os"
	"testing"
)

func TestGetDatabaseDSN_FromEnvVars(t *testing.T) {
	origUser := os.Getenv("DB_USER")
	origPassword := os.Getenv("DB_PASSWORD")
	origHost := os.Getenv("DB_HOST")
	origPort := os.Getenv("DB_PORT")
	origDatabase := os.Getenv("DB_NAME")

	defer func() {
		os.Setenv("DB_USER", origUser)
		os.Setenv("DB_PASSWORD", origPassword)
		os.Setenv("DB_HOST", origHost)
		os.Setenv("DB_PORT", origPort)
		os.Setenv("DB_NAME", origDatabase)
	}()

	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")
	os.Setenv("DB_HOST", "testhost")
	os.Setenv("DB_PORT", "3307")
	os.Setenv("DB_NAME", "testdb")

	dsn := GetDatabaseDSN()
	expected := "testuser:testpass@tcp(testhost:3307)/testdb?parseTime=true"

	if dsn != expected {
		t.Errorf("GetDatabaseDSN() = %v, want %v", dsn, expected)
	}
}

func TestGetDatabaseDSN_FromDatabaseDSNEnv(t *testing.T) {
	origDSN := os.Getenv("DATABASE_DSN")
	defer os.Setenv("DATABASE_DSN", origDSN)

	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")

	testDSN := "custom:dsn@tcp(custom:3306)/customdb?parseTime=true"
	os.Setenv("DATABASE_DSN", testDSN)

	dsn := GetDatabaseDSN()

	if dsn != testDSN {
		t.Errorf("GetDatabaseDSN() = %v, want %v", dsn, testDSN)
	}
}

func TestGetDatabaseDSN_Default(t *testing.T) {
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DATABASE_DSN")

	dsn := GetDatabaseDSN()
	expected := "myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true"

	if dsn != expected {
		t.Errorf("GetDatabaseDSN() = %v, want %v", dsn, expected)
	}
}

func TestGetDatabaseDSN_PartialEnvVars(t *testing.T) {
	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
	os.Unsetenv("DB_HOST")
	os.Unsetenv("DB_PORT")
	os.Unsetenv("DB_NAME")
	os.Unsetenv("DATABASE_DSN")

	os.Setenv("DB_USER", "testuser")
	os.Setenv("DB_PASSWORD", "testpass")

	dsn := GetDatabaseDSN()
	expected := "myapp:mypassword123@tcp(localhost:3306)/preempt?parseTime=true"

	if dsn != expected {
		t.Errorf("GetDatabaseDSN() = %v, want %v", dsn, expected)
	}

	os.Unsetenv("DB_USER")
	os.Unsetenv("DB_PASSWORD")
}
