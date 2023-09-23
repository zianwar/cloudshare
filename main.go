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
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/fsnotify/fsnotify"
	"github.com/gen2brain/beeep"
	"github.com/lithammer/shortuuid/v4"
	"golang.design/x/clipboard"
)

const (
	SERVICE_NAME = "cloudshare"
)

var (
	watchPath         string
	r2BucketDomain    string
	r2BucketName      string
	r2AccountId       string
	r2AccessKeyId     string
	r2AccessKeySecret string
)

func init() {
	watchPath = strings.TrimSpace(os.Getenv("CLOUDSHARE_WATCH_PATH"))
	r2BucketDomain = strings.TrimSpace(os.Getenv("R2_BUCKET_DOMAIN"))
	r2BucketName = strings.TrimSpace(os.Getenv("R2_BUCKET_NAME"))
	r2AccountId = strings.TrimSpace(os.Getenv("R2_ACCOUNT_ID"))
	r2AccessKeyId = strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_ID"))
	r2AccessKeySecret = strings.TrimSpace(os.Getenv("R2_ACCESS_KEY_SECRET"))

	if r2BucketDomain == "" || r2BucketName == "" || r2AccountId == "" ||
		r2AccessKeyId == "" || r2AccessKeySecret == "" || watchPath == "" {
		log.Fatalf("Missing required environment variables\n")
	}
}

func main() {
	wg := &sync.WaitGroup{}

	homePath, _ := os.UserHomeDir()
	workPath := path.Join(homePath, ".config", SERVICE_NAME)

	if err := os.MkdirAll(workPath, os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	if err := os.MkdirAll(watchPath, os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	// Create new filesystem watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	if err := watcher.Add(watchPath); err != nil {
		log.Fatal(err)
	}

	// Init returns an error if the package is not ready for use.
	if err := clipboard.Init(); err != nil {
		log.Fatalf("Failed to initialize clipboard: %v\n", err)
	}

	// Start watch loop
	wg.Add(1)
	go watchLoop(watcher, wg)

	log.Printf("Started watching directory '%s'\n", watchPath)

	wg.Wait()
}

func watchLoop(w *fsnotify.Watcher, wg *sync.WaitGroup) {
	defer wg.Done()

	// Start listening for events.
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if event.Op == fsnotify.Create {
				if err := uploadAndNotify(event.Name); err != nil {
					log.Fatalln(err)
				}
			}
		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			log.Println("error:", err)
		}
	}
}

func uploadAndNotify(filepath string) error {
	url, err := uploadFile(context.TODO(), filepath)
	if err != nil {
		return fmt.Errorf("failed to upload file '%s': %v", filepath, err)
	}

	log.Printf("Uploaded file '%s' to '%s'\n", filepath, url)

	clipboard.Write(clipboard.FmtText, []byte(url))
	if err := beeep.Notify("Link copied to clipboard", url, ""); err != nil {
		return fmt.Errorf("failed to send notification %v", err)
	}

	return nil
}

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
