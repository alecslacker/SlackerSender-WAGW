# SlackerSender — API Reference Lengkap (WhatsApp Gateway)

> Dokumen ini adalah referensi tunggal untuk **AI agent / developer** yang membangun sistem di atas gateway WhatsApp SlackerSender.
> Semua endpoint, field, respons, dan batasan di bawah diverifikasi langsung dari kode sumber `go-backend/internal/` (rilis 2026-11-20).

| | |
|---|---|
| **Produk** | SlackerSender (WAGW) — WhatsApp Gateway self-hosted, satu binary Go |
| **Base URL produksi** | `https://wasender.slackercode.us` (ganti sesuai instalasi) |
| **Base URL API** | `{APP_URL}/api` |
| **Format** | JSON (UTF-8), kecuali upload file (multipart) |
| **Autentikasi** | `Authorization: Bearer <JWT>` (dashboard) **atau** `X-API-Key: wg_...` (integrasi) |
| **Bahasa pesan error** | Indonesia |

---

## Daftar Isi

1. [Prinsip Dasar untuk Agent](#1-prinsip-dasar-untuk-agent)
2. [Autentikasi](#2-autentikasi)
3. [Rate Limit & Guard Global](#3-rate-limit--guard-global)
4. [Format Error & Kode Error](#4-format-error--kode-error)
5. [Endpoint Kompatibel Slackersender / WAMP (kirim sederhana)](#5-endpoint-kompatibel-slackersender--wamp)
6. [API Key (membuat & rotasi)](#6-api-key)
7. [Devices (perangkat WhatsApp)](#7-devices)
8. [Messages — kirim tunggal & bulk](#8-messages)
9. [Polling & Pesan Interaktif](#9-polling--pesan-interaktif)
10. [Kontak & Grup Kontak](#10-kontak--grup-kontak)
11. [Grup WhatsApp](#11-grup-whatsapp)
12. [Jadwal & Berulang](#12-jadwal--berulang)
13. [Files (media manager)](#13-files)
14. [Live Chat](#14-live-chat)
15. [Webhook Keluar (outgoing)](#15-webhook-keluar-outgoing)
16. [Webhook Masuk (inbound) & Integration Hub](#16-webhook-masuk-inbound--integration-hub)
17. [Kuota & Billing](#17-kuota--billing)
18. [Auth & Akun (JWT)](#18-auth--akun)
19. [SSE (realtime)](#19-sse-realtime)
20. [Analytics & Laporan](#20-analytics--laporan)
21. [Endpoint Lain (tabel ringkas)](#21-endpoint-lain-tabel-ringkas)
22. [Endpoint Stub — JANGAN DIPAKAI](#22-endpoint-stub--jangan-dipakai)
23. [Playbook Agent: Resep Integrasi Umum](#23-playbook-agent-resep-integrasi-umum)
24. [Katalog Kode Error](#24-katalog-kode-error)

---

## 1. Prinsip Dasar untuk Agent

Aturan wajib sebelum memanggil API apa pun:

1. **Semua kirim pesan bersifat ASYNC (queued).** Respons sukses (`status: true` / `200`) artinya pesan *diterima dan masuk antrean*, **bukan sudah sampai HP penerima**. Verifikasi pengiriman lewat: `GET /api/messages/:id/status`, field `status` pada objek message, atau webhook `message.sent` / `message.failed`.
2. **Format nomor tujuan**: prefix `62` tanpa `+` — contoh `628123456789`. Server menormalkan otomatis: `+`, spasi, `-` dibuang; awalan `0` diganti `62` (`0812…` → `62812…`). ID grup juga diterima: `1203630xxxxxxxxxx@g.us`.
3. **`via` pada riwayat pesan**: request ber-header `X-API-Key` dicatat `via: "api"`; lewat dashboard `via: "web"`.
4. **Kuota bulanan berlaku** di semua jalur kirim. Habis → `429 QUOTA_EXCEEDED` (seluruh request bulk ditolak di muka, bukan sebagian).
5. **Idempotency didukung** di `POST /api/messages/send` lewat field `idempotencyKey` — pakai ini untuk OTP/transaksi agar retry jaringan tidak mengirim dobel.
6. **Personalisasi template**: `{nama}` diganti nama kontak (fallback nomor tujuan), `{nomor}` diganti nomor tujuan.
7. **Batas konten**: teks/caption maks 10.000 karakter; body interactive maks 2.000; pertanyaan poll maks 1.000.
8. Endpoint `/api/admin/*` **hanya JWT dashboard** — API key selalu ditolak (`403 API_KEY_NOT_ALLOWED`).
9. Mode maintenance aktif → semua `/api/*` balas `503 {"code":"MAINTENANCE"}`.

---

## 2. Autentikasi

Dua jalur, keduanya membuka endpoint `/api/*` yang sama:

### a) API Key (untuk agent/sistem eksternal)

```
X-API-Key: wg_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

- Format: `wg_` + 32 karakter hex. Dibuat dari dashboard: **Setting → API Key** (atau `POST /api/api-keys`).
- Key mentah **hanya ditampilkan sekali** saat dibuat/dirotasi — simpan saat itu juga.
- Disimpan sebagai hash di server; hanya 4 digit terakhir yang bisa dilihat lagi.
- Bisa punya **scope** (batasan akses) dan **kedaluwarsa** (`expiresInDays` saat dibuat).
- Key mati otomatis bila akun pemiliknya `banned`/`suspended` (`401 ACCOUNT_DISABLED`).
- JWT dari login dashboard juga diterima header yang sama dengan format `Authorization: Bearer <token>`.

### b) Bearer JWT (dashboard)

```
Authorization: Bearer <token-dari-login>
```

- Dari `POST /api/auth/login`. Tidak dibatasi scope.
- Berlaku sesuai `JWTExpiry` server (default 24 jam); sesi bisa dicabut (`SESSION_REVOKED`).

> **Rekomendasi agent**: buat API key khusus dengan scope minimal yang dibutuhkan (mis. hanya `messages:send`), jangan pakai `full` kecuali perlu.

---

## 3. Rate Limit & Guard Global

| Guard | Batas | Berlaku pada |
|---|---|---|
| Global | 1000 request / 15 menit / IP | semua path (kecuali `/health`, `/assets/*`, `/uploads/*`, `/illustrations/*`) |
| Auth | 10 request / 1 menit / IP | `/api/auth/*` (login, register, team-login) |
| Inbox publik | 60 request / 1 menit / token | `/api/integrations/inbox/:token`, `/api/inbound-webhooks/:id/hook` |

- Lewat batas → `429 {"message":"Terlalu banyak permintaan dari IP ini...","code":"TOO_MANY_REQUESTS"}`.
- Jendela **tetap** (reset di awal jendela) — klien yang terus mencoba akan pulih setelah jendela berganti, tidak menumpuk.
- Brute-force login gagal berulang → akun terkunci sementara (`429 ACCOUNT_LOCKED`).

---

## 4. Format Error & Kode Error

Semua error berbentuk:

```json
{ "message": "Pesan manusia (Indonesia)", "code": "KODE_ERROR" }
```

- `400` validasi (`VALIDATION_ERROR`), `401` auth, `403` scope/langganan, `404` tidak ditemukan, `429` rate/kuota, `5xx` server.
- Route `/api/*` tak dikenal → `404 {"message":"Route not found","code":"NOT_FOUND"}`.
- Katalog lengkap: [bagian 24](#24-katalog-kode-error).

---

## 5. Endpoint Kompatibel Slackersender / WAMP

Untuk klien lama & integrasi paling sederhana. **Auth via field `api_key` di body** (atau header `X-API-Key`). Didaftarkan di **root** (`/send-message`) dan alias `/api/send-message`. Method POST (JSON atau form-urlencoded) **atau** GET (query string) — **selalu pakai POST** agar api_key tidak bocor di log.

| Endpoint | Method | Scope | Fungsi |
|---|---|---|---|
| `/send-message` | POST/GET | `messages:send` | kirim teks |
| `/send-media` | POST/GET | `messages:send` | kirim gambar/video/audio/dokumen via URL |
| `/send-location` | POST/GET | `messages:send` | kirim koordinat |
| `/info-device` | POST/GET | `devices:read` | status kesehatan perangkat |
| `/check-number` | POST/GET | `contacts` | cek nomor terdaftar di WA |

### 5.1 POST /send-message

```json
{
  "api_key": "wg_...",
  "sender": "6281234567890",
  "number": "6289876543210",
  "message": "Halo, pesan dari sistem",
  "reply_to": ""
}
```

| Field | Wajib | Keterangan |
|---|---|---|
| `api_key` | ya | atau header `X-API-Key` |
| `sender` | tidak | ID device (angka) / nomor HP / nama device; kosong = device default user |
| `number` | ya | tujuan; boleh **banyak** dipisah `|` (juga menerima `,` `;` baris baru) |
| `message` | ya | isi pesan (maks 10.000 karakter) |
| `reply_to` | tidak | WA message ID yang dikutip |

**Respons sukses (1 nomor):**

```json
{ "status": true, "message": "Pesan sedang dikirim", "id": 123 }
```

**Respons sukses (multi nomor)** — tambah `ids` dan `total`:

```json
{ "status": true, "message": "Pesan sedang dikirim", "id": 123, "ids": [123,124,125], "total": 3 }
```

**Aturan multi-nomor:** duplikat dibuang; dikirim berurutan dengan jeda acak 2–5 detik; kuota & batas bulk (bawaan 1000, sesuai paket) dicek untuk TOTAL di muka — kurang = seluruh request ditolak `429`. Tujuan boleh campur nomor personal dan ID grup `…@g.us`.

### 5.2 POST /send-media

```json
{
  "api_key": "wg_...",
  "sender": "6281234567890",
  "number": "6289876543210",
  "media_type": "document",
  "caption": "Invoice #1234",
  "url": "https://app.kamu.com/files/invoice-1234.pdf"
}
```

| Field | Wajib | Keterangan |
|---|---|---|
| `url` | ya | **direct link** publik (bukan tautan share Google Drive) |
| `media_type` | tidak | `image` (default) / `video` / `audio` / `document` |
| `caption` | tidak | teks pendamping (fallback: `message`) |

### 5.3 POST /send-location

```json
{
  "api_key": "wg_...", "sender": "6281234567890", "number": "6289876543210",
  "latitude": "-6.2088", "longitude": "106.8456"
}
```

`latitude`/`longitude` bertipe **string** (jaga presisi desimal); divalidasi angka.

### 5.4 POST /info-device

Body: `{api_key, number}` — `number` = ID/nomor/nama device (resolusi sama seperti `sender`).

```json
{
  "status": true,
  "info": [{
    "id": 1, "user_id": 5, "body": "6281234567890", "webhook": null,
    "status": "Connect",
    "created_at": "2026-11-01T10:00:00Z", "updated_at": "2026-11-20T08:00:00Z",
    "message_sent": 1523, "reply_when": "Personal",
    "wh_read": 1, "reject_call": 0, "wh_typing": 1
  }]
}
```

`status`: `"Connect"` / `"Disconnect"` — untuk health-check kanal.

### 5.5 POST /check-number

Body: `{api_key, sender?, number}` (satu nomor saja per request).

```json
{ "status": true, "msg": { "exists": true, "jid": "6289876543210@s.whatsapp.net" } }
```

Panggil hanya saat registrasi/perubahan nomor — hemat kuota dan rate limit.

---

## 6. API Key

Grup `/api/api-keys` — butuh JWT dashboard **atau** API key scope `full` (deny-by-default: key terbatas tidak boleh mengelola key).

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/api-keys` | daftar key (masked) |
| GET | `/api/api-keys/scopes` | daftar scope yang tersedia |
| POST | `/api/api-keys` | buat key |
| POST | `/api/api-keys/:id/rotate` | ganti key (lama langsung mati) |
| DELETE | `/api/api-keys/:id` | hapus |

**Buat key:**

```json
POST /api/api-keys
{ "name": "agent-produksi", "scopes": ["messages:send"], "expiresInDays": 90 }
```

Respons `201` (key penuh **hanya sekali** di sini):

```json
{
  "message": "API key dibuat",
  "apiKey": "wg_abc123...xyz",
  "key": { "id": 7, "name": "agent-produksi", "keyPreview": "****xyz", "isActive": true, "scopes": ["messages:send"], "expiresAt": "2027-02-18T03:00:00Z", "expired": false, "createdAt": "..." }
}
```

**Scope yang tersedia:**

| Scope | Akses |
|---|---|
| `full` | semuanya |
| `messages:send` | kirim pesan (semua varian) |
| `messages:read` | baca riwayat & status pesan |
| `contacts` | kelola kontak & grup kontak |
| `devices:read` | lihat perangkat |
| `devices:write` | kelola perangkat |
| `chat` | Live Chat (baca & balas) |
| `files` | kelola file/media |
| `webhooks` | kelola webhook |

Scopes kosong saat create = `full`. Scope tidak cocok → `403 INSUFFICIENT_SCOPE`.

---

## 7. Devices

Grup `/api/devices`. Baca = scope `devices:read`, ubah = `devices:write`.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/devices` | daftar perangkat user |
| POST | `/api/devices` | tambah perangkat |
| GET/PUT/DELETE | `/api/devices/:id` | detail / ubah / hapus (logout+hapus sesi) |
| POST | `/api/devices/:id/connect` | mulai koneksi (tampilkan QR) |
| POST | `/api/devices/:id/disconnect` | putuskan |
| GET | `/api/devices/:id/qr` | QR pairing (string + PNG base64, dirender lokal) |
| POST | `/api/devices/:id/pair-code` | kode pairing 8 digit (alternatif QR) |
| GET | `/api/devices/:id/status` | status realtime |
| GET | `/api/devices/:id/bot-deliveries` | riwayat webhook bot PPOB |
| POST | `/api/devices/:id/webhook-secret/regenerate` | ganti secret webhook per-device |

**Objek device** (field utama): `id, userId, name, phone, status, isDefault, autoOnline, readReceipts, rejectCall, typingIndicator, sentCount, webhookUrl, webhookEnabled, webhookSecret, maxRetries, retryDelay, lastSeen, connectedAt, createdAt, updatedAt`.

**Alur pairing (wajib untuk bisa kirim):**

1. `POST /api/devices {"name":"Kantor"}` → `201 {device}` (webhook secret otomatis dibuat).
2. `POST /api/devices/:id/connect` → `{"message":"Menghubungkan perangkat...","status":"connecting"}`.
3. Poll `GET /api/devices/:id/qr` → `{"qr":"2@...","qrImage":"data:image/png;base64,...","expiresAt":"..."}` — tampilkan `qrImage` ke user untuk discan; `404 NO_QR` bila belum ada.
4. Setelah discan: `GET /api/devices/:id/status` → `{"status":"connected"}` dan `phone` terisi.
   - Alternatif tanpa QR: `POST /api/devices/:id/pair-code {"phone":"6281234567890"}` → `{"code":"ABCD-EFGH","expiresIn":120}` — masukkan di HP WhatsApp.
5. `status` valid: `connected` | `connecting` | `disconnected`.

**PUT /api/devices/:id** (field opsional, semua pointer): `name, autoOnline, webhookUrl, webhookEnabled, readReceipts, rejectCall, typingIndicator, maxRetries, retryDelay, isDefault`.

> `webhookUrl` device wajib lolos validasi SSRF (http/https, host resolve ke IP publik — bukan localhost/IP privat).

---

## 8. Messages

Grup `/api/messages`. Baca = `messages:read`, kirim = `messages:send`.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/messages?page=1&limit=20` | riwayat (limit ≤ 100) → `{messages,total,page,limit}` |
| POST | `/api/messages/send` | kirim tunggal |
| POST | `/api/messages/send-bulk` | blast / campaign |
| POST | `/api/messages/check-recipients` | validasi nomor sebelum blast |
| GET | `/api/messages/bulk-jobs?limit=20` | riwayat blast |
| POST | `/api/messages/bulk-jobs/:id/retry` | ulangi penerima gagal |
| GET | `/api/messages/bulk-stats` | `{jobs,wait,sent,failed}` |
| GET | `/api/messages/:id/status` | status satu pesan → `{data: message}` |
| DELETE | `/api/messages/:id` | tarik pesan (revoke, hapus untuk semua) |

### 8.1 POST /api/messages/send

```json
{
  "deviceId": 1,
  "to": "6289876543210",
  "type": "text",
  "content": "Halo {nama}, tagihan Anda siap.",
  "mediaUrl": "",
  "fileId": null,
  "caption": "",
  "replyTo": "",
  "idempotencyKey": "invoice-1234-send"
}
```

| Field | Wajib | Keterangan |
|---|---|---|
| `deviceId` | ya | harus milik user |
| `to` | ya | nomor / ID grup `@g.us` |
| `type` | tidak | default `text`; lihat 8.3 |
| `content` | ya | isi pesan (utk media = caption utama; maks 10.000) |
| `mediaUrl` | tidak | URL direct link utk tipe media |
| `fileId` | tidak | ID dari File Manager — **menggantikan mediaUrl** (baca dari disk lokal) |
| `caption` | tidak | caption tambahan utk media |
| `replyTo` | tidak | WA message ID yang dikutip |
| `idempotencyKey` | tidak | anti dobel — key sama tidak dikirim ulang |

Respons `200 {"message":"Pesan sedang dikirim","data":{...message}}`.
Request ulang dengan `idempotencyKey` sama → `200 {"message":"Pesan sudah pernah dikirim (duplikat)","duplicate":true,"data":{...}}`.

### 8.2 POST /api/messages/send-bulk

```json
{
  "deviceIds": [1, 2],
  "recipients": ["628111111111", "628222222222", "1203630...@g.us"],
  "type": "text",
  "content": "Promo hari ini!",
  "mediaUrl": "",
  "minDelay": 3,
  "maxDelay": 8,
  "autoClean": true,
  "scheduledAt": "2026-11-21T09:00"
}
```

| Field | Keterangan |
|---|---|
| `deviceId` / `deviceIds[]` | satu saja atau banyak (rotasi round-robin + failover); minimal satu |
| `recipients[]` | wajib; maks = `maxBulk` paket (bawaan 1000) |
| `minDelay`/`maxDelay` | detik, 0–3600; default 3/8 |
| `autoClean` | `true` = nomor tak terdaftar WA dicoret otomatis sebelum kirim |
| `scheduledAt` | `YYYY-MM-DDTHH:mm` (lokal) atau RFC3339; masa depan = job `scheduled` |

Validasi: minimal satu device harus `connected` (kecuali terjadwal); `maxDelay ≥ minDelay`.
Respons: `{"message":"Bulk message dijadwalkan","job":{...},"deviceIds":[...],"cleaned":{"applied":true,"total":50,"valid":47,"excluded":3,"duplicates":0,"excludedNumbers":[...]}}`.

### 8.3 Tipe pesan yang didukung

`text`, `image`, `video`, `audio`, `document`, `sticker`, `voicenote`, `location`, `poll`, `interactive`.
Tipe media (`image` dst.) **wajib** `mediaUrl` atau `fileId`; `document` memakai `content`/`fileName` sebagai nama file.

### 8.4 Status pesan (lifecycle)

`pending` → `sent` → `delivered` → `read`; gagal `failed` (+`errorMsg`); ditarik `revoked`.
Kuota hanya menghitung outgoing `sent/delivered/read`.

### 8.5 POST /api/messages/check-recipients

```json
{ "deviceId": 1, "numbers": ["628111111111", "628999999999"] }
```

(1–1000 nomor) →

```json
{ "total": 2, "valid": ["628111111111"], "validCount": 1, "excluded": ["628999999999"], "excludedCount": 1, "duplicates": 0, "excludedNumbers": ["628999999999"] }
```

---

## 9. Polling & Pesan Interaktif

Semua di scope `messages:send`, body selalu butuh `deviceId` + `to`.

### POST /api/messages/send-poll

```json
{ "deviceId": 1, "to": "6289876543210", "question": "Kapasitas mana?", "options": ["5L","10L"], "allowMultiple": false }
```

2–12 opsi. Respons: `{"message":"Poll sedang dikirim","data":{...},"pollId":7}`.

### POST /api/messages/send-interactive (tombol)

```json
{
  "deviceId": 1, "to": "6289876543210",
  "body": "Pilih metode pembayaran:",
  "buttons": [ {"id":"va","title":"Transfer VA"}, {"id":"qris","title":"QRIS"} ],
  "footer": "Auto-reply CS Bot"
}
```

1–3 tombol; body maks 2.000 karakter.

### POST /api/messages/send-sticker & /send-voice-note

`{deviceId, to, mediaUrl}` — mediaUrl wajib (stiker: webp; voice: ogg/mp3).

### POST /api/messages/send-location

```json
{ "deviceId": 1, "to": "6289876543210", "latitude": -6.2088, "longitude": 106.8456, "name": "Kantor Pusat", "address": "Jl. ...", "live": false }
```

Angka (bukan string, beda dengan endpoint compat); rentang valid lat ±90, lon ±180.

### Manajemen poll

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/polls` | daftar poll |
| GET | `/api/polls/:id/results` | hasil suara |
| POST | `/api/polls/:id/close` | tutup poll |
| POST | `/api/polls/:id/recap` | kirim rekap hasil ke grup |

---

## 10. Kontak & Grup Kontak

Grup `/api/contacts` & `/api/contact-groups` — scope `contacts`.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/contacts?search=&page=1&limit=50` | → `{contacts,total,page,limit,contactLimit,planName}` |
| POST | `/api/contacts` | `{name, phone, email?, notes?, tags?}` (batas jumlah per paket) |
| PUT / DELETE | `/api/contacts/:id` | ubah / hapus |
| DELETE | `/api/contacts/bulk` | `{ids:[1,2,3]}` |
| POST | `/api/contacts/import` | `{contacts:[{name,phone,email?,tags?}]}` |
| POST | `/api/contacts/validate` | `{deviceId, numbers[]}` |
| POST | `/api/contacts/sync` | sinkron kontak dari phonebook device |

Grup kontak: `GET/POST /api/contact-groups` (`{name, description?, color?}`), `PUT/DELETE /:id`, `POST /bulk-delete {ids}`, `GET /export` (CSV), `GET /:id/members`, `POST /:id/members {contactIds}`, `DELETE /:id/members/:memberId`, `GET|PUT /:id/welcome-dm` (`{enabled, template}`).

---

## 11. Grup WhatsApp

| Method | Path | Body/Query | Keterangan |
|---|---|---|---|
| GET | `/api/groups?deviceId=1` | — | → `{groups:[{jid,name,participantCount,topic}]}` |
| POST | `/api/groups` | `{deviceId, name, participants[]}` | buat grup; nama ≤ 25 karakter; ≤ 100 peserta |
| POST | `/api/groups/:jid/participants` | `{deviceId, action:"add"\|"remove", participants[]}` | ubah peserta |
| PATCH | `/api/groups/:jid` | `{deviceId, name?, topic?}` | ubah nama/topik |

Untuk melihat ID grup (`@g.us`) dari dashboard: Kontak → Grup → tombol **ID Grup**.

---

## 12. Jadwal & Berulang

### Sekali jalan — `/api/schedule`

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/schedule?status=` | daftar (`{schedules:[...]}`) |
| POST | `/api/schedule` | buat |
| PUT/DELETE | `/api/schedule/:id` | ubah/hapus |
| PATCH | `/api/schedule/:id/cancel` | batalkan |

Body create:

```json
{ "deviceId": 1, "to": "6289876543210", "type": "text", "content": "Pengingat meeting", "mediaUrl": "", "sendAt": "2026-11-21T09:00:00+07:00" }
```

`sendAt` **wajib RFC3339** dan harus masa depan. Status: `pending, sending, sent, failed, cancelled`.

### Alias + kontrol — `/api/schedules`

`GET ""`, `DELETE /:id`, `PATCH /:id/cancel`, dan `PATCH /:id {"action":"pause"|"resume"|"cancel"}`.

### Berulang — `/api/recurring-schedules`

`GET/POST/PUT/DELETE`, `POST /:id/toggle` (aktif/nonaktif).

---

## 13. Files

Grup `/api/files` — scope `files`. Media tersimpan lokal di server (di-serve dari `/uploads/...`).

| Method | Path | Fungsi |
|---|---|---|
| POST | `/api/files` | upload multipart, field `file` |
| GET | `/api/files` | → `{files,total}` |
| GET | `/api/files/:id/content` | akses isi file |
| DELETE | `/api/files/:id` | hapus |

Batas: **16 MB**; tipe diizinkan: `jpg, png, gif, webp, mp4, mp3, ogg, pdf, doc, docx` (dideteksi dari isi file, bukan Content-Type). Respons `201 {file:{id,userId,fileName,originalName,mime,size,createdAt}}` — pakai `id` sebagai `fileId` di kirim pesan.

---

## 14. Live Chat

Grup `/api/chat` — scope `chat`. Untuk membangun inbox CS sendiri.

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/chat/conversations` | daftar percakapan |
| GET | `/api/chat/unread-count` | jumlah belum dibaca |
| GET | `/api/chat/history` | riwayat ringkas |
| GET | `/api/chat/messages/:phone` | pesan per kontak → `{messages,phone}` |
| POST | `/api/chat/send` | kirim balasan |
| POST | `/api/chat/ai-reply` | `{deviceId, phone, botId?}` — generate balasan AI |
| PATCH | `/api/chat/conversations/:phone/mode` | `{mode:"manual"\|"ai"\|"hybrid"}` |
| PATCH | `/api/chat/conversations/:phone/read` | tandai dibaca |
| PATCH | `/api/chat/conversations/:phone/pin` | `{pinned:true}` |
| PATCH | `/api/chat/conversations/:phone/status` | `{status:...}` (open/pending/resolved) |
| GET | `/api/chat/profile-pic?phone=` | foto profil |
| GET | `/api/chat/export` | ekspor |
| GET/POST/DELETE | `/api/chat/reminders`, `/api/chat/reminders/:id` | pengingat |
| POST | `/api/chat/upload` | upload media utk chat |
| POST | `/api/chat/react` | reaksi emoji |
| DELETE | `/api/chat/messages/:id` | hapus pesan |
| GET | `/api/chat/media` | unduh media masuk |
| POST | `/api/chat/presence/subscribe` | subscribe status online/typing |
| GET | `/api/chat/presence` | baca status |

**POST /api/chat/send:**

```json
{
  "deviceId": 1, "phone": "6289876543210",
  "content": "Baik, pesan Anda kami proses",
  "type": "text",           // text|image|video|document|audio|voicenote
  "replyTo": "", "mediaUrl": "", "fileName": ""
}
```

`content` boleh kosong bila kirim media. Tugas CS multi-agent (assign/catatan/aktivitas) ada di `/api/chat/conversations/:phone/assign|notes|activity`, `/api/chat/team-stats`.

---

## 15. Webhook Keluar (outgoing)

Dua lapis: **webhook user** (CRUD di `/api/webhooks`, scope `webhooks`) dan **webhook per-device** (field `webhookUrl`/`webhookEnabled`/`webhookSecret` di device — kirim ke URL itu untuk event device tersebut; URL berakhiran `/whatsapp/bot` memakai format bot PPOB lama).

### Event yang dipicu

| Event | Kapan | Payload |
|---|---|---|
| `message.received` | pesan masuk | `{from, senderJID, pushName, text, type, timestamp}` |
| `message.sent` | pesan keluar sukses | `{to, messageId, type, timestamp}` |
| `message.failed` | gagal kirim | `{to, type, error, timestamp}` |
| `message.delivered` / `message.read` | receipt WhatsApp | `{messageId, dbId, to, status}` |
| `subscription.activated` | langganan aktif | dari billing |
| `order.created` | order dibuat | (route order masih stub) |
| `webhook.test` | uji manual | `{test:true}` |

`events` kosong pada webhook = **langganan semua event**.

### Envelope & header

```json
{
  "event": "message.received",
  "device_id": 1,
  "payload": { "from": "6289876543210", "senderJID": "6289876543210@s.whatsapp.net", "pushName": "Budi", "text": "Halo", "type": "text", "timestamp": 1764000000 },
  "sent_at": "2026-11-20T03:04:05Z"
}
```

Header kirim:

| Header | Isi |
|---|---|
| `Content-Type` | `application/json` |
| `X-Webhook-Event` | nama event |
| `X-Webhook-Secret` | secret webhook (plaintext) |
| `X-Wagataway-Signature` | `sha256=<HMAC-SHA256 hex dari RAW BODY memakai secret>` — **verifikasi ini** |
| `X-Wagataway-Timestamp` | unix epoch (anti-replay) |

Timeout 10 detik, tanpa ikuti redirect (anti-SSRF). Verifikasi contoh (Node.js):

```js
const expected = "sha256=" + crypto.createHmac("sha256", secret).update(rawBody).digest("hex");
crypto.timingSafeEqual(Buffer.from(expected), Buffer.from(sigHeader));
```

### CRUD

| Method | Path | Fungsi |
|---|---|---|
| GET/POST | `/api/webhooks` | daftar / buat `{url, secret?, events?, deviceId?}` |
| PUT/DELETE | `/api/webhooks/:id` | ubah / hapus |
| GET | `/api/webhooks/:id/deliveries` | 20 pengiriman terakhir |
| POST | `/api/webhooks/:id/deliveries/:deliveryId/retry` | kirim ulang payload tersimpan |
| POST | `/api/webhooks/:id/test` | kirim `webhook.test` |
| GET | `/api/webhook-deliveries` | log lintas webhook |
| POST | `/api/webhook-deliveries/:id/retry` | retry by delivery id |

`events` menerima array JSON, string tunggal, atau nama event; event tak dikenal ditolak 400.

---

## 16. Webhook Masuk (inbound) & Integration Hub

Untuk sistem luar **memicu kirim pesan** tanpa API key — auth via secret/token di URL.

### 16.1 Inbound webhook (HMAC)

**Publik:** `POST /api/inbound-webhooks/:id/hook`

```json
{ "to": "6289876543210", "message": "Pesanan #99 siap diambil" }
```

- Auth: header `X-Wagataway-Signature` (HMAC-SHA256 raw body dengan secret inbound) **atau** `X-Webhook-Secret` (sama persis dengan secret).
- Bila tidak ada `message`, template inbound di-render dari payload: `{{field}}` diganti nilai field level-atas (mis. `"Halo {{nama}}, order {{order_id}} siap"`).
- Rate limit 60 req/menit per webhook; body ≤ 1 MB; kuota user berlaku.
- Respons sukses: `{"success":true,"message":"Pesan terkirim","to":"...","messageId":"3EB0..."}`.

**Manajemen** (auth user): `GET/POST /api/inbound-webhooks` (`{name, deviceId, template?}`), `GET/PUT/DELETE /:id`, `POST /:id/regenerate`, `GET /:id/logs`. Secret penuh (`fullSecret`) hanya tampil sekali saat create/regenerate.

### 16.2 Integration Hub inbox (token di URL)

**Publik:** `POST /api/integrations/inbox/:token` — kontrak sama seperti inbound (`to` + `message`/template), rate limit 60/menit per token, token salah = 404 generik. Manajemen: `GET/POST /api/integrations` (`{name, platform, deviceId, template?}`) + `GET/PUT/DELETE /:id`, `POST /:id/regenerate`, `GET /:id/logs`.

---

## 17. Kuota & Billing

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/quota` atau `/api/billing/usage` | sisa kuota bulan ini |
| GET | `/api/billing/plans` | daftar paket |
| GET | `/api/billing/subscription` | status langganan |
| GET | `/api/billing/prorate` | hitung prorata upgrade |
| POST | `/api/billing/subscribe` | beli/perpanjang |
| GET | `/api/billing/transactions` , `/:id`, `/:id/invoice.pdf` | riwayat & invoice |
| POST | `/api/billing/voucher/redeem` | tukar voucher |
| GET | `/api/billing/voucher/redemptions` | riwayat penukaran |

Respons `GET /api/quota`:

```json
{
  "planName": "Bisnis", "quota": 10000, "usedThisMonth": 1523, "remaining": 8477,
  "limit": 10000, "isUnlimited": false, "percentUsed": 15, "warning": false,
  "isTrial": false, "quotaExceeded": false,
  "subState": "active", "graceDaysLeft": 0, "graceUsedToday": 0, "graceDailyLimit": 0
}
```

Penolakan saat kirim (bentuk lengkap menyertakan objek `quota` & `subscription`):

| Kondisi | HTTP | code |
|---|---|---|
| kuota bulanan habis | 429 | `QUOTA_EXCEEDED` |
| langganan kedaluwarsa | 403 | `SUBSCRIPTION_EXPIRED` |
| batas harian masa grace tercapai | 403 | `SUBSCRIPTION_GRACE_LIMIT` |

Webhook pembayaran (publik, HMAC): `POST /api/billing/clipkupay/webhook` — header `X-Signature` = HMAC-SHA256 raw body dengan merchant API key.

---

## 18. Auth & Akun

| Method | Path | Rate | Fungsi |
|---|---|---|---|
| POST | `/api/auth/login` | 10/menit/IP | `{email,password}` |
| POST | `/api/auth/register` | 10/menit/IP | `{name,email,password,referralCode?}` (nama ≥2, password ≥6) |
| GET/PATCH | `/api/auth/me` | — | profil / ubah profil |
| POST | `/api/auth/change-password` | — | ganti password |
| POST | `/api/auth/logout` | — | cabut sesi |
| POST | `/api/auth/forgot-password` | — | minta link reset |
| POST | `/api/auth/reset-password` | — | reset dengan token |
| POST | `/api/auth/2fa/verify` | — | verifikasi kode 2FA (dengan `twofaToken` pending) |
| GET | `/api/auth/oauth/status` · `/api/auth/oauth/:provider` · `/:provider/callback` | — | login Google dll. |
| POST | `/api/auth/team-login` | 10/menit/IP | login anggota tim |

**Login sukses:**

```json
{
  "token": "eyJ...",
  "user": { "id": 5, "name": "Nama", "email": "a@b.c", "role": "user", "plan": "bisnis", "avatar": "" }
}
```

**2FA aktif** → `{"requires2FA":true,"twofaToken":"..."}` (token pending 5 menit, tidak bisa akses route lain sampai `POST /api/auth/2fa/verify`).

2FA manajemen: `GET /api/2fa/status`, `POST /api/2fa/setup|enable|disable|backup-codes/regenerate`. Sesi: `GET /api/sessions`, `DELETE /api/sessions/:id`, `POST /api/sessions/revoke-others`. Tim: `GET/POST/PUT/DELETE /api/team`, `POST /api/team/:id/toggle`.

---

## 19. SSE (realtime)

EventSource tidak bisa kirim header Authorization → pakai **ticket sekali pakai**:

```
1) POST /api/sse/ticket        (auth Bearer/API key)  →  {"ticket":"...","expiresIn":120}
2) GET  /api/sse/stream?ticket=...                    →  text/event-stream
```

Event pertama `connected {clientId}`; heartbeat tiap 60 detik (`: keepalive`). Dukungan `?token=` (JWT di URL) **sengaja dihapus** agar token tidak bocor ke log.

---

## 20. Analytics & Laporan

| Method | Path | Fungsi |
|---|---|---|
| GET | `/api/analytics/overview` · `/messages` · `/funnel` · `/devices` · `/campaigns` · `/response-time` | dasbor analitik |
| GET | `/api/analytics/export` | ekspor data |
| GET | `/api/stats/overview` | statistik umum |
| GET | `/api/reports/campaigns/:id` | laporan per campaign blast |
| GET | `/api/reports/summary` | ringkasan laporan |
| GET | `/api/public/plans` , `/api/public/stats` | publik (landing page) |
| GET | `/api/l/:code` | redirect short link |

---

## 21. Endpoint Lain (tabel ringkas)

Semua butuh auth; pola CRUD standar (`GET` daftar, `POST` buat, `PUT/PATCH /:id` ubah, `DELETE /:id` hapus).

| Grup | Path | Fungsi |
|---|---|---|
| Auto-reply | `/api/auto-reply` (+`PATCH /:id/toggle`) | balasan keyword exact/contains/startsWith + jadwal |
| AI reply | `/api/ai-reply` (+toggle, `/status`) | aturan AI per device/kontak |
| AI connection | `/api/ai-connection` (+`/test`, `/vision-check`) | kredensial provider AI (OpenAI-compatible) |
| AI search | `/api/ai-search` (+`/test`) | koneksi "Cari di Web" |
| AI chatbot FAQ | `/api/ai-chatbot` (+`/status`,`/faqs`,`/logs`) | chatbot + FAQ |
| Menu bot | `/api/menu-bots` (+`/id/items`, `/id/sessions`) | menu bertingkat interaktif |
| Canned response | `/api/canned-responses` | template balasan cepat |
| Drip campaign | `/api/drip` (+steps, enroll, `/events`, analytics, pause/resume/cancel) | sekuens pesan berjangka |
| Follow-up | `/api/followups` (+toggle) | tindak lanjut otomatis |
| Group rules | `/api/group-rules` (+toggle) | aturan moderasi grup |
| Blacklist | `/api/blacklist` | nomor diblokir |
| Short link | `/api/links` (+`/id/stats`) | pemendek + statistik |
| Notification | `/api/notifications` (+`/id/read`, `/read-all`) | notifikasi in-app |
| Template | `/api/templates` (+`/meta-config`, history, submit/approve/reject, `/send`) | template pesan |
| Bot katalog | `/api/bot-products`, `/api/bot-orders` | katalog & pesanan via bot |
| Chat backup | `/api/chat/backups` (+`/id/download`) | cadangan chat |
| Label chat | `/api/chat-labels`; assign: `/api/chat-assignments` | label & penugasan |
| Google Sheets | `/api/sheets` (+connection, `/test`, `/sync`, `/syncs`) | sinkron kontak/order |
| Affiliate | `/api/affiliate` (+earnings, rewards) | program afiliasi |

**Admin** (`/api/admin/*` — **JWT saja, API key ditolak**): users, packages, vouchers (+`/bulk`, `/analytics`), settings, analytics, trends, transactions, notifications, maintenance, activity-logs, health, billing gateway, broadcast-wa.

---

## 22. Endpoint Stub — JANGAN DIPAKAI

Endpoint berikut terdaftar tapi **belum berfungsi** (selalu balas `200 {"message":"... endpoint ready","code":"OK"}`) — jangan diandalkan agent:

- `POST /api/v1/send`, `POST /api/v1/send-bulk`, `GET /api/v1/device/status` (publik API v1 — placeholder)
- `GET /api/dashboard`, `GET /api/dashboard/stats`
- `/api/plugins/*`, `/api/cs-bot/*`, `/api/anti-banned/*`
- `POST /api/auth/otp/send`, `POST /api/auth/otp/verify`

Gunakan padanan aslinya (mis. statistik → `/api/analytics/overview` atau `/api/stats/overview`).

---

## 23. Playbook Agent: Resep Integrasi Umum

### A. Setup sekali (per instalasi)

```
1. POST /api/auth/login                    → JWT
2. POST /api/api-keys {scopes:["messages:send","messages:read"]} → simpan apiKey
3. POST /api/devices {"name":"Gateway-1"}  → deviceId
4. POST /api/devices/:id/connect → GET /:id/qr → user scan
5. GET /api/devices/:id/status sampai "connected"
```

### B. Kirim notifikasi/OTP (dengan anti-dobel)

```
POST /api/messages/send
{ deviceId, to, type:"text", content:"Kode OTP Anda: 4821", idempotencyKey:"otp-user42-login-1732080000" }
→ poll GET /api/messages/:id/status  hingga status sent/failed
```

Retry aman: ulangi POST dengan `idempotencyKey` sama → respons `duplicate:true`, tidak kirim dobel.

### C. Terima pesan masuk → balas otomatis

```
1. POST /api/webhooks {url:"https://app.kamu.com/wa-hook", secret:"..."}  (events kosong = semua)
2. Di /wa-hook: verifikasi X-Wagataway-Signature (HMAC-SHA256 raw body)
3. event message.received → business logic → POST /api/chat/send atau /api/messages/send
```

### D. Blast campaign dengan validasi

```
POST /api/messages/check-recipients → buang excluded
POST /api/messages/send-bulk {autoClean:true, minDelay:5, maxDelay:15}
GET  /api/messages/bulk-jobs → pantau job.progress / status penerima
POST /api/messages/bulk-jobs/:id/retry  (bila ada gagal)
```

### E. Memicu kirim dari sistem eksternal TANPA API key

Buat inbound webhook → beri `id` + `fullSecret` ke sistem eksternal → mereka `POST /api/inbound-webhooks/:id/hook` dengan `X-Webhook-Secret`.

### Aturan main agent

- **Polling QR**: interval 2–3 detik cukup; QR kedaluwarsa ± 20 detik, endpoint balas `404 NO_QR` saat regenerasi.
- **429 TOO_MANY_REQUESTS / QUOTA_EXCEEDED**: berhenti, tunggu jendela lewat (rate) atau upgrade paket (kuota) — jangan spam retry.
- Timeout client: 15 detik per panggilan kirim; jalankan di background job, jangan blok request utama.
- Selalu **POST** di endpoint compat (GET membocorkan api_key di access log).
- Nomor gagal permanen: cek dulu `/check-number` bila ragu (`exists:false` = tidak terdaftar WA).
- Semua pesan keluar tercatat di Live Chat (`direction: outgoing`) — tidak ada jalur kirim "senyap".

---

## 24. Katalog Kode Error

| code | HTTP | Arti |
|---|---|---|
| `VALIDATION_ERROR` | 400 | field tidak valid/kurang |
| `NOT_FOUND` | 404 | resource tidak ada / bukan milik user |
| `UNAUTHORIZED` | 401 | token tidak ada |
| `INVALID_TOKEN` / `INVALID_TOKEN_FORMAT` | 401 | JWT salah/kedaluwarsa / format salah |
| `SESSION_REVOKED` | 401 | sesi dicabut, login ulang |
| `INVALID_API_KEY` / `API_KEY_EXPIRED` | 401 | API key salah / kedaluwarsa |
| `ACCOUNT_DISABLED` | 401 | akun banned/suspended |
| `ACCOUNT_LOCKED` | 429 | brute-force login, tunggu menit yang disebut |
| `ACCOUNT_NOT_FOUND` | 401 | akun terhapus |
| `2FA_REQUIRED` | 401 | verifikasi 2FA belum selesai |
| `INSUFFICIENT_SCOPE` | 403 | API key tidak punya scope endpoint |
| `API_KEY_NOT_ALLOWED` | 403 | endpoint admin (JWT only) |
| `QUOTA_EXCEEDED` | 429 | kuota pesan bulanan habis |
| `SUBSCRIPTION_EXPIRED` | 403 | langganan berakhir |
| `SUBSCRIPTION_GRACE_LIMIT` | 403 | batas harian masa grace |
| `TOO_MANY_REQUESTS` | 429 | rate limit IP |
| `BULK_LIMIT_REACHED` | 400 | penerima > maxBulk paket (ada field `limit`) |
| `CONTACT_LIMIT_REACHED` | 403 | batas kontak paket |
| `NO_DEVICE_CONNECTED` | 400 | blast tanpa device online |
| `NO_VALID_NUMBERS` | 400 | autoClean: semua nomor tak terdaftar |
| `NO_QR` | 404 | QR belum tersedia/regenerasi |
| `PAIR_FAILED` | 502 | kode pairing gagal dibuat |
| `REVOKE_FAILED` | 400 | tarik pesan gagal |
| `INVALID_SIGNATURE` | 401 | HMAC/secret inbound webhook salah |
| `INVALID_TOKEN` (inbox) | 404 | token integrasi salah |
| `RATE_LIMITED` (inbox) | 429 | > 60/menit per token |
| `DEVICE_OFFLINE` | 503 | device inbound tidak connected |
| `SEND_FAILED` | 502 | WhatsApp menolak kirim |
| `WA_ERROR` | 400 | kesalahan umum lapisan WhatsApp |
| `DB_ERROR` | 500 | gagal database |
| `MAINTENANCE` | 503 | mode maintenance aktif |

---

*Dokumen digenerate dari kode sumber SlackerSender-WAGW. Bila endpoint berubah, regenerasi dari `go-backend/internal/handler/`.*
*Copyright © 2026 Duta Corpora Indonesia — All rights reserved.*
