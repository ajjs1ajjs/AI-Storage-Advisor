package cleanup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0.00 B"},
		{500, "500.00 B"},
		{1024, "1.00 KB"},
		{1536, "1.50 KB"},
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

func TestDryRun(t *testing.T) {
	dir := t.TempDir()

	// Create a writable test file
	writablePath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(writablePath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	result := DryRun([]string{writablePath})

	if result.TotalCount != 1 {
		t.Fatalf("expected TotalCount=1, got %d", result.TotalCount)
	}
	if len(result.WritableFiles) != 1 {
		t.Fatalf("expected 1 writable file, got %d", len(result.WritableFiles))
	}
	if !result.CanProceed {
		t.Fatal("expected CanProceed=true")
	}
}

func TestDryRunNonExistentFile(t *testing.T) {
	result := DryRun([]string{filepath.Join(os.TempDir(), "nonexistent_file_xyz")})

	if result.TotalCount != 0 {
		t.Fatalf("expected TotalCount=0, got %d", result.TotalCount)
	}
	if result.CanProceed {
		t.Fatal("expected CanProceed=false for non-existent file")
	}
}

func TestSafeDeleteFile(t *testing.T) {
	profileID := 999
	dir := t.TempDir()
	filePath := filepath.Join(dir, "delete_me.txt")
	content := []byte("test data for deletion")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	size, err := SafeDeleteFile(profileID, filePath, false)
	if err != nil {
		t.Fatalf("SafeDeleteFile failed: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("expected size %d, got %d", len(content), size)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatal("expected file to be deleted")
	}
}

func TestSafeDeleteFileNonExistent(t *testing.T) {
	_, err := SafeDeleteFile(999, filepath.Join(os.TempDir(), "nonexistent_file"), false)
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}
