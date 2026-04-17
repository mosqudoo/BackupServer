package handler

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"

	"backupserver/db"
)

type RegisterRequest struct {
	AndroidID string `json:"android_id"`
	PublicKey string `json:"public_key"`
}

func RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !safeID.MatchString(req.AndroidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	block, _ := pem.Decode([]byte(req.PublicKey))
	if block == nil {
		http.Error(w, "Invalid public key", http.StatusBadRequest)
		return
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		http.Error(w, "Invalid public key format", http.StatusBadRequest)
		return
	}

	if err := db.RegisterDevice(req.AndroidID, req.PublicKey); err != nil {
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	os.MkdirAll(filepath.Join("/opt/backupserver/data/files", req.AndroidID), 0755)
	os.MkdirAll(filepath.Join("/opt/backupserver/data/chunks", req.AndroidID), 0755)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
