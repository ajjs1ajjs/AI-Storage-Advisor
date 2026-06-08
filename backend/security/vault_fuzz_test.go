package security

import (
	"testing"

	"aisadvisor/backend/config"
	"aisadvisor/backend/db"
)

func initDB() {
	config.DbPath = ":memory:"
	db.InitDB()
}

func FuzzEncryptDecrypt(f *testing.F) {
	initDB()

	seeds := []string{
		"",
		"hello",
		"a",
		"short",
		"a longer string with spaces and special chars: !@#$%^&*()",
		"path/to/file.txt",
		"C:\\Users\\test\\file.txt",
		`{"json": "data", "nested": {"key": "value"}}`,
		"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQ...",
		"\x00\x01\x02\x7f\x80\xff",
		"   ",
		"\n\t\r",
	}

	LockVault()
	err := InitializeVault("fuzz-test-master-password")
	if err != nil {
		f.Fatal(err)
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, original string) {
		encrypted, err := Encrypt(original)
		if err != nil {
			t.Skipf("encrypt failed: %v", err)
		}

		decrypted, err := Decrypt(encrypted)
		if err != nil {
			t.Errorf("decrypt failed for encrypted(%q): %v", original, err)
		}

		if decrypted != original {
			t.Errorf("round-trip mismatch: got %q, want %q", decrypted, original)
		}
	})
}

func FuzzDecryptInvalid(f *testing.F) {
	initDB()

	seeds := []string{
		"",
		"invalid",
		"base64!invalid",
		"short",
		"\x00\x01\x02",
	}

	LockVault()
	err := InitializeVault("fuzz-test-master-password")
	if err != nil {
		f.Fatal(err)
	}

	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, ciphertext string) {
		result, err := Decrypt(ciphertext)
		if err == nil && result != "" {
			t.Logf("unexpected success decrypting %q -> %q", ciphertext, result)
		}
	})
}
