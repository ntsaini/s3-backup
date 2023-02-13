package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRead(t *testing.T) {
	wd, _ := os.Getwd()

	testFile := filepath.Join(wd, "../../../config.sample.yml")

	cfg, err := Read(testFile)

	if err != nil {
		t.Fatalf("config file read err: %v", err)
	}
	c := len(cfg.Backup.Folders)
	if c != 2 {
		t.Fatalf("expect 2 backup folder got: %v", c)
	}
}
