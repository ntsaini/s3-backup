package common

type S3Connection struct {
	BucketName             string
	DefaultPrefixToPrepend string
	ProfileName            string
	DefaultStorageClass    string
}
