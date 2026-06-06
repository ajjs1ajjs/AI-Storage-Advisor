package scanner

import "testing"

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
