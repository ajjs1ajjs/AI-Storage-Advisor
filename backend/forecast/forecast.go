package forecast

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"aisadvisor/backend/db"
	"aisadvisor/backend/logger"

	"github.com/shirou/gopsutil/v3/disk"
)

type TrendPoint struct {
	Days          float64 `json:"days"`
	ScanTime      string  `json:"scan_time"`
	ActualSize    int64   `json:"actual_size"`
	ProjectedSize float64 `json:"projected_size"`
}

type ForecastResult struct {
	Status           string       `json:"status"` // "insufficient_data", "stable", "exhaustion_risk", "normal_growth"
	Message          string       `json:"message"`
	DaysRemaining    int          `json:"days_remaining"`
	DailyGrowthBytes int64        `json:"daily_growth_bytes"`
	FreeBytes        uint64       `json:"free_bytes"`
	TotalBytes       uint64       `json:"total_bytes"`
	TrendPoints      []TrendPoint `json:"trend_points"`
}

func CalculateForecast(profileID int, scanPath string) ForecastResult {
	// Query chronological scan history
	rows, err := db.DB.Query(
		"SELECT scan_time, total_size FROM scan_history WHERE profile_id = ? ORDER BY scan_time ASC",
		profileID,
	)
	if err != nil {
		logger.Error("Error fetching scan history: %v", err)
		return ForecastResult{
			Status:        "error",
			Message:       "Failed to fetch scan history: " + err.Error(),
			DaysRemaining: -1,
		}
	}
	defer rows.Close()

	type rawPoint struct {
		scanTime  string
		totalSize int64
	}
	var rawPoints []rawPoint

	for rows.Next() {
		var rp rawPoint
		if err := rows.Scan(&rp.scanTime, &rp.totalSize); err == nil {
			rawPoints = append(rawPoints, rp)
		}
	}

	if len(rawPoints) < 2 {
		return ForecastResult{
			Status:        "insufficient_data",
			Message:       "Needs at least 2 scan history data points to project storage trends.",
			DaysRemaining: -1,
		}
	}

	// Convert scan_time string to Unix epoch seconds
	type processedPoint struct {
		days      float64
		totalSize int64
		scanTime  string
	}
	var points []processedPoint
	var firstTime float64

	timeFormats := []string{
		"2006-01-02 15:04:05",
		time.RFC3339,
	}

	for i, rp := range rawPoints {
		var t time.Time
		var parseErr error
		for _, f := range timeFormats {
			t, parseErr = time.ParseInLocation(f, rp.scanTime, time.Local)
			if parseErr == nil {
				break
			}
		}
		if parseErr != nil {
			logger.Warn("Failed to parse scan history timestamp %s: %v", rp.scanTime, parseErr)
			continue
		}

		ts := float64(t.Unix())
		if i == 0 {
			firstTime = ts
		}

		days := (ts - firstTime) / 86400.0
		points = append(points, processedPoint{
			days:      days,
			totalSize: rp.totalSize,
			scanTime:  rp.scanTime,
		})
	}

	if len(points) < 2 {
		return ForecastResult{
			Status:        "insufficient_data",
			Message:       "Insufficient valid chronological data points.",
			DaysRemaining: -1,
		}
	}

	// Ordinary Least Squares Linear Regression
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	for _, p := range points {
		sumX += p.days
		sumY += float64(p.totalSize)
		sumXY += p.days * float64(p.totalSize)
		sumX2 += p.days * p.days
	}

	denominator := (n * sumX2) - (sumX * sumX)
	var slope float64
	if math.Abs(denominator) < 1e-6 {
		// Scans happened at the same time
		first, last := points[0], points[len(points)-1]
		timeDeltaDays := last.days - first.days
		if timeDeltaDays > 1e-4 {
			slope = float64(last.totalSize-first.totalSize) / timeDeltaDays
		} else {
			slope = 0
		}
	} else {
		slope = ((n * sumXY) - (sumX * sumY)) / denominator
	}

	dailyGrowth := slope // bytes per day

	// Get disk stats
	var freeBytes uint64
	var totalBytes uint64
	usage, err := disk.Usage(scanPath)
	if err == nil {
		freeBytes = usage.Free
		totalBytes = usage.Total
	} else {
		logger.Warn("Failed to check disk usage on %s: %v", scanPath, err)
	}

	daysRemaining := -1
	status := "stable"
	message := "Storage consumption is stable or shrinking."

	if dailyGrowth > 0 {
		if freeBytes > 0 {
			daysRemaining = int(float64(freeBytes) / dailyGrowth)
			if daysRemaining < 90 {
				status = "exhaustion_risk"
			} else {
				status = "normal_growth"
			}
			message = fmt.Sprintf("Disk will be exhausted in estimated %d days.", daysRemaining)
		}
	}

	// Log forecast to database
	_, errDb := db.DB.Exec(
		"INSERT INTO forecast_history (profile_id, predicted_days_to_full, growth_rate_bytes_day) VALUES (?, ?, ?)",
		profileID, sql.NullInt64{Int64: int64(daysRemaining), Valid: daysRemaining != -1}, int64(dailyGrowth),
	)
	if errDb != nil {
		logger.Warn("Failed to insert forecast history to DB: %v", errDb)
	}

	// Compute trend points for graph
	yIntercept := (sumY - slope*sumX) / n
	trendPoints := make([]TrendPoint, len(points))
	for i, p := range points {
		projected := yIntercept + slope*p.days
		trendPoints[i] = TrendPoint{
			Days:          p.days,
			ScanTime:      p.scanTime,
			ActualSize:    p.totalSize,
			ProjectedSize: projected,
		}
	}

	return ForecastResult{
		Status:           status,
		Message:          message,
		DaysRemaining:    daysRemaining,
		DailyGrowthBytes: int64(dailyGrowth),
		FreeBytes:        freeBytes,
		TotalBytes:       totalBytes,
		TrendPoints:      trendPoints,
	}
}
