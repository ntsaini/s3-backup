package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/bmatcuk/doublestar"
)

func main() {

	configFile := "config.yml"

	config, err := getConfig(configFile)
	if err != nil {
		fmt.Println("error reading config file: ", configFile)
		fmt.Println(err)
		os.Exit(1)
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
			fmt.Printf("error in config, source for %v folder is blank", i)
			os.Exit(1)
		}

		if strings.TrimSpace(backupFolder.Dest) == "" {
			fmt.Printf("error in config, destination for %v folder is blank", i)
			os.Exit(1)
		}

		srcToDestMap[backupFolder.Src] = destPrefix + "/" + backupFolder.Dest
		subDirs, subDirErr := getSubDirectories(backupFolder.Src, globalExcludes)
		if subDirErr != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		localDirs = append(localDirs, subDirs...)
	}

	sess := getSession(config.AWS.ProfileName, config.AWS.Region)
	uploader := getS3Uploader(sess)
	bucketName := config.AWS.S3BucketName

	var wg sync.WaitGroup

	for _, localDir := range localDirs {
		wg.Add(1)
		go func(localDir string, srcToDestMap map[string]string) {
			defer wg.Done()
			processFilesAtRootOnly(localDir, srcToDestMap, bucketName, uploader)
		}(localDir, srcToDestMap)
	}
	wg.Wait()
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

func processFilesAtRootOnly(dirPath string, srcToDestMap map[string]string, bucketName string, uploader s3manager.Uploader) {
	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			//don't process the root path but don't skip iterating on files
			if path == dirPath {
				return nil
			}
			return filepath.SkipDir
		}
		destPath := getDestinationName(path, srcToDestMap)
		uploadError := uploadFileToS3(path, destPath, bucketName, uploader)
		if uploadError != nil {
			return uploadError
			// fmt.Println("error in uploading file to s3", uploadError)
		}
		// fmt.Printf("Processed file src: %s to dest: %s \n", dirPath+d.Name(), destPath)
		return nil
	})
	if err != nil {
		fmt.Println(err)
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
