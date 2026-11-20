package whatsapp

// Baris waktu untuk system prompt AI Reply. Model tidak tahu jam/tanggal; tanpa
// ini ia menebak saat prompt meminta membedakan jam kerja dan di luar jam kerja.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

import (
	"fmt"
	"strings"
	"time"
)

// wib: UTC+7 tetap. Sengaja bukan time.LoadLocation("Asia/Jakarta") — bergantung
// pada tzdata di server; Indonesia tidak memakai DST sehingga offset tetap aman.
var wib = time.FixedZone("WIB", 7*60*60)

var (
	namaHari  = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}
	namaBulan = [...]string{"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
		"Juli", "Agustus", "September", "Oktober", "November", "Desember"}
)

// clockLine: "[WAKTU SEKARANG: Jumat, 20 November 2026, pukul 11.50 WIB]".
func clockLine(now time.Time) string {
	t := now.In(wib)
	return fmt.Sprintf("[WAKTU SEKARANG: %s, %d %s %d, pukul %02d.%02d WIB]",
		namaHari[t.Weekday()], t.Day(), namaBulan[t.Month()], t.Year(), t.Hour(), t.Minute())
}

// withClock menambahkan baris waktu di AKHIR system prompt, supaya awalan prompt
// tetap stabil (menjaga cache prompt provider) dan baris itu jadi hal terakhir
// yang dibaca model.
func withClock(system string, now time.Time) string {
	system = strings.TrimRight(system, " \t\r\n")
	if system == "" {
		return clockLine(now)
	}
	return system + "\n\n" + clockLine(now)
}
