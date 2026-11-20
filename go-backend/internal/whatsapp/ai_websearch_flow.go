package whatsapp

// Alur "Cari di Web" untuk AI Reply: pencarian dijalankan SERVER lewat penyedia
// terpisah (tidak terikat model/provider AI), hasilnya disisipkan ke system prompt
// sebagai DATA tak tepercaya. Pelanggan tak pernah dibiarkan diam: pencarian gagal
// -> model tetap menjawab dengan catatan jujur.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"context"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const (
	// searchTimeout membatasi waktu pencarian; pelanggan menunggu jawaban.
	searchTimeout = 8 * time.Second
	// maxQueryRunes: kueri = teks pelanggan (dipangkas); pencarian tak perlu lebih panjang.
	maxQueryRunes = 120
)

// searchQueryFor menyusun kueri dari teks pelanggan: dirapatkan dan dipangkas.
// shortcut: kueri = teks pesan apa adanya (tanpa menyusun ulang lewat model) —
// hemat 1 panggilan, tapi pesan panjang/berbelit memberi kueri kurang tajam;
// naikkan ke penyusunan kueri oleh model bila hasil nyata terbukti buruk.
func searchQueryFor(text string) string {
	q := strings.Join(strings.Fields(text), " ")
	if r := []rune(q); len(r) > maxQueryRunes {
		q = string(r[:maxQueryRunes])
	}
	return q
}

// shouldSearch: pencarian hanya untuk pesan bercaption/teks (gambar tanpa caption
// tak punya kueri), bila config memintanya.
func shouldSearch(cfg *models.AIReplyConfig, text string) bool {
	return cfg != nil && cfg.WebSearch && strings.TrimSpace(text) != ""
}

// webSearchNote menghasilkan catatan sistem untuk satu pesan: blok hasil bila
// pencarian berhasil, catatan "tidak tersedia" bila gagal/ditolak batas, atau
// kosong ("") bila tak ada koneksi pencarian yang layak (pencarian bawaan provider
// rilis 23 tetap berlaku sebagai cadangan di askAIWithHistory).
// used=true bila blok hasil nyata dipakai (untuk log/biaya).
func webSearchNote(ctx context.Context, db *gorm.DB, guard *service.SearchGuard, searcherOf func(*models.AISearchConnection) service.Searcher,
	userID uint, cfg *models.AIReplyConfig, text string) (note string, used bool) {
	if !shouldSearch(cfg, text) {
		return "", false
	}
	conn := service.SearchConnectionForUser(db, userID)
	if conn == nil || !conn.SearchChecked() || conn.SearchStatus != models.CapabilityYes {
		// Tidak ada penyedia yang LULUS uji -> tak mencari lewat jalur server.
		return "", false
	}
	query := searchQueryFor(text)
	if query == "" {
		return "", false
	}
	if cached, ok := guard.Get(userID, conn.Provider, query); ok {
		return searchBlock(cached), true
	}
	if !guard.Allow(userID) {
		log.Warn().Uint("userID", userID).Msg("Cari di Web: batas pencarian per jam tercapai — dijawab tanpa web")
		return searchUnavailableNote, false
	}
	s := searcherOf(conn)
	if s == nil {
		log.Warn().Uint("userID", userID).Str("penyedia", conn.Provider).Msg("Cari di Web: penyedia tak bisa dibuat (kunci/konfigurasi) — dijawab tanpa web")
		return searchUnavailableNote, false
	}
	sctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()
	results, err := s.Search(sctx, query)
	if err != nil || len(results) == 0 {
		log.Warn().Err(err).Uint("userID", userID).Str("penyedia", conn.Provider).Msg("Cari di Web: pencarian gagal — dijawab tanpa web")
		return searchUnavailableNote, false
	}
	guard.Put(userID, conn.Provider, query, results)
	domains := make([]string, 0, len(results))
	for _, r := range results {
		domains = append(domains, r.Domain)
	}
	log.Info().Uint("userID", userID).Str("penyedia", conn.Provider).Int("hasil", len(results)).Strs("sumber", domains).Msg("AI Reply dengan Cari di Web (server)")
	return searchBlock(results), true
}
