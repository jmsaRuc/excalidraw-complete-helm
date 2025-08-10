package config

import (
	"os"
	"strconv"
)

type Postgres struct {
	Host     string
	Port     int
	User     string
	Password string
	DBName   string
}

type S3 struct {
	BucketName string
}

type Filesystem struct {
	LocalStoragePath string
}

type Sqlite struct {
	DataSourceName string
}

type Config struct {
	Postgres    Postgres
	S3          S3
	Filesystem  Filesystem
	Sqlite      Sqlite
	StorageType string
	Host        string
	Port        string
	LogLevel    string
	FrontendURL string
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
		StorageType: getEnv("STORAGE_TYPE", ""),
		Host:        getEnv("HOST", "0.0.0.0"),
		Port:        getEnv("PORT", "3002"),
		LogLevel:    getEnv("LOG_LEVEL", "info"),
		FrontendURL: getEnv("VITE_FRONTEND_URL", "http://localhost:3002"),
	}
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}
