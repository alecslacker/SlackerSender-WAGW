// Bootstrap admin pertama lewat CLI: `./server make-admin <email>`.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/Aldi1963/wagataway/internal/database/models"
	"gorm.io/gorm"
)

// ErrUserNotFound dikembalikan PromoteToAdmin bila email belum terdaftar.
var ErrUserNotFound = errors.New("user tidak ditemukan")

// PromoteToAdmin menaikkan akun ber-email tertentu menjadi admin.
//
// Dipakai sekali per instalasi untuk admin pertama — setelah itu admin lain
// dikelola dari panel Admin. Sengaja hanya lewat CLI (butuh akses server),
// bukan otomatis "pendaftar pertama = admin": registrasi tidak memverifikasi
// email, sehingga aturan otomatis bisa direbut orang yang mendaftar duluan.
//
// alreadyAdmin=true berarti akun memang sudah admin (tidak ada yang diubah).
// Tidak perlu login ulang: role dibaca langsung dari database tiap request.
func PromoteToAdmin(db *gorm.DB, email string) (user models.User, alreadyAdmin bool, err error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return user, false, errors.New("email wajib diisi")
	}
	// Email dicocokkan tanpa membedakan huruf besar/kecil.
	if err = db.Where("LOWER(email) = LOWER(?)", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return user, false, ErrUserNotFound
		}
		return user, false, fmt.Errorf("gagal membaca user: %w", err)
	}
	if user.Role == "admin" {
		return user, true, nil
	}
	if err = db.Model(&user).Update("role", "admin").Error; err != nil {
		return user, false, fmt.Errorf("gagal menaikkan role: %w", err)
	}
	user.Role = "admin"
	return user, false, nil
}
