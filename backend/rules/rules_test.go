package rules

import (
	"testing"
	"time"
)

func TestGetDefaultRules(t *testing.T) {
	rules := GetDefaultRules()
	if len(rules) == 0 {
		t.Fatal("expected at least one default rule")
	}

	foundTemp := false
	foundLog := false
	for _, r := range rules {
		if r.ID == "temp_old" {
			foundTemp = true
			if !r.Enabled {
				t.Fatal("temp_old rule should be enabled by default")
			}
			if r.Condition != "older_than_days" {
				t.Fatalf("temp_old condition = %q, want 'older_than_days'", r.Condition)
			}
		}
		if r.ID == "log_large" {
			foundLog = true
			if r.Category != "log" {
				t.Fatalf("log_large category = %q, want 'log'", r.Category)
			}
		}
	}

	if !foundTemp {
		t.Fatal("temp_old rule not found")
	}
	if !foundLog {
		t.Fatal("log_large rule not found")
	}
}

func TestEvaluateFileOlderThanDays(t *testing.T) {
	rules := []Rule{
		{
			ID:        "temp_old",
			Name:      "Temp files older than 30 days",
			Category:  "temp",
			Condition: "older_than_days",
			Value:     30,
			Enabled:   true,
		},
	}

	// File modified 60 days ago (guaranteed > 30 days)
	sixtyDaysAgo := time.Now().Unix() - 60*86400

	matched, name := EvaluateFile("/tmp/old.log", "temp", 1000, sixtyDaysAgo, rules)
	if !matched {
		t.Fatal("expected file 60 days old to match >= 30 days rule")
	}
	if name != "Temp files older than 30 days" {
		t.Fatalf("rule name = %q, want 'Temp files older than 30 days'", name)
	}

	// File modified 5 days ago (recent, should not match 30 day rule)
	fiveDaysAgo := time.Now().Unix() - 5*86400
	matched, _ = EvaluateFile("/tmp/recent.log", "temp", 1000, fiveDaysAgo, rules)
	if matched {
		t.Fatal("expected recent file to NOT match old file rule")
	}
}

func TestEvaluateFileLargerThanMB(t *testing.T) {
	rules := []Rule{
		{
			ID:        "log_large",
			Name:      "Log files larger than 100 MB",
			Category:  "log",
			Condition: "larger_than_mb",
			Value:     100,
			Enabled:   true,
		},
	}

	// 200 MB file
	matched, name := EvaluateFile("/var/log/app.log", "log", 200*1024*1024, 0, rules)
	if !matched {
		t.Fatal("expected 200 MB file to match > 100 MB rule")
	}
	if name != "Log files larger than 100 MB" {
		t.Fatalf("rule name = %q", name)
	}

	// 50 MB file — should NOT match
	matched, _ = EvaluateFile("/var/log/small.log", "log", 50*1024*1024, 0, rules)
	if matched {
		t.Fatal("expected 50 MB file to NOT match > 100 MB rule")
	}
}

func TestEvaluateFileDisabledRule(t *testing.T) {
	rules := []Rule{
		{
			ID:        "disabled_rule",
			Name:      "Disabled rule",
			Category:  "temp",
			Condition: "older_than_days",
			Value:     1,
			Enabled:   false,
		},
	}

	// File modified 100 days ago
	oldTs := int64(1600000000)
	matched, _ := EvaluateFile("/tmp/old.log", "temp", 1000, oldTs, rules)
	if matched {
		t.Fatal("expected disabled rule to NOT match")
	}
}

func TestEvaluateFileAllCategory(t *testing.T) {
	rules := []Rule{
		{
			ID:        "all_rule",
			Name:      "All category rule",
			Category:  "all",
			Condition: "older_than_days",
			Value:     100,
			Enabled:   true,
		},
	}

	oldTs := int64(1000000000)
	matched, _ := EvaluateFile("/any/file.txt", "other", 1000, oldTs, rules)
	if !matched {
		t.Fatal("expected 'all' category rule to match 'other' category")
	}

	matched, _ = EvaluateFile("/any/file.txt", "temp", 1000, oldTs, rules)
	if !matched {
		t.Fatal("expected 'all' category rule to match 'temp' category")
	}
}

func TestEvaluateFileNoMatch(t *testing.T) {
	rules := []Rule{
		{
			ID:        "temp_old",
			Name:      "Temp old",
			Category:  "temp",
			Condition: "older_than_days",
			Value:     30,
			Enabled:   true,
		},
	}

	now := int64(2000000000)
	matched, name := EvaluateFile("/var/log/syslog", "log", 1000, now, rules)
	if matched {
		t.Fatal("expected no match for log file with temp rules only")
	}
	if name != "" {
		t.Fatalf("expected empty name, got %q", name)
	}
}

func TestProcessRules(t *testing.T) {
	type TestFileInfo struct {
		Path           string
		Category       string
		Size           int64
		LastModifiedTS int64
		RuleMatch      *string
	}

	type TestResults struct {
		LargeFiles        []TestFileInfo
		RulesFlaggedCount int
		RulesFlaggedSize  int64
	}

	rules := []Rule{
		{
			ID:        "log_large",
			Name:      "Log files larger than 100 MB",
			Category:  "log",
			Condition: "larger_than_mb",
			Value:     100,
			Enabled:   true,
		},
	}

	results := TestResults{
		LargeFiles: []TestFileInfo{
			{Path: "/var/log/app.log", Category: "log", Size: 200 * 1024 * 1024, LastModifiedTS: 0},
			{Path: "/var/log/small.log", Category: "log", Size: 50 * 1024 * 1024, LastModifiedTS: 0},
		},
	}

	processed := ProcessRules(results, rules)

	if processed.RulesFlaggedCount != 1 {
		t.Fatalf("expected 1 flagged file, got %d", processed.RulesFlaggedCount)
	}
	if processed.RulesFlaggedSize != 200*1024*1024 {
		t.Fatalf("expected flagged size 200MB, got %d", processed.RulesFlaggedSize)
	}
	if processed.LargeFiles[0].RuleMatch == nil {
		t.Fatal("expected app.log to have RuleMatch set")
	}
	if processed.LargeFiles[1].RuleMatch != nil {
		t.Fatal("expected small.log to have RuleMatch nil")
	}
}
