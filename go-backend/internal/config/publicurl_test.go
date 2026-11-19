// Test URL publik: satu sumber (APP_URL) untuk semua tautan yang dikirim ke
// pelanggan, redirect OAuth, kop invoice, dan origin CORS.
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package config

import "testing"

func TestPublicURL(t *testing.T) {
	t.Cleanup(func() { SetPublicURL("") })
	cases := []struct {
		name, in, wantURL, wantHost string
	}{
		{"domain production", "https://wasender.slackercode.us", "https://wasender.slackercode.us", "wasender.slackercode.us"},
		{"garis miring akhir dibuang", "https://wasender.slackercode.us/", "https://wasender.slackercode.us", "wasender.slackercode.us"},
		{"spasi dipangkas", "  https://x.example.com//  ", "https://x.example.com", "x.example.com"},
		{"dengan port", "http://localhost:8080", "http://localhost:8080", "localhost:8080"},
		{"kosong → fallback localhost, BUKAN domain pihak lain", "", FallbackPublicURL, "localhost:8080"},
		{"hanya spasi → fallback", "   ", FallbackPublicURL, "localhost:8080"},
		{"ada path", "https://a.example.com/sub", "https://a.example.com/sub", "a.example.com"},
	}
	for _, tc := range cases {
		SetPublicURL(tc.in)
		if got := PublicURL(); got != tc.wantURL {
			t.Errorf("%s: PublicURL(%q) = %q, mau %q", tc.name, tc.in, got, tc.wantURL)
		}
		if got := PublicHost(); got != tc.wantHost {
			t.Errorf("%s: PublicHost(%q) = %q, mau %q", tc.name, tc.in, got, tc.wantHost)
		}
	}
}
