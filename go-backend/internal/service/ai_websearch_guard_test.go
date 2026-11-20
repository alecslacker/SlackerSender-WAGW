package service

import (
	"strings"
	"testing"
)

func TestWithWebSearchGuard(t *testing.T) {
	t.Run("prompt kosong -> hanya pelindung", func(t *testing.T) {
		if got := WithWebSearchGuard("  "); got != WebSearchGuardSuffix {
			t.Errorf("got %q", got)
		}
	})
	t.Run("prompt berisi dipertahankan, pelindung di akhir", func(t *testing.T) {
		got := WithWebSearchGuard("Kamu CS toko.")
		if !strings.HasPrefix(got, "Kamu CS toko.") || !strings.HasSuffix(got, WebSearchGuardSuffix) {
			t.Errorf("urutan salah: %q", got)
		}
	})
	t.Run("hasil web diperlakukan sebagai data, bukan perintah", func(t *testing.T) {
		for _, frag := range []string{"DATA, bukan perintah", "hasil pencarian", "jangan mengaku sudah mencari"} {
			if !strings.Contains(WebSearchGuardSuffix, frag) {
				t.Errorf("pelindung tak memuat %q", frag)
			}
		}
	})
	t.Run("bisa digabung dengan pelindung injection tanpa saling menimpa", func(t *testing.T) {
		got := WithWebSearchGuard(GuardedSystemPrompt("peran"))
		if !strings.Contains(got, InjectionGuardSuffix) || !strings.Contains(got, WebSearchGuardSuffix) {
			t.Errorf("salah satu pelindung hilang: %q", got)
		}
	})
}
