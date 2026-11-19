// Test alamat bind server HTTP (LISTEN_HOST + PORT).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package config

import "testing"

func TestListenAddr(t *testing.T) {
	cases := []struct {
		name, host, port, want string
	}{
		{"kosong = semua antarmuka (perilaku lama)", "", "8080", ":8080"},
		{"production di belakang nginx", "127.0.0.1", "8080", "127.0.0.1:8080"},
		{"spasi dipangkas", "  127.0.0.1  ", "9000", "127.0.0.1:9000"},
		{"IPv6 loopback diberi kurung otomatis", "::1", "8080", "[::1]:8080"},
		{"IPv6 sudah berkurung tidak jadi ganda", "[::1]", "8080", "[::1]:8080"},
	}
	for _, tc := range cases {
		c := &Config{ListenHost: tc.host, Port: tc.port}
		if got := c.ListenAddr(); got != tc.want {
			t.Errorf("%s: ListenAddr(host=%q, port=%q) = %q, mau %q", tc.name, tc.host, tc.port, got, tc.want)
		}
	}
}
