package main

import (
	"fmt"
	"os"

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

func uploadFileToS3(srcPath string, destPath string, bucketName string, uploader s3manager.Uploader) error {

	// Open the file
	file, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("error opening file: %v", err)
	}
	defer file.Close()

	// Upload the file to S3
	_, err = uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(destPath),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("error uploading file to S3: %v", err)
	}

	fmt.Printf("Successfully uploaded %s to %s\n", srcPath, "s3://"+bucketName+"/"+destPath)

	return nil
}
