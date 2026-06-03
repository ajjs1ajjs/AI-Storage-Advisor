package config

import (
	"log"
	"os"
	"path/filepath"
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
		log.Printf("Warning: Failed to get executable path: %v. Falling back to current directory.", err)
		AppRoot = "."
	} else {
		AppRoot = filepath.Dir(exePath)
	}

	AppDataDir = filepath.Join(AppRoot, "data")
	DbPath = filepath.Join(AppDataDir, "storage_advisor.db")
	LogDir = filepath.Join(AppDataDir, "logs")
	LogFile = filepath.Join(LogDir, "app.log")

	// Ensure directories exist
	if err := os.MkdirAll(AppDataDir, 0755); err != nil {
		log.Printf("Error creating app data dir: %v", err)
	}
	if err := os.MkdirAll(LogDir, 0755); err != nil {
		log.Printf("Error creating log dir: %v", err)
	}

	// Set up simple logger output to file
	f, err := os.OpenFile(LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(f)
	} else {
		log.Printf("Warning: Failed to open log file: %v. Logging to stdout.", err)
	}

	log.Printf("Initialized application config. Data dir: %s", AppDataDir)
}
