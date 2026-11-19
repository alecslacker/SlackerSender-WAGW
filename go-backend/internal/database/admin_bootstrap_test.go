// Test PromoteToAdmin (perintah `./server make-admin <email>`).
// DB-backed — skip bila DATABASE_URL tidak tersedia (lihat migrateTestDB).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package database

import (
	"errors"
	"testing"

	"github.com/Aldi1963/wagataway/internal/database/models"
)

const promoteTestEmail = "promote-admin@example.test"

func TestPromoteToAdmin(t *testing.T) {
	db := migrateTestDB(t)
	if err := db.AutoMigrate(&models.User{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	cleanup := func() { db.Unscoped().Where("email = ?", promoteTestEmail).Delete(&models.User{}) }
	cleanup()
	t.Cleanup(cleanup)

	seed := models.User{Name: "Calon Admin", Email: promoteTestEmail, Role: "user", Status: "active"}
	if err := db.Create(&seed).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	// 1. Email dicocokkan tanpa membedakan huruf besar/kecil + spasi dipangkas.
	u, already, err := PromoteToAdmin(db, "  PROMOTE-Admin@Example.TEST ")
	if err != nil || already || u.Role != "admin" {
		t.Fatalf("promosi pertama: user=%+v already=%v err=%v", u, already, err)
	}
	var dariDB models.User
	db.First(&dariDB, seed.ID)
	if dariDB.Role != "admin" {
		t.Fatalf("role di database %q, mau admin", dariDB.Role)
	}

	// 2. Idempoten: kedua kali tidak mengubah apa pun dan melaporkan alreadyAdmin.
	if _, already, err = PromoteToAdmin(db, promoteTestEmail); err != nil || !already {
		t.Fatalf("promosi kedua: already=%v err=%v, mau already=true", already, err)
	}

	// 3. Email tak terdaftar → ErrUserNotFound (pesan jelas di CLI).
	if _, _, err = PromoteToAdmin(db, "tidak-ada@example.test"); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("email asing: err=%v, mau ErrUserNotFound", err)
	}

	// 4. Email kosong ditolak sebelum menyentuh DB.
	if _, _, err = PromoteToAdmin(db, "   "); err == nil {
		t.Fatal("email kosong harus ditolak")
	}
}
