#!/usr/bin/env node
/*
 * Pengecek host luar — pagar agar SlackerSender tidak diam-diam bergantung
 * pada pihak lain.
 *
 * Memindai semua file yang dilacak git (tanpa dist/node_modules/lockfile/biner)
 * dan mengambil setiap host dari URL http(s)://. Gagal (exit 1) bila ada host
 * yang tidak tercantum di scripts/external-hosts.allowlist.
 *
 * Pakai:   node scripts/check-external-hosts.mjs          (dari go-backend/)
 *          node scripts/check-external-hosts.mjs --dist   (juga pindai web/dist & dist-landing)
 *
 * Author: Mas Wondho — Duta Corpora Indonesia (DCI)
 * Copyright © 2026 Duta Corpora Indonesia — All rights reserved
 */
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, statSync, existsSync } from "node:fs";
import { dirname, join, resolve, relative, extname } from "node:path";
import { fileURLToPath } from "node:url";

const here = dirname(fileURLToPath(import.meta.url));
const backendRoot = resolve(here, "..");
const repoRoot = resolve(execFileSync("git", ["rev-parse", "--show-toplevel"], { cwd: backendRoot }).toString().trim());
const scanDist = process.argv.includes("--dist");

// ── allowlist ────────────────────────────────────────────────────────────
// Format: satu pola per baris, '#' = komentar. "*.contoh.com" cocok dengan
// semua subdomain (dan domain induknya).
const allow = readFileSync(join(here, "external-hosts.allowlist"), "utf8")
  .split(/\r?\n/)
  .map((l) => l.replace(/#.*/, "").trim().toLowerCase())
  .filter(Boolean);

function allowed(host) {
  return allow.some((p) => {
    if (p.startsWith("*.")) {
      const base = p.slice(2);
      return host === base || host.endsWith("." + base);
    }
    return host === p;
  });
}

// ── daftar file ──────────────────────────────────────────────────────────
const SKIP_DIR = /(^|\/)(node_modules|dist|dist-landing|graphify-out|\.git|out|wa-sessions|uploads)\//;
const SKIP_FILE = /(^|\/)(pnpm-lock\.yaml|package-lock\.json|go\.sum|server\.log)$/;
const BINARY = new Set([".png", ".jpg", ".jpeg", ".webp", ".ico", ".woff", ".woff2", ".ttf", ".mp4", ".webm", ".gif", ".pdf", ".exe", ".zip"]);

function trackedFiles() {
  const out = execFileSync("git", ["ls-files", "-co", "--exclude-standard", "-z"], { cwd: repoRoot, maxBuffer: 1 << 28 }).toString();
  return out
    .split("\0")
    .filter(Boolean)
    .filter((f) => !SKIP_DIR.test(f) && !SKIP_FILE.test(f) && !BINARY.has(extname(f).toLowerCase()))
    .map((f) => join(repoRoot, f));
}

function walk(dir, acc = []) {
  if (!existsSync(dir)) return acc;
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    if (statSync(p).isDirectory()) walk(p, acc);
    else if (!BINARY.has(extname(name).toLowerCase())) acc.push(p);
  }
  return acc;
}

const files = trackedFiles();
if (scanDist) {
  files.push(...walk(join(backendRoot, "web", "dist")), ...walk(join(backendRoot, "web", "dist-landing")));
}

// ── pemindaian ───────────────────────────────────────────────────────────
// Host valid: label DNS dengan minimal satu titik. Pola seperti "https://..."
// atau "https://api" (potongan teks) otomatis terabaikan.
const URL_RE = /https?:\/\/([a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+)/gi;
const LOCAL = /^(localhost|127\.0\.0\.1|0\.0\.0\.0)$/;

const found = new Map(); // host -> [ "file:baris", ... ]
for (const file of files) {
  let text;
  try {
    text = readFileSync(file, "utf8");
  } catch {
    continue;
  }
  if (text.includes("\0")) continue;
  const lines = text.split("\n");
  for (let i = 0; i < lines.length; i++) {
    for (const m of lines[i].matchAll(URL_RE)) {
      const host = m[1].toLowerCase();
      if (LOCAL.test(host) || allowed(host)) continue;
      if (!found.has(host)) found.set(host, []);
      found.get(host).push(`${relative(repoRoot, file).replaceAll("\\", "/")}:${i + 1}`);
    }
  }
}

if (found.size === 0) {
  console.log(`OK — ${files.length} file dipindai, semua host luar ada di allowlist.`);
  process.exit(0);
}

console.error(`GAGAL — ${found.size} host luar belum diizinkan:\n`);
for (const [host, where] of [...found].sort((a, b) => a[0].localeCompare(b[0]))) {
  const shown = where.slice(0, 3).join(", ") + (where.length > 3 ? `, +${where.length - 3} lagi` : "");
  console.error(`  ${host}\n      ${shown}`);
}
console.error("\nBila host ini memang disengaja, tambahkan ke scripts/external-hosts.allowlist dengan komentar alasannya.");
process.exit(1);
