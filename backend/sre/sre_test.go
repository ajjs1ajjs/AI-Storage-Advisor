package sre

import (
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0.00 B"},
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{1099511627776, "1.00 TB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestParseDockerSize(t *testing.T) {
	tests := []struct {
		input     string
		wantWrite int64
		wantVirt  int64
	}{
		{"21MB (virtual 1.5GB)", 21 * 1024 * 1024, int64(1.5 * 1024 * 1024 * 1024)},
		{"500KB (virtual 2GB)", 500 * 1024, 2 * 1024 * 1024 * 1024},
		{"1GB (virtual 2GB)", 1 * 1024 * 1024 * 1024, 2 * 1024 * 1024 * 1024},
		{"100MB", 100 * 1024 * 1024, 0},
		{"invalid", 0, 0},
		{"", 0, 0},
	}

	for _, tt := range tests {
		w, v := ParseDockerSize(tt.input)
		if w != tt.wantWrite {
			t.Errorf("ParseDockerSize(%q) write = %d, want %d", tt.input, w, tt.wantWrite)
		}
		if v != tt.wantVirt {
			t.Errorf("ParseDockerSize(%q) virt = %d, want %d", tt.input, v, tt.wantVirt)
		}
	}
}

func TestCalculateHealthScore(t *testing.T) {
	emptyReport := SreReport{
		DockerActive:  false,
		Containers:    make([]ContainerInfo, 0),
		Volumes:       make([]VolumeInfo, 0),
		WindowsActive: false,
		Folders:       make(map[string]FolderInfo),
	}

	// Perfect score: no issues
	score, warnings := CalculateHealthScore(0, -1, 0, 0, 0, emptyReport)
	if score != 100 {
		t.Fatalf("expected score 100 for clean state, got %d", score)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected 0 warnings for clean state, got %d: %v", len(warnings), warnings)
	}

	// Critical: storage exhaustion in < 30 days
	score, warnings = CalculateHealthScore(0, 15, 0, 0, 0, emptyReport)
	if score > 70 {
		t.Fatalf("expected score <= 70 for critical storage, got %d", score)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for critical storage")
	}

	// Warning: storage exhaustion in < 90 days
	score, _ = CalculateHealthScore(0, 60, 0, 0, 0, emptyReport)
	if score > 85 {
		t.Fatalf("expected score <= 85 for storage warning, got %d", score)
	}

	// Duplicate waste > 10 GB
	tenGB := int64(10 * 1024 * 1024 * 1024)
	score, warnings = CalculateHealthScore(0, -1, tenGB+1, 0, 0, emptyReport)
	if score > 85 {
		t.Fatalf("expected score <= 85 for 10GB+ duplicates, got %d", score)
	}
	if len(warnings) == 0 {
		t.Fatal("expected warnings for duplicate waste")
	}

	// Duplicate waste > 1 GB
	oneGB := int64(1 * 1024 * 1024 * 1024)
	score, _ = CalculateHealthScore(0, -1, oneGB+1, 0, 0, emptyReport)
	if score > 92 {
		t.Fatalf("expected score <= 92 for 1GB+ duplicates, got %d", score)
	}

	// Log files > 5 GB
	fiveGB := int64(5 * 1024 * 1024 * 1024)
	score, warnings = CalculateHealthScore(0, -1, 0, fiveGB+1, 0, emptyReport)
	if score > 85 {
		t.Fatalf("expected score <= 85 for 5GB+ logs, got %d", score)
	}

	// Log files > 500 MB
	fiveHundredMB := int64(500 * 1024 * 1024)
	score, _ = CalculateHealthScore(0, -1, 0, fiveHundredMB+1, 0, emptyReport)
	if score > 95 {
		t.Fatalf("expected score <= 95 for 500MB+ logs, got %d", score)
	}

	// Temp files > 5 GB
	score, warnings = CalculateHealthScore(0, -1, 0, 0, fiveGB+1, emptyReport)
	if score > 90 {
		t.Fatalf("expected score <= 90 for 5GB+ temp files, got %d", score)
	}

	// Docker containers with large write layers
	reportWithDocker := SreReport{
		DockerActive: true,
		Containers: []ContainerInfo{
			{Name: "big-container", WriteSize: 2 * 1024 * 1024 * 1024},
			{Name: "small-container", WriteSize: 100 * 1024 * 1024},
		},
		Volumes:       make([]VolumeInfo, 0),
		WindowsActive: false,
		Folders:       make(map[string]FolderInfo),
	}
	score, warnings = CalculateHealthScore(0, -1, 0, 0, 0, reportWithDocker)
	if score > 90 {
		t.Fatalf("expected score <= 90 for large docker containers, got %d", score)
	}

	// Windows minidumps > 500 MB
	reportWithWindows := SreReport{
		DockerActive:  false,
		Containers:    make([]ContainerInfo, 0),
		Volumes:       make([]VolumeInfo, 0),
		WindowsActive: true,
		Folders: map[string]FolderInfo{
			"minidumps": {Path: "C:\\Windows\\Minidump", Size: 1024 * 1024 * 1024},
		},
	}
	score, warnings = CalculateHealthScore(0, -1, 0, 0, 0, reportWithWindows)
	if score > 90 {
		t.Fatalf("expected score <= 90 for large minidumps, got %d", score)
	}

	// Score should never go below 0
	score, _ = CalculateHealthScore(0, 1, 100*1024*1024*1024, 100*1024*1024*1024, 100*1024*1024*1024, reportWithDocker)
	if score < 0 {
		t.Fatalf("expected score >= 0, got %d", score)
	}

	// Score should never exceed 100
	// Add with daysRemaining = -1 for no storage issue
	score, _ = CalculateHealthScore(0, -1, 0, 0, 0, emptyReport)
	if score > 100 {
		t.Fatalf("expected score <= 100, got %d", score)
	}
}
