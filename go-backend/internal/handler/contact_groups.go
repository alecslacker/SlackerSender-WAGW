package handler

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/middleware"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/rls"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/whatsapp"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func registerContactGroupRoutes(rg *gin.RouterGroup, db *gorm.DB, wm *whatsapp.Manager) {
	g := rg.Group("/contact-groups")
	g.Use(middleware.RequireScope("contacts"))
	{
		g.GET("", listContactGroups(db))
		g.POST("", createContactGroup(db))
		g.POST("/sync", syncGroupsFromWA(db, wm))
		g.PUT("/:id", updateContactGroup(db))
		g.DELETE("/:id", deleteContactGroup(db))
		// Aksi massal. Rute statis didaftarkan sebelum "/:id/..." agar tak tertukar.
		g.POST("/bulk-delete", bulkDeleteContactGroups(db))
		g.GET("/export", exportContactGroups(db))
		g.GET("/:id/members", listGroupMembers(db))
		g.POST("/:id/members", addGroupMembers(db))
		g.DELETE("/:id/members/:memberId", removeGroupMember(db))
		// Fitur 5: setting welcome DM pribadi per grup (hanya grup hasil sync WA).
		g.GET("/:id/welcome-dm", getWelcomeDMSetting(db))
		g.PUT("/:id/welcome-dm", updateWelcomeDMSetting(db))
	}
}

// getWelcomeDMSetting mengembalikan {enabled, template} welcome DM grup.
func getWelcomeDMSetting(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
		var group models.ContactGroup
		if err := udb.Where("id = ? AND user_id = ?", id, userID).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Grup tidak ditemukan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"enabled":  group.WelcomeDMEnabled,
			"template": group.WelcomeDMTemplate,
			"waJid":    group.WAJID,
		})
	}
}

// updateWelcomeDMSetting menyimpan {enabled, template} welcome DM grup.
// Variabel template yang didukung: {nama} (nama anggota baru) dan {grup}
// (nama grup). Bila enabled=true tapi template kosong → 400.
func updateWelcomeDMSetting(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
		var group models.ContactGroup
		if err := udb.Where("id = ? AND user_id = ?", id, userID).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Grup tidak ditemukan"})
			return
		}
		var req struct {
			Enabled  bool   `json:"enabled"`
			Template string `json:"template"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Payload tidak valid"})
			return
		}
		if req.Enabled && req.Template == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Template wajib diisi bila welcome DM diaktifkan"})
			return
		}
		if err := udb.Model(&group).Updates(map[string]interface{}{
			"welcome_dm_enabled":  req.Enabled,
			"welcome_dm_template": req.Template,
		}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal menyimpan"})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"message":  "Pengaturan welcome DM disimpan",
			"enabled":  req.Enabled,
			"template": req.Template,
		})
	}
}

func listContactGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var groups []models.ContactGroup
		udb.Where("user_id = ?", userID).Order("name ASC").Find(&groups)
		c.JSON(http.StatusOK, gin.H{"groups": groups})
	}
}

func createContactGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req struct {
			Name        string `json:"name" binding:"required"`
			Description string `json:"description"`
			Color       string `json:"color"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Nama grup wajib"})
			return
		}
		group := models.ContactGroup{
			UserID: userID, Name: req.Name,
			Description: req.Description, Color: req.Color,
		}
		udb.Create(&group)
		c.JSON(http.StatusCreated, gin.H{"group": group})
	}
}

func updateContactGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
		var group models.ContactGroup
		if err := udb.Where("id = ? AND user_id = ?", id, userID).First(&group).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"message": "Grup tidak ditemukan"})
			return
		}
		var req map[string]interface{}
		c.ShouldBindJSON(&req)
		udb.Model(&group).Updates(snakeKeys(req))
		c.JSON(http.StatusOK, gin.H{"group": group})
	}
}

// maxBulkGroupIDs membatasi jumlah grup per aksi massal (bulk-delete / export).
const maxBulkGroupIDs = 500

// parseIDList mengubah "1,2,3" menjadi ID unik yang valid (urutan asli terjaga).
// Entri bukan angka / nol / ganda dibuang.
func parseIDList(raw string) []uint {
	seen := map[uint]bool{}
	var out []uint
	for _, part := range strings.Split(raw, ",") {
		n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32)
		if err != nil || n == 0 || seen[uint(n)] {
			continue
		}
		seen[uint(n)] = true
		out = append(out, uint(n))
	}
	return out
}

// bulkDeleteContactGroups menghapus banyak grup sekaligus: POST /api/contact-groups/bulk-delete
// body {"ids":[1,2,3]}. HANYA data di platform ini yang dihapus (grup + tautan
// anggotanya). Kontak TIDAK dihapus dan grup di WhatsApp tidak disentuh.
// ID milik akun lain dilewati begitu saja (tidak bocor, tidak error).
func bulkDeleteContactGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req struct {
			IDs []uint `json:"ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Pilih minimal satu grup", "code": "VALIDATION_ERROR"})
			return
		}
		if len(req.IDs) > maxBulkGroupIDs {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Terlalu banyak grup sekaligus (maks " + strconv.Itoa(maxBulkGroupIDs) + ")", "code": "VALIDATION_ERROR"})
			return
		}
		// Hanya ID yang benar-benar milik user ini.
		var mine []models.ContactGroup
		udb.Select("id").Where("user_id = ? AND id IN ?", userID, req.IDs).Find(&mine)
		ids := make([]uint, 0, len(mine))
		for _, g := range mine {
			ids = append(ids, g.ID)
		}
		if len(ids) > 0 {
			udb.Where("group_id IN ?", ids).Delete(&models.ContactGroupMember{})
			udb.Where("user_id = ? AND id IN ?", userID, ids).Delete(&models.ContactGroup{})
		}
		c.JSON(http.StatusOK, gin.H{
			"deleted": len(ids),
			"skipped": len(req.IDs) - len(ids),
			"message": strconv.Itoa(len(ids)) + " grup dihapus",
		})
	}
}

// csvSafe menetralkan sel yang akan dibaca Excel/Sheets sebagai RUMUS. Nama dari
// WhatsApp bisa diatur siapa saja ("=HYPERLINK(...)"), jadi sel yang diawali
// = + - @ (atau tab/CR) diberi apostrof di depan agar tampil sebagai teks.
func csvSafe(s string) string {
	if s == "" {
		return s
	}
	if strings.ContainsRune("=+-@\t\r", rune(s[0])) {
		return "'" + s
	}
	return s
}

// exportContactGroups mengekspor anggota grup terpilih sebagai CSV (dibuka di Excel):
// GET /api/contact-groups/export?ids=1,2,3 -> kolom grup, nama, nomor.
// Hanya grup milik user; ID akun lain diabaikan. Tanpa ids = semua grup user.
func exportContactGroups(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		ids := parseIDList(c.Query("ids"))
		if len(ids) > maxBulkGroupIDs {
			c.JSON(http.StatusBadRequest, gin.H{"message": "Terlalu banyak grup sekaligus (maks " + strconv.Itoa(maxBulkGroupIDs) + ")", "code": "VALIDATION_ERROR"})
			return
		}
		q := udb.Where("user_id = ?", userID)
		if len(ids) > 0 {
			q = q.Where("id IN ?", ids)
		}
		var groups []models.ContactGroup
		q.Order("name ASC").Find(&groups)
		if len(groups) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"message": "Tidak ada grup untuk diekspor", "code": "NOT_FOUND"})
			return
		}

		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="grup-kontak-`+time.Now().Format("20060102-150405")+`.csv"`)
		// BOM UTF-8 supaya Excel membaca nama beraksara/emoji dengan benar.
		_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"Grup", "Nama", "Nomor"})
		for _, g := range groups {
			var members []models.ContactGroupMember
			udb.Where("group_id = ?", g.ID).Preload("Contact").Order("id ASC").Find(&members)
			for _, m := range members {
				// Nomor tidak perlu csvSafe: hanya digit (divalidasi saat sync).
				_ = w.Write([]string{csvSafe(g.Name), csvSafe(m.Contact.Name), m.Contact.Phone})
			}
		}
		w.Flush()
	}
}

func deleteContactGroup(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
		udb.Where("group_id = ?", id).Delete(&models.ContactGroupMember{})
		udb.Where("id = ? AND user_id = ?", id, userID).Delete(&models.ContactGroup{})
		c.JSON(http.StatusOK, gin.H{"message": "Grup dihapus"})
	}
}

// ownedGroup memuat grup HANYA bila milik user yang login. Semua endpoint
// anggota wajib lewat sini: tanpa cek ini user lain bisa membaca/mengubah
// anggota grup milik akun lain hanya dengan menebak ID grup.
func ownedGroup(c *gin.Context, udb *gorm.DB, userID uint) (*models.ContactGroup, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil || id == 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "Grup tidak ditemukan"})
		return nil, false
	}
	var g models.ContactGroup
	if err := udb.Where("id = ? AND user_id = ?", id, userID).First(&g).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"message": "Grup tidak ditemukan"})
		return nil, false
	}
	return &g, true
}

// memberView = anggota grup beserta nama & nomor kontaknya. Dikirim langsung
// dari server agar UI tidak perlu mencocokkan ke daftar kontak (yang dibatasi
// 200 baris) — dulu kontak di luar 200 teratas tampil "Kontak #580".
type memberView struct {
	ID        uint   `json:"id"`
	GroupID   uint   `json:"groupId"`
	ContactID uint   `json:"contactId"`
	Name      string `json:"name"`
	Phone     string `json:"phone"`
}

func listGroupMembers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		g, ok := ownedGroup(c, udb, userID)
		if !ok {
			return
		}
		var members []models.ContactGroupMember
		udb.Where("group_id = ?", g.ID).Preload("Contact").Order("id ASC").Find(&members)
		out := make([]memberView, 0, len(members))
		for _, m := range members {
			out = append(out, memberView{ID: m.ID, GroupID: m.GroupID, ContactID: m.ContactID, Name: m.Contact.Name, Phone: m.Contact.Phone})
		}
		c.JSON(http.StatusOK, gin.H{"members": out})
	}
}

func addGroupMembers(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		g, ok := ownedGroup(c, udb, userID)
		if !ok {
			return
		}
		var req struct {
			ContactIDs []uint `json:"contactIds" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "contactIds wajib"})
			return
		}
		// Hanya kontak milik user ini yang boleh dimasukkan ke grupnya.
		var mine []models.Contact
		udb.Select("id").Where("user_id = ? AND id IN ?", userID, req.ContactIDs).Find(&mine)
		added := 0
		for _, ct := range mine {
			var n int64
			udb.Model(&models.ContactGroupMember{}).Where("group_id = ? AND contact_id = ?", g.ID, ct.ID).Count(&n)
			if n > 0 {
				continue // sudah anggota
			}
			if udb.Create(&models.ContactGroupMember{GroupID: g.ID, ContactID: ct.ID}).Error == nil {
				added++
			}
		}
		recountGroupMembers(udb, g.ID)
		c.JSON(http.StatusOK, gin.H{"added": added})
	}
}

// recountGroupMembers menghitung ulang member_count dari tabel anggota
// (sumber kebenaran) agar angka kartu grup tidak melenceng.
func recountGroupMembers(udb *gorm.DB, groupID uint) {
	var n int64
	udb.Model(&models.ContactGroupMember{}).Where("group_id = ?", groupID).Count(&n)
	udb.Model(&models.ContactGroup{}).Where("id = ?", groupID).Update("member_count", n)
}

func removeGroupMember(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		g, ok := ownedGroup(c, udb, userID)
		if !ok {
			return
		}
		memberId, _ := strconv.ParseUint(c.Param("memberId"), 10, 32)
		// Hanya melepas tautan anggota dari grup; kontaknya sendiri TIDAK dihapus.
		result := udb.Where("id = ? AND group_id = ?", memberId, g.ID).Delete(&models.ContactGroupMember{})
		if result.RowsAffected > 0 {
			recountGroupMembers(udb, g.ID)
		}
		c.JSON(http.StatusOK, gin.H{"message": "Member dihapus"})
	}
}
