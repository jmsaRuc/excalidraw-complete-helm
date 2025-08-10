package stores

import (
	"excalidraw-complete/core"
	"excalidraw-complete/stores/aws"
	"excalidraw-complete/stores/filesystem"
	"excalidraw-complete/stores/memory"
	"excalidraw-complete/stores/postgres"
	"excalidraw-complete/stores/sqlite"
	"fmt"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
)

func GetStore() core.DocumentStore {
	storageType := os.Getenv("STORAGE_TYPE")
	var store core.DocumentStore

	storageField := logrus.Fields{
		"storageType": storageType,
	}

	switch storageType {
	case "filesystem":
		basePath := os.Getenv("LOCAL_STORAGE_PATH")
		storageField["basePath"] = basePath
		store = filesystem.NewDocumentStore(basePath)
	case "sqlite":
		dataSourceName := os.Getenv("DATA_SOURCE_NAME")
		storageField["dataSourceName"] = dataSourceName
		store = sqlite.NewDocumentStore(dataSourceName)
	case "s3":
		bucketName := os.Getenv("S3_BUCKET_NAME")
		storageField["bucketName"] = bucketName
		store = aws.NewDocumentStore(bucketName)
	case "postgres":
		pgHost := os.Getenv("POSTGRES_HOST")
		pgPort := getEnvAsInt("POSTGRES_PORT", 5432)
		pgUser := os.Getenv("POSTGRES_USER")
		pgPass := os.Getenv("POSTGRES_PASSWORD")
		pgDbName := os.Getenv("POSTGRES_DB")
		storageField["pgHost"] = pgHost
		storageField["pgPort"] = pgPort
		storageField["pgUser"] = pgUser
		storageField["pgDbName"] = pgDbName

		psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
			"password=%s dbname=%s sslmode=disable",
			pgHost, pgPort, pgUser, pgPass, pgDbName)

		store = postgres.NewDocumentStore(psqlInfo)
	default:
		store = memory.NewDocumentStore()
		storageField["storageType"] = "in-memory"
	}
	logrus.WithFields(storageField).Info("Use storage")
	return store
}

func getEnvAsInt(name string, defaultVal int) int {
	valueStr := os.Getenv(name)
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}

	return defaultVal
}
