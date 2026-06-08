package forecast

import (
	"database/sql"
	"math"
	"testing"
	"time"

	"aisadvisor/backend/db"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) {
	t.Helper()

	tmpDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open temp database: %v", err)
	}
	t.Cleanup(func() { tmpDB.Close() })

	queries := []string{
		`CREATE TABLE IF NOT EXISTS scan_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			scan_path TEXT NOT NULL,
			scan_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			total_size INTEGER NOT NULL,
			file_count INTEGER NOT NULL,
			metadata TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS forecast_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			profile_id INTEGER NOT NULL,
			forecast_time TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			predicted_days_to_full INTEGER,
			growth_rate_bytes_day INTEGER
		)`,
	}
	for _, q := range queries {
		if _, err := tmpDB.Exec(q); err != nil {
			t.Fatalf("Failed to create table: %v\nQuery: %s", err, q)
		}
	}

	origDB := db.DB
	db.DB = tmpDB
	t.Cleanup(func() { db.DB = origDB })
}

func insertScan(t *testing.T, profileID int, scanTime string, totalSize int64) {
	t.Helper()
	_, err := db.DB.Exec(
		`INSERT INTO scan_history (profile_id, scan_path, scan_time, total_size, file_count)
		 VALUES (?, ?, ?, ?, 0)`,
		profileID, "/test/path", scanTime, totalSize,
	)
	if err != nil {
		t.Fatalf("Failed to insert scan history: %v", err)
	}
}

func TestCalculateForecast_NilDB(t *testing.T) {
	origDB := db.DB
	db.DB = nil
	t.Cleanup(func() { db.DB = origDB })

	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when db.DB is nil")
		}
	}()

	CalculateForecast(1, "/test")
}

func TestCalculateForecast_InsufficientData_ZeroRecords(t *testing.T) {
	setupTestDB(t)

	result := CalculateForecast(1, t.TempDir())

	if result.Status != "insufficient_data" {
		t.Errorf("Expected status 'insufficient_data', got %q", result.Status)
	}
	if result.DaysRemaining != -1 {
		t.Errorf("Expected DaysRemaining -1, got %d", result.DaysRemaining)
	}
}

func TestCalculateForecast_InsufficientData_OneRecord(t *testing.T) {
	setupTestDB(t)

	insertScan(t, 1, "2025-01-01 12:00:00", 1000)

	result := CalculateForecast(1, t.TempDir())

	if result.Status != "insufficient_data" {
		t.Errorf("Expected status 'insufficient_data', got %q", result.Status)
	}
	if result.DaysRemaining != -1 {
		t.Errorf("Expected DaysRemaining -1, got %d", result.DaysRemaining)
	}
}

func TestCalculateForecast_StableGrowth(t *testing.T) {
	setupTestDB(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		scanTime := baseTime.AddDate(0, 0, i).Format("2006-01-02 15:04:05")
		totalSize := int64(i * 100 * 1024 * 1024)
		insertScan(t, 1, scanTime, totalSize)
	}

	result := CalculateForecast(1, t.TempDir())

	if result.Status != "normal_growth" && result.Status != "exhaustion_risk" {
		t.Errorf("Expected status 'normal_growth' or 'exhaustion_risk', got %q", result.Status)
	}
	if result.DaysRemaining < 0 {
		t.Errorf("Expected positive DaysRemaining for growth, got %d", result.DaysRemaining)
	}
	if result.DailyGrowthBytes <= 0 {
		t.Errorf("Expected positive DailyGrowthBytes, got %d", result.DailyGrowthBytes)
	}
	if len(result.TrendPoints) == 0 {
		t.Errorf("Expected non-empty TrendPoints")
	}
}

func TestCalculateForecast_TrendPoints(t *testing.T) {
	setupTestDB(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
	const mb = 1024 * 1024
	for i := 0; i < 3; i++ {
		scanTime := baseTime.AddDate(0, 0, i).Format("2006-01-02 15:04:05")
		totalSize := int64(i) * 100 * mb
		insertScan(t, 1, scanTime, totalSize)
	}

	result := CalculateForecast(1, t.TempDir())

	if len(result.TrendPoints) != 3 {
		t.Fatalf("Expected 3 TrendPoints, got %d", len(result.TrendPoints))
	}

	expectedSizes := []int64{0, 100 * mb, 200 * mb}
	for i, tp := range result.TrendPoints {
		if tp.ActualSize != expectedSizes[i] {
			t.Errorf("TrendPoints[%d].ActualSize = %d, want %d", i, tp.ActualSize, expectedSizes[i])
		}
		diff := math.Abs(tp.ProjectedSize - float64(expectedSizes[i]))
		if diff > 1.0 {
			t.Errorf("TrendPoints[%d].ProjectedSize = %.0f, expected ~%d (diff=%.0f)", i, tp.ProjectedSize, expectedSizes[i], diff)
		}
	}

	expectedDays := []float64{0, 1, 2}
	for i, tp := range result.TrendPoints {
		if math.Abs(tp.Days-expectedDays[i]) > 0.01 {
			t.Errorf("TrendPoints[%d].Days = %.2f, want %.0f", i, tp.Days, expectedDays[i])
		}
	}
}

func TestCalculateForecast_IgnoresOtherProfileData(t *testing.T) {
	setupTestDB(t)

	insertScan(t, 2, "2025-01-01 12:00:00", 1000)
	insertScan(t, 2, "2025-01-02 12:00:00", 2000)

	result := CalculateForecast(1, t.TempDir())
	if result.Status != "insufficient_data" {
		t.Errorf("Expected 'insufficient_data' when data exists for other profile, got %q", result.Status)
	}
}

func TestCalculateForecast_LogsForecastHistory(t *testing.T) {
	setupTestDB(t)

	baseTime := time.Date(2025, 1, 1, 12, 0, 0, 0, time.Local)
	for i := 0; i < 3; i++ {
		scanTime := baseTime.AddDate(0, 0, i).Format("2006-01-02 15:04:05")
		totalSize := int64(i * 100 * 1024 * 1024)
		insertScan(t, 1, scanTime, totalSize)
	}

	CalculateForecast(1, t.TempDir())

	var count int
	err := db.DB.QueryRow("SELECT COUNT(1) FROM forecast_history WHERE profile_id = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query forecast_history: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 forecast_history row, got %d", count)
	}

	var predictedDays sql.NullInt64
	var growthRate int64
	err = db.DB.QueryRow(
		"SELECT predicted_days_to_full, growth_rate_bytes_day FROM forecast_history WHERE profile_id = 1",
	).Scan(&predictedDays, &growthRate)
	if err != nil {
		t.Fatalf("Failed to read forecast_history: %v", err)
	}
	if !predictedDays.Valid {
		t.Error("Expected predicted_days_to_full to be valid")
	}
	if growthRate <= 0 {
		t.Errorf("Expected positive growth_rate_bytes_day, got %d", growthRate)
	}
}
