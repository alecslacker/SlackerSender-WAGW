package database

import (
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/config"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
)

// TestMigrateLegacyAIHost: koneksi/setting yang masih menunjuk ai.clipku.com
// dipindah ke default config; data lain tidak disentuh; idempoten.
func TestMigrateLegacyAIHost(t *testing.T) {
	db := migrateTestDB(t)
	if err := AutoMigrate(db); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	const flagKey = "ai_legacy_host_migrated_v1"
	cleanup := func() {
		db.Exec(`DELETE FROM ai_connections WHERE key_hint = 'zzmig'`)
		db.Exec(`DELETE FROM settings WHERE key IN (?, 'ai_chat_url')`, flagKey)
		db.Exec(`DELETE FROM users WHERE email LIKE 'zzmig-%@test.local'`)
	}
	cleanup()
	t.Cleanup(cleanup)

	mkUser := func(email string) uint {
		u := models.User{UUID: "00000000-0000-4000-8000-" + email[6:18], Name: "mig", Email: email, Password: "x"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("buat user: %v", err)
		}
		return u.ID
	}
	u1 := mkUser("zzmig-aaaaaaaaaaaa@test.local")
	u2 := mkUser("zzmig-bbbbbbbbbbbb@test.local")
	u3 := mkUser("zzmig-cccccccccccc@test.local")

	conns := []models.AIConnection{
		{UserID: u1, Provider: "9router", Model: "default", BaseURL: "https://ai.clipku.com/v1", APIKeyEnc: "k", KeyHint: "zzmig", IsActive: true},
		{UserID: u2, Provider: "custom", Model: "m", BaseURL: "https://AI.clipku.com/v1", APIKeyEnc: "k", KeyHint: "zzmig", IsActive: true},
		{UserID: u3, Provider: "custom", Model: "m", BaseURL: "https://llm.example.com/v1", APIKeyEnc: "k", KeyHint: "zzmig", IsActive: true},
	}
	for i := range conns {
		if err := db.Create(&conns[i]).Error; err != nil {
			t.Fatalf("buat koneksi: %v", err)
		}
	}
	if err := db.Create(&models.Setting{Key: "ai_chat_url", Value: "https://ai.clipku.com/v1/chat/completions"}).Error; err != nil {
		t.Fatalf("buat setting: %v", err)
	}

	for i := 0; i < 2; i++ { // jalan dua kali → idempoten
		if err := migrateLegacyAIHost(db); err != nil {
			t.Fatalf("migrasi #%d: %v", i+1, err)
		}
	}

	// Variabel baru tiap baca: GORM memakai primary key yang sudah terisi sebagai kondisi tambahan.
	load := func(id uint) models.AIConnection {
		var c models.AIConnection
		if err := db.First(&c, id).Error; err != nil {
			t.Fatalf("baca koneksi %d: %v", id, err)
		}
		return c
	}
	got := load(conns[0].ID)
	if got.BaseURL != "" || got.Model != config.AIDefaultModel() {
		t.Errorf("9router: base_url=%q model=%q, mau kosong & %q", got.BaseURL, got.Model, config.AIDefaultModel())
	}
	got = load(conns[1].ID)
	if got.BaseURL != config.AIDefaultBaseURL() {
		t.Errorf("custom legacy: base_url=%q, mau %q", got.BaseURL, config.AIDefaultBaseURL())
	}
	got = load(conns[2].ID)
	if got.BaseURL != "https://llm.example.com/v1" {
		t.Errorf("custom non-legacy berubah: %q", got.BaseURL)
	}
	var n int64
	db.Model(&models.Setting{}).Where("key = ?", "ai_chat_url").Count(&n)
	if n != 0 {
		t.Errorf("ai_chat_url lama harus terhapus, sisa %d", n)
	}
	db.Model(&models.Setting{}).Where("key = ?", flagKey).Count(&n)
	if n != 1 {
		t.Errorf("flag migrasi harus tercatat tepat 1x, dapat %d", n)
	}
}
