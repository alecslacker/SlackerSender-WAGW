// Geometri komposisi (kanvas 1600×900) + timeline beat.
// Semua posisi berasal dari sini supaya kabel, paket, dan kartu selalu sejajar.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia

export const CANVAS = { width: 1600, height: 900 } as const;
export const FPS = 30;
export const DURATION_SEC = 10;
export const DURATION = FPS * DURATION_SEC; // 300 frame, loop mulus

// ── Tiga panel utama ─────────────────────────────────────────────────────────
export const TERMINAL = { x: 60, y: 210, w: 540, h: 420 } as const;
export const GATEWAY = { cx: 838, cy: 420, r: 78 } as const;
export const PHONE = { x: 1150, y: 108, w: 330, h: 646 } as const;

// Titik sambung kabel (port) — dipakai Wires & Packet agar presisi
export const PORT = {
  termOut: { x: TERMINAL.x + TERMINAL.w, y: 436 },
  gwIn: { x: GATEWAY.cx - GATEWAY.r - 2, y: GATEWAY.cy },
  gwOut: { x: GATEWAY.cx + GATEWAY.r + 2, y: GATEWAY.cy },
  phoneIn: { x: PHONE.x, y: 420 },
} as const;

// Kurva kubik jalur paket: terminal → gateway, gateway → ponsel
export type Curve = readonly [
  readonly [number, number],
  readonly [number, number],
  readonly [number, number],
  readonly [number, number],
];

export const CURVE_A: Curve = [
  [PORT.termOut.x, PORT.termOut.y],
  [PORT.termOut.x + 90, PORT.termOut.y],
  [PORT.gwIn.x - 90, PORT.gwIn.y],
  [PORT.gwIn.x, PORT.gwIn.y],
];

export const CURVE_B: Curve = [
  [PORT.gwOut.x, PORT.gwOut.y],
  [PORT.gwOut.x + 90, PORT.gwOut.y],
  [PORT.phoneIn.x - 90, PORT.phoneIn.y],
  [PORT.phoneIn.x, PORT.phoneIn.y],
];

export const curvePath = (c: Curve) =>
  `M ${c[0][0]} ${c[0][1]} C ${c[1][0]} ${c[1][1]}, ${c[2][0]} ${c[2][1]}, ${c[3][0]} ${c[3][1]}`;

// Titik pada kurva bezier kubik untuk t ∈ [0,1]
export const pointOnCurve = (c: Curve, t: number) => {
  const u = 1 - t;
  const a = u * u * u;
  const b = 3 * u * u * t;
  const cc = 3 * u * t * t;
  const d = t * t * t;
  return {
    x: a * c[0][0] + b * c[1][0] + cc * c[2][0] + d * c[3][0],
    y: a * c[0][1] + b * c[1][1] + cc * c[2][1] + d * c[3][1],
  };
};

// ── Timeline beat (detik → frame lewat helper s()) ───────────────────────────
export const s = (sec: number) => Math.round(sec * FPS);

export type Beat = {
  key: string;
  // Teks yang "diketik" di terminal
  cmd: string;
  // Respons gateway di terminal
  resp: string;
  // Isi bubble pesan di ponsel
  bubble: string;
  bubbleKind: "text" | "doc";
  typeStart: number; // mulai mengetik
  sendAt: number; // paket berangkat dari terminal
};

export const BEATS: readonly Beat[] = [
  {
    key: "otp",
    cmd: "POST /send-message",
    resp: '{ "status": true }',
    bubble: "Kode OTP Anda: 482913",
    bubbleKind: "text",
    typeStart: s(0.5),
    sendAt: s(1.7),
  },
  {
    key: "media",
    cmd: "POST /send-media",
    resp: '{ "status": true }',
    bubble: "invoice-0912.pdf",
    bubbleKind: "doc",
    typeStart: s(3.6),
    sendAt: s(4.8),
  },
] as const;

// Durasi tempuh paket (frame)
export const LEG_A = s(0.62); // terminal → gateway
export const LEG_B = s(0.62); // gateway → ponsel
export const GW_DWELL = s(0.18); // jeda proses di gateway

export const packetTimes = (b: Beat) => {
  const legAStart = b.sendAt;
  const gwAt = legAStart + LEG_A;
  const legBStart = gwAt + GW_DWELL;
  const arriveAt = legBStart + LEG_B;
  return { legAStart, gwAt, legBStart, arriveAt };
};

// Webhook balik (ponsel → gateway → terminal) setelah pesan dibaca.
// Semua waktu diturunkan di sini supaya paket & baris terminal selalu sinkron.
const HOOK_AT = s(6.6);
const HOOK_LEG = s(0.45);
const HOOK_GAP = s(0.32);
export const WEBHOOK = {
  at: HOOK_AT, // paket berangkat dari ponsel (= pesan terakhir dibaca)
  leg: HOOK_LEG,
  legAStart: HOOK_AT + HOOK_GAP, // gateway → terminal
  arriveTerminal: HOOK_AT + HOOK_GAP + HOOK_LEG, // baris webhook muncul
  event: "message.sent",
} as const;

// Fade out global agar frame terakhir ≈ frame pertama (loop mulus)
export const OUTRO_START = s(9.15);
