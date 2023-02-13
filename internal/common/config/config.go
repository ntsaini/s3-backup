package config

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar"
	"gopkg.in/yaml.v2"
)

type BackupFolderConfig struct {
	SrcPath        string `yaml:"srcPath"`
	DestPrefix     string `yaml:"destPrefix"`
	Gzip           bool   `yaml:"gzip"`
	S3StorageClass string `yaml:"s3StorageClass"`
}

type BackupFolderConfigCollection []BackupFolderConfig

type Config struct {
	AWS struct {
		S3BucketName string `yaml:"s3BucketName"`
		ProfileName  string `yaml:"profileName"`
		Region       string `yaml:"region"`
	}
	Backup struct {
		DefaultS3StorageClass  string `yaml:"defaultS3StorageClass"`
		DefaultPrefixToPrepend string `yaml:"defaultPrefixToPrepend"`
		Folders                BackupFolderConfigCollection
		GlobalExcludes         []string `yaml:"globalExcludes"`
	}
}

func Read(configPath string) (*Config, error) {
	// Create config structure
	config := &Config{}

	// Open config file
	file, err := os.Open(configPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	if err := d.Decode(&config); err != nil {
		return nil, err
	}

	return config, nil
}

func (c *Config) Validate() bool {
	if len(c.Backup.Folders) == 0 {
		log.Println("error in config, no backup folders provided")
		return false
	}
	return true
}

func (c *Config) AllBackupSubDirs() (allBackupSubDirs []BackupFolderConfig) {

	for i, backupFolder := range c.Backup.Folders {
		if strings.TrimSpace(backupFolder.SrcPath) == "" {
			log.Fatalf("error in config, source for %v folder is blank", i)
		}

		if strings.TrimSpace(backupFolder.DestPrefix) == "" {
			log.Fatalf("error in config, destination for %v folder is blank", i)
		}

		backupDir := BackupFolderConfig{
			SrcPath:        backupFolder.SrcPath,
			DestPrefix:     backupFolder.DestPrefix,
			Gzip:           backupFolder.Gzip,
			S3StorageClass: backupFolder.S3StorageClass,
		}

		backupSubDirs, subDirErr := getBackupSubDirs(backupDir, c.Backup.GlobalExcludes)

		if subDirErr != nil {
			log.Fatalf("error in getting subdirectories for %v, error: %v", backupFolder.SrcPath, subDirErr)
		}
		allBackupSubDirs = append(allBackupSubDirs, backupSubDirs...)
	}

	return
}

func getBackupSubDirs(backupRootDir BackupFolderConfig, excludeDirs []string) ([]BackupFolderConfig, error) {

	var backupSubDirs []BackupFolderConfig

	err := filepath.WalkDir(backupRootDir.SrcPath, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}

		//append a filepath separator to help with directory based pattern matching
		path = path + string(filepath.Separator)

		if isExcluded(path, excludeDirs) {
			return filepath.SkipDir
		}

		destPrefix := strings.Replace(path, backupRootDir.SrcPath, backupRootDir.DestPrefix, 1)
		destPrefix = strings.Replace(destPrefix, "\\", "/", -1)
		backupSubDirs = append(backupSubDirs, BackupFolderConfig{
			SrcPath:        path,
			DestPrefix:     destPrefix,
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
