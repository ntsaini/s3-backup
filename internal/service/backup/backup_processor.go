package backup

import (
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/ntsaini/s3-backup/internal/common"
	"github.com/ntsaini/s3-backup/internal/common/config"
	"github.com/ntsaini/s3-backup/internal/service/s3upload"
)

type BackupProcessor struct {
	S3Connection *common.S3Connection
	Uploader     *s3upload.S3UploadHelper
}

func (b *BackupProcessor) Process(folders config.BackupFolderConfigCollection) {
	//Process each sub directory concurrently in a go routine
	var wg sync.WaitGroup

	for _, backupSubDir := range folders {
		wg.Add(1)
		go func(backupSubDir config.BackupFolderConfig) {
			defer wg.Done()
			b.processBackupSubDir(backupSubDir)
		}(backupSubDir)
	}
	wg.Wait()
}

func (b *BackupProcessor) processBackupSubDir(backupDir config.BackupFolderConfig) {
	dirDestPrefix := getProperPrefix(b.S3Connection.DefaultPrefixToPrepend) + getProperPrefix(backupDir.DestPrefix)

	storageClass := b.storageClass(backupDir)

	s3ObjectMap, _ := b.Uploader.ListFiles(b.S3Connection.BucketName, dirDestPrefix)

	log.Printf("Processing backup sub directory %v\n", backupDir)

	err := filepath.WalkDir(backupDir.SrcPath,
		b.filepathProcessor(s3ObjectMap, dirDestPrefix, storageClass, backupDir))

	if err != nil {
		log.Println(err)
	}

	log.Printf("Finished Processing backup sub directory %v\n", backupDir)
}

func (b *BackupProcessor) filepathProcessor(s3ObjectMap map[string]*s3.Object,
	dirDestPrefix string, storageClass string, backupDir config.BackupFolderConfig) fs.WalkDirFunc {
	return func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			//don't process the root path but don't skip iterating on files
			if path == backupDir.SrcPath {
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

		if b.Uploader.FileExists(destKey, fileInfo, s3ObjectMap) {
			log.Printf("File already exists in s3 src: %v, dest: %v \n", path, destKey)
			return nil
		}
		s3url, uploadError := b.Uploader.UploadFile(path, destKey,
			b.S3Connection.BucketName,
			backupDir.Gzip,
			storageClass)

		if uploadError != nil {
			log.Printf("error in uploading file to s3, file: %v, error: %v \n", path, uploadError)
			return uploadError
		}
		log.Printf("Uploaded file src: %s to dest: %s \n", path, s3url)
		return nil
	}
}

func (b *BackupProcessor) storageClass(backupDir config.BackupFolderConfig) string {
	storageClass := backupDir.S3StorageClass
	if strings.TrimSpace(storageClass) == "" {
		storageClass = b.S3Connection.DefaultStorageClass
	}
	return storageClass
}

// Append "/" if it doesn't exist
func getProperPrefix(prefix string) string {
	properPrefix := strings.TrimSpace(prefix)
	if !strings.HasSuffix(properPrefix, "/") && properPrefix != "" {
		properPrefix = properPrefix + "/"
	}
	return properPrefix
}
