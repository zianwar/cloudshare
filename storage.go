package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/lithammer/shortuuid/v4"
)

// StorageClient handles Cloudflare R2 storage operations
type StorageClient struct {
	client *s3.Client
}

func NewStorageClient() *StorageClient {
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
		log.Fatalf("failed to configure s3 client: %v", err)
	}

	return &StorageClient{client: s3.NewFromConfig(cfg)}
}

func (sc *StorageClient) UploadFile(filepath string) (string, error) {
	objectKey := fmt.Sprintf("%s/%s%s", "shots", shortuuid.New(), path.Ext(filepath))
	bytes, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("failed to read file for upload: %v", err)
	}

	uploadLargeObject(sc.client, r2BucketName, objectKey, bytes)
	if err != nil {
		return "", fmt.Errorf("failed to upload file to R2: %v", err)
	}

	return fmt.Sprintf("%s/%s", r2BucketDomain, objectKey), nil
}

func (u *StorageClient) DeleteObject(bucketName string, objectKey string) error {
	_, err := u.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(objectKey),
	})
	if err != nil {
		log.Printf("Couldn't delete object to %v:%v: %v\n",
			bucketName, objectKey, err)
	}
	log.Println("deleted", objectKey, "from", bucketName)

	return err
}

func (u *StorageClient) ListObjects(bucketName string, prefix string) ([]string, error) {
	// Get the list of items
	resp, err := u.client.ListObjectsV2(context.Background(), &s3.ListObjectsV2Input{
		Bucket: aws.String(bucketName),
		Prefix: aws.String(prefix),
	})
	if err != nil {
		log.Printf("Unable to list items in bucket %q, %v", bucketName, err)
	}
	log.Printf("Listed %d items from '%s/%s'\n", len(resp.Contents), bucketName, prefix)

	items := []struct {
		key          string
		lastModified *time.Time
	}{}
	for _, item := range resp.Contents {
		items = append(items, struct {
			key          string
			lastModified *time.Time
		}{
			key:          *item.Key,
			lastModified: item.LastModified,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].lastModified.After(*items[j].lastModified)
	})

	keys := []string{}
	for _, v := range items {
		keys = append(keys, v.key)
	}

	return keys, nil
}

// UploadLargeObject uses an upload manager to upload data to an object in a bucket.
// The upload manager breaks large data into parts and uploads the parts concurrently.
func uploadLargeObject(s3Client *s3.Client, bucketName string, objectKey string, largeObject []byte) error {
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
	uploader := manager.NewUploader(s3Client, func(u *manager.Uploader) {
		u.PartSize = partMiBs * 1024 * 1024
	})

	_, err := uploader.Upload(context.TODO(), &s3.PutObjectInput{
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
