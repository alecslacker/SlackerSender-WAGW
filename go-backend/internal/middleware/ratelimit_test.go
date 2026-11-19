package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// Penghitung harus reset setelah jendela berlalu, walau klien terus
// mengetuk selama diblokir.
func TestRateLimitResetDariAwalJendela(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(NewRateLimiter(2, 150*time.Millisecond).Middleware())
	r.GET("/x", func(c *gin.Context) { c.Status(http.StatusOK) })

	hit := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Real-IP", "1.2.3.4")
		r.ServeHTTP(w, req)
		return w.Code
	}

	if hit() != 200 || hit() != 200 || hit() != 429 {
		t.Fatal("batas 2 permintaan tidak ditegakkan")
	}
	// Terus mengetuk selama diblokir; paling lambat dua jendela kemudian
	// harus ada permintaan yang lolos lagi.
	for i := 0; i < 8; i++ {
		time.Sleep(40 * time.Millisecond)
		if hit() == 200 {
			return
		}
	}
	t.Fatal("penghitung tidak reset setelah jendela berlalu")
}
