package profile

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"aisadvisor/backend/db"
	"aisadvisor/backend/security"
)

type SavedSetting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type SavedAIProvider struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	ConfigJSON string `json:"config_json"`
	IsSelected int    `json:"is_selected"`
}

type SavedSSHHost struct {
	Name        string `json:"name"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	AuthType    string `json:"auth_type"`
	Credentials string `json:"credentials"`
}

type SavedAnalysisResult struct {
	Path           string `json:"path"`
	Category       string `json:"category"`
	Size           int64  `json:"size"`
	RiskScore      int    `json:"risk_score"`
	Recommendation string `json:"recommendation"`
	IsIgnored      int    `json:"is_ignored"`
}

type SavedScanHistory struct {
	ScanPath        string                `json:"scan_path"`
	ScanTime        string                `json:"scan_time"`
	TotalSize       int64                 `json:"total_size"`
	FileCount       int                   `json:"file_count"`
	Metadata        string                `json:"metadata"`
	AnalysisResults []SavedAnalysisResult `json:"analysis_results"`
}

type ProfilePackage struct {
	ProfileName string             `json:"profile_name"`
	Settings    []SavedSetting     `json:"settings"`
	AIProviders []SavedAIProvider  `json:"ai_providers"`
	SSHHosts    []SavedSSHHost     `json:"ssh_hosts"`
	ScanHistory []SavedScanHistory `json:"scan_history"`
}

func ExportProfile(profileID int, filePath string, password string) error {
	// 1. Fetch Profile Name
	var profileName string
	err := db.DB.QueryRow("SELECT profile_name FROM profiles WHERE id = ?", profileID).Scan(&profileName)
	if err != nil {
		return fmt.Errorf("profile not found: %w", err)
	}

	// 2. Fetch Settings
	rowsSettings, err := db.DB.Query("SELECT setting_key, setting_value FROM settings WHERE profile_id = ?", profileID)
	if err != nil {
		return err
	}
	defer rowsSettings.Close()
	var settings []SavedSetting
	for rowsSettings.Next() {
		var s SavedSetting
		if err := rowsSettings.Scan(&s.Key, &s.Value); err == nil {
			settings = append(settings, s)
		}
	}

	// 3. Fetch AI Providers
	rowsAI, err := db.DB.Query("SELECT name, type, config_json, is_selected FROM ai_providers WHERE profile_id = ?", profileID)
	if err != nil {
		return err
	}
	defer rowsAI.Close()
	var aiProviders []SavedAIProvider
	for rowsAI.Next() {
		var ap SavedAIProvider
		var encryptedConfig string
		if err := rowsAI.Scan(&ap.Name, &ap.Type, &encryptedConfig, &ap.IsSelected); err == nil {
			decrypted, err := security.Decrypt(encryptedConfig)
			if err != nil {
				ap.ConfigJSON = ""
			} else {
				ap.ConfigJSON = decrypted
			}
			aiProviders = append(aiProviders, ap)
		}
	}

	// 4. Fetch SSH Hosts
	rowsSSH, err := db.DB.Query("SELECT name, host, port, username, auth_type, credentials FROM ssh_hosts WHERE profile_id = ?", profileID)
	if err != nil {
		return err
	}
	defer rowsSSH.Close()
	var sshHosts []SavedSSHHost
	for rowsSSH.Next() {
		var sh SavedSSHHost
		var encryptedCred sql.NullString
		if err := rowsSSH.Scan(&sh.Name, &sh.Host, &sh.Port, &sh.Username, &sh.AuthType, &encryptedCred); err == nil {
			if encryptedCred.Valid && encryptedCred.String != "" {
				decrypted, err := security.Decrypt(encryptedCred.String)
				if err == nil {
					sh.Credentials = decrypted
				}
			}
			sshHosts = append(sshHosts, sh)
		}
	}

	// 5. Fetch Scan History & Analysis Results
	rowsScan, err := db.DB.Query("SELECT id, scan_path, scan_time, total_size, file_count, metadata FROM scan_history WHERE profile_id = ?", profileID)
	if err != nil {
		return err
	}
	defer rowsScan.Close()
	var scanHistory []SavedScanHistory
	for rowsScan.Next() {
		var scanID int
		var sh SavedScanHistory
		var metadata sql.NullString
		if err := rowsScan.Scan(&scanID, &sh.ScanPath, &sh.ScanTime, &sh.TotalSize, &sh.FileCount, &metadata); err == nil {
			if metadata.Valid {
				sh.Metadata = metadata.String
			}

			// Fetch analysis results associated with this scan
			rowsAR, errAR := db.DB.Query("SELECT path, category, size, risk_score, recommendation, is_ignored FROM analysis_results WHERE scan_id = ?", scanID)
			if errAR == nil {
				var arList []SavedAnalysisResult
				for rowsAR.Next() {
					var ar SavedAnalysisResult
					var rec sql.NullString
					if errScan := rowsAR.Scan(&ar.Path, &ar.Category, &ar.Size, &ar.RiskScore, &rec, &ar.IsIgnored); errScan == nil {
						if rec.Valid {
							ar.Recommendation = rec.String
						}
						arList = append(arList, ar)
					}
				}
				rowsAR.Close()
				sh.AnalysisResults = arList
			}
			scanHistory = append(scanHistory, sh)
		}
	}

	// Assemble payload
	payload := ProfilePackage{
		ProfileName: profileName,
		Settings:    settings,
		AIProviders: aiProviders,
		SSHHosts:    sshHosts,
		ScanHistory: scanHistory,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize profile payload: %w", err)
	}

	// Encryption Vault derivation
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	key := security.DeriveArchiveKey(password, salt)
	encryptedPayload, err := security.EncryptArchive(payloadBytes, key)
	if err != nil {
		return err
	}

	// Output format: salt (16 bytes) + encryptedPayload (which includes nonce prepended)
	combined := append(salt, encryptedPayload...)
	return os.WriteFile(filePath, combined, 0644)
}

func ImportProfile(userID int, filePath string, password string) (string, error) {
	if !security.IsUnlocked() {
		return "", errors.New("vault is locked. User must log in first to import profiles")
	}

	combined, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	if len(combined) < 28 { // 16 bytes salt + 12 bytes nonce + ciphertext
		return "", errors.New("invalid profile file (too small)")
	}

	salt := combined[:16]
	encryptedData := combined[16:]

	// Derive key and decrypt
	key := security.DeriveArchiveKey(password, salt)
	payloadBytes, err := security.DecryptArchive(encryptedData, key)
	if err != nil {
		return "", fmt.Errorf("decryption failed: incorrect password or corrupted data: %w", err)
	}

	var payload ProfilePackage
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return "", fmt.Errorf("failed to parse profile payload: %w", err)
	}

	profileName := payload.ProfileName
	if profileName == "" {
		profileName = "Imported Workspace"
	}

	// Start Database transaction
	tx, err := db.DB.Begin()
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	// Insert profile
	var profileID int64
	res, err := tx.Exec("INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, ?, 0)", userID, profileName)
	if err != nil {
		// Integrity error fallback: try timestamp suffix
		timestamp := time.Now().Unix()
		profileName = fmt.Sprintf("%s (%d)", profileName, timestamp)
		res, err = tx.Exec("INSERT INTO profiles (user_id, profile_name, is_active) VALUES (?, ?, 0)", userID, profileName)
		if err != nil {
			return "", err
		}
	}
	profileID, err = res.LastInsertId()
	if err != nil {
		return "", err
	}

	// Insert Settings
	for _, s := range payload.Settings {
		_, err = tx.Exec("INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, ?, ?)", profileID, s.Key, s.Value)
		if err != nil {
			return "", err
		}
	}

	// Insert AI Providers (encrypt config under current session key)
	for _, ap := range payload.AIProviders {
		encryptedConfig, err := security.Encrypt(ap.ConfigJSON)
		if err != nil {
			return "", err
		}
		_, err = tx.Exec("INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, ?)", profileID, ap.Name, ap.Type, encryptedConfig, ap.IsSelected)
		if err != nil {
			return "", err
		}
	}

	// Insert SSH Hosts (encrypt credentials under current session key)
	for _, sh := range payload.SSHHosts {
		var encryptedCred sql.NullString
		if sh.Credentials != "" {
			enc, err := security.Encrypt(sh.Credentials)
			if err != nil {
				return "", err
			}
			encryptedCred = sql.NullString{String: enc, Valid: true}
		}

		_, err = tx.Exec("INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials) VALUES (?, ?, ?, ?, ?, ?, ?)", profileID, sh.Name, sh.Host, sh.Port, sh.Username, sh.AuthType, encryptedCred)
		if err != nil {
			return "", err
		}
	}

	// Insert Scan history & analysis results
	for _, sh := range payload.ScanHistory {
		resScan, err := tx.Exec("INSERT INTO scan_history (profile_id, scan_path, scan_time, total_size, file_count, metadata) VALUES (?, ?, ?, ?, ?, ?)", profileID, sh.ScanPath, sh.ScanTime, sh.TotalSize, sh.FileCount, sh.Metadata)
		if err != nil {
			return "", err
		}
		scanID, err := resScan.LastInsertId()
		if err != nil {
			return "", err
		}

		for _, ar := range sh.AnalysisResults {
			_, err = tx.Exec("INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation, is_ignored) VALUES (?, ?, ?, ?, ?, ?, ?)", scanID, ar.Path, ar.Category, ar.Size, ar.RiskScore, ar.Recommendation, ar.IsIgnored)
			if err != nil {
				return "", err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return profileName, nil
}
