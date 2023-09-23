package main

import (
	"fmt"
	"log"
	"os"
	"path"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sevlyar/go-daemon"
	"golang.design/x/clipboard"
)

const (
	SERVICE_NAME  = "cloudshare"
	MEDIA_DIRNAME = "Shots"
)

// To terminate the daemon use:
//
//	kill `cat [SERVICE_NAME].pid`
func main() {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalln(err)
	}

	dctx := &daemon.Context{
		PidFileName: fmt.Sprintf("%s.pid", SERVICE_NAME),
		PidFilePerm: 0644,
		LogFileName: fmt.Sprintf("%s.log", SERVICE_NAME),
		LogFilePerm: 0640,
		WorkDir:     "./",
		Umask:       027,
		Args:        []string{"[go-daemon sample]"},
	}

	d, err := dctx.Reborn()
	if err != nil {
		log.Fatal("Unable to run: ", err)
	}
	if d != nil {
		return
	}
	defer dctx.Release()

	// Init returns an error if the package is not ready for use.
	if err := clipboard.Init(); err != nil {
		log.Fatalf("Failed to initialize clipboard: %v\n", err)
	}

	mediaDir := path.Join(homeDir, MEDIA_DIRNAME)
	wg := &sync.WaitGroup{}

	// Ensure directory exists
	os.MkdirAll(mediaDir, os.ModePerm)
	if err != nil {
		log.Fatalln(err)
	}

	// Create new filesystem watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	// Start watch loop
	wg.Add(1)
	go watchLoop(watcher, wg)

	err = watcher.Add(mediaDir)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Daemon started - watching %s\n", mediaDir)

	wg.Wait()
}
