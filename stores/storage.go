package stores

import (
	"excalidraw-complete/config"
	"excalidraw-complete/core"
	"excalidraw-complete/stores/aws"
	"excalidraw-complete/stores/filesystem"
	"excalidraw-complete/stores/memory"
	"excalidraw-complete/stores/postgres"
	"excalidraw-complete/stores/sqlite"
	"fmt"

	"github.com/sirupsen/logrus"
)

func GetStore(config *config.Config) core.DocumentStore {
	storageType := config.StorageType
	var store core.DocumentStore

	storageField := logrus.Fields{
		"storageType": storageType,
	}

	switch storageType {
	case "filesystem":
		basePath := config.Filesystem.LocalStoragePath
		storageField["basePath"] = basePath
		store = filesystem.NewDocumentStore(basePath)
	case "sqlite":
		dataSourceName := config.Sqlite.DataSourceName
		storageField["dataSourceName"] = dataSourceName
		store = sqlite.NewDocumentStore(dataSourceName)
	case "s3":
		bucketName := config.S3.BucketName
		storageField["bucketName"] = bucketName
		store = aws.NewDocumentStore(bucketName)
	case "postgres":
		pgHost := config.Postgres.Host
		pgPort := config.Postgres.Port
		pgUser := config.Postgres.User
		pgPass := config.Postgres.Password
		pgDbName := config.Postgres.DBName
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
