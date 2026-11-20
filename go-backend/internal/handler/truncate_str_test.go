package handler

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// Ringkasan percakapan Live Chat dipotong 200 byte; emoji tak boleh terbelah
// (UTF-8 tidak valid ditolak PostgreSQL, SQLSTATE 22021).
func TestTruncateStrTidakMemotongEmoji(t *testing.T) {
	long := strings.Repeat("a", 198) + "🙏🙏" // emoji mulai di byte 198, batas 200 jatuh di tengahnya
	got := truncateStr(long, 200)
	if !utf8.ValidString(got) {
		t.Fatalf("UTF-8 tidak valid: %q", got)
	}
	if got != strings.Repeat("a", 198) {
		t.Errorf("hasil = %d byte, mau 198 'a' saja", len(got))
	}
	if truncateStr("abc", 10) != "abc" || truncateStr("abc", 0) != "" {
		t.Error("kasus dasar salah")
	}
	for max := 0; max <= len(long)+1; max++ {
		g := truncateStr(long, max)
		if !utf8.ValidString(g) || len(g) > max || !strings.HasPrefix(long, g) {
			t.Fatalf("max=%d: hasil tidak valid: %q", max, g)
		}
	}
}
