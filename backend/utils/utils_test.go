package utils

import "testing"

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

func TestFormatSizeFractional(t *testing.T) {
	// 1.5 KB
	got := FormatSize(1536)
	if got != "1.50 KB" {
		t.Errorf("expected '1.50 KB', got %q", got)
	}

	// 2.75 MB
	got = FormatSize(2883584)
	if got != "2.75 MB" {
		t.Errorf("expected '2.75 MB', got %q", got)
	}
}

func TestFormatSizeEdgeCases(t *testing.T) {
	// Exactly 1 KB boundary
	got := FormatSize(1024)
	if got != "1.00 KB" {
		t.Errorf("expected '1.00 KB', got %q", got)
	}

	// Exactly 1 MB boundary
	got = FormatSize(1048576)
	if got != "1.00 MB" {
		t.Errorf("expected '1.00 MB', got %q", got)
	}
}

func TestFormatSizeNegative(t *testing.T) {
	// Negative should still produce some output (no crash)
	got := FormatSize(-1)
	if got == "" {
		t.Fatal("expected non-empty string for negative input")
	}
}
