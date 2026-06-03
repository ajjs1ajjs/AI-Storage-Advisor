package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"aisadvisor/backend/cleanup"
	"aisadvisor/backend/db"
	"aisadvisor/backend/forecast"
	"aisadvisor/backend/providers"
	"aisadvisor/backend/rules"
	"aisadvisor/backend/scanner"
	"aisadvisor/backend/security"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx          context.Context
	profileID    int
	userID       int
	cancelScan   context.CancelFunc
	activeScanCh chan bool
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		profileID:    1,
		userID:       1,
		activeScanCh: make(chan bool, 1),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	security.InitVault()
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
				"id":        id,
				"scan_path": path,
				"scan_time": scanTime,
				"total_size": totalSize,
				"total_size_formatted": scanner.FormatSize(totalSize),
				"file_count": fileCount,
			})
		}
	}
	return results, nil
}

// GetSSHHosts fetches saved SSH servers
func (a *App) GetSSHHosts() ([]map[string]interface{}, error) {
	rows, err := db.DB.Query("SELECT id, name, host, port, username, auth_type, credentials FROM ssh_hosts WHERE profile_id = ?", a.profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id, port int
		var name, host, username, authType string
		var credentials sql.NullString
		if err := rows.Scan(&id, &name, &host, &port, &username, &authType, &credentials); err == nil {
			var decCred string
			if credentials.Valid && credentials.String != "" {
				decCred, _ = security.Decrypt(credentials.String)
			}
			results = append(results, map[string]interface{}{
				"id":          id,
				"name":        name,
				"host":        host,
				"port":        port,
				"username":    username,
				"auth_type":   authType,
				"credentials": decCred,
			})
		}
	}
	return results, nil
}

// AddSSHHost creates a new server item
func (a *App) AddSSHHost(name, host string, port int, username, authType, credentials string) error {
	encCred := ""
	if credentials != "" {
		var err error
		encCred, err = security.Encrypt(credentials)
		if err != nil {
			return err
		}
	}

	_, err := db.DB.Exec(
		"INSERT INTO ssh_hosts (profile_id, name, host, port, username, auth_type, credentials) VALUES (?, ?, ?, ?, ?, ?, ?)",
		a.profileID, name, host, port, username, authType, encCred,
	)
	return err
}

// EditSSHHost updates existing server credentials
func (a *App) EditSSHHost(id int, name, host string, port int, username, authType, credentials string) error {
	encCred := ""
	if credentials != "" {
		var err error
		encCred, err = security.Encrypt(credentials)
		if err != nil {
			return err
		}
	}

	_, err := db.DB.Exec(
		"UPDATE ssh_hosts SET name = ?, host = ?, port = ?, username = ?, auth_type = ?, credentials = ? WHERE id = ? AND profile_id = ?",
		name, host, port, username, authType, encCred, id, a.profileID,
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

// GetAIProviders fetches all AI configs
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
			decConfig, _ := security.Decrypt(encConfig)
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
	// Cancel any active scan first
	a.CancelScan()

	var scanCtx context.Context
	scanCtx, a.cancelScan = context.WithCancel(context.Background())

	go func() {
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
			var host, username, authType, credentials string
			var port int
			errHost := db.DB.QueryRow("SELECT host, port, username, auth_type, credentials FROM ssh_hosts WHERE id = ? AND profile_id = ?", hostID, a.profileID).Scan(&host, &port, &username, &authType, &credentials)
			if errHost != nil {
				runtime.EventsEmit(a.ctx, "scan:finished", map[string]interface{}{"error": "Saved SSH Host credentials not found."})
				return
			}
			decCredentials := ""
			if credentials != "" {
				decCredentials, _ = security.Decrypt(credentials)
			}

			cfgMap := map[string]interface{}{
				"host":        host,
				"port":        float64(port),
				"username":    username,
				"auth_type":   authType,
				"credentials": decCredentials,
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
			scanID, _ := resHistory.LastInsertId()
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
					_, _ = db.DB.Exec(
						"INSERT INTO analysis_results (scan_id, path, category, size, risk_score, recommendation) VALUES (?, ?, ?, ?, ?, ?)",
						scanID, f.Path, cg.cat, f.Size, 0, f.RuleMatch,
					)
					cnt++
				}
			}

			// Save duplicates metadata
			for hash, dupPaths := range results.DuplicateGroups {
				for _, dp := range dupPaths {
					_, _ = db.DB.Exec(
						"INSERT INTO duplicate_results (scan_id, file_hash, file_path, file_size) VALUES (?, ?, ?, ?)",
						scanID, hash, dp.Path, dp.Size,
					)
				}
			}
		}

		runtime.EventsEmit(a.ctx, "scan:finished", results)
	}()

	return "Scan started in background..."
}

// CancelScan aborts active traversals
func (a *App) CancelScan() {
	if a.cancelScan != nil {
		a.cancelScan()
		a.cancelScan = nil
	}
}

// DryRunCleanup checks file sizes and write access
func (a *App) DryRunCleanup(filePaths []string) (cleanup.DryRunResult, error) {
	return cleanup.DryRun(filePaths), nil
}

// SafeDeleteFiles processes file list and sends updates
func (a *App) SafeDeleteFiles(filePaths []string, useRecycleBin bool) {
	go func() {
		var deletedCount int
		var sizeFreed int64
		var failedPaths []map[string]interface{}

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
			"size_freed":          sizeFreed,
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
	user += "\nProvide clear, structured markdown recommendations, risks of deleting, and an actionable cleanup plan. Write your response entirely in Ukrainian language."

	return providers.QueryAI(cfg, system, user)
}

// GetStorageForecast runs chronological size regression
func (a *App) GetStorageForecast(scanPath string) (forecast.ForecastResult, error) {
	return forecast.CalculateForecast(a.profileID, scanPath), nil
}
