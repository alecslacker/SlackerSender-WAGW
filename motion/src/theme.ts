// theme.ts — satu-satunya sumber warna, easing, spring, dan font animasi hero.
// DILARANG menulis hex/easing langsung di komponen.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import { Easing } from "remotion";
import { loadFont as loadVT323 } from "@remotion/google-fonts/VT323";
import { loadFont as loadJetBrains } from "@remotion/google-fonts/JetBrainsMono";

const vt323 = loadVT323("normal", { weights: ["400"], subsets: ["latin"] });
const jetbrains = loadJetBrains("normal", {
  weights: ["400", "500", "700"],
  subsets: ["latin"],
});

export const theme = {
  colors: {
    // Palet terminal SlackerSender — selaras token index.css web
    bg: "#050805",
    bgAlt: "#0a120b",
    panel: "#071007",
    panelHead: "#0b1a0e",
    line: "#16351e",
    lineDim: "#0f2414",
    // Warna hero (paket data + node gateway) — maksimal satu fokus per frame
    primary: "#22c55e",
    primaryBright: "#86efac",
    text: "#4ade80",
    textDim: "#2fb35d",
    textMute: "#1f6b36",
    white: "#e7fbe9",
    // Aksen kecil: centang "dibaca" ala WhatsApp, traffic light terminal
    accent: "#38bdf8",
    red: "#ff5f57",
    amber: "#febc2e",
    glow: "rgba(34, 197, 94, 0.45)",
    glowSoft: "rgba(34, 197, 94, 0.18)",
  },
  fonts: {
    display: vt323.fontFamily,
    mono: jetbrains.fontFamily,
  },
  // Kurva easing. Linear DILARANG.
  ease: {
    out: Easing.bezier(0.16, 1, 0.3, 1), // easeOutExpo — entrance
    inOut: Easing.bezier(0.83, 0, 0.17, 1), // easeInOutQuint — perpindahan
    in: Easing.bezier(0.7, 0, 0.84, 0), // khusus exit
    flow: Easing.bezier(0.45, 0, 0.2, 1), // paket data melaju di jalur
  },
  spring: {
    snappy: { damping: 14, stiffness: 160, mass: 0.6 },
    smooth: { damping: 20, stiffness: 90, mass: 1 },
    bouncy: { damping: 11, stiffness: 170, mass: 0.7 },
    counter: { damping: 30, stiffness: 60, mass: 1 }, // angka menghitung naik
  },
} as const;
