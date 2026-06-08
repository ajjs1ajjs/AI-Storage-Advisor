package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
	"aisadvisor/backend/config"
)

func TestInitDB(t *testing.T) {
	origDbPath := config.DbPath
	t.Cleanup(func() {
		config.DbPath = origDbPath
		if DB != nil {
			DB.Close()
			DB = nil
		}
	})

	config.DbPath = ":memory:"

	InitDB()

	if DB == nil {
		t.Fatal("DB global should not be nil after InitDB")
	}

	expectedTables := []string{
		"users", "profiles", "ai_providers", "ssh_hosts",
		"scan_history", "analysis_results", "cleanup_history",
		"duplicate_results", "forecast_history", "settings",
	}

	for _, table := range expectedTables {
		var name string
		err := DB.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?",
			table,
		).Scan(&name)
		if err == sql.ErrNoRows {
			t.Errorf("Expected table %q to exist, but it was not found", table)
		} else if err != nil {
			t.Errorf("Error querying for table %q: %v", table, err)
		}
	}

	var count int

	err := DB.QueryRow("SELECT COUNT(1) FROM users WHERE id = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Error querying default user: %v", err)
	}
	if count == 0 {
		t.Error("Default user (id=1) was not seeded")
	}

	err = DB.QueryRow("SELECT COUNT(1) FROM profiles WHERE id = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Error querying default profile: %v", err)
	}
	if count == 0 {
		t.Error("Default profile (id=1) was not seeded")
	}

	err = DB.QueryRow("SELECT COUNT(1) FROM settings WHERE profile_id = 1 AND setting_key = 'theme'").Scan(&count)
	if err != nil {
		t.Fatalf("Error querying theme setting: %v", err)
	}
	if count == 0 {
		t.Error("Default theme setting was not seeded")
	}
}
