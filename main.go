package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

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

	// Get a list of files in the local directory recursively
	var localDirs []string

	for _, backupFolder := range backFolders {
		subDirs, subDirErr := getSubDirectories(backupFolder.Src, globalExcludes)
		if subDirErr != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		localDirs = append(localDirs, subDirs...)
	}

	for _, localDir := range localDirs {
		fmt.Println(localDir)
	}

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
