package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
)

var safeID = regexp.MustCompile(`^[a-zA-Z0-9_-]{8,64}$`)

func sanitizeHashKey(key string) string {
	safe := ""
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '_' || c == '-' {
			safe += string(c)
		} else {
			safe += "_"
		}
		if len(safe) >= 64 {
			break
		}
	}
	return safe
}

type InitRequest struct {
	AndroidID  string `json:"android_id"`
	DeviceInfo string `json:"device_info"`
}

func InitUser(w http.ResponseWriter, r *http.Request) {
	var req InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if !safeID.MatchString(req.AndroidID) {
		http.Error(w, "Invalid android_id", http.StatusBadRequest)
		return
	}

	os.MkdirAll(filepath.Join("/opt/backupserver/data/files", req.AndroidID), 0755)
	os.MkdirAll(filepath.Join("/opt/backupserver/data/chunks", req.AndroidID), 0755)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
