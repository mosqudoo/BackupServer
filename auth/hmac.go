package auth

import (
	"bytes"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"io"
	"net/http"
	"strconv"
	"time"

	"backupserver/db"
)

// 验证RSA签名
func verifyRSA(publicKeyPEM string, message, sigBytes []byte) bool {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return false
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return false
	}
	hash := sha256.Sum256(message)
	err = rsa.VerifyPKCS1v15(rsaPub, crypto.SHA256, hash[:], sigBytes)
	return err == nil
}

func HMACMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 读取请求头
		androidID := r.Header.Get("X-Android-ID")
		timestamp  := r.Header.Get("X-Timestamp")
		sigB64     := r.Header.Get("X-Signature")

		if androidID == "" || timestamp == "" || sigB64 == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// 验证时间戳防重放（5分钟窗口）
		ts, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil {
			http.Error(w, "Invalid timestamp", http.StatusBadRequest)
			return
		}
		diff := time.Now().Unix() - ts
		if diff > 300 || diff < -300 {
			http.Error(w, "Timestamp expired", http.StatusUnauthorized)
			return
		}

		// 读取body
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// 注册接口不需要验证签名（设备还没有公钥）
		if r.URL.Path == "/api/register" {
			next.ServeHTTP(w, r)
			return
		}

		// 其他接口验证RSA签名
		pubKey, err := db.GetDevicePublicKey(androidID)
		if err != nil {
			http.Error(w, "Device not registered", http.StatusForbidden)
			return
		}

		// 签名内容：timestamp + androidID + body
		message := []byte(timestamp + androidID + string(body))

		sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
		if err != nil {
			http.Error(w, "Invalid signature", http.StatusBadRequest)
			return
		}

		if !verifyRSA(pubKey, message, sigBytes) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}
