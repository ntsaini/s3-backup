package s3upload

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3UploadHelper struct {
	S3client *s3.S3
	Uploader *s3manager.Uploader
}

func NewUploader(profileName, AWSRegion string) *S3UploadHelper {
	sess := newSession(profileName, AWSRegion)
	uploader := s3manager.NewUploader(&sess)
	s3Svc := s3.New(&sess)

	return &S3UploadHelper{
		S3client: s3Svc,
		Uploader: uploader,
	}
}

// TODO: Add storage class
func (u *S3UploadHelper) UploadFile(srcPath string,
	destKey string,
	bucketName string,
	gzipFile bool,
	storageClass string) (string, error) {

	var body io.Reader
	var buf bytes.Buffer

	var contentEncoding string
	var contentType string

	// Open the file
	file, err := os.Open(srcPath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	if gzipFile {
		gz := gzip.NewWriter(&buf)
		defer gz.Close()

		_, err = io.Copy(gz, file)
		if err != nil {
			return "", fmt.Errorf("error compressing file: %v", err)
		}

		if err = gz.Close(); err != nil {
			return "", fmt.Errorf("error closing gzip writer: %v", err)
		}

		body = bytes.NewReader(buf.Bytes())
		contentEncoding = "gzip"
		contentType, _ = getContentType(srcPath)
	} else {
		body = file
	}

	// Upload the file to S3
	_, err = u.Uploader.Upload(&s3manager.UploadInput{
		Bucket:          aws.String(bucketName),
		Key:             aws.String(destKey),
		Body:            body,
		ContentType:     aws.String(contentType),
		ContentEncoding: aws.String(contentEncoding),
		StorageClass:    aws.String(storageClass),
	})
	if err != nil {
		return "", fmt.Errorf("error uploading file to S3: %v", err)
	}

	s3Url := "s3://" + bucketName + "/" + destKey

	return s3Url, nil
}
func (u *S3UploadHelper) FileExists(destKey string, fileInfo fs.FileInfo, s3ObjectMap map[string]*s3.Object) bool {
	//only compare name and timestamps
	var found bool
	var s3FileTime time.Time
	s3Object, ok := s3ObjectMap[destKey]
	if ok {
		found = true
		s3FileTime = *s3Object.LastModified
	}

	if found && fileInfo.ModTime().Before(s3FileTime) {
		return true
	}

	return false
}

// TODO: add paging for more that 1000 files
// only returns the files in the root folder
func (u *S3UploadHelper) ListFiles(bucket, prefix string) (map[string]*s3.Object, error) {
	input := &s3.ListObjectsV2Input{
		Bucket:    aws.String(bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int64(1000),
	}
	result, err := u.S3client.ListObjectsV2(input)
	if err != nil {
		return nil, err
	}
	s3ObjectMap := make(map[string]*s3.Object, len(result.Contents))
	for _, s3Object := range result.Contents {
		s3ObjectMap[*s3Object.Key] = s3Object
	}
	return s3ObjectMap, nil
}
