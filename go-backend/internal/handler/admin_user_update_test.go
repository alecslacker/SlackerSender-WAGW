package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Test edit user admin (temuan error-handling): adminUpdateUser dulu memakai
// db.Model(&user).Updates(req) dari map mentah TANPA cek error — kunci
// camelCase yang bukan nama kolom (notifyWa, twoFaEnabled) membuat UPDATE
// gagal total ("no such column") dan error-nya ditelan lalu dibalas 200:
// edit user adalah no-op diam-diam. Sekarang kunci dinormalisasi ke nama
// kolom DB (camelCase menang bila keduanya dikirim, kunci tak dikenal
// dibuang) dan error UPDATE dikembalikan, bukan 200.

func seedUserUpdateUser(t *testing.T, db *gorm.DB, email string) models.User {
	t.Helper()
	if err := db.AutoMigrate(&models.User{}, &models.AdminActivityLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Unscoped().Where("email = ?", email).Delete(&models.User{})
	u := models.User{
		Name: "User Uji", Email: email, Password: "hash-lama",
		Role: "user", Plan: "free", Status: "active", Timezone: "Asia/Jakarta",
	}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u
}

// ensurePlan memastikan paket berslug tertentu ada di DB test (edit paket
// user kini divalidasi ke tabel plans). Paket yang dibuat helper ini dihapus
// lagi saat test selesai; paket yang sudah ada dibiarkan apa adanya.
func ensurePlan(t *testing.T, db *gorm.DB, slug, name string) models.Plan {
	t.Helper()
	if err := db.AutoMigrate(&models.Plan{}, &models.Subscription{}); err != nil {
		t.Fatalf("migrate plan: %v", err)
	}
	var p models.Plan
	if err := db.Where("slug = ?", slug).First(&p).Error; err == nil {
		return p
	}
	p = models.Plan{Name: name, Slug: slug, Price: 100000, Duration: 30, IsActive: true}
	if err := db.Create(&p).Error; err != nil {
		t.Fatalf("seed paket %s: %v", slug, err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&models.Plan{}, p.ID) })
	return p
}

func deleteUserByEmail(t *testing.T, db *gorm.DB, email string) {
	t.Helper()
	db.Unscoped().Where("email = ?", email).Delete(&models.User{})
}

func userUpdateRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Meniru middleware JWT: menanam userID admin di konteks agar
	// logAdminAction (via middleware.GetUserID) tidak panic.
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(92001))
		c.Next()
	})
	r.PUT("/admin/users/:id", adminUpdateUser(db))
	return r
}

func callUserUpdate(t *testing.T, r *gin.Engine, id uint, body map[string]interface{}) (int, map[string]interface{}) {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/admin/users/%d", id), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	var parsed map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("respons bukan JSON (status %d): %s", w.Code, w.Body.String())
	}
	return w.Code, parsed
}

func readUser(t *testing.T, db *gorm.DB, id uint) models.User {
	t.Helper()
	var u models.User
	if err := db.First(&u, id).Error; err != nil {
		t.Fatalf("baca user: %v", err)
	}
	return u
}

func TestAdminUpdateUserCamelCase(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-camel@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)

	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{
		"notifyWa": "6281234567890", "twoFaEnabled": true,
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, mau 200", status)
	}
	got := readUser(t, db, u.ID)
	if got.NotifyWA != "6281234567890" {
		t.Errorf("notify_wa = %q, mau 6281234567890 (notifyWa tidak tersimpan)", got.NotifyWA)
	}
	if !got.TwoFAEnabled {
		t.Errorf("two_fa_enabled = false, mau true (twoFaEnabled tidak tersimpan)")
	}
}

func TestAdminUpdateUserFormFields(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-form@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)
	ensurePlan(t, db, "pro", "Pro")
	defer db.Where("user_id = ?", u.ID).Delete(&models.Subscription{})

	r := userUpdateRouter(db)
	// Bentuk body persis form admin (Admin.tsx updateField): satu kunci
	// sekali, role/plan/status.
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{"role": "admin"})
	if status != http.StatusOK {
		t.Fatalf("status role = %d, mau 200", status)
	}
	status, _ = callUserUpdate(t, r, u.ID, map[string]interface{}{"plan": "pro"})
	if status != http.StatusOK {
		t.Fatalf("status plan = %d, mau 200", status)
	}
	status, _ = callUserUpdate(t, r, u.ID, map[string]interface{}{"status": "suspended"})
	if status != http.StatusOK {
		t.Fatalf("status status = %d, mau 200", status)
	}
	got := readUser(t, db, u.ID)
	if got.Role != "admin" || got.Plan != "pro" || got.Status != "suspended" {
		t.Errorf("role/plan/status = %q/%q/%q, mau admin/pro/suspended", got.Role, got.Plan, got.Status)
	}
}

func TestAdminUpdateUserMixedSpellingCamelWins(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-dual@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)

	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{
		"notifyWa": "6281111111111", "notify_wa": "6282222222222",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, mau 200", status)
	}
	got := readUser(t, db, u.ID)
	if got.NotifyWA != "6281111111111" {
		t.Errorf("notify_wa = %q, mau 6281111111111 (camelCase harus menang)", got.NotifyWA)
	}
}

func TestAdminUpdateUserUnknownKeysIgnored(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-unknown@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)

	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{
		"role": "admin", "foo": "bar", "id": 999, "uuid": "bukan-uuid",
	})
	if status != http.StatusOK {
		t.Fatalf("status = %d, mau 200", status)
	}
	got := readUser(t, db, u.ID)
	if got.Role != "admin" {
		t.Errorf("role = %q, mau admin", got.Role)
	}
	if got.ID != u.ID {
		t.Errorf("id = %d, mau %d (id tidak boleh berubah)", got.ID, u.ID)
	}
}

func TestAdminUpdateUserPasswordNotChanged(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-pass@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)

	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{
		"password": "password-baru-123",
	})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400 (password saja bukan perubahan valid)", status)
	}
	got := readUser(t, db, u.ID)
	if got.Password != "hash-lama" {
		t.Errorf("password berubah (tidak boleh diubah lewat endpoint ini)")
	}
}

func TestAdminUpdateUserInvalidRole(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-role@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)

	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{"role": "superadmin"})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400", status)
	}
	got := readUser(t, db, u.ID)
	if got.Role != "user" {
		t.Errorf("role = %q, mau tetap user", got.Role)
	}
}

func TestAdminUpdateUserDuplicateEmail(t *testing.T) {
	db := auditTestDB(t)
	email1 := "uji-user-dup1@contoh.id"
	email2 := "uji-user-dup2@contoh.id"
	seedUserUpdateUser(t, db, email1)
	defer deleteUserByEmail(t, db, email1)
	u2 := seedUserUpdateUser(t, db, email2)
	defer deleteUserByEmail(t, db, email2)

	r := userUpdateRouter(db)
	status, res := callUserUpdate(t, r, u2.ID, map[string]interface{}{"email": email1})
	if status != http.StatusConflict {
		t.Fatalf("status = %d, mau 409 — body: %v", status, res)
	}
	got := readUser(t, db, u2.ID)
	if got.Email != email2 {
		t.Errorf("email = %q, mau tetap %q", got.Email, email2)
	}
}

func TestAdminUpdateUserNotFound(t *testing.T) {
	db := auditTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.AdminActivityLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := userUpdateRouter(db)
	status, _ := callUserUpdate(t, r, 999999999, map[string]interface{}{"role": "admin"})
	if status != http.StatusNotFound {
		t.Fatalf("status = %d, mau 404", status)
	}
}

// Regresi "Langganan dashboard tetap Lite setelah admin ganti paket ke Pro":
// dropdown paket admin dulu hanya mengubah label users.plan, sementara kartu
// Langganan, kuota, dan batas fitur dibaca dari tabel subscriptions. Sekarang
// ganti paket ikut mengganti langganan aktif (pola yang sama dengan aktivasi
// pembayaran).
func TestAdminUpdateUserPlanReplacesSubscription(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-plan-sub@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)
	defer db.Where("user_id = ?", u.ID).Delete(&models.Subscription{})
	lite := ensurePlan(t, db, "lite", "Lite")
	pro := ensurePlan(t, db, "pro", "Pro")

	now := time.Now()
	old := models.Subscription{
		UserID: u.ID, PlanID: lite.ID, Status: "active",
		StartDate: now.AddDate(0, 0, -2), EndDate: now.AddDate(0, 0, 5), IsTrial: true,
	}
	if err := db.Create(&old).Error; err != nil {
		t.Fatalf("seed langganan lite: %v", err)
	}

	r := userUpdateRouter(db)
	if status, body := callUserUpdate(t, r, u.ID, map[string]interface{}{"plan": "pro"}); status != http.StatusOK {
		t.Fatalf("status = %d, body %v", status, body)
	}

	var active []models.Subscription
	db.Where("user_id = ? AND status = ?", u.ID, "active").Find(&active)
	if len(active) != 1 || active[0].PlanID != pro.ID || active[0].IsTrial {
		t.Fatalf("langganan aktif = %+v, mau tepat 1 baris paket pro non-trial", active)
	}
	var lama models.Subscription
	db.First(&lama, old.ID)
	if lama.Status != "expired" {
		t.Errorf("langganan lite lama status = %q, mau expired", lama.Status)
	}
	if got := readUser(t, db, u.ID); got.Plan != "pro" {
		t.Errorf("users.plan = %q, mau pro", got.Plan)
	}

	// Turun ke free: semua langganan di-expire, tidak ada baris baru.
	if status, _ := callUserUpdate(t, r, u.ID, map[string]interface{}{"plan": "free"}); status != http.StatusOK {
		t.Fatalf("status free = %d", status)
	}
	var n int64
	db.Model(&models.Subscription{}).Where("user_id = ? AND status = ?", u.ID, "active").Count(&n)
	if n != 0 {
		t.Errorf("langganan aktif setelah turun ke free = %d, mau 0", n)
	}
}

func TestAdminUpdateUserUnknownPlanRejected(t *testing.T) {
	db := auditTestDB(t)
	email := "uji-user-plan-ngawur@contoh.id"
	u := seedUserUpdateUser(t, db, email)
	defer deleteUserByEmail(t, db, email)
	if err := db.AutoMigrate(&models.Plan{}, &models.Subscription{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	status, _ := callUserUpdate(t, userUpdateRouter(db), u.ID, map[string]interface{}{"plan": "paket-tidak-ada"})
	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, mau 400", status)
	}
	if got := readUser(t, db, u.ID); got.Plan != "free" {
		t.Errorf("users.plan = %q, mau tetap free", got.Plan)
	}
}
