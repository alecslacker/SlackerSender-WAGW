# IMPLEMENTATION PLAN: Kompatibilitas Endpoint Slackersender

> Author: Mas Wondho — Duta Corpora Indonesia (DCI)
> Copyright © 2026 Duta Corpora Indonesia
> Tanggal: 2026-11-17
> Sumber kontrak: `graphify-out/api-slackersender-global.md` (dokumentasi API Slackersender v2.0)

## Overview

Menyamakan endpoint pengiriman gateway WaGataway (repo wblash) dengan kontrak API Slackersender,
sehingga driver/aplikasi yang sudah ditulis untuk Slackersender bisa dipakai tanpa mengubah kode —
cukup arahkan base URL ke gateway ini. **Bukan** menambahkan fitur-fitur Slackersender; hanya
kesamaan kontrak endpoint pengiriman (request/response), dipetakan ke mesin pengiriman WaGataway.

## Context7 Pre-Check

- Library eksternal yang disentuh: tidak ada library baru (Go stdlib + whatsmeow yang sudah ada).
- API whatsmeow `Client.IsOnWhatsApp` sudah dipakai repo (`manager.go:274`, `group_mgmt.go:182`) —
  tidak ada API baru yang perlu diverifikasi ke Context7. Go stdlib `strconv.ParseFloat` standar.
- Resiko API drift whatsmeow: rendah, karena hanya memakai API yang sudah terpakai di repo ini.

## Skill yang digunakan

- Tidak ada skill khusus yang cocok (task Go backend murni, mengikuti pola kode repo).

## Gap Analysis (hasil pembacaan kode)

| Endpoint Slackersender | Kondisi di wblash | Aksi |
|---|---|---|
| `POST /send-message` | ✅ sudah ada (`wamp_compat.go`, root path, kontrak sama) | Tambah mode GET |
| `POST /send-media` | ⚠️ ada tapi `media_type` **diabaikan** — selalu dikirim sebagai `image` | Map `media_type` → tipe kirim WA |
| `POST /send-location` | ❌ belum ada | Endpoint baru |
| `POST /info-device` | ❌ belum ada | Endpoint baru (map `Device` model → shape Slackersender) |
| `POST /check-number` | ❌ belum ada | Endpoint baru (reuse `IsOnWhatsApp`, kembalikan JID) |

Catatan kontrak Slackersender (dari dokumen sumber):

- Auth via field body `api_key` (bukan header) — sudah sama dengan pola wamp_compat.
- Semua endpoint mendukung mode POST **dan** GET (query string); POST direkomendasikan.
- Respons sukses = HTTP 200 + `status: true`. Format nomor: prefix `62` tanpa `+`.
- `sender` = nomor/ID/nama device pengirim; `number` = nomor penerima.
- `send-media`: `url` direct link; `media_type` ∈ {image, video, audio, document}; caption opsional.
- `send-location`: `latitude`/`longitude` dikirim sebagai **string** (jaga presisi desimal).
- `info-device`: param hanya `api_key` + `number` (nomor device); respons `info` = array device.
- `check-number`: respons `msg.exists` + `msg.jid`.

## Skill Reference

Skill: tidak ada skill domain yang cocok (backend Go murni, pola mengikuti repo).

## Tasks

### Task 1 — Perluas lapisan kompatibilitas (`internal/handler/wamp_compat.go`)
— Files: `go-backend/internal/handler/wamp_compat.go` — Complexity: Medium

1. `registerWampCompatRoutes` → daftarkan:
   - `POST + GET /send-message`, `POST + GET /send-media` (mode GET: query string, dirouting
     Gin `ShouldBind` otomatis; api_key via GET memang berisiko, tapi ini kesamaan kontrak yang diminta).
   - `POST + GET /send-location`, `POST + GET /info-device`, `POST + GET /check-number`.
2. `wampSend` (send-message & send-media): map `media_type` → `SendOptions.Type`
   (`image/video/audio/document`; kosong/tak dikenal → `image` = perilaku lama, backward-compatible).
3. Handler baru yang memakai ulang blok auth + kuota + resolve sender yang sama:
   - `send-location`: parse `latitude`/`longitude` string → float64; tolak bila bukan angka valid
     (HTTP 400, `status:false`). Kirim via `SendOptions{Type:"location", Latitude, Longitude}`.
   - `info-device`: resolve device by nomor (pakai `resolveWampSender`), map ke shape Slackersender:
     `{id, user_id, body: Phone, webhook: WebhookURL|null, status: "Connect"/"Disconnect",
     created_at, updated_at, message_sent, reply_when: "Personal", wh_read, reject_call, wh_typing}`
     — `message_sent` dihitung `count(messages WHERE device_id AND direction='outgoing'
     AND status IN sent/delivered/read)` (sama dengan pola SentCount di `device.go:48-65`).
   - `check-number`: reuse `resolveWampSender` untuk device, lalu `whatsapp.Manager.CheckNumberJID`
     (Task 2) → `{"status": true, "msg": {"exists": bool, "jid": "..."}}`.
4. Ekstrak helper kecil `authenticateAPIKey(db, apiKey) (*models.ApiKey, error)` dipakai bersama
   oleh semua handler compat (auth + expiry + update last_used) — menghapus duplikasi bila nanti
   ada endpoint lain.

### Task 2 — Manager: cek nomor + JID (`internal/whatsapp/manager.go`)
— Files: `go-backend/internal/whatsapp/manager.go` — Complexity: Low

Tambah `CheckNumberJID(deviceID uint, phone string) (exists bool, jid string, err error)` —
sama seperti `CheckNumberRegistered` tapi mengembalikan JID dari respons `IsOnWhatsApp`.
`CheckNumberRegistered` dibiarkan tetap (dipakai `/api/contacts/validate`), didelegasikan saja.

### Task 3 — Test unit handler compat
— Files: `go-backend/internal/handler/wamp_compat_test.go` (baru) — Complexity: Medium

Ikuti pola test handler lain (`clipkupay_webhook_test.go`, `auth_trial_test.go`):
- Test map media_type (image/video/audio/document/unknown→image).
- Test send-location: valid, koordinat bukan angka → 400.
- Test info-device: shape respons (field `status`, `info[0].body`, `message_sent`).
- Test check-number: device terputus → error respons; (cek ke WA asli tidak bisa di-mock penuh,
  jadi path sukses dites via manager test terpisah bila perlu — out of scope minimal).
- Test auth: api_key salah → 401; number kosong → 400.

### Task 4 — Verifikasi + update peta
— Files: tidak mengubah kode — Complexity: Low

1. `go build ./...` + `go vet ./...` di `go-backend/` (butuh CGO karena `mattn/go-sqlite3`;
   bila gcc tidak tersedia di mesin, dilaporkan apa adanya).
2. `go test ./internal/handler/ -run 'Wamp|Slacker|Compat' -v` (targeted).
3. `graphify update .` (WAJIB setelah edit signifikan — aturan peta).

## YAGNI — yang TIDAK dikerjakan

- Tidak ada perubahan frontend React (endpoint ini untuk driver eksternal).
- Tidak menambah fitur Slackersender lain (reply_when logic, auto-read tracking UI, dsb.).
- Tidak mengubah kontrak endpoint `/api/*` yang sudah ada.
- Tidak menambah kolom/tabel database baru (semua map dari data yang sudah ada).
- Tidak menghapus/mengubah perilaku PPOB WAMP yang sudah jalan (`ppob.clipku.com`).

## Risiko & Mitigasi

| Risiko | Mitigasi |
|---|---|
| PPOB WAMP yang sudah jalan bisa rusak bila mapping media berubah | `media_type` kosong/tak dikenal tetap → `image` (perilaku lama utuh); PPOB mengirim field sama |
| GET mode dengan api_key di URL bocor di access log | Kontrak Slackersender memang menawarkan 2 mode; hanya mendaftarkan rute GET, tidak mengubah rekomendasi POST di dokumen gateway |
| gcc/CGO tidak tersedia → build gagal di mesin lokal | Laporkan apa adanya; verifikasi bisa lanjut via `go vet` parsial atau di server Mas Wondho |
| `IsOnWhatsApp` rate limit WhatsApp saat check-number dipanggil massal | Konsisten dengan `/api/contacts/validate` yang sudah ada; endpoint ini untuk pemakaian per-nomor (sesuai panduan dokumen: panggil hanya saat registrasi/perubahan nomor) |
| Shape respons `info-device` tidak 100% sama field | Field list diambil dari contoh respons dokumen sumber; field tambahan Slackersender yang tidak relevan (mis. `reply_when` dinamis) dikirim konstanta `"Personal"` |
| Kuota: check-number/info-device tidak/kurang dicek kuota | Tidak mengirim pesan → tidak memotong kuota; hanya send-* yang dicek kuota (konsisten dengan wampSend sekarang) |

---
Mas Wondho, konfirmasi untuk lanjut?
