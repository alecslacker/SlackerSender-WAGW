package handler

import (
	"os"
	"testing"
)

// TestMain menyetel secret enkripsi khusus tes SEBELUM tes mana pun berjalan.
// security.encryptionKey FAIL-CLOSED (tanpa secret, Encrypt menolak) dan di-cache
// sekali per proses (sync.Once), jadi harus ada sebelum pemanggilan pertama.
// Hanya bila belum disetel dari luar, agar tidak menimpa lingkungan CI.
func TestMain(m *testing.M) {
	if os.Getenv("AI_KEYS_SECRET") == "" && os.Getenv("JWT_SECRET") == "" {
		_ = os.Setenv("AI_KEYS_SECRET", "rahasia-khusus-tes-handler-bukan-produksi")
	}
	os.Exit(m.Run())
}
