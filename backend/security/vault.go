package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log"

	"golang.org/x/crypto/argon2"
)

var sessionKey []byte

// InitVault automatically unlocks the vault with a static master password for portable mode
func InitVault() {
	staticPass := "AIStorageAdvisorStaticMasterPassword123!"
	staticSaltB64 := "c3RhdGljX3NhbHRfZm9yX3BvcnRhYmxlX21vZGU="
	
	salt, err := base64.StdEncoding.DecodeString(staticSaltB64)
	if err != nil {
		log.Printf("Error decoding static salt: %v", err)
		return
	}

	SetSessionPassword(staticPass, salt)
}

func SetSessionPassword(password string, salt []byte) {
	// Derive 32 bytes (256 bits) key using Argon2id
	// Python: time_cost=3, memory_cost=65536, parallelism=4, hash_len=32, type=Type.ID
	key := argon2.IDKey([]byte(password), salt, 3, 65536, 4, 32)
	sessionKey = key
	log.Println("Vault session key derived and loaded into memory.")
}

func ClearSession() {
	sessionKey = nil
	log.Println("Vault session key cleared from memory.")
}

func IsUnlocked() bool {
	return sessionKey != nil
}

// DeriveArchiveKey derives a 256-bit key from a password and salt using Argon2 for profile archive encryption.
func DeriveArchiveKey(password string, salt []byte) []byte {
	// Python: time_cost=2, memory_cost=32768, parallelism=2, hash_len=32, type=Type.ID
	return argon2.IDKey([]byte(password), salt, 2, 32768, 2, 32)
}

func Encrypt(plaintext string) (string, error) {
	if !IsUnlocked() {
		return "", errors.New("vault is locked. Session key not initialized")
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)
	combined := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(combined), nil
}

func Decrypt(ciphertextB64 string) (string, error) {
	if !IsUnlocked() {
		return "", errors.New("vault is locked. Session key not initialized")
	}

	combined, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}

	if len(combined) < 12 {
		return "", errors.New("invalid ciphertext length")
	}

	nonce := combined[:12]
	ciphertext := combined[12:]

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// EncryptArchive encrypts bytes with custom derived key using AES-GCM
func EncryptArchive(plaintext []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := aesgcm.Seal(nil, nonce, plaintext, nil)
	return append(nonce, ciphertext...), nil
}

// DecryptArchive decrypts bytes with custom derived key using AES-GCM
func DecryptArchive(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) < 12 {
		return nil, errors.New("ciphertext too short")
	}

	nonce := ciphertext[:12]
	actualCiphertext := ciphertext[12:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return aesgcm.Open(nil, nonce, actualCiphertext, nil)
}
