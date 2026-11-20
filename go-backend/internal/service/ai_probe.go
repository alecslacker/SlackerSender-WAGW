package service

// Uji kemampuan membaca gambar (vision) SEBELUM dipakai: model diberi gambar PNG
// 128x128 berisi dua pita warna (atas/bawah) acak, lalu ditanya warnanya. Dianggap
// mendukung hanya bila KEDUA warna disebut benar pada posisinya — menangkap proxy
// yang diam-diam membuang gambar (tebakan buta = 1/12). Tidak terikat model/API:
// berlaku untuk semua provider lewat jalur Complete yang sama dengan pemakaian nyata.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"regexp"
	"strings"
)

// Status hasil probe. "unknown" = belum bisa disimpulkan (galat jaringan, 429, 5xx,
// kunci ditolak, jawaban kosong) dan TIDAK boleh disimpan sebagai "tidak mendukung".
const ProbeUnknown = "unknown"

// probeColors: nama kanonik (Indonesia) + warna piksel yang jelas berbeda.
var probeColors = []struct {
	Name string
	RGBA color.RGBA
}{
	{"merah", color.RGBA{R: 220, G: 30, B: 30, A: 255}},
	{"biru", color.RGBA{R: 30, G: 60, B: 220, A: 255}},
	{"hijau", color.RGBA{R: 30, G: 160, B: 60, A: 255}},
	{"kuning", color.RGBA{R: 240, G: 210, B: 20, A: 255}},
}

// colorAlias: kata warna (ID + EN) -> nama kanonik.
var colorAlias = map[string]string{
	"merah": "merah", "red": "merah",
	"biru": "biru", "blue": "biru",
	"hijau": "hijau", "green": "hijau",
	"kuning": "kuning", "yellow": "kuning",
}

const probeImageSize = 128

// probeImagePNG membuat PNG dua pita: separuh atas warna top, separuh bawah bottom.
func probeImagePNG(top, bottom color.RGBA) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, probeImageSize, probeImageSize))
	for y := 0; y < probeImageSize; y++ {
		c := top
		if y >= probeImageSize/2 {
			c = bottom
		}
		for x := 0; x < probeImageSize; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

var (
	labelTop    = regexp.MustCompile(`\b(atas|top)\b`)
	labelBottom = regexp.MustCompile(`\b(bawah|bottom)\b`)
	colorWord   = regexp.MustCompile(`\b(merah|red|biru|blue|hijau|green|kuning|yellow)\b`)
)

// colorAfter: warna pertama yang muncul dalam 30 karakter setelah kemunculan
// pertama label. Menerima format tetap ("ATAS=merah") maupun kalimat bebas
// ("bagian atas berwarna merah"). Kosong bila tak ditemukan.
func colorAfter(lower string, label *regexp.Regexp) string {
	loc := label.FindStringIndex(lower)
	if loc == nil {
		return ""
	}
	end := loc[1] + 30
	if end > len(lower) {
		end = len(lower)
	}
	if m := colorWord.FindString(lower[loc[1]:end]); m != "" {
		return colorAlias[m]
	}
	return ""
}

// parseProbeAnswer mengambil warna atas & bawah dari jawaban model.
func parseProbeAnswer(answer string) (top, bottom string) {
	lower := strings.ToLower(answer)
	return colorAfter(lower, labelTop), colorAfter(lower, labelBottom)
}

// ProbeResult: hasil satu uji kemampuan. Status = "yes" | "no" | ProbeUnknown.
type ProbeResult struct {
	Status string
	Detail string // penjelasan singkat untuk UI/log (tanpa isi rahasia)
}

// probePrompt memaksa format yang mudah diperiksa, tanpa membocorkan jawabannya.
const probePrompt = "Lihat gambar ini. Gambar terdiri dari dua pita warna: separuh ATAS dan separuh BAWAH. " +
	"Sebutkan warna masing-masing. Jawab PERSIS dengan format: ATAS=<warna>, BAWAH=<warna>. " +
	"Pilih warna dari: merah, biru, hijau, kuning."

// classifyProbeError: galat dari provider -> hasil. Hanya penolakan eksplisit
// request (HTTP 400/415/422) yang berarti "model tak menerima gambar"; sisanya
// (jaringan, timeout, 401/403/404, 429, 5xx) belum bisa disimpulkan.
func classifyProbeError(err error) ProbeResult {
	msg := err.Error()
	rejected := isBadRequest(err) || strings.Contains(msg, "error 415") || strings.Contains(msg, "error 422")
	// 400 juga muncul untuk kunci salah, saldo habis, atau nama model salah — itu
	// BUKAN bukti model tak bisa melihat gambar. "no" hanya bila galat itu sendiri
	// menyebut gambar/vision; selain itu belum bisa disimpulkan.
	if rejected && mentionsImage(msg) {
		return ProbeResult{Status: "no", Detail: "provider menolak input gambar (" + shortErr(msg) + ")"}
	}
	if rejected {
		return ProbeResult{Status: ProbeUnknown, Detail: "provider menolak permintaan (" + shortErr(msg) + ") — periksa kunci, saldo, atau nama model"}
	}
	return ProbeResult{Status: ProbeUnknown, Detail: shortErr(msg)}
}

// imageHints: kata di badan galat yang menandakan penolakan karena GAMBAR.
var imageHints = []string{"image", "vision", "multimodal", "multi-modal", "media type", "media_type",
	"content type", "content_type", "modalit", "gambar", "visual", "image_url"}

func mentionsImage(msg string) bool {
	l := strings.ToLower(msg)
	for _, h := range imageHints {
		if strings.Contains(l, h) {
			return true
		}
	}
	return false
}

// refusalHints: jawaban yang terang-terangan menyatakan tak bisa melihat gambar.
var refusalHints = []string{"tidak bisa melihat", "tidak dapat melihat", "tidak bisa membaca gambar",
	"tidak dapat membaca gambar", "tidak ada gambar", "gambar tidak", "cannot see", "can't see",
	"cannot view", "can't view", "unable to see", "unable to view", "no image", "do not see an image",
	"don't see an image", "not able to see", "cannot process image", "text-only", "text only"}

func isRefusal(answer string) bool {
	l := strings.ToLower(answer)
	for _, h := range refusalHints {
		if strings.Contains(l, h) {
			return true
		}
	}
	return false
}

// shortErr memotong pesan galat provider (bisa panjang / memuat isi respons).
func shortErr(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > 160 {
		return string(r[:160]) + "…"
	}
	return s
}

// probeVisionAnswer menilai jawaban model terhadap warna yang benar.
func probeVisionAnswer(answer, wantTop, wantBottom string) ProbeResult {
	if strings.TrimSpace(answer) == "" {
		return ProbeResult{Status: ProbeUnknown, Detail: "model menjawab kosong"}
	}
	top, bottom := parseProbeAnswer(answer)
	if top == wantTop && bottom == wantBottom {
		return ProbeResult{Status: "yes", Detail: "model menyebut warna atas dan bawah dengan benar"}
	}
	// "no" hanya bila ada bukti: model menyebut KEDUA warna tetapi salah (menebak),
	// atau terang-terangan mengaku tak bisa melihat. Jawaban yang tak bisa diurai
	// (bertele-tele, mengulang soal) belum cukup untuk mematikan fitur gambar.
	if top != "" && bottom != "" {
		return ProbeResult{Status: "no", Detail: "model tidak melihat gambar (menjawab warna yang salah)"}
	}
	if isRefusal(answer) {
		return ProbeResult{Status: "no", Detail: "model menyatakan tidak bisa melihat gambar"}
	}
	return ProbeResult{Status: ProbeUnknown, Detail: "jawaban model tak bisa dinilai — coba cek ulang"}
}

// ProbeVision menguji apakah koneksi (provider+model) benar-benar bisa membaca
// gambar. pick(n) mengembalikan indeks acak [0,n) — disuntik agar bisa dites.
// Memakai jalur Complete yang sama dengan balasan nyata (klien anti-SSRF, format
// per provider), jadi hasilnya mewakili pemakaian sebenarnya.
func (s *AIService) ProbeVision(provider AIProvider, model string, pick func(n int) int) ProbeResult {
	ti := pick(len(probeColors))
	bi := pick(len(probeColors) - 1)
	if bi >= ti { // dua warna pasti berbeda
		bi++
	}
	top, bottom := probeColors[ti], probeColors[bi]
	data, err := probeImagePNG(top.RGBA, bottom.RGBA)
	if err != nil {
		return ProbeResult{Status: ProbeUnknown, Detail: "gagal membuat gambar uji"}
	}
	resp, err := s.Complete(ChatRequest{
		Provider:  provider,
		Model:     model,
		MaxTokens: AIReplyMaxTokens, // model reasoning perlu ruang berpikir
		Messages: []ChatMessage{{
			Role: "user", Content: probePrompt,
			Images: []ChatImage{{MIME: "image/png", Data: data}},
		}},
	})
	if err != nil {
		return classifyProbeError(err)
	}
	return probeVisionAnswer(resp.Content, top.Name, bottom.Name)
}
