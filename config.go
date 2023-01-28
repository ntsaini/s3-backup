package main

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	AWS struct {
		S3BucketName   string `yaml:"s3BucketName"`
		S3StorageClass string `yaml:"s3StorageClass"`
		ProfileName    string `yaml:"profileName"`
	}
	Backup struct {
		PrependHostnameToDest bool `yaml:"prependHostnameToDest"`
		Folders               []struct {
			Src  string `yaml:"src"`
			Dest string `yaml:"dest"`
		}
		GlobalExcludes []string `yaml:"globalExcludes"`
	}
}

func getConfig(configPath string) (*Config, error) {
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
