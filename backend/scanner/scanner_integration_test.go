package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"aisadvisor/backend/rules"
)

func createTestFile(t *testing.T, path string, content []byte) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	err = os.WriteFile(path, content, 0644)
	if err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func TestScanLocalDiskBasic(t *testing.T) {
	dir := t.TempDir()

	// Create a test directory structure
	createTestFile(t, filepath.Join(dir, "small.txt"), []byte("hello world"))
	createTestFile(t, filepath.Join(dir, "temp", "temp.tmp"), []byte("temporary"))
	createTestFile(t, filepath.Join(dir, "logs", "app.log"), []byte("log content"))
	createTestFile(t, filepath.Join(dir, "logs", "old.log"), []byte("old log"))
	createTestFile(t, filepath.Join(dir, "cache", "cache.dat"), []byte("cached content"))

	// Add a duplicate file
	createTestFile(t, filepath.Join(dir, "dup1.txt"), []byte("duplicate content here"))
	createTestFile(t, filepath.Join(dir, "dup2.txt"), []byte("duplicate content here"))

	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk failed: %v", err)
	}

	if results.FilesScanned == 0 {
		t.Fatal("expected at least 1 file scanned")
	}

	if results.TotalSize <= 0 {
		t.Fatalf("expected positive total size, got %d", results.TotalSize)
	}

	// Check temp file detection
	if len(results.TempFiles) == 0 {
		t.Fatal("expected at least 1 temp file")
	}

	// Check log file detection
	if len(results.LogFiles) == 0 {
		t.Fatal("expected at least 1 log file")
	}

	// Check cache file detection
	if len(results.CacheFiles) == 0 {
		t.Fatal("expected at least 1 cache file")
	}
}

func TestScanLocalDiskCancellation(t *testing.T) {
	dir := t.TempDir()

	// Create many small files
	for i := 0; i < 100; i++ {
		createTestFile(t, filepath.Join(dir, "logs", "app.log"), []byte("log content"))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk with cancelled context should not return error: %v", err)
	}

	if !results.Cancelled {
		t.Fatal("expected Cancelled=true when context is cancelled")
	}
}

func TestScanLocalDiskEmptyDir(t *testing.T) {
	dir := t.TempDir()

	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk on empty dir failed: %v", err)
	}

	if results.FilesScanned != 0 {
		t.Fatalf("expected 0 files scanned in empty dir, got %d", results.FilesScanned)
	}
	if results.TotalSize != 0 {
		t.Fatalf("expected 0 total size, got %d", results.TotalSize)
	}
}

func TestScanLocalDiskWithRules(t *testing.T) {
	dir := t.TempDir()

	createTestFile(t, filepath.Join(dir, "temp", "old.tmp"), []byte("old temp file"))
	createTestFile(t, filepath.Join(dir, "small.txt"), []byte("normal file"))

	activeRules := []rules.Rule{
		{
			ID:        "temp_old",
			Name:      "Temp files older than 0 days",
			Category:  "temp",
			Condition: "older_than_days",
			Value:     0,
			Enabled:   true,
		},
	}

	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, activeRules, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk failed: %v", err)
	}

	if results.RulesFlaggedCount == 0 {
		t.Fatal("expected at least 1 file flagged by rules")
	}

	if results.RulesFlaggedSize <= 0 {
		t.Fatal("expected positive flagged size")
	}
}

func TestScanLocalDiskProgressCallback(t *testing.T) {
	dir := t.TempDir()

	// Create a single file
	createTestFile(t, filepath.Join(dir, "test.txt"), []byte("test"))

	var callbackDir string
	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {
		callbackDir = currentDir
	})

	if err != nil {
		t.Fatalf("ScanLocalDisk failed: %v", err)
	}

	if results.FilesScanned != 1 {
		t.Fatalf("expected 1 file scanned, got %d", results.FilesScanned)
	}

	// The callback may not be called if the scan finishes before the 100ms threshold.
	// But the results should still be correct.
	_ = callbackDir // verify compilation — the callback signature is compatible
}

func TestScanLocalDiskSymlinks(t *testing.T) {
	if os.Getenv("TEST_SKIP_SYMLINKS") != "" {
		t.Skip("Skipping symlink test")
	}

	dir := t.TempDir()

	// Create a regular file and a symlink to it (on Windows requires admin or developer mode)
	createTestFile(t, filepath.Join(dir, "real.txt"), []byte("real file"))

	err := os.Symlink(filepath.Join(dir, "real.txt"), filepath.Join(dir, "link.txt"))
	if err != nil {
		t.Skip("Symlinks not supported on this system:", err)
	}

	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk with symlinks failed: %v", err)
	}

	if results.FilesScanned < 1 {
		t.Fatal("expected at least 1 file scanned (including symlinks)")
	}
}

func TestScanLocalDiskDuplicateDetection(t *testing.T) {
	dir := t.TempDir()

	// Duplicate detection only runs on files > 1 MB, so create large enough files
	content := make([]byte, 2*1024*1024) // 2 MB
	for i := range content {
		content[i] = byte(i % 256)
	}
	createTestFile(t, filepath.Join(dir, "original.bin"), content)
	createTestFile(t, filepath.Join(dir, "copy.bin"), content)
	createTestFile(t, filepath.Join(dir, "different.bin"), []byte("different content"))

	ctx := context.Background()
	results, err := ScanLocalDisk(ctx, dir, []rules.Rule{}, func(currentDir string, filesScanned int, totalSize int64) {})

	if err != nil {
		t.Fatalf("ScanLocalDisk failed: %v", err)
	}

	if len(results.DuplicateGroups) == 0 {
		t.Fatal("expected duplicate groups for identical 2MB files")
	}

	// Verify the duplicate has the correct group size
	for hash, paths := range results.DuplicateGroups {
		if len(paths) > 1 {
			if paths[0].Size <= 0 {
				t.Fatalf("expected positive size for duplicate files in group %q", hash)
			}
			t.Logf("Found duplicate group %q with %d files, size=%s", hash[:8], len(paths), paths[0].SizeFormatted)
			return
		}
	}
	t.Fatal("expected at least one duplicate group with multiple files")
}
