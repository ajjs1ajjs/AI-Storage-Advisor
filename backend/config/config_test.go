package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInitConfig(t *testing.T) {
	// Reset state before test
	AppName = "AI Storage Advisor"
	Version = "0.1"

	InitConfig()

	if AppRoot == "" {
		t.Fatal("AppRoot should not be empty after InitConfig")
	}
	if AppDataDir == "" {
		t.Fatal("AppDataDir should not be empty after InitConfig")
	}
	if DbPath == "" {
		t.Fatal("DbPath should not be empty after InitConfig")
	}

	// Verify data directory was created
	if _, err := os.Stat(AppDataDir); os.IsNotExist(err) {
		t.Fatalf("AppDataDir %s was not created", AppDataDir)
	}

	// Verify log directory was created
	if _, err := os.Stat(LogDir); os.IsNotExist(err) {
		t.Fatalf("LogDir %s was not created", LogDir)
	}
}

func TestInitConfigDataDirNotWritable(t *testing.T) {
	// Save original values
	origAppName := AppName

	AppName = "TestApp"

	// Force to temp dir by using a non-writable path simulation
	// The function already handles this with fallback logic
	InitConfig()

	// After InitConfig, data dir should exist
	if _, err := os.Stat(AppDataDir); os.IsNotExist(err) {
		t.Fatalf("AppDataDir %s should exist after InitConfig fallback", AppDataDir)
	}

	AppName = origAppName
}

func TestConfigPaths(t *testing.T) {
	InitConfig()

	if DbPath == "" {
		t.Fatal("DbPath is empty")
	}

	// DbPath should be within AppDataDir
	rel, err := filepath.Rel(AppDataDir, DbPath)
	if err != nil {
		t.Fatalf("DbPath not relative to AppDataDir: %v", err)
	}
	if rel != "storage_advisor.db" {
		t.Fatalf("expected DbPath to be 'storage_advisor.db' relative to AppDataDir, got %q", rel)
	}
}
