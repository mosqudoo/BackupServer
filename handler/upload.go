package handler

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"backupserver/db"
)

// 每设备存储上限：10GB
const maxStoragePerDevice int64 = 100 * 1024 * 1024 * 1024

type ChunkRequest struct {
	AndroidID  string `json:"android_id"`
	HashKey    string `json:"hash_key"`
	MimeType   string `json:"mime_type"`
	ChunkIndex int    `json:"chunk_index"`
	ChunkTotal int    `json:"chunk_total"`
	ChunkSize  int    `json:"chunk_size"`
	Data       string `json:"data"`
}

func UploadChunk(w http.ResponseWriter, r *http.Request) {
	var req ChunkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}
	if !safeID.MatchString(req.AndroidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	// 检查存储配额
	usage := db.GetDeviceUsage(req.AndroidID)
	if usage >= maxStoragePerDevice {
		http.Error(w, "Storage quota exceeded", http.StatusForbidden)
		return
	}

	if db.IsUploaded(req.AndroidID, req.HashKey) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "already_uploaded"})
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		http.Error(w, "Invalid data", http.StatusBadRequest)
		return
	}

	safeHash := sanitizeHashKey(req.HashKey)
	chunkDir := filepath.Join("/opt/backupserver/data/chunks", req.AndroidID, safeHash)
	os.MkdirAll(chunkDir, 0755)

	chunkPath := filepath.Join(chunkDir, fmt.Sprintf("%05d", req.ChunkIndex))
	if err := os.WriteFile(chunkPath, data, 0644); err != nil {
		http.Error(w, "Write failed", http.StatusInternalServerError)
		return
	}

	db.SaveChunk(req.AndroidID, req.HashKey, req.ChunkIndex, req.ChunkTotal)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
