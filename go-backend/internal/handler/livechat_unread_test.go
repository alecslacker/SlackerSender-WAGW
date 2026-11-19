package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// unreadTestDB: SQLite memori (tanpa RLS PostgreSQL — yang diuji di sini
// hanya logika query/hitungan, bukan isolasi antar user).
func unreadTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.ChatInbox{}, &models.ChatConversation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func unreadRouter(db *gorm.DB) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", uint(5)); c.Next() })
	r.GET("/chat/unread-count", getUnreadCount(db))
	r.PATCH("/chat/conversations/:phone/read", markConversationRead(db))
	return r
}

func seedUnread(db *gorm.DB) {
	db.Create(&models.User{ID: 5, UUID: "00000000-0000-0000-0000-000000000005", Name: "T", Email: "t@test.local", Password: "x"})
	// Nomor yang SAMA di dua device + satu nomor lain + satu percakapan diarsipkan.
	db.Create(&models.ChatConversation{UserID: 5, DeviceID: 1, Phone: "6281", UnreadCount: 3, Status: "open"})
	db.Create(&models.ChatConversation{UserID: 5, DeviceID: 2, Phone: "6281", UnreadCount: 2, Status: "open"})
	db.Create(&models.ChatConversation{UserID: 5, DeviceID: 1, Phone: "6282", UnreadCount: 4, Status: "pending"})
	db.Create(&models.ChatConversation{UserID: 5, DeviceID: 1, Phone: "6283", UnreadCount: 9, Status: "archived"})
	db.Create(&models.ChatConversation{UserID: 99, DeviceID: 1, Phone: "6284", UnreadCount: 50, Status: "open"}) // milik user lain
	for _, d := range []uint{1, 2} {
		for i := 0; i < 2; i++ {
			db.Create(&models.ChatInbox{UserID: 5, DeviceID: d, Phone: "6281", Direction: "in", IsRead: false})
		}
	}
}

func getJSON(r *gin.Engine, method, path string) (int, map[string]interface{}) {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	var out map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &out)
	return w.Code, out
}

func TestUnreadCountMengikutiDeviceDanStatus(t *testing.T) {
	db := unreadTestDB(t)
	seedUnread(db)
	r := unreadRouter(db)

	cases := []struct {
		nama string
		path string
		want float64
	}{
		{"semua device, status default open+pending", "/chat/unread-count", 3 + 2 + 4},
		{"hanya device 1", "/chat/unread-count?deviceId=1", 3 + 4},
		{"hanya device 2", "/chat/unread-count?deviceId=2", 2},
		{"status archived", "/chat/unread-count?status=archived", 9},
		{"status all", "/chat/unread-count?status=all", 3 + 2 + 4 + 9},
		{"device tanpa percakapan", "/chat/unread-count?deviceId=77", 0},
	}
	for _, tc := range cases {
		code, body := getJSON(r, http.MethodGet, tc.path)
		if code != http.StatusOK || body["unreadCount"] != tc.want {
			t.Errorf("%s: HTTP %d, unreadCount=%v, mau %v", tc.nama, code, body["unreadCount"], tc.want)
		}
	}
	// Milik user lain (userID 99) tidak boleh ikut terhitung di akun 5.
	if _, body := getJSON(r, http.MethodGet, "/chat/unread-count?status=all"); body["unreadCount"] == float64(3+2+4+9+50) {
		t.Error("percakapan user lain ikut terhitung")
	}
}

func TestUnreadCountStatusTakDikenal(t *testing.T) {
	db := unreadTestDB(t)
	r := unreadRouter(db)
	if code, _ := getJSON(r, http.MethodGet, "/chat/unread-count?status=ngawur"); code != http.StatusBadRequest {
		t.Errorf("status tak dikenal harus 400, dapat %d", code)
	}
}

func TestMarkReadHanyaDeviceYangDibuka(t *testing.T) {
	db := unreadTestDB(t)
	seedUnread(db)
	r := unreadRouter(db)

	if code, _ := getJSON(r, http.MethodPatch, "/chat/conversations/6281/read?deviceId=1"); code != http.StatusOK {
		t.Fatalf("PATCH read: HTTP %d", code)
	}
	var d1, d2 models.ChatConversation
	db.Where("device_id = 1 AND phone = ?", "6281").First(&d1)
	db.Where("device_id = 2 AND phone = ?", "6281").First(&d2)
	if d1.UnreadCount != 0 {
		t.Errorf("device 1 harus 0, dapat %d", d1.UnreadCount)
	}
	if d2.UnreadCount != 2 {
		t.Errorf("device 2 TIDAK boleh ikut terhapus, dapat %d (mau 2)", d2.UnreadCount)
	}
	var unreadD1, unreadD2 int64
	db.Model(&models.ChatInbox{}).Where("device_id = 1 AND is_read = ?", false).Count(&unreadD1)
	db.Model(&models.ChatInbox{}).Where("device_id = 2 AND is_read = ?", false).Count(&unreadD2)
	if unreadD1 != 0 || unreadD2 != 2 {
		t.Errorf("is_read salah: device1=%d (mau 0) device2=%d (mau 2)", unreadD1, unreadD2)
	}
}

func TestMarkReadTanpaDeviceIdKompatMundur(t *testing.T) {
	db := unreadTestDB(t)
	seedUnread(db)
	r := unreadRouter(db)

	getJSON(r, http.MethodPatch, "/chat/conversations/6281/read")
	var n int64
	db.Model(&models.ChatConversation{}).Where("phone = ? AND unread_count > 0 AND user_id = 5", "6281").Count(&n)
	if n != 0 {
		t.Errorf("tanpa deviceId perilaku lama (semua device) harus tetap berlaku, tersisa %d", n)
	}
}
