"""Uji stream SSE produksi: register akun uji -> tiket -> buka /api/stream ~6 dtk.

Pemakaian: python -I e2e_sse.py <base_url>

Author: Mas Wondho — Duta Corpora Indonesia (DCI)
Copyright © 2026 Duta Corpora Indonesia
"""
import json
import secrets
import sys
import time
import urllib.request

BASE = sys.argv[1].rstrip("/")
email = f"e2e-{secrets.token_hex(4)}@example.invalid"
pw = secrets.token_urlsafe(14)


def post(path, body=None, token=None):
    req = urllib.request.Request(BASE + path, data=json.dumps(body or {}).encode(), method="POST")
    req.add_header("Content-Type", "application/json")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    with urllib.request.urlopen(req, timeout=20) as r:
        return json.loads(r.read().decode())


token = post("/api/auth/register", {"name": "E2E SSE", "email": email, "password": pw})["token"]
ticket = post("/api/sse/ticket", token=token)["ticket"]

t0 = time.time()
req = urllib.request.Request(BASE + "/api/stream?ticket=" + ticket)
req.add_header("Accept", "text/event-stream")
with urllib.request.urlopen(req, timeout=20) as r:
    print("status:", r.status, "| content-type:", r.headers.get("Content-Type"))
    print("cache-control:", r.headers.get("Cache-Control"), "| x-accel-buffering:", r.headers.get("X-Accel-Buffering"))
    first = r.readline().decode().strip()
    print(f"baris pertama setelah {time.time() - t0:.2f} dtk (harus <1 dtk = tidak di-buffer): {first}")
    r.readline()  # data:
print("EMAIL_UJI=" + email)
