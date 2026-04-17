package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"backupserver/db"
)

type CompleteRequest struct {
	AndroidID  string `json:"android_id"`
	HashKey    string `json:"hash_key"`
	ChunkTotal int    `json:"chunk_total"`
	Filename   string `json:"filename"`
}

func UploadComplete(w http.ResponseWriter, r *http.Request) {
	var req CompleteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !safeID.MatchString(req.AndroidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	if db.IsUploaded(req.AndroidID, req.HashKey) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_done"})
		return
	}

	safeHash := sanitizeHashKey(req.HashKey)
	chunkDir := filepath.Join("/opt/backupserver/data/chunks",
		req.AndroidID, safeHash)

	for i := 0; i < req.ChunkTotal; i++ {
		if _, err := os.Stat(filepath.Join(chunkDir, fmt.Sprintf("%05d", i))); err != nil {
			http.Error(w, fmt.Sprintf("Missing chunk %d", i), http.StatusBadRequest)
			return
		}
	}

	safeFilename := filepath.Base(req.Filename)

	// 确保用户目录存在
	userDir := filepath.Join("/opt/backupserver/data/files", req.AndroidID)
	os.MkdirAll(userDir, 0755)

	finalPath := filepath.Join(userDir, safeFilename)

	if _, err := os.Stat(finalPath); err == nil {
		ext := filepath.Ext(safeFilename)
		name := safeFilename[:len(safeFilename)-len(ext)]
		finalPath = filepath.Join(userDir,
			fmt.Sprintf("%s_%s%s", name, safeHash[:8], ext))
	}

	out, err := os.Create(finalPath)
	if err != nil {
		http.Error(w, "Create file failed", http.StatusInternalServerError)
		return
	}

	var totalSize int64
	for i := 0; i < req.ChunkTotal; i++ {
		data, err := os.ReadFile(filepath.Join(chunkDir, fmt.Sprintf("%05d", i)))
		if err != nil {
			out.Close()
			http.Error(w, "Read chunk failed", http.StatusInternalServerError)
			return
		}
		out.Write(data)
		totalSize += int64(len(data))
	}
	out.Close()

	db.MarkUploaded(req.AndroidID, req.HashKey, safeFilename, totalSize)
	os.RemoveAll(chunkDir)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"size":   fmt.Sprintf("%d", totalSize),
	})
}

func CheckUploaded(w http.ResponseWriter, r *http.Request) {
	androidID := r.URL.Query().Get("android_id")
	hashKey   := r.URL.Query().Get("hash_key")

	if !safeID.MatchString(androidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{
		"uploaded": db.IsUploaded(androidID, hashKey),
	})
}

func GetResume(w http.ResponseWriter, r *http.Request) {
	androidID := r.URL.Query().Get("android_id")
	hashKey   := r.URL.Query().Get("hash_key")

	if !safeID.MatchString(androidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{
		"next_chunk": db.GetNextChunk(androidID, hashKey),
	})
}
