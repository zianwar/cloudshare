package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lithammer/shortuuid/v4"
)

func uploadFile(ctx context.Context, filepath string) (string, error) {
	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", r2AccountId),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(r2AccessKeyId, r2AccessKeySecret, "")),
	)
	if err != nil {
		return "", fmt.Errorf("failed to configure s3 client: %v", err)
	}

	client := s3.NewFromConfig(cfg)

	objectKey := fmt.Sprintf("%s/%s%s", "shots", shortuuid.New(), path.Ext(filepath))
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file for upload: %v", err)
	}

	uploadLargeObject(ctx, client, r2BucketName, objectKey, bytes)
	if err != nil {
		return "", fmt.Errorf("failed to upload file to R2: %v", err)
	}

	return fmt.Sprintf("%s/%s", r2BucketDomain, objectKey), nil
}

// UploadLargeObject uses an upload manager to upload data to an object in a bucket.
// The upload manager breaks large data into parts and uploads the parts concurrently.
func uploadLargeObject(ctx context.Context, client *s3.Client, bucketName string, objectKey string, largeObject []byte) error {
	largeBuffer := bytes.NewReader(largeObject)
	contentType := http.DetectContentType(largeObject)
	log.Printf("Detected content type for '%s' is '%s'", objectKey, contentType)

	// fallback to checking file ext  in case we failed to detect content-type
	if contentType == "application/octet-stream" {
		if strings.HasSuffix(objectKey, "mp4") {
			contentType = "video/mp4"
		} else if strings.HasSuffix(objectKey, "png") {
			contentType = "image/png"
		}
		log.Printf("Using fallback content type '%s' for file '%s'", contentType, objectKey)
	}

	var partMiBs int64 = 10
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})

	_, err := uploader.Upload(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(objectKey),
		Body:        largeBuffer,
		ContentType: &contentType,
	})
	if err != nil {
		log.Printf("Couldn't upload large object to %v:%v: %v\n",
			bucketName, objectKey, err)
	}

	return err
}
