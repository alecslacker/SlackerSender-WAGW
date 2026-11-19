// Test role & status "live": role dibaca dari database tiap request, bukan dari
// salinan di token — promosi admin berlaku tanpa login ulang, penurunan role
// dan pemblokiran akun berlaku seketika.
//
// DB-backed (postgres wagataway_test; skip bila DATABASE_URL tidak tersedia).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/config"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/rls"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// liveTestDB membuka DB test wagataway_test (DSN dari DATABASE_URL, path diganti
// ke database test — tidak pernah menyentuh DB live). Skip bila tidak tersedia.
func liveTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL tidak tersedia — lewati test DB")
	}
	u, err := url.Parse(dsn)
	if err != nil {
		t.Skipf("DATABASE_URL tidak valid: %v", err)
	}
	u.Path = "/wagataway_test"
	db, err := gorm.Open(postgres.New(postgres.Config{DriverName: rls.DriverName, DSN: u.String()}), &gorm.Config{})
	if err != nil {
		t.Skipf("postgres test tidak terjangkau: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil || sqlDB.Ping() != nil {
		t.Skip("postgres test tidak terjangkau")
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	rls.Register(db)
	if err := db.AutoMigrate(&models.User{}, &models.ApiKey{}, &models.Session{}); err != nil {
		t.Fatalf("AutoMigrate: %v", err)
	}
	return db
}

const liveTestEmail = "live-role@example.test"

func seedLiveUser(t *testing.T, db *gorm.DB, role, status string) models.User {
	t.Helper()
	cleanup := func() {
		db.Unscoped().Where("user_id IN (?)", db.Model(&models.User{}).Select("id").Where("email = ?", liveTestEmail)).Delete(&models.ApiKey{})
		db.Unscoped().Where("user_id IN (?)", db.Model(&models.User{}).Select("id").Where("email = ?", liveTestEmail)).Delete(&models.Session{})
		db.Unscoped().Where("email = ?", liveTestEmail).Delete(&models.User{})
	}
	cleanup()
	t.Cleanup(cleanup)
	u := models.User{Name: "live-role", Email: liveTestEmail, Role: role, Status: status}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// jalankanAuth mengeksekusi AuthRequired + AdminRequired dengan token JWT yang
// membawa roleToken (salinan saat login) dan mengembalikan status HTTP-nya.
func jalankanAuth(t *testing.T, db *gorm.DB, u models.User, roleToken string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWTSecret: "rahasia-uji-live-role"}
	SetAPIKeyDB(db)

	claims := &Claims{
		UserID: u.ID, Email: u.Email, Role: roleToken,
		// Tanpa ID (jti) → pemeriksaan tabel sesi dilewati (token legacy);
		// yang diuji di sini murni role/status live.
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))},
	}
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		t.Fatalf("sign jwt: %v", err)
	}

	r := gin.New()
	r.GET("/admin-area", AuthRequired(cfg), AdminRequired(), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/admin-area", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	r.ServeHTTP(w, req)
	return w.Code
}

func TestLiveRolePromoteTanpaLoginUlang(t *testing.T) {
	db := liveTestDB(t)
	u := seedLiveUser(t, db, "user", "active")

	// Token terbit saat akun masih "user" → ditolak panel admin.
	if got := jalankanAuth(t, db, u, "user"); got != http.StatusForbidden {
		t.Fatalf("user biasa: status %d, mau 403", got)
	}
	// Dinaikkan di database; TOKEN YANG SAMA (role di token masih "user").
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Update("role", "admin").Error; err != nil {
		t.Fatalf("promote: %v", err)
	}
	if got := jalankanAuth(t, db, u, "user"); got != http.StatusOK {
		t.Fatalf("setelah dinaikkan, token lama harus langsung diterima: status %d, mau 200", got)
	}
}

func TestLiveRoleDemoteBerlakuSeketika(t *testing.T) {
	db := liveTestDB(t)
	u := seedLiveUser(t, db, "admin", "active")

	// Token lama masih mengaku "admin", DB juga admin → lolos.
	if got := jalankanAuth(t, db, u, "admin"); got != http.StatusOK {
		t.Fatalf("admin aktif: status %d, mau 200", got)
	}
	// Diturunkan di database; token lama tetap mengaku "admin".
	if err := db.Model(&models.User{}).Where("id = ?", u.ID).Update("role", "user").Error; err != nil {
		t.Fatalf("demote: %v", err)
	}
	if got := jalankanAuth(t, db, u, "admin"); got != http.StatusForbidden {
		t.Fatalf("admin yang diturunkan HARUS langsung ditolak (token basi): status %d, mau 403", got)
	}
}

func TestLiveRoleAkunDiblokirDitolak(t *testing.T) {
	db := liveTestDB(t)
	for _, status := range []string{"banned", "suspended"} {
		u := seedLiveUser(t, db, "admin", status)
		if got := jalankanAuth(t, db, u, "admin"); got != http.StatusUnauthorized {
			t.Fatalf("status %q: %d, mau 401 ACCOUNT_DISABLED", status, got)
		}
	}
}

func TestLiveRoleAkunDihapusDitolak(t *testing.T) {
	db := liveTestDB(t)
	u := seedLiveUser(t, db, "admin", "active")
	db.Unscoped().Where("id = ?", u.ID).Delete(&models.User{})
	if got := jalankanAuth(t, db, u, "admin"); got != http.StatusUnauthorized {
		t.Fatalf("akun sudah dihapus: %d, mau 401", got)
	}
}

// Sub-akun tim JANGAN mewarisi admin milik pemilik: roleToken "team_*" tetap
// dipertahankan walau pemilik (UserID di token) adalah admin.
func TestLiveRoleSubAkunTimTidakMewarisiAdmin(t *testing.T) {
	db := liveTestDB(t)
	owner := seedLiveUser(t, db, "admin", "active")
	for _, teamRole := range []string{"team_member", "team_admin", "team_viewer"} {
		if got := jalankanAuth(t, db, owner, teamRole); got != http.StatusForbidden {
			t.Fatalf("%s milik pemilik admin: %d, mau 403 (tidak boleh ke panel admin)", teamRole, got)
		}
	}
	// ... tetapi pemilik yang diblokir juga mematikan sub-akunnya.
	if err := db.Model(&models.User{}).Where("id = ?", owner.ID).Update("status", "banned").Error; err != nil {
		t.Fatalf("ban: %v", err)
	}
	if got := jalankanAuth(t, db, owner, "team_member"); got != http.StatusUnauthorized {
		t.Fatalf("pemilik di-ban → sub-akun tim harus ikut ditolak: %d, mau 401", got)
	}
}

func TestLiveRoleAPIKeyIkutStatusPemilik(t *testing.T) {
	db := liveTestDB(t)
	u := seedLiveUser(t, db, "user", "active")
	const raw = "kunci-live-role-uji"
	ak := models.ApiKey{UserID: u.ID, Name: "live-role-key", KeyHash: models.HashAPIKey(raw), IsActive: true}
	if err := db.Create(&ak).Error; err != nil {
		t.Fatalf("seed key: %v", err)
	}
	SetAPIKeyDB(db)
	gin.SetMode(gin.TestMode)
	cfg := &config.Config{JWTSecret: "x"}
	r := gin.New()
	r.GET("/p", AuthRequired(cfg), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"role": GetRole(c)}) })

	call := func() int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/p", nil)
		req.Header.Set("X-API-Key", raw)
		r.ServeHTTP(w, req)
		return w.Code
	}
	if got := call(); got != http.StatusOK {
		t.Fatalf("key milik user aktif: %d, mau 200", got)
	}
	db.Model(&models.User{}).Where("id = ?", u.ID).Update("status", "banned")
	if got := call(); got != http.StatusUnauthorized {
		t.Fatalf("key milik user yang di-ban: %d, mau 401", got)
	}
}
