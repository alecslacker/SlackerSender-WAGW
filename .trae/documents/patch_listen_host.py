"""Patch pohon sumber rilis: tambah LISTEN_HOST (bind 127.0.0.1) tanpa membawa
perubahan lain dari working copy.

Pemakaian: python -I patch_listen_host.py <root pohon sumber go-backend> <berkas listenaddr_test.go>

Author: Mas Wondho — Duta Corpora Indonesia (DCI)
Copyright © 2026 Duta Corpora Indonesia
"""
import pathlib
import shutil
import sys

root = pathlib.Path(sys.argv[1])
test_src = pathlib.Path(sys.argv[2])


def sub(rel, old, new):
    p = root / rel
    with open(p, "r", encoding="utf-8", newline="") as f:
        s = f.read()
    assert "\r\n" not in s, f"{rel}: CRLF tak terduga"
    n = s.count(old)
    assert n == 1, f"{rel}: pola harus muncul tepat 1x, muncul {n}x: {old[:70]!r}"
    with open(p, "w", encoding="utf-8", newline="\n") as f:
        f.write(s.replace(old, new))
    print(f"[ok] {rel}: {old.splitlines()[0].strip()[:60]!r}")


CFG = "internal/config/config.go"
MAIN = "cmd/server/main.go"

sub(CFG,
    '\t"log"\n\t"os"\n\t"strings"\n\t"time"',
    '\t"log"\n\t"net"\n\t"os"\n\t"strings"\n\t"time"')

sub(CFG,
    '\tPort    string\n\tGinMode string\n\tAppName string\n\tAppURL  string\n',
    '\tPort       string\n\tListenHost string\n\tGinMode    string\n\tAppName    string\n\tAppURL     string\n')

sub(CFG,
    'func Load() *Config {\n',
    '// ListenAddr mengembalikan alamat bind server HTTP. LISTEN_HOST kosong = semua\n'
    '// antarmuka (cocok untuk Docker). Production di belakang nginx/aaPanel sebaiknya\n'
    '// diisi 127.0.0.1: port aplikasi jadi tak terjangkau dari luar, sehingga header\n'
    '// X-Real-IP milik nginx (dipakai rate limiter) tidak bisa dipalsukan langsung.\n'
    'func (c *Config) ListenAddr() string {\n'
    '\thost := strings.Trim(strings.TrimSpace(c.ListenHost), "[]")\n'
    '\treturn net.JoinHostPort(host, c.Port)\n'
    '}\n\n'
    'func Load() *Config {\n')

sub(CFG,
    '\t\tPort:    getEnv("PORT", "8080"),\n\t\tGinMode: getEnv("GIN_MODE", "debug"),\n'
    '\t\tAppName: getEnv("APP_NAME", "SlackerSender"),\n\t\tAppURL:  getEnv("APP_URL", "http://localhost:8080"),\n',
    '\t\tPort:       getEnv("PORT", "8080"),\n\t\tListenHost: getEnv("LISTEN_HOST", ""),\n'
    '\t\tGinMode:    getEnv("GIN_MODE", "debug"),\n\t\tAppName:    getEnv("APP_NAME", "SlackerSender"),\n'
    '\t\tAppURL:     getEnv("APP_URL", "http://localhost:8080"),\n')

sub(MAIN, 'import (\n\t"context"\n\t"fmt"\n\t"net/http"', 'import (\n\t"context"\n\t"net/http"')
sub(MAIN, 'Addr:         fmt.Sprintf(":%s", cfg.Port),', 'Addr:         cfg.ListenAddr(),')
sub(MAIN, 'log.Info().Str("port", cfg.Port).Msg("Server listening")',
    'log.Info().Str("addr", cfg.ListenAddr()).Msg("Server listening")')

dst = root / "internal/config/listenaddr_test.go"
assert not dst.exists(), "listenaddr_test.go sudah ada di pohon rilis"
with open(test_src, "r", encoding="utf-8", newline="") as f:
    body = f.read().replace("\r\n", "\n")
with open(dst, "w", encoding="utf-8", newline="\n") as f:
    f.write(body)
print("[ok] internal/config/listenaddr_test.go ditambahkan")
