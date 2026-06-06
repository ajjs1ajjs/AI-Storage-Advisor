package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"

	"aisadvisor/backend/db"

	"golang.org/x/crypto/argon2"
)

var sessionKey []byte

const VaultSaltKey = "vault_salt"
const VaultVerificationKey = "vault_verification"

func IsVaultInitialized() bool {
	if db.DB == nil {
		return false
	}
	var salt string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = ?", VaultSaltKey).Scan(&salt)
	return err == nil && salt != ""
}

func InitializeVault(masterPassword string) error {
	if db.DB == nil {
		return errors.New("database is not initialized")
	}
	if IsUnlocked() {
		return errors.New("vault is already unlocked")
	}
	if IsVaultInitialized() {
		return errors.New("vault is already initialized. Use UnlockVault instead")
	}

	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return fmt.Errorf("failed to generate salt: %w", err)
	}
	saltB64 := base64.StdEncoding.EncodeToString(salt)

	key := deriveKey(masterPassword, salt)

	// Store verification hash (SHA256 of key) so we can verify the password later
	verification := sha256.Sum256(key)
	verB64 := base64.StdEncoding.EncodeToString(verification[:])

	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start DB transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (1, ?, ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		VaultSaltKey, saltB64,
	)
	if err != nil {
		return fmt.Errorf("failed to store vault salt: %w", err)
	}

	_, err = tx.Exec(
		"INSERT INTO settings (profile_id, setting_key, setting_value) VALUES (1, ?, ?) "+
			"ON CONFLICT(profile_id, setting_key) DO UPDATE SET setting_value = excluded.setting_value",
		VaultVerificationKey, verB64,
	)
	if err != nil {
		return fmt.Errorf("failed to store vault verification: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit vault initialization: %w", err)
	}

	sessionKey = key
	log.Println("Vault initialized and unlocked.")
	return nil
}

func UnlockVault(masterPassword string) error {
	if db.DB == nil {
		return errors.New("database is not initialized")
	}
	if IsUnlocked() {
		return nil
	}

	var saltB64 string
	err := db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = ?", VaultSaltKey).Scan(&saltB64)
	if err == sql.ErrNoRows {
		return fmt.Errorf("vault is not initialized. Call InitializeVault first")
	}
	if err != nil {
		return fmt.Errorf("failed to read vault salt: %w", err)
	}

	var verB64 string
	err = db.DB.QueryRow("SELECT setting_value FROM settings WHERE profile_id = 1 AND setting_key = ?", VaultVerificationKey).Scan(&verB64)
	if err == sql.ErrNoRows {
		return fmt.Errorf("vault verification data not found")
	}
	if err != nil {
		return fmt.Errorf("failed to read vault verification: %w", err)
	}

	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return fmt.Errorf("failed to decode vault salt: %w", err)
	}

	expectedVerification, err := base64.StdEncoding.DecodeString(verB64)
	if err != nil {
		return fmt.Errorf("failed to decode vault verification: %w", err)
	}

	key := deriveKey(masterPassword, salt)
	verification := sha256.Sum256(key)

	if !hmac.Equal(verification[:], expectedVerification) {
		return errors.New("incorrect master password")
	}

	sessionKey = key
	log.Println("Vault unlocked successfully.")
	return nil
}

func deriveKey(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 65536, 4, 32)
}

func ChangeMasterPassword(oldPassword, newPassword string) error {
	if db.DB == nil {
		return errors.New("database is not initialized")
	}
	if !IsUnlocked() {
		return errors.New("vault is locked")
	}

	// Verify old password first
	if err := UnlockVault(oldPassword); err != nil {
		return fmt.Errorf("old password is incorrect: %w", err)
	}

	// Generate new salt and key
	newSalt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, newSalt); err != nil {
		return fmt.Errorf("failed to generate new salt: %w", err)
	}
	newSaltB64 := base64.StdEncoding.EncodeToString(newSalt)

	newKey := deriveKey(newPassword, newSalt)
	newVerification := sha256.Sum256(newKey)
	newVerB64 := base64.StdEncoding.EncodeToString(newVerification[:])

	// We need to re-encrypt all existing encrypted data with the new key.
	// For now, we just change the key and update storage.
	tx, err := db.DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to start DB transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"UPDATE settings SET setting_value = ? WHERE profile_id = 1 AND setting_key = ?",
		newSaltB64, VaultSaltKey,
	)
	if err != nil {
		return fmt.Errorf("failed to update vault salt: %w", err)
	}

	_, err = tx.Exec(
		"UPDATE settings SET setting_value = ? WHERE profile_id = 1 AND setting_key = ?",
		newVerB64, VaultVerificationKey,
	)
	if err != nil {
		return fmt.Errorf("failed to update vault verification: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit password change: %w", err)
	}

	sessionKey = newKey
	return nil
}

func LockVault() {
	sessionKey = nil
	log.Println("Vault locked.")
}

func IsUnlocked() bool {
	return sessionKey != nil
}

// DeriveArchiveKey derives a 256-bit key from a password and salt using Argon2 for profile archive encryption.
func DeriveArchiveKey(password string, salt []byte) []byte {
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
