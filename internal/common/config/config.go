package config

import (
	"os"

	"gopkg.in/yaml.v2"
)

type Config struct {
	AWS struct {
		S3BucketName string `yaml:"s3BucketName"`
		ProfileName  string `yaml:"profileName"`
		Region       string `yaml:"region"`
	}
	Backup struct {
		DefaultS3StorageClass  string `yaml:"defaultS3StorageClass"`
		DefaultPrefixToPrepend string `yaml:"defaultPrefixToPrepend"`
		Folders                []struct {
			SrcPath        string `yaml:"srcPath"`
			DestPrefix     string `yaml:"destPrefix"`
			Gzip           bool   `yaml:"gzip"`
			S3StorageClass string `yaml:"s3StorageClass"`
		}
		GlobalExcludes []string `yaml:"globalExcludes"`
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
