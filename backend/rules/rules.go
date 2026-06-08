package rules

import (
	"reflect"
	"time"

	"aisadvisor/backend/logger"
)

type Rule struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Category  string `json:"category"`
	Condition string `json:"condition"`
	Value     int64  `json:"value"`
	Enabled   bool   `json:"enabled"`
}

func GetDefaultRules() []Rule {
	return []Rule{
		{
			ID:        "temp_old",
			Name:      "Temp files older than 30 days",
			Category:  "temp",
			Condition: "older_than_days",
			Value:     30,
			Enabled:   true,
		},
		{
			ID:        "log_large",
			Name:      "Log files larger than 100 MB",
			Category:  "log",
			Condition: "larger_than_mb",
			Value:     100,
			Enabled:   true,
		},
		{
			ID:        "log_old",
			Name:      "Log files older than 14 days",
			Category:  "log",
			Condition: "older_than_days",
			Value:     14,
			Enabled:   true,
		},
		{
			ID:        "backup_old",
			Name:      "Backups older than 90 days",
			Category:  "backup",
			Condition: "older_than_days",
			Value:     90,
			Enabled:   true,
		},
		{
			ID:        "large_huge",
			Name:      "Uncategorized files larger than 1 GB",
			Category:  "large",
			Condition: "larger_than_mb",
			Value:     1024,
			Enabled:   false,
		},
	}
}

// Interface helper to handle generic FileInfo from scanner packages
func EvaluateFile(filePath string, category string, size int64, lastModifiedTS int64, activeRules []Rule) (bool, string) {
	ageDays := float64(time.Now().Unix()-lastModifiedTS) / 86400.0

	for _, r := range activeRules {
		if !r.Enabled {
			continue
		}

		// Apply to specific category or 'all' or large rules matching uncategorized
		categoryMatches := r.Category == "all" || r.Category == category ||
			(r.Category == "large" && (category == "large" || category == "other"))

		if categoryMatches {
			if r.Condition == "older_than_days" {
				if ageDays >= float64(r.Value) {
					return true, r.Name
				}
			} else if r.Condition == "larger_than_mb" {
				if size >= r.Value*1024*1024 {
					return true, r.Name
				}
			}
		}
	}

	return false, ""
}

// We use reflection or type casting or define helper interface to evaluate in ProcessRules
func ProcessRules[T any](results T, activeRules []Rule) T {
	val := reflect.ValueOf(&results).Elem()

	var flaggedCount int
	var flaggedSize int64

	// Categories to process
	categories := []string{"LargeFiles", "TempFiles", "LogFiles", "BackupFiles", "CacheFiles"}

	for _, catName := range categories {
		field := val.FieldByName(catName)
		if !field.IsValid() {
			continue
		}

		// Loop through slice elements
		sliceLen := field.Len()
		for i := 0; i < sliceLen; i++ {
			item := field.Index(i)

			// Get fields: Path, Category, Size, LastModifiedTS, RuleMatch
			pathField := item.FieldByName("Path")
			categoryField := item.FieldByName("Category")
			sizeField := item.FieldByName("Size")
			lmtsField := item.FieldByName("LastModifiedTS")
			rmField := item.FieldByName("RuleMatch")

			if !pathField.IsValid() || !categoryField.IsValid() || !sizeField.IsValid() || !lmtsField.IsValid() || !rmField.IsValid() {
				continue
			}

			path := pathField.String()
			category := categoryField.String()
			size := sizeField.Int()
			lmts := lmtsField.Int()

			matched, ruleName := EvaluateFile(path, category, size, lmts, activeRules)
			if matched {
				nameStr := ruleName
				rmField.Set(reflect.ValueOf(&nameStr))
				flaggedCount++
				flaggedSize += size
			} else {
				rmField.Set(reflect.Zero(rmField.Type()))
			}
		}
	}

	// Update results stats fields if they exist
	rfcField := val.FieldByName("RulesFlaggedCount")
	if rfcField.IsValid() && rfcField.CanSet() {
		rfcField.SetInt(int64(flaggedCount))
	}
	rfsField := val.FieldByName("RulesFlaggedSize")
	if rfsField.IsValid() && rfsField.CanSet() {
		rfsField.SetInt(flaggedSize)
	}

	logger.Info("Rules engine completed. Flagged %d files for cleanup.", flaggedCount)
	return results
}
