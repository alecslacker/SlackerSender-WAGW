// Test CORS: origin yang diizinkan mengikuti URL publik (APP_URL).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/config"
	"github.com/gin-gonic/gin"
)

func corsHeaders(t *testing.T, origin string) http.Header {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS())
	r.GET("/x", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	r.ServeHTTP(w, req)
	return w.Header()
}

func TestCORSMengikutiURLPublik(t *testing.T) {
	t.Cleanup(func() { config.SetPublicURL("") })
	config.SetPublicURL("https://wasender.slackercode.us")

	// Origin aplikasi sendiri → diizinkan.
	if got := corsHeaders(t, "https://wasender.slackercode.us").Get("Access-Control-Allow-Origin"); got != "https://wasender.slackercode.us" {
		t.Fatalf("origin aplikasi sendiri: Allow-Origin=%q, mau diizinkan", got)
	}
	// Domain lama yang dulu di-hardcode → TIDAK lagi diizinkan.
	if got := corsHeaders(t, "https://wa.clipku.com").Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("domain lama diizinkan diam-diam: Allow-Origin=%q", got)
	}
	// Origin asing → ditolak.
	if got := corsHeaders(t, "https://jahat.example").Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("origin asing diizinkan: %q", got)
	}
	// Tanpa Origin (curl / server-to-server) → tetap lewat, tanpa header Allow-Origin.
	h := corsHeaders(t, "")
	if h.Get("Access-Control-Allow-Origin") != "" || h.Get("Access-Control-Allow-Credentials") != "true" {
		t.Fatalf("tanpa Origin: header tak sesuai: %v", h)
	}
}
