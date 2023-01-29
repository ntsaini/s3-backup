package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bmatcuk/doublestar"
)

type BackupSubDir struct {
	Src            string
	Dest           string
	Gzip           bool
	S3StorageClass string
}

type S3Connection struct {
	Session             session.Session
	Uploader            s3manager.Uploader
	S3Svc               s3.S3
	BucketName          string
	DestPrefix          string
	ProfileName         string
	DefaultStorageClass string
}

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

	config, err := getConfig(configFile)
	if err != nil {
		log.Fatalf("error reading config file: %v. error: %v\n", configFile, err)
	}

	globalExcludes := config.Backup.GlobalExcludes
	backFolders := config.Backup.Folders

	if len(backFolders) == 0 {
		log.Fatalf("error in config, no backup folders provided")
	}

	//Divide the task into backup subdirectories
	var allBackupSubDirs []BackupSubDir

	for i, backupFolder := range backFolders {
		if strings.TrimSpace(backupFolder.Src) == "" {
			log.Fatalf("error in config, source for %v folder is blank", i)
		}

		if strings.TrimSpace(backupFolder.Dest) == "" {
			log.Fatalf("error in config, destination for %v folder is blank", i)
		}

		backupDir := BackupSubDir{
			Src:            backupFolder.Src,
			Dest:           backupFolder.Dest,
			Gzip:           backupFolder.Gzip,
			S3StorageClass: backupFolder.S3StorageClass,
		}
		backupSubDirs, subDirErr := getBackupSubDirs(backupDir, globalExcludes)
		if subDirErr != nil {
			log.Fatalf("error in getting subdirectories for %v, error: %v", backupFolder.Src, err)
		}
		allBackupSubDirs = append(allBackupSubDirs, backupSubDirs...)
	}

	//Get S3 Connection
	var destPrefix string
	if config.Backup.PrependHostnameToDest {
		hostName, hostErr := os.Hostname()
		if hostErr == nil {
			destPrefix = strings.ToLower(hostName)
		}
	}
	sess := getSession(config.AWS.ProfileName, config.AWS.Region)
	uploader := getS3Uploader(sess)
	s3Svc := getS3Svc(sess)

	s3Connection := S3Connection{
		Session:             sess,
		Uploader:            uploader,
		S3Svc:               s3Svc,
		BucketName:          config.AWS.S3BucketName,
		ProfileName:         config.AWS.ProfileName,
		DestPrefix:          destPrefix,
		DefaultStorageClass: config.AWS.DefaultS3StorageClass,
	}

	//Process each sub directory concurrently in a go routine
	var wg sync.WaitGroup

	for _, backupSubDir := range allBackupSubDirs {
		wg.Add(1)
		go func(backupSubDir BackupSubDir, s3Connection S3Connection) {
			defer wg.Done()
			processBackupSubDir(backupSubDir, s3Connection)
		}(backupSubDir, s3Connection)
	}
	wg.Wait()

	log.Println("Backup finished")
}

func getBackupSubDirs(backupRootDir BackupSubDir, excludeDirs []string) ([]BackupSubDir, error) {

	var backupSubDirs []BackupSubDir

	err := filepath.WalkDir(backupRootDir.Src, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}

		//append a filepath separator to help with directory based pattern matching
		path = path + string(filepath.Separator)

		if isExcluded(path, excludeDirs) {
			return filepath.SkipDir
		}

		destPrefix := strings.Replace(path, backupRootDir.Src, backupRootDir.Dest, 1)
		destPrefix = strings.Replace(destPrefix, "\\", "/", -1)
		backupSubDirs = append(backupSubDirs, BackupSubDir{
			Src:            path,
			Dest:           destPrefix,
			Gzip:           backupRootDir.Gzip,
			S3StorageClass: backupRootDir.S3StorageClass,
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return backupSubDirs, nil
}

func isExcluded(path string, excludeList []string) bool {

	for _, excludePattern := range excludeList {
		if matched, _ := doublestar.PathMatch(excludePattern, path); matched {
			return true
		}
	}
	return false
}

func processBackupSubDir(backupDir BackupSubDir, s3Connection S3Connection) {

	dirDestPrefix := backupDir.Dest
	if strings.TrimSpace(s3Connection.DestPrefix) != "" {
		dirDestPrefix = s3Connection.DestPrefix + "/" + dirDestPrefix
	}
	storageClass := backupDir.S3StorageClass
	if strings.TrimSpace(storageClass) == "" {
		storageClass = s3Connection.DefaultStorageClass
	}
	s3ObjectMap, _ := getFilesInS3(s3Connection.BucketName, dirDestPrefix, s3Connection.S3Svc)

	log.Printf("Processing backup sub directory %v\n", backupDir)

	err := filepath.WalkDir(backupDir.Src, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			//don't process the root path but don't skip iterating on files
			if path == backupDir.Src {
				return nil
			}
			return filepath.SkipDir
		}
		// only the key not the full s3 path
		destKey := dirDestPrefix + d.Name()
		if backupDir.Gzip {
			destKey = destKey + ".gz"
		}

		fileInfo, _ := d.Info()

		if fileExistInS3(destKey, fileInfo, s3ObjectMap) {
			log.Printf("File already exists in s3 src: %v, dest: %v \n", path, destKey)
			return nil
		}
		s3url, uploadError := uploadFileToS3(path, destKey, s3Connection.BucketName, s3Connection.Uploader, backupDir.Gzip, storageClass)
		if uploadError != nil {
			log.Printf("error in uploading file to s3, file: %v, error: %v \n", path, uploadError)
			return uploadError
		}
		log.Printf("Uploaded file src: %s to dest: %s \n", path, s3url)
		return nil
	})
	if err != nil {
		log.Println(err)
	}
	log.Printf("Finished Processing backup sub directory %v\n", backupDir)
}
