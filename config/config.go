package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Redis configuration struct
type Redis struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// Postgres configuration struct
type Postgres struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

// S3 configuration struct
type S3 struct {
	BucketName string
}

// Filesystem configuration struct
type Filesystem struct {
	LocalStoragePath string
}

// Sqlite configuration struct
type Sqlite struct {
	DataSourceName string
}

// Config is the main configuration struct
type Config struct {
	Postgres                    Postgres
	S3                          S3
	Filesystem                  Filesystem
	Sqlite                      Sqlite
	Redis                       Redis
	StorageType                 string
	Host                        string
	Port                        string
	LogLevel                    string
	FrontendURL                 string
	WebSocketFirebaseHandlerURL string
	HAActive                    bool
	addedAllowedOrigins         []string
}

// New returns a new Config struct
func New() *Config {
	return &Config{
		Postgres: Postgres{
			Host:     getEnv("POSTGRES_HOST", ""),
			Port:     getEnvAsInt("POSTGRES_PORT", 5432),
			User:     getEnv("POSTGRES_USER", ""),
			Password: getEnv("POSTGRES_PASSWORD", ""),
			DBName:   getEnv("POSTGRES_DB", ""),
		},
		S3: S3{
			BucketName: getEnv("S3_BUCKET_NAME", ""),
		},
		Filesystem: Filesystem{
			LocalStoragePath: getEnv("LOCAL_STORAGE_PATH", "/tmp/excalidraw/"),
		},
		Sqlite: Sqlite{
			DataSourceName: getEnv("DATA_SOURCE_NAME", "test.db"),
		},
		Redis: Redis{
			Host:     getEnv("REDIS_HOST", "127.0.0.1"),
			Port:     getEnvAsInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvAsInt("REDIS_DB", 0),
		},
		StorageType:         getEnv("STORAGE_TYPE", ""),
		Host:                getEnv("HOST", "0.0.0.0"),
		Port:                getEnv("PORT", "3002"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		FrontendURL:         getEnv("VITE_FRONTEND_URL", "http://localhost:3002"),
		addedAllowedOrigins: getEnvAsStringArray("ALLOWED_ORIGINS", []string{}),
		// HA Variable:
		HAActive:                    getEnvAsBool("HA_ACTIVE", false),
		WebSocketFirebaseHandlerURL: getEnv("WEBSOCKET_FIREBASE_HANDLER_URL", getEnv("VITE_FRONTEND_URL", "http://localhost:3002")),
	}
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

func getEnvAsStringArray(name string, defaultVal []string) []string {
	valueStr := getEnv(name, "")
	if valueStr == "" {
		return defaultVal
	}
	return strings.Split(valueStr, ",")
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into boolean or return a default value
func getEnvAsBool(name string, defaultVal bool) bool {
	valueStr := getEnv(name, "")
	if valueStr == "" {
		return defaultVal
	}
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return defaultVal
}

// AllowedOrigins returns the allowed origins for CORS based on the configuration.
func (c *Config) AllowedOrigins() []string {
	var defaultAllowedOrigins []string

	wsBaseURL := strings.Split(c.WebSocketFirebaseHandlerURL, "://")[1]
	defaultAllowedOrigins = append(defaultAllowedOrigins, c.FrontendURL, c.WebSocketFirebaseHandlerURL, fmt.Sprintf("wss://%s", wsBaseURL))

	if len(c.addedAllowedOrigins) != 0 {
		return append(defaultAllowedOrigins, c.addedAllowedOrigins...)
	}
	return defaultAllowedOrigins
}
