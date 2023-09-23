package main

import (
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
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
