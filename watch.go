package main

import (
	"log"
	"os"
	"path"
	"strings"

	"github.com/fsnotify/fsnotify"
)

func NewWatcher(watchPath string) *fsnotify.Watcher {
	// ensure watchPath directory exists
	if err := os.MkdirAll(watchPath, os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	if err := watcher.Add(watchPath); err != nil {
		log.Fatal(err)
	}
	return watcher
}

func watchLoop(w *fsnotify.Watcher, onCreate func(string) error) {
	// Start listening for events.
	for {
		select {
		case event, ok := <-w.Events:
			if !ok {
				return
			}

			if event.Op == fsnotify.Create {
				if strings.HasPrefix(path.Base(event.Name), ".") {
					log.Println("Ignoring hidden file:", event.Name)
					continue
				}
				if err := onCreate(event.Name); err != nil {
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
