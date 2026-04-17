package db

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func Init(path string) error {
	var err error
	DB, err = sql.Open("sqlite3", path+"?_journal=WAL&_timeout=5000")
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		CREATE TABLE IF NOT EXISTS uploaded_files (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			android_id  TEXT NOT NULL,
			hash_key    TEXT NOT NULL,
			filename    TEXT,
			size        INTEGER,
			uploaded_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(android_id, hash_key)
		);
		CREATE TABLE IF NOT EXISTS chunks (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			android_id  TEXT NOT NULL,
			hash_key    TEXT NOT NULL,
			chunk_index INTEGER NOT NULL,
			chunk_total INTEGER NOT NULL,
			saved_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(android_id, hash_key, chunk_index)
		);
		CREATE TABLE IF NOT EXISTS devices (
			id           INTEGER PRIMARY KEY AUTOINCREMENT,
			android_id   TEXT NOT NULL UNIQUE,
			public_key   TEXT NOT NULL,
			registered_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			banned       INTEGER DEFAULT 0
		);
	`)
	return err
}

func IsUploaded(androidID, hashKey string) bool {
	var count int
	DB.QueryRow(
		"SELECT COUNT(*) FROM uploaded_files WHERE android_id=? AND hash_key=?",
		androidID, hashKey,
	).Scan(&count)
	return count > 0
}

func MarkUploaded(androidID, hashKey, filename string, size int64) error {
	_, err := DB.Exec(
		`INSERT OR IGNORE INTO uploaded_files(android_id, hash_key, filename, size)
		 VALUES(?,?,?,?)`,
		androidID, hashKey, filename, size,
	)
	return err
}

func SaveChunk(androidID, hashKey string, index, total int) error {
	_, err := DB.Exec(
		`INSERT OR IGNORE INTO chunks(android_id, hash_key, chunk_index, chunk_total)
		 VALUES(?,?,?,?)`,
		androidID, hashKey, index, total,
	)
	return err
}

func GetNextChunk(androidID, hashKey string) int {
	rows, err := DB.Query(
		"SELECT chunk_index FROM chunks WHERE android_id=? AND hash_key=? ORDER BY chunk_index",
		androidID, hashKey,
	)
	if err != nil {
		return 0
	}
	defer rows.Close()

	expected := 0
	for rows.Next() {
		var idx int
		rows.Scan(&idx)
		if idx != expected {
			return expected
		}
		expected++
	}
	return expected
}

// 注册设备公钥
func RegisterDevice(androidID, publicKey string) error {
	_, err := DB.Exec(
		`INSERT INTO devices(android_id, public_key)
		 VALUES(?,?)
		 ON CONFLICT(android_id) DO UPDATE SET public_key=excluded.public_key`,
		androidID, publicKey,
	)
	return err
}

// 获取设备公钥
func GetDevicePublicKey(androidID string) (string, error) {
	var pubKey string
	var banned int
	err := DB.QueryRow(
		"SELECT public_key, banned FROM devices WHERE android_id=?",
		androidID,
	).Scan(&pubKey, &banned)
	if err != nil {
		return "", err
	}
	if banned == 1 {
		return "", sql.ErrNoRows
	}
	return pubKey, nil
}

// 检查设备是否已注册
func IsDeviceRegistered(androidID string) bool {
	var count int
	DB.QueryRow(
		"SELECT COUNT(*) FROM devices WHERE android_id=? AND banned=0",
		androidID,
	).Scan(&count)
	return count > 0
}

// 获取已注册设备总数
func GetDeviceCount() int {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM devices WHERE banned=0").Scan(&count)
	return count
}

// 获取某设备已用存储量（字节）
func GetDeviceUsage(androidID string) int64 {
	var total sql.NullInt64
	DB.QueryRow(
		"SELECT SUM(size) FROM uploaded_files WHERE android_id=?",
		androidID,
	).Scan(&total)
	if total.Valid {
		return total.Int64
	}
	return 0
}
