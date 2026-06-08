package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os/exec"
	goRuntime "runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"aisadvisor/backend/cleanup"
	"aisadvisor/backend/config"
	"aisadvisor/backend/db"
	"aisadvisor/backend/forecast"
	"aisadvisor/backend/profile"
	"aisadvisor/backend/providers"
	"aisadvisor/backend/rules"
	"aisadvisor/backend/scanner"
	"aisadvisor/backend/security"
	"aisadvisor/backend/sre"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	profileID    int
	userID       int
	cancelScan   context.CancelFunc
	scanMu       sync.Mutex
	scanWg       sync.WaitGroup
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		profileID: 1,
		userID:    1,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	config.InitConfig()
	db.InitDB()
}

// BrowseFolder opens a native directory dialog and returns selected path
func (a *App) BrowseFolder() (string, error) {
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Folder to Scan",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// GetTheme loads current theme preference
func (a *App) GetTheme() (string, error) {
	var theme string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = ? AND setting_key = 'theme'", a.profileID).Scan(&theme)
	if err != nil {
		if err == sql.ErrNoRows {
			return "dark", nil
		}
		return "dark", err
	}
	return theme, nil
}

// SaveTheme saves theme preference
func (a *App) SaveTheme(theme string) error {
	_, err := db.DB.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, 'theme', ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		a.profileID, theme,
	)
	return err
}

// Vault methods for master password management
func (a *App) IsVaultInitialized() bool {
	return security.IsVaultInitialized()
}

func (a *App) IsVaultUnlocked() bool {
	return security.IsUnlocked()
}

func (a *App) InitializeVault(masterPassword string) error {
	return security.InitializeVault(masterPassword)
}

func (a *App) UnlockVault(masterPassword string) error {
	return security.UnlockVault(masterPassword)
}

func (a *App) LockVault() {
	security.LockVault()
}

func (a *App) ChangeMasterPassword(oldPassword, newPassword string) error {
	return security.ChangeMasterPassword(oldPassword, newPassword)
}

func (a *App) ExportProfile(filePath string, password string) error {
	return profile.ExportProfile(a.profileID, filePath, password)
}

func (a *App) ImportProfile(filePath string, password string) (string, error) {
	return profile.ImportProfile(a.userID, filePath, password)
}

// GetRecentScans fetches historical scan summary metadata
func (a *App) GetRecentScans() ([]map[string]interface{}, error) {
	rows, err := db.DB.Query("SELECT id, scan_path, scan_time, total_size, file_count FROM scan_history WHERE profile_id = ? ORDER BY scan_time DESC LIMIT 10", a.profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var path, scanTime string
		var totalSize int64
		var fileCount int
		if err := rows.Scan(&id, &path, &scanTime, &totalSize, &fileCount); err == nil {
			results = append(results, map[string]interface{}{
				"id":                   id,
				"scan_path":            path,
				"scan_time":            scanTime,
				"total_size":           totalSize,
				"total_size_formatted": scanner.FormatSize(totalSize),
				"file_count":           fileCount,
			})
		}
	}
	return results, nil
}

// GetSSHHosts fetches saved SSH servers
func (a *App) GetSSHHosts() ([]map[string]interface{}, error) {
	rows, err := db.DB.Query("SELECT id, name, host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE profile_id = ?", a.profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, port int
		var name, host, username, authType string
		var credentials, keyPassphrase sql.NullString
		if err := rows.Scan(&id, &name, &host, &port, &username, &authType, &credentials, &keyPassphrase); err == nil {
			var decCred string
			if credentials.Valid && credentials.String != "" {
				var decErr error
				decCred, decErr = security.Decrypt(credentials.String)
				if decErr != nil {
					log.Printf("Warning: failed to decrypt SSH credentials for host %s: %v", host, decErr)
					decCred = ""
				}
			}
		kp := ""
		if keyPassphrase.Valid && keyPassphrase.String != "" {
			var decErr error
			kp, decErr = security.Decrypt(keyPassphrase.String)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase for host %s: %v", host, decErr)
				kp = ""
			}
		}
			results = append(results, map[string]interface{}{
				"id":             id,
				"name":           name,
				"host":           host,
				"port":           port,
				"username":       username,
				"auth_type":      authType,
				"credentials":    decCred,
				"key_passphrase": kp,
			})
		}
	}
	return results, nil
}

// AddSSHHost creates a new server item
func (a *App) AddSSHHost(name, host string, port int, username, authType, credentials, keyPassphrase string) error {
	encCred := ""
	if credentials != "" {
		var err error
		encCred, err = security.Encrypt(credentials)
		if err != nil {
			return err
		}
	}

	encPassphrase := ""
	if keyPassphrase != "" {
		var err error
		encPassphrase, err = security.Encrypt(keyPassphrase)
		if err != nil {
			return err
		}
	}

	_, err := db.DB.Exec(
		"INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials, key_passphrase) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		a.profileID, name, host, port, username, authType, encCred, encPassphrase,
	)
	return err
}

// EditSSHHost updates existing server credentials
func (a *App) EditSSHHost(id int, name, host string, port int, username, authType, credentials, keyPassphrase string) error {
	encCred := ""
	if credentials != "" {
		var err error
		encCred, err = security.Encrypt(credentials)
		if err != nil {
			return err
		}
	}

	encPassphrase := ""
	if keyPassphrase != "" {
		var err error
		encPassphrase, err = security.Encrypt(keyPassphrase)
		if err != nil {
			return err
		}
	}

	_, err := db.DB.Exec(
		"UPDATE ssh_hosts SET name = ?, host = ?, port = ?, username = ?, auth_type = ?, credentials = ?, key_passphrase = ? WHERE id = ? AND profile_id = ?",
		name, host, port, username, authType, encCred, encPassphrase, id, a.profileID,
	)
	return err
}

// DeleteSSHHost removes saved SSH server
func (a *App) DeleteSSHHost(id int) error {
	_, err := db.DB.Exec("DELETE FROM ssh_hosts WHERE id = ? AND profile_id = ?", id, a.profileID)
	return err
}

// SaveScanRules persists scan conditions JSON
func (a *App) SaveScanRules(rulesJSON string) error {
	_, err := db.DB.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, 'scan_rules', ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		a.profileID, rulesJSON,
	)
	return err
}

// GetScanRules fetches stored rules JSON
func (a *App) GetScanRules() (string, error) {
	var val string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = ? AND setting_key = 'scan_rules'", a.profileID).Scan(&val)
	if err != nil {
		if err == sql.ErrNoRows {
			// Populate defaults
			def, _ := json.Marshal(rules.GetDefaultRules())
			return string(def), nil
		}
		return "", err
	}
	return val, nil
}

// SaveSetting persists generic settings
func (a *App) SaveSetting(key, value string) error {
	_, err := db.DB.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (?, ?, ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		a.profileID, key, value,
	)
	return err
}

// GetSetting fetches generic settings
func (a *App) GetSetting(key string) (string, error) {
	var val string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE setting_key = ? AND profile_id = ?", key, a.profileID).Scan(&val)
	if err != nil {
		return "", err
	}
	return val, nil
}

// GetAIProviders fetches configured providers
func (a *App) GetAIProviders() ([]map[string]interface{}, error) {
	rows, err := db.DB.Query("SELECT name, type, config_json, is_selected FROM ai_providers WHERE profile_id = ?", a.profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var name, typeVal, encConfig string
		var isSelected int
		if err := rows.Scan(&name, &typeVal, &encConfig, &isSelected); err == nil {
			var decErr error
			decConfig, decErr := security.Decrypt(encConfig)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt AI provider config for %s: %v", name, decErr)
				decConfig = ""
			}
			results = append(results, map[string]interface{}{
				"name":        name,
				"type":        typeVal,
				"config_json": decConfig,
				"is_selected": isSelected,
			})
		}
	}
	return results, nil
}

// SaveAIProvider saves/registers an AI configuration
func (a *App) SaveAIProvider(name, typeVal, configJSON string, isSelected int) error {
	encConfig, err := security.Encrypt(configJSON)
	if err != nil {
		return err
	}

	// If isSelected is 1, unselect other providers first
	if isSelected == 1 {
		_, _ = db.DB.Exec("UPDATE ai_providers SET is_selected = 0 WHERE profile_id = ?", a.profileID)
	}

	// Insert or replace provider configuration
	_, err = db.DB.Exec(
		"INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, ?) "+
			"ON CONFLICT(profile_id, name) DO UPDATE SET type = excluded.type, config_json = excluded.config_json, is_selected = excluded.is_selected",
		a.profileID, name, typeVal, encConfig, isSelected,
	)
	if err != nil {
		// Fallback for drivers that don't support ON CONFLICT on non-unique (in case schema is without unique constraint)
		var id int
		errCheck := db.DB.QueryRow("SELECT id FROM ai_providers WHERE profile_id = ? AND name = ?", a.profileID, name).Scan(&id)
		if errCheck == nil {
			_, err = db.DB.Exec("UPDATE ai_providers SET type = ?, config_json = ?, is_selected = ? WHERE id = ?", typeVal, encConfig, isSelected, id)
		} else {
			_, err = db.DB.Exec("INSERT INTO ai_providers (profile_id, name, type, config_json, is_selected) VALUES (?, ?, ?, ?, ?)", a.profileID, name, typeVal, encConfig, isSelected)
		}
	}

	return err
}

// ConnectionResult represents the test connection result
type ConnectionResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// TestAIProviderConnection tests connection with raw values
func (a *App) TestAIProviderConnection(providerType, apiKey, baseURL, model string, temp float64) ConnectionResult {
	cfg := providers.ProviderConfig{
		Type:        providerType,
		APIKey:      apiKey,
		BaseURL:     baseURL,
		Model:       model,
		Temperature: temp,
	}
	success, msg := providers.TestConnection(cfg)
	return ConnectionResult{
		Success: success,
		Message: msg,
	}
}

// GetAIModels fetches model names from specific provider
func (a *App) GetAIModels(providerType, apiKey, baseURL string) ([]string, error) {
	cfg := providers.ProviderConfig{
		Type:    providerType,
		APIKey:  apiKey,
		BaseURL: baseURL,
	}
	return providers.GetAvailableModels(cfg)
}

// StartScan triggers local, Network Share, or SSH Remote Linux file traversal
func (a *App) StartScan(connType string, hostID int, scanPath string) string {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()

	// Cancel any active scan first
	if a.cancelScan != nil {
		a.cancelScan()
	}
	a.scanWg.Wait()

	var scanCtx context.Context
	scanCtx, a.cancelScan = context.WithCancel(context.Background())

	a.scanWg.Add(1)
	go func() {
		defer a.scanWg.Done()

		// Load active rules
		var activeRules []rules.Rule
		rulesStr, err := a.GetScanRules()
		if err == nil {
			_ = json.Unmarshal([]byte(rulesStr), &activeRules)
		}

		var results scanner.ScanResults
		var scanErr error

		if connType == "SSH Remote Linux" {
			// Fetch host configuration
			var host, username, authType, credentials, encPassphrase string
			var port int
			errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
			if errHost != nil {
				runtime.EventsEmit(a.ctx, "scan:finished", map[string]interface{}{"error": "Saved SSH Host credentials not found."})
				return
			}
			decCredentials := ""
			if credentials != "" {
				var decErr error
				decCredentials, decErr = security.Decrypt(credentials)
				if decErr != nil {
					log.Printf("Warning: failed to decrypt SSH credentials for scan: %v", decErr)
				}
			}
			keyPassphrase := ""
			if encPassphrase != "" {
				var decErr error
				keyPassphrase, decErr = security.Decrypt(encPassphrase)
				if decErr != nil {
					log.Printf("Warning: failed to decrypt SSH key passphrase for scan: %v", decErr)
				}
			}

			cfgMap := map[string]interface{}{
				"host":           host,
				"port":           float64(port),
				"username":       username,
				"auth_type":      authType,
				"credentials":    decCredentials,
				"key_passphrase": keyPassphrase,
			}

			results, scanErr = scanner.ScanRemoteSSH(scanCtx, cfgMap, scanPath, activeRules, func(status string, filesScanned int, totalSize int64) {
				runtime.EventsEmit(a.ctx, "scan:progress", map[string]interface{}{
					"status":        status,
					"files_scanned": filesScanned,
					"total_size":    totalSize,
				})
			})
		} else {
			// Local or UNC Network Share
			results, scanErr = scanner.ScanLocalDisk(scanCtx, scanPath, activeRules, func(currentDir string, filesScanned int, totalSize int64) {
				runtime.EventsEmit(a.ctx, "scan:progress", map[string]interface{}{
					"status":        currentDir,
					"files_scanned": filesScanned,
					"total_size":    totalSize,
				})
			})
		}

		if scanErr != nil {
			runtime.EventsEmit(a.ctx, "scan:finished", map[string]interface{}{"error": scanErr.Error()})
			return
		}

		if results.Cancelled {
			runtime.EventsEmit(a.ctx, "scan:finished", map[string]interface{}{"cancelled": true})
			return
		}

		// Save scan history metadata in SQLite database
		resHistory, errHist := db.DB.Exec(
			"INSERT INTO scan_history (profile_id, scan_path, total_size, file_count) VALUES (?, ?, ?, ?)",
			a.profileID, scanPath, results.TotalSize, results.FilesScanned,
		)
		if errHist == nil {
			scanID, errID := resHistory.LastInsertId()
			if errID == nil {
				// Start a transaction for bulk insertions to prevent disk sync bottleneck
				tx, errTx := db.DB.Begin()
				if errTx == nil {
					// Insert top 15 large / temp / log analysis results for AI indexing
					categories := []struct {
						files []scanner.FileInfo
						cat   string
					}{
						{results.LargeFiles, "large"},
						{results.TempFiles, "temp"},
						{results.LogFiles, "log"},
						{results.BackupFiles, "backup"},
						{results.CacheFiles, "cache"},
					}
					for _, cg := range categories {
						cnt := 0
						for _, f := range cg.files {
							if cnt >= 15 {
								break
							}
							if _, err := tx.Exec(
								"INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation) VALUES (?, ?, ?, ?, ?, ?)",
								scanID, f.Path, cg.cat, f.Size, 0, f.RuleMatch,
							); err != nil {
								log.Printf("Warning: failed to insert analysis result: %v", err)
							}
							cnt++
						}
					}

					// Save duplicates metadata
					for hash, dupPaths := range results.DuplicateGroups {
						for _, dp := range dupPaths {
							if _, err := tx.Exec(
								"INSERT INTO duplicate_results (scan_id, file_hash, file_path, file_size) VALUES (?, ?, ?, ?)",
								scanID, hash, dp.Path, dp.Size,
							); err != nil {
								log.Printf("Warning: failed to insert duplicate result: %v", err)
							}
						}
					}

					if err := tx.Commit(); err != nil {
						log.Printf("Warning: failed to commit scan results transaction: %v", err)
					}
				}
			}
		}

		runtime.EventsEmit(a.ctx, "scan:finished", results)
	}()

	return "Scan started in background..."
}

// CancelScan aborts active traversals
func (a *App) CancelScan() {
	a.scanMu.Lock()
	defer a.scanMu.Unlock()
	if a.cancelScan != nil {
		a.cancelScan()
		a.cancelScan = nil
	}
}

// DryRunCleanup checks file sizes and write access
func (a *App) DryRunCleanup(connType string, hostID int, filePaths []string) (cleanup.DryRunResult, error) {
	if connType == "SSH Remote Linux" {
		var host, username, authType, credentials, encPassphrase string
		var port int
		errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
		if errHost != nil {
			return cleanup.DryRunResult{}, errHost
		}
		decCredentials := ""
		if credentials != "" {
			var decErr error
			decCredentials, decErr = security.Decrypt(credentials)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH credentials for dry run: %v", decErr)
			}
		}
		keyPassphrase := ""
		if encPassphrase != "" {
			var decErr error
			keyPassphrase, decErr = security.Decrypt(encPassphrase)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase for dry run: %v", decErr)
			}
		}
		client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
		if err != nil {
			return cleanup.DryRunResult{}, err
		}
		defer client.Close()

		var totalCount int
		var totalSize int64
		writableFiles := make([]map[string]interface{}, 0)

		// Get actual file sizes via stat
		for _, p := range filePaths {
			statCmd := fmt.Sprintf("stat -c%%s %s 2>/dev/null || echo 0", scanner.ShellQuote(p))
			out, err := scanner.RunSSHCommand(client, statCmd)
			size := int64(0)
			if err == nil {
				size, _ = strconv.ParseInt(strings.TrimSpace(out), 10, 64)
			}
			totalCount++
			totalSize += size
			writableFiles = append(writableFiles, map[string]interface{}{
				"path": p,
				"size": size,
			})
		}

		return cleanup.DryRunResult{
			TotalCount:         totalCount,
			TotalSize:          totalSize,
			TotalSizeFormatted: scanner.FormatSize(totalSize),
			WritableFiles:      writableFiles,
			RestrictedFiles:    []map[string]interface{}{},
			CanProceed:         len(writableFiles) > 0,
		}, nil
	}
	return cleanup.DryRun(filePaths), nil
}

// SafeDeleteFiles processes file list and sends updates
func (a *App) SafeDeleteFiles(connType string, hostID int, filePaths []string, useRecycleBin bool) {
	go func() {
		var deletedCount int
		var sizeFreed int64
		var failedPaths []map[string]interface{}

		if connType == "SSH Remote Linux" {
			var host, username, authType, credentials, encPassphrase string
			var port int
			errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
			if errHost != nil {
				runtime.EventsEmit(a.ctx, "delete:finished", map[string]interface{}{
					"deleted_count":        0,
					"size_freed":           0,
					"size_freed_formatted": "0 B",
					"failed_paths":         []map[string]interface{}{{"path": "SSH", "error": "Host not found"}},
				})
				return
			}
			decCredentials := ""
			if credentials != "" {
				var decErr error
				decCredentials, decErr = security.Decrypt(credentials)
				if decErr != nil {
					log.Printf("Warning: failed to decrypt SSH credentials for delete: %v", decErr)
				}
			}
			keyPassphrase := ""
			if encPassphrase != "" {
				var decErr error
				keyPassphrase, decErr = security.Decrypt(encPassphrase)
				if decErr != nil {
					log.Printf("Warning: failed to decrypt SSH key passphrase for delete: %v", decErr)
				}
			}
			client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
			if err != nil {
				runtime.EventsEmit(a.ctx, "delete:finished", map[string]interface{}{
					"deleted_count":        0,
					"size_freed":           0,
					"size_freed_formatted": "0 B",
					"failed_paths":         []map[string]interface{}{{"path": "SSH", "error": err.Error()}},
				})
				return
			}
			defer client.Close()

			// Get file sizes before batch delete
			for _, p := range filePaths {
				sizeCmd := fmt.Sprintf("stat -c%%s %s 2>/dev/null || echo 0", scanner.ShellQuote(p))
				if out, err := scanner.RunSSHCommand(client, sizeCmd); err == nil {
					if s, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64); err == nil {
						sizeFreed += s
					}
				}
			}

			// Batch delete via SSH
			batchSize := 50
			for i := 0; i < len(filePaths); i += batchSize {
				end := i + batchSize
				if end > len(filePaths) {
					end = len(filePaths)
				}
				batch := filePaths[i:end]

				escapedBatch := make([]string, len(batch))
				for j, p := range batch {
					escapedBatch[j] = scanner.ShellQuote(p)
				}

				runtime.EventsEmit(a.ctx, "delete:progress", map[string]interface{}{
					"current_index": i,
					"total_files":   len(filePaths),
					"current_file":  fmt.Sprintf("Batch SSH delete %d-%d", i, end),
				})

				rmCmd := fmt.Sprintf("rm -f %s", strings.Join(escapedBatch, " "))
				_, errCmd := scanner.RunSSHCommand(client, rmCmd)
				if errCmd != nil {
					for _, p := range batch {
						failedPaths = append(failedPaths, map[string]interface{}{
							"path":  p,
							"error": errCmd.Error(),
						})
					}
				} else {
					deletedCount += len(batch)
					for _, p := range batch {
						if _, err := db.DB.Exec("INSERT INTO cleanup_history (profile_id, cleaned_path, size_freed, status) VALUES (?, ?, ?, 'success')", a.profileID, p, 0); err != nil {
							log.Printf("Warning: failed to log cleanup: %v", err)
						}
					}
				}
			}

			runtime.EventsEmit(a.ctx, "delete:finished", map[string]interface{}{
				"deleted_count":        deletedCount,
				"size_freed":           sizeFreed,
				"size_freed_formatted": scanner.FormatSize(sizeFreed),
				"failed_paths":         failedPaths,
			})
			return
		}

		// Local deletion logic
		for idx, p := range filePaths {
			runtime.EventsEmit(a.ctx, "delete:progress", map[string]interface{}{
				"current_index": idx,
				"total_files":   len(filePaths),
				"current_file":  p,
			})

			freed, err := cleanup.SafeDeleteFile(a.profileID, p, useRecycleBin)
			if err != nil {
				failedPaths = append(failedPaths, map[string]interface{}{
					"path":  p,
					"error": err.Error(),
				})
			} else {
				deletedCount++
				sizeFreed += freed
			}
		}

		runtime.EventsEmit(a.ctx, "delete:finished", map[string]interface{}{
			"deleted_count":        deletedCount,
			"size_freed":           sizeFreed,
			"size_freed_formatted": scanner.FormatSize(sizeFreed),
			"failed_paths":         failedPaths,
		})
	}()
}

// GenerateAIRecommendation queries selected model for Ukrainian recommendations
func (a *App) GenerateAIRecommendation(diskSummary string, filesList []scanner.FileInfo) (string, error) {
	// Fetch active provider config
	var name, typeVal, encConfig string
	err := db.DB.QueryRow("SELECT name, type, config_json FROM ai_providers WHERE profile_id = ? AND is_selected = 1", a.profileID).Scan(&name, &typeVal, &encConfig)
	if err != nil {
		return "", errors.New("no active AI provider selected in Settings")
	}

	decConfig, err := security.Decrypt(encConfig)
	if err != nil {
		return "", errors.New("failed to decrypt AI provider configuration")
	}

	var cfg providers.ProviderConfig
	if err := json.Unmarshal([]byte(decConfig), &cfg); err != nil {
		return "", err
	}
	cfg.Type = typeVal

	system := providers.GetRecommendationSystemPrompt()
	user := fmt.Sprintf("Review the disk state and suggest items to clean up.\nDisk Status:\n%s\n\nTop Large / Temp / Log files found:\n", diskSummary)
	for i, f := range filesList {
		if i >= 15 {
			break
		}
		user += fmt.Sprintf("- %s (%s) - Category: %s\n", f.Path, f.SizeFormatted, f.Category)
	}
	lang := "Ukrainian"
	var dbLang string
	errLang := db.DB.QueryRow("SELECT setting_value FROM settings WHERE setting_key = 'ai_language' AND profile_id = ?", a.profileID).Scan(&dbLang)
	if errLang == nil && dbLang != "" {
		lang = dbLang
	}

	user += fmt.Sprintf("\nProvide clear, structured markdown recommendations, risks of deleting, and an actionable cleanup plan. Write your response entirely in %s language.", lang)

	return providers.QueryAI(cfg, system, user)
}

// QueryAIChat sends the conversational message history to the selected AI provider
func (a *App) QueryAIChat(history []providers.ChatMessage) (string, error) {
	// Fetch active provider config
	var name, typeVal, encConfig string
	err := db.DB.QueryRow("SELECT name, type, config_json FROM ai_providers WHERE profile_id = ? AND is_selected = 1", a.profileID).Scan(&name, &typeVal, &encConfig)
	if err != nil {
		return "", errors.New("no active AI provider selected in Settings")
	}

	decConfig, err := security.Decrypt(encConfig)
	if err != nil {
		return "", errors.New("failed to decrypt AI provider configuration")
	}

	var cfg providers.ProviderConfig
	if err := json.Unmarshal([]byte(decConfig), &cfg); err != nil {
		return "", err
	}
	cfg.Type = typeVal

	system := providers.GetRecommendationSystemPrompt()
	return providers.QueryAIChat(cfg, system, history)
}

// ClearContainerLogs truncates logs for a specific Docker container
func (a *App) ClearContainerLogs(connType string, hostID int, containerID string) error {
	if connType == "SSH Remote Linux" {
		var host, username, authType, credentials, encPassphrase string
		var port int
		errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
		if errHost != nil {
			return errHost
		}
		decCredentials := ""
		if credentials != "" {
			var decErr error
			decCredentials, decErr = security.Decrypt(credentials)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH credentials: %v", decErr)
			}
		}
		keyPassphrase := ""
		if encPassphrase != "" {
			var decErr error
			keyPassphrase, decErr = security.Decrypt(encPassphrase)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase: %v", decErr)
			}
		}
		client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
		if err != nil {
			return err
		}
		defer client.Close()

		// Retrieve container log path and truncate it
		cmd := fmt.Sprintf("log_path=$(docker inspect --format='{{.LogPath}}' %s) && (sudo truncate -s 0 \"$log_path\" || truncate -s 0 \"$log_path\")", scanner.ShellQuote(containerID))
		errCh := make(chan error, 1)
		go func() {
			_, err := scanner.RunSSHCommand(client, cmd)
			errCh <- err
		}()
		select {
		case err := <-errCh:
			return err
		case <-time.After(60 * time.Second):
			client.Close()
			return fmt.Errorf("SSH command timed out after 60 seconds")
		}
	} else {
		// Local truncate using a helper container to handle WSL Docker Desktop paths
		logPathCmd := exec.Command("docker", "inspect", "--format", "{{.LogPath}}", containerID)
		logPathBytes, err := logPathCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to locate container log path: %w", err)
		}
		logPath := strings.TrimSpace(string(logPathBytes))
		if logPath == "" {
			return fmt.Errorf("empty log path returned")
		}

		// Run alpine truncate helper container
		helperCmd := exec.Command("docker", "run", "--rm", "-v", "/var/lib/docker:/var/lib/docker", "alpine", "truncate", "-s", "0", logPath)
		return helperCmd.Run()
	}
}

// ClearPackageCache runs clean commands for package manager caches
func (a *App) ClearPackageCache(connType string, hostID int, cleanCmd string, cachePath string) error {
	if !isAllowedPackageCleanCommand(cleanCmd) {
		return errors.New("package cache cleanup command is not allowed")
	}

	if connType == "SSH Remote Linux" {
		var host, username, authType, credentials, encPassphrase string
		var port int
		errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
		if errHost != nil {
			return errHost
		}
		decCredentials := ""
		if credentials != "" {
			var decErr error
			decCredentials, decErr = security.Decrypt(credentials)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH credentials: %v", decErr)
			}
		}
		keyPassphrase := ""
		if encPassphrase != "" {
			var decErr error
			keyPassphrase, decErr = security.Decrypt(encPassphrase)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase: %v", decErr)
			}
		}
		client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
		if err != nil {
			return err
		}
		defer client.Close()

		errCh := make(chan error, 1)
		go func() {
			_, err := scanner.RunSSHCommand(client, cleanCmd)
			errCh <- err
		}()
		select {
		case err := <-errCh:
			return err
		case <-time.After(60 * time.Second):
			client.Close()
			return fmt.Errorf("SSH command timed out after 60 seconds")
		}
	} else {
		// Run command locally using native cmd / shell
		var cmd *exec.Cmd
		if goRuntime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", cleanCmd)
		} else {
			cmd = exec.Command("sh", "-c", cleanCmd)
		}
		return cmd.Run()
	}
}

// PruneDockerSystem runs docker system prune
func (a *App) PruneDockerSystem(connType string, hostID int) error {
	cmdStr := "docker system prune -af --volumes"
	if connType == "SSH Remote Linux" {
		var host, username, authType, credentials, encPassphrase string
		var port int
		errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
		if errHost != nil {
			return errHost
		}
		decCredentials := ""
		if credentials != "" {
			var decErr error
			decCredentials, decErr = security.Decrypt(credentials)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH credentials: %v", decErr)
			}
		}
		keyPassphrase := ""
		if encPassphrase != "" {
			var decErr error
			keyPassphrase, decErr = security.Decrypt(encPassphrase)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase: %v", decErr)
			}
		}
		client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
		if err != nil {
			return err
		}
		defer client.Close()

		errCh := make(chan error, 1)
		go func() {
			_, err := scanner.RunSSHCommand(client, "sudo "+cmdStr+" || "+cmdStr)
			errCh <- err
		}()
		select {
		case err := <-errCh:
			return err
		case <-time.After(60 * time.Second):
			client.Close()
			return fmt.Errorf("SSH command timed out after 60 seconds")
		}
	}

	// Local
	cmd := exec.Command("docker", "system", "prune", "-af", "--volumes")
	return cmd.Run()
}

func isAllowedPackageCleanCommand(cleanCmd string) bool {
	allowed := map[string]bool{
		"npm cache clean --force":                                 true,
		"pip cache purge":                                         true,
		"dotnet nuget locals all --clear":                         true,
		"sudo apt-get clean || apt-get clean":                     true,
		"sudo dnf clean all || sudo yum clean all":                true,
		"sudo pacman -Scc --noconfirm || pacman -Scc --noconfirm": true,
		"sudo zypper clean -a || zypper clean -a":                 true,
		"rm -rf ~/.cargo/registry/cache/*":                        true,
		"go clean -cache -modcache":                               true,
	}
	return allowed[strings.TrimSpace(cleanCmd)]
}

// VacuumJournaldLogs clears old journald logs
func (a *App) VacuumJournaldLogs(connType string, hostID int) error {
	cmdStr := "journalctl --vacuum-time=3d"
	if connType == "SSH Remote Linux" {
		var host, username, authType, credentials, encPassphrase string
		var port int
		errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials, key_passphrase FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials, &encPassphrase)
		if errHost != nil {
			return errHost
		}
		decCredentials := ""
		if credentials != "" {
			var decErr error
			decCredentials, decErr = security.Decrypt(credentials)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH credentials: %v", decErr)
			}
		}
		keyPassphrase := ""
		if encPassphrase != "" {
			var decErr error
			keyPassphrase, decErr = security.Decrypt(encPassphrase)
			if decErr != nil {
				log.Printf("Warning: failed to decrypt SSH key passphrase: %v", decErr)
			}
		}
		client, err := scanner.ConnectSSH(host, port, username, authType, decCredentials, keyPassphrase)
		if err != nil {
			return err
		}
		defer client.Close()

		errCh := make(chan error, 1)
		go func() {
			_, err := scanner.RunSSHCommand(client, "sudo "+cmdStr+" || "+cmdStr)
			errCh <- err
		}()
		select {
		case err := <-errCh:
			return err
		case <-time.After(60 * time.Second):
			client.Close()
			return fmt.Errorf("SSH command timed out after 60 seconds")
		}
	}

	// Local
	var cmd *exec.Cmd
	if goRuntime.GOOS == "windows" {
		// Not applicable to Windows
		return errors.New("Journald vacuum is not applicable to Windows")
	} else {
		cmd = exec.Command("sudo", "journalctl", "--vacuum-time=3d")
	}
	return cmd.Run()
}

// ClearWindowsEventLogs clears all Windows event logs using wevtutil
func (a *App) ClearWindowsEventLogs(connType string, hostID int) error {
	if connType == "SSH Remote Linux" || goRuntime.GOOS != "windows" {
		return errors.New("Windows Event Logs can only be cleared on a local Windows machine")
	}

	// PowerShell command to clear all event logs
	psCmd := `wevtutil el | Foreach-Object { wevtutil cl "$_" }`
	cmd := exec.Command("powershell", "-Command", psCmd)
	return cmd.Run()
}

// GetStorageForecast runs chronological size regression
func (a *App) GetStorageForecast(scanPath string) (forecast.ForecastResult, error) {
	return forecast.CalculateForecast(a.profileID, scanPath), nil
}

// CalculateHealthScore computes storage health score from scan results
func (a *App) CalculateHealthScore(totalSize int64, duplicateWaste int64, logSize int64, tempSize int64, sreDataJSON string) (map[string]interface{}, error) {
	var sreData sre.SreReport
	if err := json.Unmarshal([]byte(sreDataJSON), &sreData); err != nil {
		return nil, fmt.Errorf("failed to parse SRE data: %w", err)
	}

	// Get forecast to determine days remaining
	forecastData := forecast.CalculateForecast(a.profileID, "")
	daysRemaining := -1
	if forecastData.Status != "insufficient_data" {
		daysRemaining = forecastData.DaysRemaining
	}

	score, warnings := sre.CalculateHealthScore(totalSize, daysRemaining, duplicateWaste, logSize, tempSize, sreData)
	return map[string]interface{}{
		"score":    score,
		"warnings": warnings,
	}, nil
}
