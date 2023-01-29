package main

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bmatcuk/doublestar"
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

	config, err := getConfig(configFile)
	if err != nil {
		log.Fatalf("error reading config file: %v. error: %v\n", configFile, err)
	}

	globalExcludes := config.Backup.GlobalExcludes
	backFolders := config.Backup.Folders

	var destPrefix string
	if config.Backup.PrependHostnameToDest {
		hostName, hostErr := os.Hostname()
		if hostErr == nil {
			destPrefix = strings.ToLower(hostName)
		}
	}

	// Get a list of files in the local directory recursively
	var localDirs []string
	var srcToDestMap = make(map[string]string)

	for i, backupFolder := range backFolders {
		if strings.TrimSpace(backupFolder.Src) == "" {
			log.Fatalf("error in config, source for %v folder is blank", i)
		}

		if strings.TrimSpace(backupFolder.Dest) == "" {
			log.Fatalf("error in config, destination for %v folder is blank", i)
		}

		srcToDestMap[backupFolder.Src] = destPrefix + "/" + backupFolder.Dest
		subDirs, subDirErr := getSubDirectories(backupFolder.Src, globalExcludes)
		if subDirErr != nil {
			log.Fatalf("error in getting subdirectories for %v, error: %v", backupFolder.Src, err)
		}
		localDirs = append(localDirs, subDirs...)
	}

	sess := getSession(config.AWS.ProfileName, config.AWS.Region)
	uploader := getS3Uploader(sess)
	bucketName := config.AWS.S3BucketName

	var wg sync.WaitGroup

	for _, localDir := range localDirs {
		wg.Add(1)
		go func(localDir string, srcToDestMap map[string]string, gzip bool) {
			defer wg.Done()
			processFilesAtRootOnly(localDir, srcToDestMap, bucketName, uploader, gzip)
		}(localDir, srcToDestMap, false)
	}
	wg.Wait()

	log.Println("Backup finished")
}

func getSubDirectories(rootDir string, excludeDirs []string) ([]string, error) {

	var subDirs []string

	err := filepath.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}

		//append a filepath separator to help with directory based pattern matching
		path = path + string(filepath.Separator)

		if isExcluded(path, excludeDirs) {
			return filepath.SkipDir
		}

		subDirs = append(subDirs, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return subDirs, nil
}

func isExcluded(path string, excludeList []string) bool {

	for _, excludePattern := range excludeList {
		if matched, _ := doublestar.PathMatch(excludePattern, path); matched {
			return true
		}
	}
	return false
}

func processFilesAtRootOnly(dirPath string,
	srcToDestMap map[string]string,
	bucketName string,
	uploader s3manager.Uploader,
	gzip bool) {

	destPrefix := getDestinationName(dirPath, srcToDestMap)
	s3Svc := getS3Svc(getSession("desktop_backup_user", "us-west-2"))
	s3ObjectMap, _ := getFilesInS3(bucketName, destPrefix, s3Svc)
	log.Printf("Processing local directory %v to s3 destination %v\n", dirPath, destPrefix)

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			//don't process the root path but don't skip iterating on files
			if path == dirPath {
				return nil
			}
			return filepath.SkipDir
		}
		// only the key not the full s3 path
		destKey := destPrefix + d.Name()
		if gzip {
			destKey = destKey + ".gz"
		}

		fileInfo, _ := d.Info()

		if fileExistInS3(destKey, fileInfo, s3ObjectMap) {
			log.Printf("File already exists in s3 src: %v, dest: %v \n", path, destKey)
			return nil
		}
		s3url, uploadError := uploadFileToS3(path, destKey, bucketName, uploader, gzip)
		if uploadError != nil {
			return uploadError
			// log.Println("error in uploading file to s3", uploadError)
		}
		log.Printf("Uploaded file src: %s to dest: %s \n", path, s3url)
		return nil
	})
	if err != nil {
		log.Println(err)
	}
}

func getDestinationName(path string, srcToDestMap map[string]string) string {
	var destPath string
	for k, v := range srcToDestMap {
		if strings.HasPrefix(path, k) {
			destPath = strings.Replace(path, k, v, 1)
			destPath = strings.Replace(destPath, "\\", "/", -1)
			break
		}
	}
	return destPath
}
