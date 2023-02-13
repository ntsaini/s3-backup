package main

import (
	"log"
	"os"

	"github.com/ntsaini/s3-backup/internal/common"
	"github.com/ntsaini/s3-backup/internal/common/config"
	"github.com/ntsaini/s3-backup/internal/service/backup"
	"github.com/ntsaini/s3-backup/internal/service/s3upload"
)

func main() {

	configFile := "config.yml"
	logFile := "s3-backup.log"

	file, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	defer file.Close()

	log.SetOutput(file)
	log.SetFlags(log.Ldate | log.Lmicroseconds)

	log.Println("Backup started")

	cfg, err := config.Read(configFile)
	if err != nil {
		log.Fatalf("error reading config file: %v. error: %v\n", configFile, err)
	}

	if ok := cfg.Validate(); !ok {
		return
	}

	executor := &backup.BackupProcessor{
		Uploader: s3upload.NewUploader(cfg.AWS.ProfileName, cfg.AWS.Region),
		S3Connection: &common.S3Connection{
			BucketName:             cfg.AWS.S3BucketName,
			ProfileName:            cfg.AWS.ProfileName,
			DefaultPrefixToPrepend: cfg.Backup.DefaultPrefixToPrepend,
			DefaultStorageClass:    cfg.Backup.DefaultS3StorageClass,
		},
	}

	executor.Process(cfg.AllBackupSubDirs())

	log.Println("Backup finished")
}
