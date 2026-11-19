// Komposisi hero SlackerSender — loop 10 dtk, 1600×900.
// Cerita: developer mengirim lewat API → gateway SlackerSender memproses →
// pesan WhatsApp tiba di ponsel pelanggan → webhook status balik ke developer.
// Tumpukan 5 lapis: backdrop → aset (panel) → grafik (kabel/paket/chip) → grade → grain+scanline+vignette.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { AbsoluteFill } from "remotion";
import { Backdrop } from "../components/Backdrop";
import { Wires } from "../components/Wires";
import { Terminal } from "../components/Terminal";
import { Gateway } from "../components/Gateway";
import { Phone } from "../components/Phone";
import { StatChips } from "../components/StatChips";
import { Grade, Grain, Scanlines, Vignette } from "../components/Finish";

export const HeroLoop: React.FC = () => (
  <AbsoluteFill>
    {/* 1. Latar mesh + grid */}
    <Backdrop />
    {/* 2–3. Panel utama + grafik penghubung */}
    <Wires />
    <Terminal />
    <Phone />
    <Gateway />
    <StatChips />
    {/* 4. Color grade */}
    <Grade />
    {/* 5. Grain + scanline + vignette (paling atas) */}
    <Grain />
    <Scanlines />
    <Vignette />
  </AbsoluteFill>
);
