package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"backupserver/db"
)

// ★★★ 注册密钥，必须和客户端jni_bridge.cpp里的一致 ★★★
// ★★★ 部署前请改成你自己的随机字符串 ★★★
const regSecret = "Kj8#mZ2$pQ9xR4vL7nW1bY6cF3gT0hU5"

// 最大允许注册的设备数量
const maxDevices = 500

type RegisterRequest struct {
	AndroidID string `json:"android_id"`
	PublicKey string `json:"public_key"`
	RegToken  string `json:"reg_token"`
	RegTS     string `json:"reg_ts"`
}

// 验证注册令牌：HMAC-SHA256(regSecret, android_id + timestamp)
func verifyRegToken(androidID, timestamp, token string) bool {
	if androidID == "" || timestamp == "" || token == "" {
		return false
	}

	// 验证时间戳（5分钟窗口，防重放）
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	diff := time.Now().Unix() - ts
	if diff > 300 || diff < -300 {
		return false
	}

	// 计算期望的HMAC
	mac := hmac.New(sha256.New, []byte(regSecret))
	mac.Write([]byte(androidID + timestamp))
	expected := hex.EncodeToString(mac.Sum(nil))

	// 恒定时间比较，防时序攻击
	return hmac.Equal([]byte(expected), []byte(token))
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

	// ── 验证注册令牌 ──────────────────────────
	if !verifyRegToken(req.AndroidID, req.RegTS, req.RegToken) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// ── 检查设备数量上限 ──────────────────────
	count := db.GetDeviceCount()
	// 如果设备已注册过（更新公钥），不受上限限制
	if count >= maxDevices && !db.IsDeviceRegistered(req.AndroidID) {
		http.Error(w, "Device limit reached", http.StatusForbidden)
		return
	}

	// ── 验证公钥格式 ─────────────────────────
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
