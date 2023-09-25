package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/gen2brain/beeep"
	"golang.design/x/clipboard"
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
	log.SetFlags(log.LstdFlags | log.Lshortfile)

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

	clipboard.Init()
}

func main() {
	storageClient := NewStorageClient()

	watcher := NewWatcher(watchPath)
	defer watcher.Close()

	// On file create handler
	onCreate := func(filepath string) error {
		url, err := storageClient.UploadFile(filepath)
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

	// Start watch loop
	go watchLoop(watcher, onCreate)

	log.Printf("Started watching directory '%s'\n", watchPath)

	// Start web server for managing uploads
	server := Server{
		Port:          "80",
		StorageClient: storageClient,
	}
	server.Start()
}
