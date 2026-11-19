package handler

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode"
	"time"

	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/database/models"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/middleware"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/quota"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/rls"
	"github.com/alecslacker/SlackerSender-WAGW/go-backend/internal/whatsapp"
	"github.com/gin-gonic/gin"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"gorm.io/gorm"
)

// digitsOnly: nomor HP valid dari JID WhatsApp (hanya digit, 6-16 karakter)
var digitsOnly = regexp.MustCompile(`^[0-9]{6,16}$`)

type syncDeviceRequest struct {
	DeviceID uint `json:"deviceId" binding:"required"`
}

// resolveSyncClient memastikan device milik user dan sedang terhubung,
// lalu mengembalikan client whatsmeow yang aktif.
func resolveSyncClient(c *gin.Context, db *gorm.DB, wm *whatsapp.Manager, deviceID uint) (*whatsmeow.Client, error) {
	userID := middleware.GetUserID(c)
	db = rls.Scoped(db, userID)
	var device models.Device
	if err := db.Where("id = ? AND user_id = ?", deviceID, userID).First(&device).Error; err != nil {
		return nil, fmt.Errorf("perangkat tidak ditemukan")
	}
	client := wm.GetClient(deviceID)
	if client == nil || wm.GetStatus(deviceID) != "connected" || !client.IsConnected() {
		return nil, fmt.Errorf("perangkat %q belum terhubung — hubungkan dulu di Dashboard lalu coba lagi", device.Name)
	}
	return client, nil
}

// waContactName memilih nama terbaik dari info kontak WA
func waContactName(info types.ContactInfo, fallback string) string {
	for _, n := range []string{info.FullName, info.FirstName, info.PushName, info.BusinessName} {
		if strings.TrimSpace(n) != "" {
			return strings.TrimSpace(n)
		}
	}
	return fallback
}

// cleanContactName membersihkan nama dari WhatsApp: buang karakter kontrol,
// rapikan spasi, batasi 200 karakter (kolom contacts.name = 255).
func cleanContactName(n string) string {
	n = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, n)
	n = strings.TrimSpace(n)
	if r := []rune(n); len(r) > 200 {
		n = strings.TrimSpace(string(r[:200]))
	}
	return n
}

// waNameLookup membuat pencari nama (nomor -> nama) dari penyimpanan perangkat
// WhatsApp: nama kontak di HP lebih dulu, lalu nama panggilan WhatsApp orang
// itu. Peserta grup sering tersimpan di bawah alamat LID, jadi bila alamat
// nomor tak punya nama, dicoba LID-nya. Hasil di-cache; "" = tidak ada nama.
func waNameLookup(ctx context.Context, client *whatsmeow.Client) func(phone string) string {
	cache := map[string]string{}
	return func(phone string) string {
		if n, ok := cache[phone]; ok {
			return n
		}
		name := ""
		if client != nil && client.Store != nil && client.Store.Contacts != nil {
			pn := types.NewJID(phone, types.DefaultUserServer)
			if info, err := client.Store.Contacts.GetContact(ctx, pn); err == nil {
				name = cleanContactName(waContactName(info, ""))
			}
			if name == "" && client.Store.LIDs != nil {
				if lid, err := client.Store.LIDs.GetLIDForPN(ctx, pn); err == nil && !lid.IsEmpty() {
					if info, err := client.Store.Contacts.GetContact(ctx, lid); err == nil {
						name = cleanContactName(waContactName(info, ""))
					}
				}
			}
		}
		if name == phone {
			name = "" // nama yang cuma nomor tidak ada gunanya
		}
		cache[phone] = name
		return name
	}
}

// upsertSyncContact mencari kontak user berdasar nomor; buat baru bila belum ada,
// atau lengkapi nama yang masih kosong. Aman dipanggil berulang (idempotent).
func upsertSyncContact(db *gorm.DB, userID uint, phone, name string) (created, filled bool) {
	c, f, _ := upsertSyncContactCapped(db, userID, phone, name, nil)
	return c, f
}

// remainingContactSlots menghitung sisa slot kontak paket aktif user:
// -1 = tanpa batas (MaxContacts <= 0) atau paket/DB gagal dibaca
// (fail-open: sync lama tetap jalan).
func remainingContactSlots(db *gorm.DB, userID uint) int {
	plan, _, err := quota.EffectivePlan(db, userID)
	if err != nil || plan == nil || plan.MaxContacts <= 0 {
		return -1
	}
	var count int64
	if err := db.Model(&models.Contact{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		return -1
	}
	rem := plan.MaxContacts - int(count)
	if rem < 0 {
		rem = 0
	}
	return rem
}

// upsertSyncContactCapped seperti upsertSyncContact, tapi TIDAK membuat
// kontak baru bila *remaining sudah 0 (batas kontak paket tercapai —
// Item 7). remaining nil = tanpa batas. capped=true bila pembuatan
// diblokir batas paket; kontak yang sudah ada tetap bisa dilengkapi
// namanya (tidak menghabiskan slot).
func upsertSyncContactCapped(db *gorm.DB, userID uint, phone, name string, remaining *int) (created, filled, capped bool) {
	var existing models.Contact
	if err := db.Where("user_id = ? AND phone = ?", userID, phone).First(&existing).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			if remaining != nil && *remaining <= 0 {
				return false, false, true
			}
			if db.Create(&models.Contact{UserID: userID, Name: name, Phone: phone}).Error == nil {
				if remaining != nil {
					*remaining--
				}
				return true, false, false
			}
		}
		return false, false, false
	}
	// Lengkapi hanya nama yang kosong atau yang cuma berisi nomornya sendiri
	// (hasil sync lama). Nama yang sudah diisi pengguna tidak pernah ditimpa.
	cur := strings.TrimSpace(existing.Name)
	if (cur == "" || cur == phone) && name != "" && name != phone {
		db.Model(&existing).Update("name", name)
		return false, true, false
	}
	return false, false, false
}

// POST /api/contacts/sync — tarik seluruh kontak dari perangkat WA yang terhubung.
// Body: { "deviceId": 1 }
func syncContactsFromWA(db *gorm.DB, wm *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req syncDeviceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "deviceId wajib diisi", "code": "VALIDATION_ERROR"})
			return
		}
		client, err := resolveSyncClient(c, udb, wm, req.DeviceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error(), "code": "DEVICE_OFFLINE"})
			return
		}

		if client.Store.Contacts == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Penyimpanan kontak tidak tersedia", "code": "WA_ERROR"})
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		waContacts, err := client.Store.Contacts.GetAllContacts(ctx)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membaca kontak dari WhatsApp: " + err.Error(), "code": "WA_ERROR"})
			return
		}

		added, filled, failed, capped := 0, 0, 0, 0
		// Batas kontak paket aktif (Item 7): sync berhenti menambah
		// kontak baru saat slot habis; kontak lama tetap bisa dilengkapi.
		rem := remainingContactSlots(udb, userID)
		var remPtr *int
		if rem >= 0 {
			remPtr = &rem
		}
		for jid, info := range waContacts {
			// hanya kontak personal (bukan grup g.us / lid / broadcast)
			if jid.Server != types.DefaultUserServer {
				continue
			}
			phone := jid.User
			if !digitsOnly.MatchString(phone) {
				continue
			}
			func() {
				defer func() {
					// Satu kontak rusak tidak boleh menggagalkan seluruh sync.
					if r := recover(); r != nil {
						failed++
					}
				}()
				created, nameFilled, wasCapped := upsertSyncContactCapped(udb, userID, phone, waContactName(info, phone), remPtr)
				if created {
					added++
				} else if nameFilled {
					filled++
				} else if wasCapped {
					capped++
				}
			}()
		}

		msg := fmt.Sprintf("Sync selesai: %d kontak baru, %d nama dilengkapi", added, filled)
		if capped > 0 {
			msg += fmt.Sprintf(", %d dilewati (batas kontak paket tercapai — upgrade paket untuk menambah kontak)", capped)
		}
		if failed > 0 {
			msg += fmt.Sprintf(", %d dilewati (gagal diproses)", failed)
		}
		c.JSON(http.StatusOK, gin.H{
			"added":             added,
			"updated":           filled,
			"failed":            failed,
			"skippedDueToLimit": capped,
			"message":           msg,
		})
	}
}

// groupParticipantPhones mengubah daftar peserta grup WA menjadi nomor HP unik.
// WhatsApp kini memakai alamat LID ("123@lid") di grup; nomor aslinya ada di
// PhoneNumber, atau dipetakan lewat resolveLID (cache LID→PN). Peserta LID yang
// nomornya disembunyikan WhatsApp tidak bisa dipetakan — dihitung di unresolved
// agar pengguna tahu, bukan hilang diam-diam. Entri bukan nomor (bot, dsb) dilewati.
func groupParticipantPhones(parts []types.GroupParticipant, resolveLID func(lidUser string) string) (phones []string, unresolved int) {
	seen := make(map[string]bool, len(parts))
	for _, p := range parts {
		phone := ""
		switch {
		case !p.PhoneNumber.IsEmpty() && p.PhoneNumber.Server == types.DefaultUserServer:
			phone = p.PhoneNumber.User
		case p.JID.Server == types.DefaultUserServer:
			phone = p.JID.User
		case p.JID.Server == types.HiddenUserServer:
			if resolveLID != nil {
				phone = resolveLID(p.JID.User)
			}
			if phone == "" {
				unresolved++
				continue
			}
		default:
			continue
		}
		if !digitsOnly.MatchString(phone) || seen[phone] {
			continue
		}
		seen[phone] = true
		phones = append(phones, phone)
	}
	return phones, unresolved
}

// POST /api/contact-groups/sync — tarik grup WA yang diikuti perangkat beserta anggotanya.
// Grup di-upsert berdasar JID WA (kolom wa_jid); anggota di-upsert jadi kontak dan
// ditautkan sebagai anggota grup. Body: { "deviceId": 1 }
func syncGroupsFromWA(db *gorm.DB, wm *whatsapp.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := middleware.GetUserID(c)
		udb := rls.Scoped(db, userID)
		var req syncDeviceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "deviceId wajib diisi", "code": "VALIDATION_ERROR"})
			return
		}
		client, err := resolveSyncClient(c, udb, wm, req.DeviceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": err.Error(), "code": "DEVICE_OFFLINE"})
			return
		}

		waGroups, err := wm.GetGroups(req.DeviceID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Gagal membaca grup dari WhatsApp", "code": "WA_ERROR"})
			return
		}

		groupsAdded, groupsUpdated, membersLinked, capped, membersUnresolved, namesFilled := 0, 0, 0, 0, 0, 0
		nameCtx, nameCancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer nameCancel()
		nameOf := waNameLookup(nameCtx, client)
		resolveLID := func(lidUser string) string { return wm.ResolvePNForLID(req.DeviceID, lidUser) }
		// Batas kontak paket aktif (Item 7): anggota grup baru berhenti
		// dibuat saat slot kontak habis; penautan anggota lama tetap jalan.
		rem := remainingContactSlots(udb, userID)
		var remPtr *int
		if rem >= 0 {
			remPtr = &rem
		}
		for _, g := range waGroups {
			if g == nil {
				continue
			}
			waJID := g.JID.String()
			name := strings.TrimSpace(g.Name)
			if name == "" {
				name = waJID
			}

			// kumpulkan nomor anggota yang valid; peserta LID dipetakan ke nomor asli
			phones, unresolved := groupParticipantPhones(g.Participants, resolveLID)
			membersUnresolved += unresolved

			var grp models.ContactGroup
			err := udb.Where("user_id = ? AND waj_id = ?", userID, waJID).First(&grp).Error
			if err == gorm.ErrRecordNotFound {
				grp = models.ContactGroup{
					UserID:      userID,
					Name:        name,
					Description: strings.TrimSpace(g.Topic),
					MemberCount: len(phones),
					WAJID:       waJID,
				}
				if err := udb.Create(&grp).Error; err != nil {
					continue
				}
				groupsAdded++
			} else if err == nil {
				udb.Model(&grp).Updates(map[string]interface{}{
					"name":         name,
					"description":  strings.TrimSpace(g.Topic),
					"member_count": len(phones),
				})
				groupsUpdated++
			} else {
				continue
			}

			// upsert tiap anggota jadi kontak, lalu tautkan ke grup
			for _, phone := range phones {
				contactName := nameOf(phone)
				if contactName == "" {
					contactName = phone
				}
				_, nameFilled, wasCapped := upsertSyncContactCapped(udb, userID, phone, contactName, remPtr)
				if nameFilled {
					namesFilled++
				}
				if wasCapped {
					capped++
					continue
				}
				var contact models.Contact
				if err := udb.Where("user_id = ? AND phone = ?", userID, phone).First(&contact).Error; err != nil {
					continue
				}
				var link models.ContactGroupMember
				if err := udb.Where("group_id = ? AND contact_id = ?", grp.ID, contact.ID).First(&link).Error; err == gorm.ErrRecordNotFound {
					if udb.Create(&models.ContactGroupMember{GroupID: grp.ID, ContactID: contact.ID}).Error == nil {
						membersLinked++
					}
				}
			}
		}

		msg := fmt.Sprintf("Sync selesai: %d grup baru, %d grup diperbarui, %d anggota ditautkan", groupsAdded, groupsUpdated, membersLinked)
		if namesFilled > 0 {
			msg += fmt.Sprintf(", %d nama kontak dilengkapi", namesFilled)
		}
		if membersUnresolved > 0 {
			msg += fmt.Sprintf(", %d peserta dilewati (WhatsApp menyembunyikan nomornya)", membersUnresolved)
		}
		c.JSON(http.StatusOK, gin.H{
			"groupsAdded":       groupsAdded,
			"groupsUpdated":     groupsUpdated,
			"membersLinked":     membersLinked,
			"membersUnresolved": membersUnresolved,
			"namesFilled":       namesFilled,
			"skippedDueToLimit": capped,
			"message":           msg,
		})
	}
}
