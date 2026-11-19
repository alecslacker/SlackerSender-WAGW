# Deploy production SlackerSender — server-cbt

Author: Mas Wondho — Duta Corpora Indonesia (DCI)
Copyright © 2026 Duta Corpora Indonesia — All rights reserved

Dikerjakan 2026-11-19. Domain: https://wasender.slackercode.us. Dokumen ini TIDAK berisi kata sandi.

## Susunan di server

| Komponen | Lokasi / nilai |
|---|---|
| Aplikasi (binary Go) | `/www/wwwroot/wasender.slackercode.us/slackersender-server` (root:slackersender 0750) |
| Frontend | `/www/wwwroot/wasender.slackercode.us/web/dist` (128 berkas, read-only untuk layanan) |
| Data yang boleh ditulis | `uploads/`, `public/`, `data/`, `wa-sessions/` (pemilik `slackersender`; `wa-sessions` 0700) |
| Konfigurasi + rahasia | `/etc/slackersender/slackersender.env` (root:root 0600). Berisi DATABASE_URL, JWT_SECRET, AI_KEYS_SECRET (acak, dibuat di server) |
| Layanan | `systemctl status slackersender` (user `slackersender`, hardening ketat, MemoryMax 2G) |
| Bind | `127.0.0.1:8080` (`LISTEN_HOST=127.0.0.1`) — tidak terjangkau dari internet |
| Database | PostgreSQL 16.10 lokal, DB `slackersender`, role non-superuser (RLS efektif) |
| Go toolchain | `/home/ubuntu/slackersender-build/go` (1.26.11, sha256 cocok go.dev) |
| Cadangan vhost | `/root/backup-slackersender/` (sha256 asli 267d43e7…) |

## Jalur lalu lintas

`internet → nginx :443 → 127.0.0.1:8080 (app Go)`. nginx menimpa `X-Real-IP`/`X-Forwarded-For` dengan `$remote_addr` (rate limiter memakai header ini). `/api/stream` (SSE) tanpa buffering. HTTP dialihkan 301 ke HTTPS di `location /` (jalur `.well-known` tetap terbuka untuk sertifikat).

Catatan penting: di server ini nginx adalah pintu publik (80/443); OpenLiteSpeed berjalan di belakangnya (port 8190) untuk 36 vhost lain. Untuk domain ini OLS dilewati dan nginx langsung proxy ke aplikasi Go. Vhost OLS `wasender.slackercode.us` tidak diubah.

## Rollback vhost

```bash
sudo cp -a /root/backup-slackersender/wasender.slackercode.us.conf.20261119-102118 \
  /www/server/panel/vhost/nginx/wasender.slackercode.us.conf
sudo /www/server/nginx/sbin/nginx -t && sudo /www/server/nginx/sbin/nginx -s reload
```

PERINGATAN: mengubah pengaturan situs ini lewat panel aaPanel bisa menimpa vhost nginx buatan tangan. Setelah itu cek `proxy_pass http://127.0.0.1:8080` masih ada.

## Admin

Admin produksi: `irawanriswandha@gmail.com` (diangkat 2026-11-19). Admin berikutnya dikelola dari panel Admin. Bila perlu mengangkat lewat server (akun harus sudah terdaftar), jalankan sebagai root agar env terbaca:

```bash
cd /www/wwwroot/wasender.slackercode.us
sudo bash -c 'set -a; . /etc/slackersender/slackersender.env; set +a; ./slackersender-server make-admin anda@domain.com'
```

## Update versi (cara yang dipakai pada rilis 3)

1. Pastikan HEAD bersih, tes lulus, lalu `pnpm build`. Bangun tarball: `python -I .trae/documents/build_release.py <go-backend> <rilis.tar.gz>` (tulis sha256 otomatis).
2. Unggah, cek sha256, ekstrak ke `src3/`, build di server saat layanan lama masih jalan (tanpa downtime): `nice -n 19 ionice -c3 go build -p 1 -trimpath -ldflags="-s -w"` (CGO aktif; ±1 mnt dengan cache).
3. Cadangkan binary + `web/dist` lama + `pg_dump --no-owner --enable-row-security -Fc` (tanpa `--enable-row-security` dump GAGAL karena RLS dipaksa) ke `/root/backup-slackersender/rilisN-<waktu>/`; folder terbaru dicatat di `/root/backup-slackersender/TERAKHIR`.
4. Ganti binary + dist lalu start; skrip memeriksa `/health` hingga 30 dtk dan rollback otomatis bila gagal sehat. Server hanya mendaftarkan SPA bila `web/dist/index.html` ada saat startup.
5. Hapus akun uji E2E (`email like 'e2e-%@example.invalid'` beserta data turunannya) — jangan sentuh akun lain.

## Rilis berjalan

Rilis 3 = commit `0829267` (module Go `github.com/alecslacker/SlackerSender-WAGW/go-backend`): CSP ketat, font lokal (tanpa Google Fonts), AI bawaan 9router, migrasi data host AI lama (berjalan sekali, flag `ai_legacy_host_migrated_v1`), `LISTEN_HOST`. Tes lokal 261 PASS / 0 SKIP / 0 FAIL (2 run). Binary sha256 `2aa1c2b0…`. Cadangan rilis 2 + dump DB: `/root/backup-slackersender/rilis2-20261119-120810/`.

Rollback ke rilis 2: hentikan layanan, kembalikan `slackersender-server` dan `dist/` dari folder cadangan itu, start. Dump DB hanya perlu dipakai bila migrasi merusak data (tidak terjadi pada rilis 3).

## Hasil verifikasi (produksi, via HTTPS)

- `/health`, `/`, `/login`, `/api-docs`, `/hero/*`, logo, sitemap: 200.
- HTTP→HTTPS 301; `/.env` dan `/.git/config`: 404.
- CORS: origin `https://wa.clipku.com` ditolak, `https://wasender.slackercode.us` diizinkan.
- 5 endpoint compat (POST + GET): kunci palsu → 401; scope ditegakkan (key `messages:send` ditolak 403 di `info-device`, key `devices:read` ditolak 403 di `send-message`).
- SSE: terbuka 0,12 dtk, tidak tertahan buffer.
- Port 8080 dari luar: tertutup. Situs lain di server: tidak terganggu.
- Akun/data uji dihapus setelah tiap uji (sisa: 1 akun admin, 5 plans, 0 api_keys).
- Rilis 3: header CSP aktif di halaman (tidak di `/api`), konsol browser 0 error/peringatan, 0 permintaan ke luar origin, video hero berputar, font VT323/Inter/JetBrains Mono termuat dari server sendiri; E2E compat + scope + SSE diulang dan lulus.

## Belum terverifikasi

- Pengiriman pesan WhatsApp nyata (butuh scan QR device); pesan uji baru bisa gagal "perangkat tidak ditemukan".
- Pembayaran Clipku Pay dan AI (belum diisi kuncinya).
- Port 5432 server masih terbuka ke internet (keputusan Mas Wondho: dibiarkan dulu).
