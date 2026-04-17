package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"backupserver/auth"
	"backupserver/db"
	"backupserver/handler"

	"github.com/gorilla/mux"
)

func main() {
	if err := db.Init("/opt/backupserver/backup.db"); err != nil {
		log.Fatalf("DB init failed: %v", err)
	}

	os.MkdirAll("/opt/backupserver/data/files", 0755)
	os.MkdirAll("/opt/backupserver/data/chunks", 0755)

	r := mux.NewRouter()
	api := r.PathPrefix("/api").Subrouter()
	api.Use(auth.RSAAuthMiddleware)

	api.HandleFunc("/register",        handler.RegisterDevice).Methods("POST")
	api.HandleFunc("/init",            handler.InitUser).Methods("POST")
	api.HandleFunc("/upload/chunk",    handler.UploadChunk).Methods("POST")
	api.HandleFunc("/upload/complete", handler.UploadComplete).Methods("POST")
	api.HandleFunc("/uploaded",        handler.CheckUploaded).Methods("GET")
	api.HandleFunc("/resume",          handler.GetResume).Methods("GET")

	srv := &http.Server{
		Addr:         "127.0.0.1:9090",
		Handler:      r,
		ReadTimeout:  120 * time.Second,
		WriteTimeout: 120 * time.Second,
	}

	log.Println("Backup server starting on 127.0.0.1:9090")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
