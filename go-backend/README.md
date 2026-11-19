# SlackerSender — Go Backend

Backend SlackerSender: REST API, WhatsApp via **whatsmeow**, dan dashboard React yang disajikan dari satu binary Go. Panduan penggunaan lengkap ada di [README utama](../README.md).

## Tech Stack

| Komponen | Teknologi |
|----------|-----------|
| Language | Go 1.26 (CGO wajib: `mattn/go-sqlite3`) |
| HTTP Framework | Gin |
| ORM | GORM |
| Database | PostgreSQL 16 + Row Level Security |
| WhatsApp | whatsmeow |
| Frontend | React + Vite + TailwindCSS v4 |
| Workers | Goroutine + robfig/cron |
| Auth | JWT + 2FA/TOTP + Google OAuth |
| AI | Endpoint OpenAI-compatible |

## Quick Start

```bash
cp .env.example .env
# Isi DATABASE_URL dan JWT_SECRET (minimal 32 karakter, acak)

createdb wagataway
psql "$DATABASE_URL" -f seed_plans.sql

CGO_ENABLED=1 go run ./cmd/server
# frontend, terminal terpisah:
cd web && pnpm install && pnpm dev
```

Dashboard: `http://localhost:5173` · API: `http://localhost:8080/api`

Alternatif Docker Compose (app + PostgreSQL 16 + Redis):

```bash
docker compose up --build -d
```

Perlu diingat: compose tidak mengisi `JWT_SECRET`/`APP_NAME`; ubah juga password PostgreSQL bawaan sebelum dipakai untuk hal selain pengembangan lokal.

## Build & Deploy

```bash
make build                       # binary → ./bin/server
cd web && pnpm build             # dashboard → ./web/dist
CGO_ENABLED=1 go build -ldflags="-s -w" -o server ./cmd/server
```

Server mendaftarkan SPA fallback hanya bila `web/dist/index.html` sudah ada saat startup. Di belakang reverse proxy, set `LISTEN_HOST=127.0.0.1` agar port aplikasi tidak terbuka publik.

Admin pertama: `./server make-admin nama@domain.com`.

## Pemeriksaan

```bash
go build ./...
go vet ./...
make check-hosts             # pagar host luar
make check-hosts ARGS=--dist # termasuk hasil build frontend
```

## Project Structure

```
go-backend/
├── cmd/server/          # Entry point + subcommand make-admin
├── internal/
│   ├── config/          # Environment + ListenAddr()
│   ├── database/        # Koneksi, migrasi, model
│   ├── handler/         # HTTP handler (termasuk wamp_compat.go)
│   ├── middleware/      # Auth, scope API key, CORS, rate limit, security headers
│   ├── rls/             # Row Level Security via driver pgx
│   ├── security/        # Enkripsi kredensial AI/webhook
│   ├── whatsapp/        # Session manager + rotasi blast
│   └── worker/          # Scheduler background
├── scripts/             # Pemeriksa host luar + allowlist
├── web/                 # React frontend + web/dist hasil build
├── seed_plans.sql
└── Dockerfile / docker-compose.yml / Makefile
```

## Arsitektur

Satu binary menangani REST API, koneksi WhatsApp, worker background, dan static file dashboard. Endpoint kompatibel Slackersender (`/send-message`, `/send-media`, `/send-location`, `/info-device`, `/check-number`) berada di root, bukan di bawah `/api`.

---

Copyright © 2026 Duta Corpora Indonesia — All rights reserved. Lisensi: [MIT](../LICENSE).
