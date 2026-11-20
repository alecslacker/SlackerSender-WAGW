package whatsapp

import (
	"strings"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/service"
)

func TestClockLineKonversiWIB(t *testing.T) {
	cases := []struct {
		nama string
		in   time.Time
		want string
	}{
		// UTC 04:50 = 11.50 WIB
		{"UTC ke WIB", time.Date(2026, 11, 20, 4, 50, 0, 0, time.UTC),
			"[WAKTU SEKARANG: Jumat, 20 November 2026, pukul 11.50 WIB]"},
		// UTC 20:00 hari Kamis = 03.00 WIB hari JUMAT (tanggal ikut maju)
		{"tanggal maju melewati tengah malam", time.Date(2026, 11, 19, 20, 0, 0, 0, time.UTC),
			"[WAKTU SEKARANG: Jumat, 20 November 2026, pukul 03.00 WIB]"},
		// zona lain tetap dikonversi ke WIB
		{"zona non-UTC", time.Date(2026, 11, 20, 9, 5, 0, 0, time.FixedZone("X", 9*3600)),
			"[WAKTU SEKARANG: Jumat, 20 November 2026, pukul 07.05 WIB]"},
		{"Minggu", time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC),
			"[WAKTU SEKARANG: Minggu, 22 November 2026, pukul 10.00 WIB]"},
		{"Januari, jam satu digit", time.Date(2027, 1, 4, 18, 9, 0, 0, time.UTC),
			"[WAKTU SEKARANG: Selasa, 5 Januari 2027, pukul 01.09 WIB]"},
	}
	for _, c := range cases {
		t.Run(c.nama, func(t *testing.T) {
			if got := clockLine(c.in); got != c.want {
				t.Errorf("clockLine = %q, mau %q", got, c.want)
			}
		})
	}
}

// Semua hari & bulan terisi (cegah indeks meleset / string kosong).
func TestNamaHariBulanLengkap(t *testing.T) {
	for i, h := range namaHari {
		if h == "" {
			t.Errorf("hari %d kosong", i)
		}
	}
	for i := 1; i <= 12; i++ {
		if namaBulan[i] == "" {
			t.Errorf("bulan %d kosong", i)
		}
	}
}

// Pemasangan baris waktu ke pesan LLM — inti fitur; tanpa tes ini, menghapus
// pemanggilan withClock tak akan gagal di mana pun.
func TestAssembleMessagesMemasangBarisWaktuDiSystem(t *testing.T) {
	now := time.Date(2026, 11, 20, 4, 50, 0, 0, time.UTC)
	img := service.ChatImage{MIME: "image/png", Data: []byte{1}}
	hist := []service.ChatMessage{
		{Role: "user", Content: "halo"},
		{Role: "assistant", Content: "hai"},
		{Role: "assistant", Content: "ada lagi?"}, // assistant di ujung dibuang normalizer
	}
	msgs := assembleMessages("Kamu Duta.", hist, "pesan terbaru", []service.ChatImage{img}, now)

	if len(msgs) != 3 {
		t.Fatalf("mau system+2 riwayat... dapat %d pesan: %+v", len(msgs), msgs)
	}
	if msgs[0].Role != "system" || !strings.HasSuffix(msgs[0].Content, clockLine(now)) ||
		!strings.HasPrefix(msgs[0].Content, "Kamu Duta.") {
		t.Errorf("system harus berawalan prompt & berakhiran baris waktu: %q", msgs[0].Content)
	}
	n := 0
	for _, m := range msgs {
		if m.Role == "system" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("system harus tepat 1, dapat %d", n)
	}
	last := msgs[len(msgs)-1]
	if last.Role != "user" || last.Content != "pesan terbaru" || len(last.Images) != 1 {
		t.Errorf("pesan terakhir harus user + gambar: %+v", last)
	}
	// baris waktu TIDAK bocor ke pesan user/riwayat
	for _, m := range msgs[1:] {
		if strings.Contains(m.Content, "WAKTU SEKARANG") {
			t.Errorf("baris waktu bocor ke %s: %q", m.Role, m.Content)
		}
	}
}

func TestWithClock(t *testing.T) {
	now := time.Date(2026, 11, 20, 4, 50, 0, 0, time.UTC)
	t.Run("di akhir, awalan prompt tak berubah", func(t *testing.T) {
		got := withClock("Kamu Duta.\n", now)
		if !strings.HasPrefix(got, "Kamu Duta.") || !strings.HasSuffix(got, clockLine(now)) {
			t.Errorf("urutan salah: %q", got)
		}
		if strings.Count(got, "[WAKTU SEKARANG") != 1 {
			t.Errorf("baris waktu harus tepat 1: %q", got)
		}
	})
	t.Run("prompt kosong -> hanya baris waktu", func(t *testing.T) {
		if got := withClock("  \n", now); got != clockLine(now) {
			t.Errorf("got %q", got)
		}
	})
	t.Run("waktu sama untuk panggilan sama (ulangan konsisten)", func(t *testing.T) {
		if withClock("x", now) != withClock("x", now) {
			t.Error("harus deterministik untuk now yang sama")
		}
	})
}
