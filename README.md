# SlackerSender — WhatsApp API Gateway Indonesia

[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go 1.26](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](go-backend/go.mod)
[![React](https://img.shields.io/badge/React-Vite-61DAFB?logo=react)](go-backend/web/package.json)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-16-336791?logo=postgresql)](go-backend/docker-compose.yml)

**WhatsApp Gateway + REST API self-hosted untuk bisnis dan developer Indonesia.** Satu binary Go menyajikan dashboard React dan seluruh API — tanpa Node.js di server produksi.

> **Coba langsung:** [wasender.slackercode.us](https://wasender.slackercode.us)
>
> Repo ini: [github.com/alecslacker/SlackerSender-WAGW](https://github.com/alecslacker/SlackerSender-WAGW)

## Daftar Isi

- [Tentang](#tentang)
- [Fitur](#fitur)
- [Tech Stack](#tech-stack)
- [Quick Start](#quick-start)
- [Deploy Production](#deploy-production)
- [API](#api)
- [Arsitektur](#arsitektur)
- [Keamanan](#keamanan)
- [Kemandirian Dependensi](#kemandirian-dependensi)
- [Konfigurasi](#konfigurasi)
- [Struktur Proyek](#struktur-proyek)
- [Berkontribusi](#berkontribusi)
- [Disclaimer](#disclaimer)
- [Lisensi & Atribusi](#lisensi--atribusi)

## Tentang

SlackerSender adalah WhatsApp gateway self-hosted: hubungkan nomor WhatsApp lewat QR atau kode pairing, lalu kirim pesan, jalankan campaign, balas pelanggan otomatis, dan kelola semuanya dari dashboard atau REST API.

Yang membedakan:

- **Satu binary** — backend Go + frontend React disajikan dari satu proses. Tidak perlu Node.js di server.
- **Multi-device** — banyak nomor WhatsApp dalam satu instalasi, dengan rotasi pengirim dan failover saat blast.
- **SaaS-ready** — paket langganan, kuota, QRIS, voucher, dan panel admin.
- **Kompatibel dengan API Slackersender** — 5 endpoint kirim/cek dengan `api_key` di body untuk sistem yang sudah ada.
- **Bilingual** — antarmuka Bahasa Indonesia dan English.
- **Mandiri** — QR dan font dirender lokal; AI memakai endpoint sendiri. Lihat [Kemandirian Dependensi](#kemandirian-dependensi).

## Fitur

### Pesan & Blast

- **Multi-device** — hubungkan banyak nomor (QR / kode pairing), pilih pengirim default.
- **Kirim pesan** — teks, gambar, dokumen, video, stiker, voice note, lokasi, polling, dan tombol interaktif.
- **Blast massal** — jeda acak antar pengiriman, rotasi round-robin, failover antar device.
- **Validasi nomor** — normalisasi & pengecekan nomor sebelum blast.
- **Webhook pesan** — `message.sent/delivered/read/failed` dengan tanda tangan HMAC-SHA256.

### Otomatisasi

- **Auto reply** — keyword (exact/contains/startsWith) + jadwal aktif.
- **Chatbot menu** — menu bertingkat dengan session per pengguna.
- **AI reply** — OpenAI-compatible (OpenAI, Anthropic, atau endpoint sendiri).
- **Welcome grup & DM** — sambutan otomatis anggota baru.
- **Rekap polling** — rekap hasil polling otomatis.

### Live Chat & CS

- **Live chat** — balas chat pelanggan manual, kelola label dan assign.
- **Presence real-time** — indikator online/mengetik via SSE.
- **CS bot dua arah** — webhook ke sistem eksternal dengan shared secret (`WAMP_BOT_SECRET`).

### Kontak & Grup

- **Kontak & grup** — import batch, blacklist, sinkronisasi dari WhatsApp.
- **Validasi kontak** — cek nomor terdaftar sebelum mengirim.

### Penjadwalan

- **Scheduled messages** — kirim terjadwal satu kali.
- **Drip campaign** — sequence otomatis dengan delay per step.

### Integrasi

- **Integration Hub** — konektor untuk Google Forms, Google Sheets, WooCommerce, WordPress, Shopify, Typeform, Tally, Jotform, Zapier, Make, Pabbly, IFTTT, Stripe, Midtrans, Xendit, Telegram, Discord, Slack, GitHub, Trello, Notion, Airtable, HubSpot.
- **Webhooks** — event per-device (on/off) + HMAC-SHA256, log pengiriman.
- **API Slackersender** — endpoint root kompatibel untuk sistem lama.

### Billing & Langganan

- **Paket** — Free, Lite, Regular, Pro, Master (`go-backend/seed_plans.sql`, aman dijalankan ulang).
- **QRIS** — pembayaran via Clipku Pay; QR dirender lokal di server.
- **Voucher** — kode trial/paket, penukaran, dan riwayat.
- **Transaksi** — catatan pembayaran lengkap.

### Produktivitas UI

- **Command palette** — `Ctrl/⌘+K` untuk lompat halaman dan aksi cepat.
- **Onboarding 3 langkah** — hubungkan device → kirim pesan tes → selesai.
- **Tema terminal** — hitam + hijau, font mono, animasi halus.
- **Bilingual penuh** — pilihan bahasa tersimpan per user.

### Admin

- **Panel admin** — pengguna, paket, voucher, transaksi, log, kesehatan sistem, notifikasi.
- **Maintenance mode** — toggle lewat panel atau API.
- **make-admin** — angkat admin pertama lewat CLI.

## Tech Stack

| Layer | Teknologi |
|-------|-----------|
| Backend | Go 1.26 + Gin |
| WhatsApp | whatsmeow (native Go, tanpa Node.js) |
| Database | PostgreSQL 16 + GORM + Row Level Security |
| Frontend | React + Vite + TailwindCSS v4 + Kumo UI |
| Font | Self-host via `@fontsource` (tanpa CDN) |
| AI | Endpoint OpenAI-compatible (default: 9router milik sendiri) |
| Auth | JWT + 2FA/TOTP + Google OAuth |
| Real-time | SSE (Server-Sent Events) |
| Pembayaran | QRIS via Clipku Pay |
| Deploy | Binary tunggal / Docker Compose / systemd |

## Quick Start

Prasyarat: Go 1.26+ dengan CGO, Node.js 22+ dan pnpm (khusus dev frontend), PostgreSQL 16, dan `psql`.

```bash
cd go-backend

# 1. Siapkan environment
cp .env.example .env
# Isi DATABASE_URL dan JWT_SECRET (min. 32 karakter, acak).
# JWT_SECRET dengan nilai default membuat server berhenti — ini disengaja.

# 2. Buat database
createdb wagataway
psql "$DATABASE_URL" -f seed_plans.sql

# 3. Jalankan backend (auto-migrate)
CGO_ENABLED=1 go run ./cmd/server
# atau: make dev

# 4. Jalankan frontend di terminal lain
cd web && pnpm install && pnpm dev
```

Buka `http://localhost:5173` untuk dashboard. API di `http://localhost:8080/api` (atau sesuai `PORT`).

CGO wajib karena whatsmeow memakai `mattn/go-sqlite3` untuk sesi per-device.

### Opsi Docker Compose

```bash
cd go-backend
docker compose up --build -d
```

Compose menyediakan app, PostgreSQL 16, dan Redis. Port `5432` tidak diekspos ke publik. `JWT_SECRET`, `APP_NAME`, dan variabel AI **tidak** diisi oleh compose — export atau tambahkan lewat environment sebelum menjalankan; sebaiknya ubah juga password PostgreSQL bawaan di `docker-compose.yml`.

## Deploy Production

Binary dan `web/dist` disajikan oleh satu proses Go. Panduan lengkap (tanpa kredensial) ada di [`.trae/documents/deploy-production.md`](.trae/documents/deploy-production.md).

```bash
cd go-backend
CGO_ENABLED=1 go build -ldflags="-s -w" -o server ./cmd/server
cd web && pnpm build     # menghasilkan web/dist
cd .. && ./server        # serve API + dashboard
```

Hal yang mudah terlewat:

- **SPA fallback hanya didaftarkan bila `web/dist/index.html` ada saat startup.** Sinkronkan `dist` sampai lengkap, lalu restart server.
- **`web/dist` ikut ter-commit** di repo ini, supaya hasil build yang dijalankan produksi selalu cocok dengan sumbernya.
- **Bind ke loopback** bila ada reverse proxy: `LISTEN_HOST=127.0.0.1`. Kosong = semua antarmuka (default, cocok untuk Docker). Ini mencegah port aplikasi terjangkau langsung sehingga header `X-Real-IP` dari proxy tidak bisa dipalsukan.
- **PostgreSQL harus dijalankan dengan role non-superuser.** RLS tidak berlaku untuk superuser, jadi isolasi data antar-user tidak terjamin bila memakai `postgres`.
- Pakai reverse proxy (nginx) untuk HTTPS dan arahkan ke `127.0.0.1:8080`. Contoh unit systemd dan vhost ada di dokumen deploy.

### Membuat admin pertama

Registrasi selalu menghasilkan akun biasa — sengaja tidak ada aturan "pendaftar pertama jadi admin", karena registrasi tanpa verifikasi email bisa direbut orang lain.

```bash
./server make-admin nama@domain.com
# Docker: docker compose exec app ./server make-admin nama@domain.com
```

Menu Admin muncul setelah halaman di-refresh, tanpa login ulang. Role dan status (`banned` / `suspended`) dibaca dari database di setiap request, sehingga perubahan berlaku seketika. Admin berikutnya dikelola dari panel Admin.

### Pagar dependensi luar

```bash
cd go-backend
make check-hosts          # periksa host di sumber
make check-hosts ARGS=--dist   # termasuk hasil build (lihat Makefile/release.yml)
```

Daftar yang diizinkan ada di `scripts/external-hosts.allowlist`.

## API

Base URL: `{APP_URL}/api`. Autentikasi dashboard memakai Bearer token (JWT); API eksternal memakai API key dengan scope atau Bearer token.

### Endpoint kompatibel Slackersender (root, bukan `/api`)

Api key dikirim di body (`api_key`) atau header `X-API-Key`. Mendukung JSON dan form-urlencoded, GET dan POST.

| Method | Path | Scope | Parameter utama |
|--------|------|-------|-----------------|
| POST/GET | `/send-message` | `messages:send` | `api_key`, `number`, `message`, `sender?`, `reply_to?` |
| POST/GET | `/send-media` | `messages:send` | `api_key`, `number`, `url`, `media_type?`, `caption?` |
| POST/GET | `/send-location` | `messages:send` | `api_key`, `number`, `latitude`, `longitude` |
| POST/GET | `/info-device` | `devices:read` | `api_key`, `number` (nomor/ID/nama device) |
| POST/GET | `/check-number` | `contacts` | `api_key`, `number`, `sender?` |

`sender` boleh ID device, nomor HP, atau nama device; kosong = device default. `media_type`: `image` (default), `video`, `audio`, `document`. Respons mengantre pesan (`status: true` berarti diterima untuk dikirim, bukan sudah terkirim ke WhatsApp). Contoh lengkap ada di halaman **API Docs** di dashboard.

### Publik

| Method | Path | Deskripsi |
|--------|------|-----------|
| POST | `/api/auth/login` | Login (mendukung 2FA) |
| POST | `/api/auth/register` | Register |
| POST | `/api/auth/forgot-password` | Minta link reset |
| POST | `/api/auth/reset-password` | Reset password |
| GET | `/api/l/:code` | Alihkan short link |
| POST | `/api/billing/clipkupay/webhook` | Webhook pembayaran (X-Signature) |

### Dilindungi (Bearer token atau API key)

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET | `/api/devices` | Daftar perangkat |
| POST | `/api/devices/:id/connect` | Hubungkan WhatsApp |
| GET | `/api/devices/:id/qr` | QR code (dirender lokal) |
| POST | `/api/devices/:id/pair-code` | Kode pairing |
| GET | `/api/devices/:id/status` | Status perangkat |
| POST | `/api/messages/send` | Kirim pesan |
| POST | `/api/messages/send-bulk` | Blast pesan |
| GET | `/api/messages/:id/status` | Status pesan |
| GET | `/api/contacts` | Daftar kontak |
| POST | `/api/contacts/sync` | Sinkron kontak dari WA |
| POST | `/api/contacts/validate` | Validasi nomor |
| GET | `/api/contact-groups` | Daftar grup |
| POST | `/api/contact-groups/sync` | Sinkron grup dari WA |
| GET | `/api/chat/conversations` | Daftar percakapan |
| POST | `/api/chat/send` | Balas chat manual |
| GET | `/api/chat/history` | Riwayat chat |
| GET | `/api/ai-reply` | Konfigurasi AI reply |
| GET | `/api/auto-reply` | Aturan auto reply |
| GET | `/api/menu-bots` | Chatbot menu |
| GET | `/api/drip` | Drip campaign |
| GET | `/api/schedule` | Pesan terjadwal |
| GET | `/api/analytics/overview` | Statistik |
| GET | `/api/billing/plans` | Daftar paket |
| POST | `/api/billing/subscribe` | Buat pembayaran QRIS |
| POST | `/api/billing/voucher/redeem` | Tukar voucher |
| GET | `/api/api-keys` | Kelola API key |
| GET | `/api/sessions` | Sesi aktif |
| GET | `/api/integrations` | Integration Hub |
| GET | `/api/stream` | SSE real-time |

### Admin (`/api/admin/*`)

| Method | Path | Deskripsi |
|--------|------|-----------|
| GET/POST | `/api/admin/users` | Kelola pengguna |
| GET | `/api/admin/analytics` | Analitik global |
| GET | `/api/admin/transactions` | Semua transaksi |
| PUT | `/api/admin/maintenance` | Toggle maintenance |

Daftar lengkap dan contoh curl tersedia di halaman **API Docs** (`/api-docs`) dan **Playground** (`/api-playground`) pada dashboard.

## Arsitektur

```
Satu binary Go
├── HTTP Server (Gin) ─── REST API + SSE + static files + SPA fallback
├── WhatsApp Manager ──── sesi whatsmeow per-device (SQLite)
├── Background Workers ── scheduler (bulk, drip, scheduled, backup chat)
├── AI Service ────────── endpoint OpenAI-compatible
├── RLS Layer ─────────── PostgreSQL Row Level Security per user
└── web/dist ──────────── dashboard React yang disajikan langsung
```

Tidak ada microservice wajib. `motion/` adalah project Remotion terpisah yang hanya dipakai untuk merender video hero dan tidak dibutuhkan saat runtime.

## Keamanan

- **2FA/TOTP** — verifikasi dua langkah + kode cadangan sekali pakai.
- **Lockout login** — kunci 15 menit setelah 5 kali gagal; rate limiter global, publik, dan auth.
- **Notifikasi IP baru** — peringatan login dari IP asing (dedup).
- **Manajemen sesi** — lihat dan cabut sesi aktif per perangkat.
- **API key** — scope (`full`, `messages:send`, `messages:read`, `contacts`, `devices:read`, `devices:write`, `chat`, `files`, `webhooks`), expiry, rotasi.
- **Webhook** — tanda tangan HMAC-SHA256.
- **RLS** — lapisan kedua di atas filter aplikasi; wajib pakai role database non-superuser.
- **Enkripsi** — kredensial AI dan webhook dienkripsi AES-256-GCM; kunci dari `AI_KEYS_SECRET` atau `JWT_SECRET` (fail-closed).
- **CSP** — `script-src`/`connect-src 'self'` untuk halaman web (kecuali `/api/`).
- **Backup chat** — cadangan percakapan harian dengan retensi.

Laporkan celah keamanan lewat [GitHub Issues](https://github.com/alecslacker/SlackerSender-WAGW/issues). Jangan mempublikasikan detail sebelum diperbaiki.

## Kemandirian Dependensi

Proyek ini sengaja mengurangi ketergantungan ke pihak ketiga:

- **QR WhatsApp** dirender di backend. Kode QR tidak dikirim ke layanan pembuat QR luar.
- **Font** self-host lewat `@fontsource`; tidak ada permintaan ke Google Fonts.
- **AI** memakai endpoint OpenAI-compatible yang bisa diarahkan ke server sendiri. Default bawaan: `https://9router.dutacorpora.co.id/v1` dengan model `ag/gemini-3.8-flash-low`. Ganti lewat `AI_DEFAULT_BASE_URL` dan `AI_DEFAULT_MODEL` bila memindah server. Base URL divalidasi untuk mencegah SSRF.
- **Clipku Pay** tetap dipakai untuk QRIS; base URL bisa diatur lewat `CLIPKUPAY_BASE_URL`.

Perintah `make check-hosts` menggagalkan build bila ada host luar yang tidak tercantum di `scripts/external-hosts.allowlist`.

## Konfigurasi

Semua konfigurasi lewat environment variable; lihat [`go-backend/.env.example`](go-backend/.env.example) untuk daftar lengkap. Ringkasannya:

```env
PORT=8080
LISTEN_HOST=127.0.0.1        # kosong = dengar di semua antarmuka
APP_NAME=SlackerSender
APP_URL=https://wasender.slackercode.us
DATABASE_URL=postgres://user:pass@localhost:5432/wagataway?sslmode=disable
JWT_SECRET=ganti-dengan-string-acak-min-32-karakter
AI_KEYS_SECRET=secret-acak-lainnya
WA_SESSIONS_DIR=./wa-sessions

# AI (OpenAI-compatible)
AI_DEFAULT_BASE_URL=https://9router.dutacorpora.co.id/v1
AI_DEFAULT_MODEL=ag/gemini-3.8-flash-low

# QRIS
CLIPKUPAY_API_KEY=
CLIPKUPAY_BASE_URL=https://m.clipku.com
CLIPKUPAY_WEBHOOK_URL=https://wasender.slackercode.us/api/billing/clipkupay/webhook

# Opsional
OPENAI_API_KEY=
ANTHROPIC_API_KEY=
SMTP_HOST=smtp.gmail.com
SMTP_USER=email@gmail.com
SMTP_PASS=app-password
WAMP_BOT_SECRET=shared-secret
```

Jangan pernah commit `.env` atau secret. Gunakan file env dengan izin `0600` di server.

## Struktur Proyek

```
SlackerSender-WAGW/
├── go-backend/
│   ├── cmd/server/          # Entry point + subcommand make-admin
│   ├── internal/
│   │   ├── config/          # Environment + alamat dengar
│   │   ├── database/        # Koneksi, migrasi, model
│   │   ├── handler/         # HTTP handler (termasuk wamp_compat.go)
│   │   ├── middleware/      # Auth, scope, CORS, rate limit, security headers
│   │   ├── rls/             # Row Level Security via driver pgx
│   │   ├── security/        # Enkripsi kredensial
│   │   ├── whatsapp/        # Manager sesi whatsmeow + rotasi blast
│   │   └── worker/          # Scheduler background
│   ├── scripts/             # Pemeriksa host luar + allowlist
│   ├── web/                 # Dashboard React + web/dist hasil build
│   ├── seed_plans.sql
│   └── Dockerfile / docker-compose.yml / Makefile
├── motion/                  # Project Remotion untuk video hero (opsional)
└── .trae/documents/         # Rencana, panduan deploy dan skrip pendukung
```

## Berkontribusi

Kontribusi diterima — bug report, saran fitur, maupun pull request.

1. Fork repo ini, buat branch dari `main`.
2. Ikuti gaya kode yang ada (Go: `gofmt`; frontend: Prettier).
3. Jangan sertakan secret atau kredensial.
4. Jalankan `go build ./...`, `go vet ./...`, dan `pnpm build` sebelum mengirim perubahan.
5. Buka pull request dengan deskripsi yang jelas.

Pertanyaan dan diskusi: [GitHub Issues](https://github.com/alecslacker/SlackerSender-WAGW/issues).

## Disclaimer

SlackerSender memakai **unofficial WhatsApp Web API** (melalui library whatsmeow) dan **tidak berafiliasi dengan WhatsApp / Meta**. Penggunaan API tidak resmi berisiko dibatasi atau diblokir oleh WhatsApp — gunakan nomor cadangan dan patuhi ketentuan layanan WhatsApp. Perangkat lunak disediakan "sebagaimana adanya".

## Lisensi & Atribusi

MIT — lihat [LICENSE](LICENSE).

Copyright © 2026 Duta Corpora Indonesia — All rights reserved.

Kode awal proyek ini berasal dari fork **WaGataway** oleh Aldi1963 di bawah lisensi MIT. Hak cipta dan teks lisensi asli tetap dipertahankan sesuai ketentuan MIT. Pengembangan lanjutan, rebranding SlackerSender, dan perubahan fitur oleh **Mas Wondho — Duta Corpora Indonesia (DCI)**.
