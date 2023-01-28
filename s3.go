package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func getSession(profileName string, awsRegion string) session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: profileName,
		Config: aws.Config{
			Region: aws.String(awsRegion),
		},
	}))
	return *sess
}

func getS3Uploader(sess session.Session) s3manager.Uploader {
	return *s3manager.NewUploader(&sess)
}

func uploadFileToS3(srcPath string,
	destPath string,
	bucketName string,
	uploader s3manager.Uploader,
	gzipFile bool) error {

	var body io.Reader
	var buf bytes.Buffer

	var contentEncoding string
	var contentType string

	// Open the file
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	if gzipFile {
		gz := gzip.NewWriter(&buf)
		defer gz.Close()

		_, err = io.Copy(gz, file)
		if err != nil {
			return fmt.Errorf("error compressing file: %v", err)
		}

		if err = gz.Close(); err != nil {
			return fmt.Errorf("error closing gzip writer: %v", err)
		}

		body = bytes.NewReader(buf.Bytes())
		destPath = destPath + ".gz"
		contentEncoding = "gzip"
		// addKnownExtensionTypes()
		contentType, _ = getContentType(srcPath)
		// fmt.Println(contentType)
	} else {
		body = file
	}

	// Upload the file to S3
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket:          aws.String(bucketName),
		Key:             aws.String(destPath),
		Body:            body,
		ContentType:     aws.String(contentType),
		ContentEncoding: aws.String(contentEncoding),
	})
	if err != nil {
		return fmt.Errorf("error uploading file to S3: %v", err)
	}

	fmt.Printf("Successfully uploaded %s to %s\n", srcPath, "s3://"+bucketName+"/"+destPath)

	return nil
}

func getContentType(filePath string) (string, error) {
	ext := filepath.Ext(filePath)
	contentType := mime.TypeByExtension(ext)
	if contentType != "" {
		return contentType, nil
	}

	// if can't find by extension type to get by reading file contents
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	_, err = file.Read(buffer)
	if err != nil {
		return "", err
	}

	contentType = http.DetectContentType(buffer)

	return contentType, nil
}

// func addKnownExtensionTypes() {
// 	mime.AddExtensionType(".gitignore", "text/plain")
// 	mime.AddExtensionType(".md", "text/plain")
// }
