package profile

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"aisadvisor/backend/db"
	"aisadvisor/backend/security"
)

func createTestTables(t *testing.T, dbConn *sql.DB) {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			password_hash TEXT NOT NULL,
			salt TEXT NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			profile_name TEXT NOT NULL,
			is_active INTEGER DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
			UNIQUE(user_id, profile_name)
		)`,
		`CREATE TABLE IF NOT EXISTS ai_providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			config_json TEXT NOT NULL,
			is_selected INTEGER DEFAULT 0,
			FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS ssh_hosts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			host TEXT NOT NULL,
			port INTEGER DEFAULT 22,
			username TEXT NOT NULL,
			auth_type TEXT NOT NULL,
			credentials TEXT,
			key_passphrase TEXT DEFAULT '',
			FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS scan_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			scan_path TEXT NOT NULL,
			scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			total_size INTEGER NOT NULL,
			file_count INTEGER NOT NULL,
			metadata TEXT,
			FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS analysis_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			scan_id INTEGER NOT NULL,
			path TEXT NOT NULL,
			category TEXT NOT NULL,
			size INTEGER NOT NULL,
			risk_score INTEGER DEFAULT 0,
			recommendation TEXT,
			is_ignored INTEGER DEFAULT 0,
			FOREIGN KEY (scan_id) REFERENCES scan_history(id) ON DELETE CASCADE
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			setting_key TEXT NOT NULL,
			setting_value TEXT NOT NULL,
			FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE,
			UNIQUE(profile_id, setting_key)
		)`,
	}
	for _, q := range queries {
		if _, err := dbConn.Exec(q); err != nil {
			t.Fatalf("Failed to create table: %v\nQuery: %s", err, q)
		}
	}
}

func setupTestDB(t *testing.T) string {
	t.Helper()

	security.LockVault()

	tmpDir, err := os.MkdirTemp("", "profile-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	dbPath := filepath.Join(tmpDir, "test.db")
	testDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to open test DB: %v", err)
	}

	_, _ = testDB.Exec("PRAGMA busy_timeout=5000;")
	_, _ = testDB.Exec("PRAGMA journal_mode=WAL;")

	createTestTables(t, testDB)

	_, err = testDB.Exec("INSERT INTO users (id, username, password_hash, salt) VALUES (1, 'default_user', 'N/A', 'N/A')")
	if err != nil {
		testDB.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to seed user: %v", err)
	}

	_, err = testDB.Exec("INSERT INTO profiles (id, user_id, profile_name, is_active) VALUES (1, 1, 'test_profile', 1)")
	if err != nil {
		testDB.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to seed profile: %v", err)
	}

	origDB := db.DB
	db.DB = testDB

	if err := security.InitializeVault("test-password"); err != nil {
		db.DB = origDB
		testDB.Close()
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to initialize vault: %v", err)
	}

	t.Cleanup(func() {
		db.DB = origDB
		security.LockVault()
		testDB.Close()
		os.RemoveAll(tmpDir)
	})

	return tmpDir
}

func seedTestProfileData(t *testing.T) {
	t.Helper()

	encryptedConfig, err := security.Encrypt(`{"api_key":"sk-test123","model":"gpt-4"}`)
	if err != nil {
		t.Fatalf("Failed to encrypt AI provider config: %v", err)
	}

	encryptedCred, err := security.Encrypt("supersshpassword")
	if err != nil {
		t.Fatalf("Failed to encrypt SSH credential: %v", err)
	}

	encryptedPassphrase, err := security.Encrypt("mykeypass")
	if err != nil {
		t.Fatalf("Failed to encrypt SSH passphrase: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, ?)`,
		1, "test-ai", "api", encryptedConfig, 1)
	if err != nil {
		t.Fatalf("Failed to insert AI provider: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials, key_passphrase) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		1, "test-server", "192.168.1.100", 22, "admin", "password", encryptedCred, encryptedPassphrase)
	if err != nil {
		t.Fatalf("Failed to insert SSH host: %v", err)
	}

	result, err := db.DB.Exec(`INSERT INTO scan_history (profile_id, scan_path, scan_time, total_size, file_count, metadata) VALUES (?, ?, ?, ?, ?, ?)`,
		1, "/home/user", "2024-01-15T10:30:00Z", 1048576, 150, `{"source":"manual"}`)
	if err != nil {
		t.Fatalf("Failed to insert scan history: %v", err)
	}

	scanID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("Failed to get scan ID: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation, is_ignored) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		scanID, "/home/user/largefile.log", "log", 524288, 30, "Consider archiving old logs", 0)
	if err != nil {
		t.Fatalf("Failed to insert analysis result: %v", err)
	}

	_, err = db.DB.Exec(`INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation, is_ignored) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		scanID, "/home/user/temp.tmp", "temp", 1024, 5, "", 0)
	if err != nil {
		t.Fatalf("Failed to insert analysis result: %v", err)
	}
}

func TestExportProfile(t *testing.T) {
	tmpDir := setupTestDB(t)
	seedTestProfileData(t)

	exportPath := filepath.Join(tmpDir, "export.dat")
	err := ExportProfile(1, exportPath, "export-password")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	info, err := os.Stat(exportPath)
	if err != nil {
		t.Fatalf("Export file not found: %v", err)
	}
	if info.Size() < 28 {
		t.Fatalf("Export file too small: %d bytes (minimum 28)", info.Size())
	}

	data, err := os.ReadFile(exportPath)
	if err != nil {
		t.Fatalf("Failed to read export: %v", err)
	}
	if len(data) < 16 {
		t.Fatalf("Export too short for salt")
	}

	salt := data[:16]
	encrypted := data[16:]
	key := security.DeriveArchiveKey("export-password", salt)
	decrypted, err := security.DecryptArchive(encrypted, key)
	if err != nil {
		t.Fatalf("Failed to decrypt exported data: %v", err)
	}

	var pkg ProfilePackage
	if err := json.Unmarshal(decrypted, &pkg); err != nil {
		t.Fatalf("Failed to unmarshal profile: %v", err)
	}
	if pkg.ProfileName != "test_profile" {
		t.Fatalf("Expected profile name 'test_profile', got %q", pkg.ProfileName)
	}
	if len(pkg.AIProviders) != 1 {
		t.Fatalf("Expected 1 AI provider, got %d", len(pkg.AIProviders))
	}
	if len(pkg.SSHHosts) != 1 {
		t.Fatalf("Expected 1 SSH host, got %d", len(pkg.SSHHosts))
	}
	if len(pkg.ScanHistory) != 1 {
		t.Fatalf("Expected 1 scan history, got %d", len(pkg.ScanHistory))
	}
	if len(pkg.ScanHistory[0].AnalysisResults) != 2 {
		t.Fatalf("Expected 2 analysis results, got %d", len(pkg.ScanHistory[0].AnalysisResults))
	}
}

func TestImportProfile(t *testing.T) {
	tmpDir := setupTestDB(t)
	seedTestProfileData(t)

	_, err := db.DB.Exec("INSERT INTO users (id, username, password_hash, salt) VALUES (2, 'import_user', 'N/A', 'N/A')")
	if err != nil {
		t.Fatalf("Failed to insert import user: %v", err)
	}

	exportPath := filepath.Join(tmpDir, "export.dat")
	err = ExportProfile(1, exportPath, "export-password")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	profileName, err := ImportProfile(2, exportPath, "export-password")
	if err != nil {
		t.Fatalf("ImportProfile failed: %v", err)
	}
	if profileName != "test_profile" {
		t.Fatalf("Expected profile name 'test_profile', got %q", profileName)
	}

	var count int
	err = db.DB.QueryRow("SELECT COUNT(1) FROM profiles WHERE profile_name = ?", "test_profile").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query imported profile: %v", err)
	}
	if count != 2 {
		t.Fatalf("Expected 2 profiles named 'test_profile', got %d", count)
	}
}

func TestImportProfileWrongPassword(t *testing.T) {
	tmpDir := setupTestDB(t)
	seedTestProfileData(t)

	exportPath := filepath.Join(tmpDir, "export.dat")
	err := ExportProfile(1, exportPath, "export-password")
	if err != nil {
		t.Fatalf("ExportProfile failed: %v", err)
	}

	_, err = ImportProfile(1, exportPath, "wrong-password")
	if err == nil {
		t.Fatal("Expected error when importing with wrong password, got nil")
	}
}

func TestImportProfileCorruptedData(t *testing.T) {
	tmpDir := setupTestDB(t)

	corruptPath := filepath.Join(tmpDir, "corrupt.dat")
	err := os.WriteFile(corruptPath, []byte("this is not a valid encrypted profile"), 0644)
	if err != nil {
		t.Fatalf("Failed to write corrupted file: %v", err)
	}

	_, err = ImportProfile(1, corruptPath, "test-password")
	if err == nil {
		t.Fatal("Expected error when importing corrupted data, got nil")
	}
}

func TestExportProfileNonExistent(t *testing.T) {
	tmpDir := setupTestDB(t)

	exportPath := filepath.Join(tmpDir, "export.dat")
	err := ExportProfile(999, exportPath, "test-password")
	if err == nil {
		t.Fatal("Expected error when exporting non-existent profile, got nil")
	}
}
