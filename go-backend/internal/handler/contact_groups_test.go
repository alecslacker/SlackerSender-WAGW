// Test anggota grup kontak: kepemilikan (anti akses lintas akun), nama/nomor per
// anggota, dan pengisian nama saat sync (contact_groups.go, contact_sync.go).
//
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func groupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.Contact{}, &models.ContactGroup{}, &models.ContactGroupMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// groupTestRouter: semua request dijalankan sebagai user `loginAs`.
func groupTestRouter(db *gorm.DB, loginAs uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", loginAs); c.Next() })
	r.GET("/contact-groups/:id/members", listGroupMembers(db))
	r.POST("/contact-groups/:id/members", addGroupMembers(db))
	r.DELETE("/contact-groups/:id/members/:memberId", removeGroupMember(db))
	return r
}

func doJSON(r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// seedGroups: user 5 punya grup + 1 anggota; user 6 punya kontak sendiri.
func seedGroups(db *gorm.DB) (grp models.ContactGroup, mem models.ContactGroupMember, c5, c6 models.Contact) {
	db.Create(&models.User{ID: 5, UUID: "00000000-0000-0000-0000-000000000005", Name: "A", Email: "a@test.local", Password: "x"})
	db.Create(&models.User{ID: 6, UUID: "00000000-0000-0000-0000-000000000006", Name: "B", Email: "b@test.local", Password: "x"})
	c5 = models.Contact{UserID: 5, Name: "Budi Santoso", Phone: "6281000000001"}
	c6 = models.Contact{UserID: 6, Name: "Milik Akun Lain", Phone: "6282000000009"}
	db.Create(&c5)
	db.Create(&c6)
	grp = models.ContactGroup{UserID: 5, Name: "Grup Lima", MemberCount: 1}
	db.Create(&grp)
	mem = models.ContactGroupMember{GroupID: grp.ID, ContactID: c5.ID}
	db.Create(&mem)
	return
}

func itoa(n uint) string { return strconv.Itoa(int(n)) }

// Regresi: dulu list/add/remove anggota tidak memeriksa pemilik grup — user lain
// bisa membaca dan menghapus anggota grup orang lain hanya dengan menebak ID.
func TestGroupMembersOwnership(t *testing.T) {
	db := groupTestDB(t)
	grp, mem, _, c6 := seedGroups(db)
	intruder := groupTestRouter(db, 6)

	// membaca grup milik orang lain -> 404, tanpa bocor data
	w := doJSON(intruder, http.MethodGet, "/contact-groups/"+itoa(grp.ID)+"/members", "")
	if w.Code != http.StatusNotFound || strings.Contains(w.Body.String(), "Budi") {
		t.Errorf("baca grup orang lain: HTTP %d (%s), mau 404 tanpa data", w.Code, w.Body.String())
	}

	// menghapus anggota grup orang lain -> 404 dan anggota TETAP ada
	w = doJSON(intruder, http.MethodDelete, "/contact-groups/"+itoa(grp.ID)+"/members/"+itoa(mem.ID), "")
	var sisa int64
	db.Model(&models.ContactGroupMember{}).Where("id = ?", mem.ID).Count(&sisa)
	if w.Code != http.StatusNotFound || sisa != 1 {
		t.Errorf("hapus anggota grup orang lain: HTTP %d, anggota tersisa=%d, mau 404 dan 1", w.Code, sisa)
	}

	// menambah anggota ke grup orang lain -> 404, tidak ada anggota baru
	w = doJSON(intruder, http.MethodPost, "/contact-groups/"+itoa(grp.ID)+"/members", `{"contactIds":[`+itoa(c6.ID)+`]}`)
	var total int64
	db.Model(&models.ContactGroupMember{}).Where("group_id = ?", grp.ID).Count(&total)
	if w.Code != http.StatusNotFound || total != 1 {
		t.Errorf("tambah anggota ke grup orang lain: HTTP %d, total anggota=%d, mau 404 dan 1", w.Code, total)
	}

	// ID bukan angka / tidak ada -> 404
	for _, id := range []string{"abc", "0", "99999"} {
		w = doJSON(groupTestRouter(db, 5), http.MethodGet, "/contact-groups/"+id+"/members", "")
		if w.Code != http.StatusNotFound {
			t.Errorf("id %q: HTTP %d, mau 404", id, w.Code)
		}
	}
}

// Pemilik sah tetap bisa; kontak milik akun lain TIDAK bisa dimasukkan ke grupnya;
// anggota ganda dicegah; member_count dihitung ulang dari tabel anggota.
func TestGroupMembersOwnerFlow(t *testing.T) {
	db := groupTestDB(t)
	grp, mem, c5, c6 := seedGroups(db)
	extra := models.Contact{UserID: 5, Name: "Siti", Phone: "6281000000002"}
	db.Create(&extra)
	owner := groupTestRouter(db, 5)

	// daftar anggota memuat nama + nomor langsung (bukan tebakan dari 200 kontak)
	w := doJSON(owner, http.MethodGet, "/contact-groups/"+itoa(grp.ID)+"/members", "")
	var list struct {
		Members []memberView `json:"members"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &list); err != nil || w.Code != http.StatusOK {
		t.Fatalf("daftar anggota: HTTP %d (%s) err=%v", w.Code, w.Body.String(), err)
	}
	if len(list.Members) != 1 || list.Members[0].Name != "Budi Santoso" || list.Members[0].Phone != "6281000000001" {
		t.Fatalf("anggota tanpa nama/nomor: %+v", list.Members)
	}

	// tambah: kontak sendiri + kontak akun lain + kontak yang sudah anggota + id ngawur
	body := `{"contactIds":[` + itoa(extra.ID) + `,` + itoa(c6.ID) + `,` + itoa(c5.ID) + `,424242]}`
	w = doJSON(owner, http.MethodPost, "/contact-groups/"+itoa(grp.ID)+"/members", body)
	var added struct{ Added int }
	_ = json.Unmarshal(w.Body.Bytes(), &added)
	if w.Code != http.StatusOK || added.Added != 1 {
		t.Errorf("tambah anggota: HTTP %d, ditambah=%d, mau 200 dan 1 (hanya Siti)", w.Code, added.Added)
	}
	var g models.ContactGroup
	db.First(&g, grp.ID)
	if g.MemberCount != 2 {
		t.Errorf("member_count = %d, mau 2", g.MemberCount)
	}

	// hapus anggota: hanya tautan terlepas, kontaknya TETAP ada
	w = doJSON(owner, http.MethodDelete, "/contact-groups/"+itoa(grp.ID)+"/members/"+itoa(mem.ID), "")
	var kontak, tautan int64
	db.Model(&models.Contact{}).Where("id = ?", c5.ID).Count(&kontak)
	db.Model(&models.ContactGroupMember{}).Where("id = ?", mem.ID).Count(&tautan)
	db.First(&g, grp.ID)
	if w.Code != http.StatusOK || kontak != 1 || tautan != 0 || g.MemberCount != 1 {
		t.Errorf("hapus anggota: HTTP %d kontak=%d tautan=%d member_count=%d, mau 200/1/0/1", w.Code, kontak, tautan, g.MemberCount)
	}
}

func TestCleanContactName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"  Budi  ", "Budi"},
		{"Bu\u0000di\n", "Budi"},
		{"", ""},
		{strings.Repeat("a", 300), strings.Repeat("a", 200)},
		{"Ahmad 🌙 Fauzi", "Ahmad 🌙 Fauzi"},
	}
	for _, tc := range cases {
		if got := cleanContactName(tc.in); got != tc.want {
			t.Errorf("cleanContactName(%q) = %q, mau %q", tc.in, got, tc.want)
		}
	}
}

// Sync boleh melengkapi nama yang kosong atau yang cuma berisi nomor, tetapi
// TIDAK PERNAH menimpa nama yang sudah diisi pengguna.
func TestUpsertSyncContactNameFill(t *testing.T) {
	db := groupTestDB(t)
	db.Create(&models.User{ID: 5, UUID: "00000000-0000-0000-0000-000000000005", Name: "A", Email: "a@test.local", Password: "x"})
	db.Create(&models.Contact{UserID: 5, Name: "6281000000001", Phone: "6281000000001"}) // hasil sync lama (nama = nomor)
	db.Create(&models.Contact{UserID: 5, Name: "Nama Buatan Saya", Phone: "6281000000002"})
	db.Create(&models.Contact{UserID: 5, Name: "", Phone: "6281000000003"})

	cases := []struct {
		phone, newName, wantName string
		wantFilled               bool
	}{
		{"6281000000001", "Budi dari WhatsApp", "Budi dari WhatsApp", true},
		{"6281000000002", "Nama WhatsApp Lain", "Nama Buatan Saya", false}, // jangan ditimpa
		{"6281000000003", "Siti", "Siti", true},
		{"6281000000001", "6281000000001", "Budi dari WhatsApp", false}, // nama cuma nomor: abaikan
	}
	for _, tc := range cases {
		_, filled, _ := upsertSyncContactCapped(db, 5, tc.phone, tc.newName, nil)
		var c models.Contact
		db.Where("user_id = ? AND phone = ?", 5, tc.phone).First(&c)
		if c.Name != tc.wantName || filled != tc.wantFilled {
			t.Errorf("%s + %q: nama=%q filled=%v, mau %q/%v", tc.phone, tc.newName, c.Name, filled, tc.wantName, tc.wantFilled)
		}
	}
}

func TestCSVSafe(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Budi", "Budi"},
		{"", ""},
		{`=HYPERLINK("http://jahat","klik")`, `'=HYPERLINK("http://jahat","klik")`},
		{"+62812", "'+62812"},
		{"-1+1", "'-1+1"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\tTab", "'\tTab"},
		{"Ada = di tengah", "Ada = di tengah"},
	}
	for _, tc := range cases {
		if got := csvSafe(tc.in); got != tc.want {
			t.Errorf("csvSafe(%q) = %q, mau %q", tc.in, got, tc.want)
		}
	}
}

func TestParseIDList(t *testing.T) {
	got := parseIDList(" 3, 1,abc,0,3,-2,,7 ")
	want := []uint{3, 1, 7}
	if len(got) != len(want) {
		t.Fatalf("parseIDList = %v, mau %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseIDList = %v, mau %v", got, want)
		}
	}
	if len(parseIDList("")) != 0 {
		t.Error("string kosong harus menghasilkan daftar kosong")
	}
}

func bulkRouter(db *gorm.DB, loginAs uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("userID", loginAs); c.Next() })
	r.POST("/contact-groups/bulk-delete", bulkDeleteContactGroups(db))
	r.GET("/contact-groups/export", exportContactGroups(db))
	return r
}

// Hapus massal: grup + tautan anggota hilang, KONTAK tetap ada, grup akun lain
// tidak tersentuh dan ID asing dilewati tanpa bocor.
func TestBulkDeleteContactGroups(t *testing.T) {
	db := groupTestDB(t)
	g1, _, c5, c6 := seedGroups(db)
	g2 := models.ContactGroup{UserID: 5, Name: "Grup Dua"}
	db.Create(&g2)
	db.Create(&models.ContactGroupMember{GroupID: g2.ID, ContactID: c5.ID})
	gOrang := models.ContactGroup{UserID: 6, Name: "Grup Orang Lain"}
	db.Create(&gOrang)
	db.Create(&models.ContactGroupMember{GroupID: gOrang.ID, ContactID: c6.ID})

	// kasus kosong / ngawur -> 400
	r := bulkRouter(db, 5)
	for _, body := range []string{`{}`, `{"ids":[]}`, `bukan json`} {
		if w := doJSON(r, http.MethodPost, "/contact-groups/bulk-delete", body); w.Code != http.StatusBadRequest {
			t.Errorf("body %q: HTTP %d, mau 400", body, w.Code)
		}
	}

	// hapus: 2 grup sendiri + 1 grup orang lain + 1 id tak ada
	body := `{"ids":[` + itoa(g1.ID) + `,` + itoa(g2.ID) + `,` + itoa(gOrang.ID) + `,99999]}`
	w := doJSON(r, http.MethodPost, "/contact-groups/bulk-delete", body)
	var res struct{ Deleted, Skipped int }
	_ = json.Unmarshal(w.Body.Bytes(), &res)
	if w.Code != http.StatusOK || res.Deleted != 2 || res.Skipped != 2 {
		t.Fatalf("bulk-delete: HTTP %d deleted=%d skipped=%d, mau 200/2/2 (%s)", w.Code, res.Deleted, res.Skipped, w.Body.String())
	}
	var sisaGrup5, sisaTautan, kontak5, kontak6, grupOrang, tautanOrang int64
	db.Model(&models.ContactGroup{}).Where("user_id = ?", 5).Count(&sisaGrup5)
	db.Model(&models.ContactGroupMember{}).Where("group_id IN ?", []uint{g1.ID, g2.ID}).Count(&sisaTautan)
	db.Model(&models.Contact{}).Where("user_id = ?", 5).Count(&kontak5)
	db.Model(&models.Contact{}).Where("user_id = ?", 6).Count(&kontak6)
	db.Model(&models.ContactGroup{}).Where("id = ?", gOrang.ID).Count(&grupOrang)
	db.Model(&models.ContactGroupMember{}).Where("group_id = ?", gOrang.ID).Count(&tautanOrang)
	if sisaGrup5 != 0 || sisaTautan != 0 {
		t.Errorf("grup sendiri belum terhapus: grup=%d tautan=%d", sisaGrup5, sisaTautan)
	}
	if kontak5 != 1 || kontak6 != 1 {
		t.Errorf("KONTAK ikut terhapus! user5=%d user6=%d, mau 1 dan 1", kontak5, kontak6)
	}
	if grupOrang != 1 || tautanOrang != 1 {
		t.Errorf("grup/anggota akun lain tersentuh: grup=%d tautan=%d, mau 1 dan 1", grupOrang, tautanOrang)
	}
}

func TestExportContactGroups(t *testing.T) {
	db := groupTestDB(t)
	g1, _, _, c6 := seedGroups(db)
	// kontak ber-nama berbahaya + grup akun lain
	jahat := models.Contact{UserID: 5, Name: `=HYPERLINK("http://jahat","klik")`, Phone: "6281000000099"}
	db.Create(&jahat)
	db.Create(&models.ContactGroupMember{GroupID: g1.ID, ContactID: jahat.ID})
	gOrang := models.ContactGroup{UserID: 6, Name: "Grup Orang Lain"}
	db.Create(&gOrang)
	db.Create(&models.ContactGroupMember{GroupID: gOrang.ID, ContactID: c6.ID})

	r := bulkRouter(db, 5)
	w := doJSON(r, http.MethodGet, "/contact-groups/export?ids="+itoa(g1.ID)+","+itoa(gOrang.ID), "")
	if w.Code != http.StatusOK || !strings.HasPrefix(w.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(w.Header().Get("Content-Disposition"), "attachment") {
		t.Fatalf("export: HTTP %d, ct=%q, cd=%q", w.Code, w.Header().Get("Content-Type"), w.Header().Get("Content-Disposition"))
	}
	body := w.Body.String()
	if !strings.HasPrefix(body, "\xEF\xBB\xBF") {
		t.Error("CSV tanpa BOM UTF-8")
	}
	for _, mau := range []string{"Grup,Nama,Nomor", "Grup Lima,Budi Santoso,6281000000001"} {
		if !strings.Contains(body, mau) {
			t.Errorf("CSV tidak memuat %q:\n%s", mau, body)
		}
	}
	if strings.Contains(body, "Milik Akun Lain") || strings.Contains(body, "6282000000009") || strings.Contains(body, "Grup Orang Lain") {
		t.Errorf("CSV membocorkan data akun lain:\n%s", body)
	}
	if strings.Contains(body, `,=HYPERLINK`) || strings.Contains(body, `"=HYPERLINK`) {
		t.Errorf("sel rumus tidak dinetralkan:\n%s", body)
	}
	if !strings.Contains(body, `'=HYPERLINK`) {
		t.Errorf("sel rumus seharusnya diberi apostrof:\n%s", body)
	}
	// hanya grup orang lain -> tidak ada yang bisa diekspor (404), bukan data bocor
	w = doJSON(r, http.MethodGet, "/contact-groups/export?ids="+itoa(gOrang.ID), "")
	if w.Code != http.StatusNotFound {
		t.Errorf("export grup orang lain: HTTP %d, mau 404", w.Code)
	}
}
