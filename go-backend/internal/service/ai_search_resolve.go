package service

// Memuat koneksi pencarian milik user dan membuat Searcher-nya. Dipisah dari
// pipeline WhatsApp agar mudah dites (tanpa klien WhatsApp).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/security"
	"gorm.io/gorm"
)

// SearchConnectionForUser: koneksi pencarian AKTIF milik user, atau nil.
// Kunci tidak didekripsi di sini.
func SearchConnectionForUser(db *gorm.DB, userID uint) *models.AISearchConnection {
	var conn models.AISearchConnection
	if err := db.Where("user_id = ? AND is_active = ?", userID, true).First(&conn).Error; err != nil {
		return nil
	}
	return &conn
}

// SearcherForConnection membuat Searcher dari koneksi tersimpan. Mengembalikan
// nil bila kunci tak bisa didekripsi atau konfigurasi tak lengkap (pemanggil
// menjawab TANPA web, tidak diam).
func SearcherForConnection(conn *models.AISearchConnection) Searcher {
	if conn == nil {
		return nil
	}
	// Validasi ulang saat dipakai (bukan hanya saat simpan): DNS bisa berubah dan
	// data lama bisa lolos dari aturan yang lebih longgar dulu.
	if conn.BaseURL != "" {
		if err := security.ValidateOutboundURL(conn.BaseURL); err != nil {
			return nil
		}
	}
	key := ""
	if conn.APIKeyEnc != "" {
		k, err := security.Decrypt(conn.APIKeyEnc)
		if err != nil {
			return nil
		}
		key = k
	}
	s, err := NewSearcher(SearcherConfig{Provider: conn.Provider, APIKey: key, BaseURL: conn.BaseURL})
	if err != nil {
		return nil
	}
	return s
}
