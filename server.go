package main

import (
	"embed"

	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

//go:embed templates/*.html
var content embed.FS

type Server struct {
	Port          string
	StorageClient *StorageClient
}

type Link struct {
	IsVideo bool
	IsImage bool
	Url     string
}

func (s Server) Start() {
	r := mux.NewRouter()
	r.HandleFunc("/", s.Index).Methods("GET")
	r.HandleFunc("/delete", s.Delete).Methods("POST")

	addr := fmt.Sprintf(":%s", s.Port)
	log.Printf("Listening %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func (s Server) Index(w http.ResponseWriter, r *http.Request) {
	keys, err := s.StorageClient.ListObjects(r2BucketName, "shots")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error %s", err.Error()), 500)
		return
	}

	renderTemplate(w, "index.html", struct {
		Links []Link
	}{
		Links: createLinks(keys),
	})
}

func (s Server) Delete(w http.ResponseWriter, r *http.Request) {
	url := r.FormValue("url")
	if strings.TrimSpace(url) == "" {
		http.Error(w, "invalid url", 400)
		return
	}

	key := strings.TrimPrefix(strings.TrimPrefix(url, r2BucketDomain), "/")
	if err := s.StorageClient.DeleteObject(r2BucketName, key); err != nil {
		http.Error(w, fmt.Sprintf("Error %s", err.Error()), 500)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	t, err := template.ParseFS(content, "templates/*.html")
	if err != nil {
		http.Error(w, fmt.Sprintf("Error %s", err.Error()), 500)
		return
	}

	err = t.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error %s", err.Error()), 500)
		return
	}
}

func createLinks(keys []string) []Link {
	links := []Link{}
	for _, key := range keys {
		links = append(links, Link{
			IsVideo: strings.HasSuffix(key, "mp4"),
			IsImage: strings.HasSuffix(key, "png"),
			Url:     fmt.Sprintf("%s/%s", r2BucketDomain, key),
		})
	}
	return links
}
