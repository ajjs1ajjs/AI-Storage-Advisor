package security

import (
	"testing"
)

func TestEncryptDecryptWithoutInit(t *testing.T) {
	// Ensure vault is locked for this test
	LockVault()

	_, err := Encrypt("test")
	if err == nil {
		t.Fatal("expected error when encrypting with locked vault")
	}

	_, err = Decrypt("dGVzdA==")
	if err == nil {
		t.Fatal("expected error when decrypting with locked vault")
	}
}

func TestIsUnlocked(t *testing.T) {
	LockVault()
	if IsUnlocked() {
		t.Fatal("expected vault to be locked after LockVault()")
	}
}

func TestDeriveArchiveKey(t *testing.T) {
	salt := []byte("test-salt-16-bytes!")
	key1 := DeriveArchiveKey("password123", salt)
	key2 := DeriveArchiveKey("password123", salt)
	key3 := DeriveArchiveKey("different", salt)

	if len(key1) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key1))
	}

	if string(key1) != string(key2) {
		t.Fatal("same password + salt should produce same key")
	}

	if string(key1) == string(key3) {
		t.Fatal("different passwords should produce different keys")
	}
}

func TestEncryptArchiveDecryptArchive(t *testing.T) {
	salt := []byte("test-archive-salt!")
	key := DeriveArchiveKey("archive-password", salt)
	original := []byte("sensitive profile data")

	encrypted, err := EncryptArchive(original, key)
	if err != nil {
		t.Fatalf("EncryptArchive failed: %v", err)
	}

	decrypted, err := DecryptArchive(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptArchive failed: %v", err)
	}

	if string(decrypted) != string(original) {
		t.Fatalf("round-trip mismatch: got %q, want %q", decrypted, original)
	}

	// Test with wrong key
	wrongKey := DeriveArchiveKey("wrong-password", salt)
	_, err = DecryptArchive(encrypted, wrongKey)
	if err == nil {
		t.Fatal("expected decryption with wrong key to fail")
	}
}

func TestEncryptArchiveDecryptArchiveEmpty(t *testing.T) {
	salt := []byte("another-test-salt")
	key := DeriveArchiveKey("password", salt)

	encrypted, err := EncryptArchive([]byte{}, key)
	if err != nil {
		t.Fatalf("EncryptArchive empty failed: %v", err)
	}

	decrypted, err := DecryptArchive(encrypted, key)
	if err != nil {
		t.Fatalf("DecryptArchive empty failed: %v", err)
	}

	if len(decrypted) != 0 {
		t.Fatalf("expected empty result, got %d bytes", len(decrypted))
	}
}

func TestDecryptArchiveInvalidInput(t *testing.T) {
	salt := []byte("test-salt-12345678")
	key := DeriveArchiveKey("password", salt)

	// Too short input
	_, err := DecryptArchive([]byte("short"), key)
	if err == nil {
		t.Fatal("expected error for input < 12 bytes")
	}

	// Invalid ciphertext
	_, err = DecryptArchive(make([]byte, 20), key)
	if err == nil {
		t.Fatal("expected error for invalid ciphertext")
	}
}

func TestEncryptArchiveKeySize(t *testing.T) {
	// AES requires 16, 24, or 32 byte keys. A 7-byte key should fail.
	shortKey := make([]byte, 7)
	_, err := EncryptArchive([]byte("data"), shortKey)
	if err == nil {
		t.Fatal("expected error with 7-byte key (invalid AES key size)")
	}
}

func TestDeriveArchiveKeySaltSize(t *testing.T) {
	salt := []byte("16bytes_salt!!!")
	key := DeriveArchiveKey("password", salt)
	if len(key) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(key))
	}
}


