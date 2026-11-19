"""Uji end-to-end produksi SlackerSender: register -> login -> API key -> endpoint compat -> scope.

Pemakaian: python -I e2e_prod.py <base_url>
Akun uji dibuat dengan email acak; kata sandi acak hanya hidup di memori proses ini.
Hasil berisi nama akun uji di baris terakhir supaya bisa dihapus.

Author: Mas Wondho — Duta Corpora Indonesia (DCI)
Copyright © 2026 Duta Corpora Indonesia
"""
import json
import secrets
import sys
import urllib.error
import urllib.request

BASE = sys.argv[1].rstrip("/")


def call(method, path, body=None, token=None, key=None):
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    req.add_header("Content-Type", "application/json")
    req.add_header("User-Agent", "slackersender-e2e/1.0")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    if key:
        req.add_header("X-API-Key", key)
    try:
        with urllib.request.urlopen(req, timeout=20) as r:
            raw = r.read().decode()
            return r.status, raw
    except urllib.error.HTTPError as e:
        return e.code, e.read().decode()


def js(raw):
    try:
        return json.loads(raw)
    except Exception:
        return {"_raw": raw[:160]}


def show(label, status, raw, maxlen=170):
    print(f"{label:<44} [{status}] {raw[:maxlen].replace(chr(10), ' ')}")


email = f"e2e-{secrets.token_hex(4)}@example.invalid"
pw = secrets.token_urlsafe(14)
print("akun uji:", email)

st, raw = call("POST", "/api/auth/register", {"name": "E2E Uji", "email": email, "password": pw})
show("register", st, raw, 120)
d = js(raw)
token = d.get("token") or (d.get("data") or {}).get("token")
if not token:
    st, raw = call("POST", "/api/auth/login", {"email": email, "password": pw})
    show("login", st, raw, 120)
    d = js(raw)
    token = d.get("token") or (d.get("data") or {}).get("token")
if not token:
    print("TIDAK DAPAT TOKEN, berhenti")
    sys.exit(1)

st, raw = call("GET", "/api/auth/me", token=token)
show("auth/me (role harus 'user')", st, raw, 150)

st, raw = call("GET", "/api/api-keys/scopes", token=token)
show("daftar scope", st, raw, 200)

st, raw = call("POST", "/api/api-keys", {"name": "uji-send", "scopes": ["messages:send"]}, token=token)
show("buat API key scope messages:send", st, "(isi disembunyikan)" if st < 300 else raw)
send_key = js(raw).get("apiKey")

st, raw = call("POST", "/api/api-keys", {"name": "uji-read", "scopes": ["devices:read"]}, token=token)
show("buat API key scope devices:read", st, "(isi disembunyikan)" if st < 300 else raw)
read_key = js(raw).get("apiKey")

print("--- endpoint compat dengan key BENAR (device belum ada -> tidak boleh 5xx):")
if send_key:
    st, raw = call("POST", "/send-message", {"api_key": send_key, "sender": "628111111111", "number": "628222222222", "message": "uji"})
    show("send-message (key send)  -> diizinkan", st, raw)
    st, raw = call("POST", "/info-device", {"api_key": send_key, "number": "628111111111"})
    show("info-device  (key send)  -> HARUS 403", st, raw)
if read_key:
    st, raw = call("POST", "/info-device", {"api_key": read_key, "number": "628111111111"})
    show("info-device  (key read)  -> diizinkan", st, raw)
    st, raw = call("POST", "/send-message", {"api_key": read_key, "sender": "628111111111", "number": "628222222222", "message": "uji"})
    show("send-message (key read)  -> HARUS 403", st, raw)

print("--- tiket SSE + stream lewat nginx:")
st, raw = call("POST", "/api/sse/ticket", token=token)
show("sse/ticket", st, "(isi disembunyikan)" if st < 300 else raw)
print("EMAIL_UJI=" + email)
