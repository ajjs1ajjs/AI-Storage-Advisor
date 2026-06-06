package scanner

import (
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
		{1125899906842624, "1.00 PB"},
	}

	for _, tt := range tests {
		got := FormatSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestShellQuote(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "'simple'"},
		{"/path/to/file", "'/path/to/file'"},
		{"/tmp/a'b; rm -rf /", "'/tmp/a'\"'\"'b; rm -rf /'"},
		{"", "''"},
		{"file with spaces", "'file with spaces'"},
	}

	for _, tt := range tests {
		got := ShellQuote(tt.input)
		if got != tt.want {
			t.Errorf("ShellQuote(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSortAndLimitFiles(t *testing.T) {
	files := []FileInfo{
		{Path: "/a", Size: 100},
		{Path: "/b", Size: 300},
		{Path: "/c", Size: 200},
		{Path: "/d", Size: 50},
	}

	// Test sorting
	result := sortAndLimitFiles(files, 4)
	if len(result) != 4 {
		t.Fatalf("expected 4 files, got %d", len(result))
	}
	if result[0].Size != 300 || result[1].Size != 200 || result[2].Size != 100 || result[3].Size != 50 {
		t.Fatal("files not sorted by size descending")
	}

	// Test limiting
	result = sortAndLimitFiles(files, 2)
	if len(result) != 2 {
		t.Fatalf("expected 2 files, got %d", len(result))
	}
	if result[0].Size != 300 || result[1].Size != 200 {
		t.Fatal("expected top 2 largest files")
	}

	// Test empty
	result = sortAndLimitFiles([]FileInfo{}, 10)
	if result == nil || len(result) != 0 {
		t.Fatal("expected empty result for empty input")
	}
}

func TestParseMD5SumLine(t *testing.T) {
	tests := []struct {
		line     string
		wantHash string
		wantPath string
		wantOk   bool
	}{
		{"d41d8cd98f00b204e9800998ecf8427e  /tmp/file", "d41d8cd98f00b204e9800998ecf8427e", "/tmp/file", true},
		{"d41d8cd98f00b204e9800998ecf8427e *binary_file", "d41d8cd98f00b204e9800998ecf8427e", "binary_file", true},
		{"d41d8cd98f00b204e9800998ecf8427e  /tmp/file with spaces.log", "d41d8cd98f00b204e9800998ecf8427e", "/tmp/file with spaces.log", true},
		{"", "", "", false},
		{"tooshort", "", "", false},
		{"d41d8cd98f00b204e9800998ecf8427e", "", "", false},
	}

	for _, tt := range tests {
		hash, path, ok := parseMD5SumLine(tt.line)
		if ok != tt.wantOk {
			t.Errorf("parseMD5SumLine(%q) ok = %v, want %v", tt.line, ok, tt.wantOk)
		}
		if hash != tt.wantHash {
			t.Errorf("parseMD5SumLine(%q) hash = %q, want %q", tt.line, hash, tt.wantHash)
		}
		if path != tt.wantPath {
			t.Errorf("parseMD5SumLine(%q) path = %q, want %q", tt.line, path, tt.wantPath)
		}
	}
}

func TestShellQuoteEscapesSingleQuotes(t *testing.T) {
	got := shellQuote("/tmp/a'b; rm -rf /")
	want := "'/tmp/a'\"'\"'b; rm -rf /'"

	if got != want {
		t.Fatalf("shellQuote() = %q, want %q", got, want)
	}
}

func TestParseMD5SumLineKeepsPathsWithSpaces(t *testing.T) {
	hash, path, ok := parseMD5SumLine("d41d8cd98f00b204e9800998ecf8427e  /tmp/file with spaces.log")
	if !ok {
		t.Fatal("expected md5sum line to parse")
	}
	if hash != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatalf("hash = %q", hash)
	}
	if path != "/tmp/file with spaces.log" {
		t.Fatalf("path = %q", path)
	}
}
