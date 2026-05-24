// Package config handles all configuration aspects of Vikunja.
// It reads configuration from environment variables and config files.
package config

import (
	"os"
	"strconv"
	"time"
)

// DatabaseType represents the type of database backend
type DatabaseType string

const (
	DatabaseTypeMySQL    DatabaseType = "mysql"
	DatabaseTypePostgres DatabaseType = "postgres"
	DatabaseTypeSQLite   DatabaseType = "sqlite"
)

// Config holds all configuration values for the application
type Config struct {
	// Server configuration
	Server ServerConfig

	// Database configuration
	Database DatabaseConfig

	// JWT configuration
	JWT JWTConfig

	// Log configuration
	Log LogConfig
}

// ServerConfig holds HTTP server related configuration
type ServerConfig struct {
	Host           string
	Port           int
	FrontendURL    string
	EnableCORSAll  bool
	MaxUploadSize  int64
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Type     DatabaseType
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Path     string // used for SQLite
	MaxOpen  int
	MaxIdle  int
}

// JWTConfig holds JWT token configuration
type JWTConfig struct {
	Secret     string
	Expiration time.Duration
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level  string
	Format string // "text" or "json"
	Path   string
}

// Load reads configuration from environment variables and returns a Config struct.
// Environment variables take precedence over default values.
func Load() *Config {
	cfg := &Config{
		Server: ServerConfig{
			Host:          getEnv("VIKUNJA_SERVER_HOST", "0.0.0.0"),
			Port:          getEnvInt("VIKUNJA_SERVER_PORT", 3456),
			FrontendURL:   getEnv("VIKUNJA_SERVICE_FRONTENDURL", "http://localhost:4200"),
			EnableCORSAll: getEnvBool("VIKUNJA_SERVER_CORS_ALL", false),
			MaxUploadSize: getEnvInt64("VIKUNJA_FILES_MAXSIZE", 20*1024*1024), // 20MB default
		},
		Database: DatabaseConfig{
			Type:     DatabaseType(getEnv("VIKUNJA_DATABASE_TYPE", string(DatabaseTypeSQLite))),
			Host:     getEnv("VIKUNJA_DATABASE_HOST", "localhost"),
			Port:     getEnvInt("VIKUNJA_DATABASE_PORT", 3306),
			User:     getEnv("VIKUNJA_DATABASE_USER", "vikunja"),
			Password: getEnv("VIKUNJA_DATABASE_PASSWORD", ""),
			Database: getEnv("VIKUNJA_DATABASE_DATABASE", "vikunja"),
			Path:     getEnv("VIKUNJA_DATABASE_PATH", "./vikunja.db"),
			MaxOpen:  getEnvInt("VIKUNJA_DATABASE_MAXOPENCONNECTIONS", 100),
			MaxIdle:  getEnvInt("VIKUNJA_DATABASE_MAXIDLECONNECTIONS", 50),
		},
		JWT: JWTConfig{
			Secret:     getEnv("VIKUNJA_SERVICE_JWT_SECRET", "changeme"),
			Expiration: getEnvDuration("VIKUNJA_SERVICE_JWT_TTL", 259200*time.Second), // 3 days
		},
		Log: LogConfig{
			Level:  getEnv("VIKUNJA_LOG_LEVEL", "INFO"),
			Format: getEnv("VIKUNJA_LOG_FORMAT", "text"),
			Path:   getEnv("VIKUNJA_LOG_PATH", ""),
		},
	}
	return cfg
}

func getEnv(key, defaultValue string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvInt64(key string, defaultValue int64) int64 {
	if val, ok := os.LookupEnv(key); ok {
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if val, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return defaultValue
}
