package security

import (
	"testing"

	"aisadvisor/backend/config"
	"aisadvisor/backend/db"
)

func BenchmarkEncrypt(b *testing.B) {
	config.DbPath = ":memory:"
	db.InitDB()
	LockVault()
	if err := InitializeVault("bench-password"); err != nil {
		b.Fatal(err)
	}

	payloads := []string{
		"",
		"short",
		"a medium length string with some data",
		`{"json": "payload", "nested": {"key": "value"}, "array": [1,2,3,4,5]}`,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Encrypt(payloads[i%len(payloads)])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecrypt(b *testing.B) {
	config.DbPath = ":memory:"
	db.InitDB()
	LockVault()
	if err := InitializeVault("bench-password"); err != nil {
		b.Fatal(err)
	}

	payload := "benchmark payload for decrypt test"
	encrypted, err := Encrypt(payload)
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Decrypt(encrypted)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncryptParallel(b *testing.B) {
	config.DbPath = ":memory:"
	db.InitDB()
	LockVault()
	if err := InitializeVault("bench-password"); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := Encrypt("parallel encrypt test")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
