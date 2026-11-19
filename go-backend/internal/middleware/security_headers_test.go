package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func cspFor(t *testing.T, path string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeaders())
	r.GET("/*any", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w.Header().Get("Content-Security-Policy")
}

// CSP dipasang di halaman/aset, tidak di /api/.
func TestSecurityHeadersCSPScope(t *testing.T) {
	for _, p := range []string{"/", "/dashboard", "/assets/x.js"} {
		if cspFor(t, p) == "" {
			t.Errorf("%s: CSP seharusnya terpasang", p)
		}
	}
	for _, p := range []string{"/api/health", "/api/billing/invoice.pdf"} {
		if got := cspFor(t, p); got != "" {
			t.Errorf("%s: CSP tidak boleh dipasang di /api/, dapat %q", p, got)
		}
	}
}

// Kunci keputusan: skrip/koneksi/font hanya 'self'; tidak ada host luar,
// tidak ada 'unsafe-inline'/'unsafe-eval' pada script-src.
func TestContentSecurityPolicyStaysStrict(t *testing.T) {
	directives := map[string]string{}
	for _, part := range strings.Split(ContentSecurityPolicy, ";") {
		part = strings.TrimSpace(part)
		if name, val, ok := strings.Cut(part, " "); ok {
			directives[name] = val
		}
	}
	for name, want := range map[string]string{
		"script-src":      "'self'",
		"connect-src":     "'self'",
		"default-src":     "'self'",
		"object-src":      "'none'",
		"frame-ancestors": "'self'",
	} {
		if directives[name] != want {
			t.Errorf("%s = %q, mau %q", name, directives[name], want)
		}
	}
	if strings.Contains(directives["font-src"], "http") {
		t.Errorf("font-src tidak boleh memuat host luar: %q", directives["font-src"])
	}
}
