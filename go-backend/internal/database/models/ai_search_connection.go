package models

import (
	"time"

	"gorm.io/gorm"
)

// Status hasil uji kemampuan (dipakai AIConnection.VisionStatus dan
// AISearchConnection.SearchStatus). Kosong = belum pernah dicek.
const (
	CapabilityYes = "yes"
	CapabilityNo  = "no"
)

// AISearchConnection adalah koneksi penyedia pencarian web milik user: pencarian
// dijalankan SERVER kita (bukan fitur bawaan model), hasilnya disisipkan ke prompt
// sebagai data. Karena itu pencarian tidak terikat model/provider AI mana pun.
// Satu user satu koneksi. Kunci terenkripsi seperti AIConnection.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
type AISearchConnection struct {
	ID        uint   `gorm:"primaryKey" json:"id"`
	UserID    uint   `gorm:"uniqueIndex;not null" json:"userId"`
	Provider  string `gorm:"size:20;not null" json:"provider"` // zai | brave | searxng
	BaseURL   string `gorm:"size:255" json:"baseUrl"`          // hanya searxng (instance milik sendiri)
	APIKeyEnc string `gorm:"type:text" json:"-"`               // terenkripsi AES-GCM; kosong untuk searxng tanpa kunci
	KeyHint   string `gorm:"size:12" json:"keyHint"`           // 4 karakter terakhir, untuk tampilan UI
	IsActive  bool   `gorm:"default:true" json:"isActive"`

	// Hasil uji sebelum dipakai. Berlaku HANYA bila CheckedProvider sama dengan
	// Provider sekarang — ganti penyedia = otomatis "belum dicek".
	SearchStatus    string     `gorm:"size:10" json:"searchStatus"`
	CheckedProvider string     `gorm:"size:20" json:"checkedProvider"`
	CheckedAt       *time.Time `json:"checkedAt"`

	CreatedAt time.Time      `json:"createdAt"`
	UpdatedAt time.Time      `json:"updatedAt"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	User User `gorm:"foreignKey:UserID" json:"-"`
}

// ResetVisionIfChanged mengosongkan hasil uji gambar bila yang diuji berubah:
// provider, base URL, model, atau kunci. Dipanggil SEBELUM nilai baru ditetapkan.
// Tanpa ini, status "yes"/"no" lama menempel pada backend yang tak pernah diuji
// (mis. ganti kunci/URL dengan nama model sama).
func (c *AIConnection) ResetVisionIfChanged(provider, model, baseURL string, keyChanged bool) {
	if c.Provider != provider || c.Model != model || c.BaseURL != baseURL || keyChanged {
		c.VisionStatus, c.VisionModel, c.VisionCheckedAt = "", "", nil
	}
}

// ResetSearchIfChanged: sama untuk hasil uji pencarian (penyedia, URL, atau kunci berubah).
func (c *AISearchConnection) ResetSearchIfChanged(provider, baseURL string, keyChanged bool) {
	if c.Provider != provider || c.BaseURL != baseURL || keyChanged {
		c.SearchStatus, c.CheckedProvider, c.CheckedAt = "", "", nil
	}
}

// VisionChecked: status vision berlaku bila model yang dicek == model sekarang.
func (c *AIConnection) VisionChecked() bool {
	return c != nil && c.VisionStatus != "" && c.VisionModel == c.Model
}

// SearchChecked: status pencarian berlaku bila penyedia yang dicek == penyedia sekarang.
func (c *AISearchConnection) SearchChecked() bool {
	return c != nil && c.SearchStatus != "" && c.CheckedProvider == c.Provider
}
