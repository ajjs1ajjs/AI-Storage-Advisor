package forecast

import (
	"fmt"
	"testing"

	"aisadvisor/backend/config"
	"aisadvisor/backend/db"
)

func BenchmarkCalculateForecast(b *testing.B) {
	config.DbPath = ":memory:"
	db.InitDB()

	for i := 0; i < 1000; i++ {
		hours := fmt.Sprintf("-%d hours", i)
		totalSize := int64(1000000000 + i*100000)
		_, err := db.DB.Exec(
			"INSERT INTO scan_history (profile_id, scan_path, scan_time, total_size, file_count) VALUES (1, '/test/path', datetime('now', ?), ?, 0)",
			hours, totalSize,
		)
		if err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateForecast(1, "/test/path")
	}
}

func BenchmarkCalculateForecastEmptyDB(b *testing.B) {
	config.DbPath = ":memory:"
	db.InitDB()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateForecast(1, "/test/path")
	}
}
