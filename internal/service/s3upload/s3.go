package s3upload

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
)

func newSession(profileName string, awsRegion string) session.Session {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		Profile: profileName,
		Config: aws.Config{
			Region: aws.String(awsRegion),
		},
	}))
	return *sess
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
