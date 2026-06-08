package scanner

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkFormatSize(b *testing.B) {
	sizes := []int64{0, 512, 1024, 1536, 1048576, 1073741824, 1099511627776}
	for i := 0; i < b.N; i++ {
		FormatSize(sizes[i%len(sizes)])
	}
}

func BenchmarkShellQuote(b *testing.B) {
	paths := []string{
		"/simple/path/file.txt",
		"/path with spaces/file.txt",
		"/path/with/single'quote/file.txt",
		"/path/with/multiple  spaces  and'symbols/file'name.txt",
	}
	for i := 0; i < b.N; i++ {
		ShellQuote(paths[i%len(paths)])
	}
}

func BenchmarkGetFastHash(b *testing.B) {
	dir := b.TempDir()
	filePath := filepath.Join(dir, "bench.dat")
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getFastHash(ctx, filePath)
	}
}

func BenchmarkParseMD5SumLine(b *testing.B) {
	lines := []string{
		"d41d8cd98f00b204e9800998ecf8427e  file.txt",
		"d41d8cd98f00b204e9800998ecf8427e /path/to/file with spaces.txt",
		"d41d8cd98f00b204e9800998ecf8427e  /path/with/long/extension/file.name.with.dots.txt",
	}
	for i := 0; i < b.N; i++ {
		parseMD5SumLine(lines[i%len(lines)])
	}
}
