package whatsapp

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Memotong di tengah emoji menghasilkan UTF-8 tidak valid yang ditolak PostgreSQL
// (SQLSTATE 22021) — kasus produksi: ringkasan percakapan berisi 📷 / 🙏.
func TestTruncateTidakMemotongKarakterMultibyte(t *testing.T) {
	cases := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"pendek tak dipotong", "halo", 10, "halo"},
		{"ASCII tepat batas", "abcde", 5, "abcde"},
		{"ASCII dipotong", "abcdef", 3, "abc"},
		{"batas jatuh di tengah emoji 4 byte", "ab🙏cd", 3, "ab"},
		{"emoji muat tepat", "ab🙏cd", 6, "ab🙏"},
		{"batas 1 byte setelah awal emoji", "ab🙏cd", 4, "ab"},
		{"huruf 2 byte (é)", "caf\u00e9bar", 4, "caf"},
		{"batas nol", "abc", 0, ""},
		{"batas negatif", "abc", -1, ""},
		{"kosong", "", 5, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := truncate(c.in, c.max)
			if got != c.want {
				t.Errorf("truncate(%q,%d) = %q, mau %q", c.in, c.max, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("hasil bukan UTF-8 valid: %q", got)
			}
		})
	}
}

// Untuk semua titik potong: hasil selalu valid, tidak melebihi batas, dan awalan dari asal.
func TestTruncateSemuaTitikPotongValid(t *testing.T) {
	s := "Foto 📷 produk 🙏 terima kasih — selamat ✅ \u00e9\u4e2d\u6587"
	for max := 0; max <= len(s)+2; max++ {
		got := truncate(s, max)
		if !utf8.ValidString(got) {
			t.Fatalf("max=%d: UTF-8 tidak valid: %q", max, got)
		}
		if len(got) > max {
			t.Fatalf("max=%d: panjang %d melebihi batas", max, len(got))
		}
		if !strings.HasPrefix(s, got) {
			t.Fatalf("max=%d: %q bukan awalan asal", max, got)
		}
	}
}
