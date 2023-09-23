package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gen2brain/beeep"
	"golang.design/x/clipboard"
)

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
	if err := beeep.Notify("Link copied to clipboard", url, "assets/icon.png"); err != nil {
		return fmt.Errorf("failed to send notification %v", err)
	}

	return nil
}
