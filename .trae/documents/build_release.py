"""Bangun tarball rilis SlackerSender: sumber Go + web/dist + seed (tanpa rahasia).

Pemakaian: python build_release.py <folder go-backend> <berkas .tar.gz keluaran>

Author: Mas Wondho — Duta Corpora Indonesia (DCI)
Copyright © 2026 Duta Corpora Indonesia
"""
import hashlib
import io
import os
import sys
import tarfile

TOP = ["cmd", "internal", "go.mod", "go.sum", "seed_plans.sql"]
SKIP_DIRS = {"node_modules", ".git", "graphify-out"}
SKIP_SUFFIX = (".exe", ".new", ".log")
SKIP_NAMES = {".env"}
# Berkas teks sumber: CRLF hasil checkout Windows dinormalkan ke LF.
TEXT_SUFFIX = (".go", ".mod", ".sum", ".sql", ".md", ".txt")


def add_bytes(tf, arcname, data):
    ti = tarfile.TarInfo(arcname)
    ti.size = len(data)
    ti.mode = 0o644
    tf.addfile(ti, io.BytesIO(data))


def add_file(tf, full, arcname):
    data = open(full, "rb").read()
    if full.endswith(TEXT_SUFFIX):
        data = data.replace(b"\r\n", b"\n")
    add_bytes(tf, arcname, data)


def main(src, out):
    count = 0
    with tarfile.open(out, "w:gz") as tf:
        for top in TOP:
            p = os.path.join(src, top)
            if os.path.isfile(p):
                add_file(tf, p, "slackersender/" + top)
                count += 1
                continue
            for root, dirs, files in os.walk(p):
                dirs[:] = [d for d in dirs if d not in SKIP_DIRS]
                for f in files:
                    if f in SKIP_NAMES or f.endswith(SKIP_SUFFIX):
                        continue
                    full = os.path.join(root, f)
                    rel = os.path.relpath(full, src).replace(os.sep, "/")
                    add_file(tf, full, "slackersender/" + rel)
                    count += 1
        dist = os.path.join(src, "web", "dist")
        for root, _dirs, files in os.walk(dist):
            for f in files:
                full = os.path.join(root, f)
                rel = os.path.relpath(full, dist).replace(os.sep, "/")
                add_file(tf, full, "slackersender/web/dist/" + rel)
                count += 1
    digest = hashlib.sha256(open(out, "rb").read()).hexdigest()
    base = os.path.basename(out)
    open(out.replace(".tar.gz", ".sha256"), "w").write(f"{digest}  {base}\n")
    print(f"{count} berkas, {os.path.getsize(out) / 1024 / 1024:.1f} MB, sha256={digest}")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
