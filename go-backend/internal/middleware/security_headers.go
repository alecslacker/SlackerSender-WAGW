package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// ContentSecurityPolicy membatasi sumber resource yang boleh dimuat browser.
// Intinya: skrip, koneksi (fetch/SSE), dan font HANYA dari origin sendiri —
// jadi dependensi pihak luar tidak bisa menyelinap lewat commit baru tanpa
// sengaja diizinkan di sini.
//
// Pengecualian yang disengaja:
//   - img-src/media-src mengizinkan https: karena gambar katalog dan media chat
//     bisa berupa URL luar yang diisi pengguna (data, bukan kode).
//   - style-src 'unsafe-inline' karena splash boot & React memakai <style>/style=.
const ContentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self'; " +
	"style-src 'self' 'unsafe-inline'; " +
	"font-src 'self' data:; " +
	"img-src 'self' data: blob: https:; " +
	"media-src 'self' blob: https:; " +
	"connect-src 'self'; " +
	"object-src 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'; " +
	"frame-ancestors 'self'"

// SecurityHeaders menambahkan baseline security header ke setiap respons.
// CSP dipasang untuk halaman/aset, TIDAK untuk /api/ (respons JSON/PDF/stream;
// CSP ketat bisa mengganggu penampil PDF invoice).
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "SAMEORIGIN")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		if p := c.Request.URL.Path; p != "/api" && !strings.HasPrefix(p, "/api/") {
			c.Header("Content-Security-Policy", ContentSecurityPolicy)
		}
		c.Next()
	}
}
