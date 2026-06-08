package config

import (
	"log"
	"os"
	"path/filepath"

	"aisadvisor/backend/logger"
)

var (
	AppName     = "AI Storage Advisor"
	Version     = "0.1"
	AppRoot     string
	AppDataDir  string
	DbPath      string
	LogDir      string
	LogFile     string
)

func InitConfig() {
	// Determine executable folder for portable mode
	exePath, err := os.Executable()
	if err != nil {
		logger.Warn("Failed to get executable path: %v. Falling back to current directory.", err)
		AppRoot = "."
	} else {
		AppRoot = filepath.Dir(exePath)
	}

	AppDataDir = filepath.Join(AppRoot, "data")

	// Fallback to local AppData if the executable is running from a network drive
	// or if the data directory is not writable.
	useLocalFallback := false
	if isNetworkDrive(AppRoot) {
		useLocalFallback = true
		logger.Info("Running from a network drive (%s). Activating local AppData fallback.", AppRoot)
	} else {
		// Test write access to AppDataDir
		if err := os.MkdirAll(AppDataDir, 0755); err != nil {
			useLocalFallback = true
			logger.Info("Failed to create data directory: %v. Activating local AppData fallback.", err)
		} else {
			testFile := filepath.Join(AppDataDir, ".write_test")
			if f, err := os.Create(testFile); err != nil {
				useLocalFallback = true
				logger.Info("No write permission in data directory: %v. Activating local AppData fallback.", err)
			} else {
				f.Close()
				os.Remove(testFile)
			}
		}
	}

	if useLocalFallback {
		configDir, err := os.UserConfigDir()
		if err == nil {
			AppDataDir = filepath.Join(configDir, AppName)
		} else {
			AppDataDir = filepath.Join(os.TempDir(), AppName)
		}
		logger.Info("Using local data directory: %s", AppDataDir)
	}

	DbPath = filepath.Join(AppDataDir, "storage_advisor.db")
	LogDir = filepath.Join(AppDataDir, "logs")
	LogFile = filepath.Join(LogDir, "app.log")

	// Ensure final directories exist
	if err := os.MkdirAll(AppDataDir, 0755); err != nil {
		logger.Error("creating app data dir: %v", err)
	}
	if err := os.MkdirAll(LogDir, 0755); err != nil {
		logger.Error("creating log dir: %v", err)
	}

	// Set up simple logger output to file
	f, err := os.OpenFile(LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
	} else {
		logger.Warn("Failed to open log file: %v. Logging to stdout.", err)
	}

	logger.Info("Initialized application config. Data dir: %s", AppDataDir)
}
