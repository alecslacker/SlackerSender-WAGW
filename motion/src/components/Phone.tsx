// Ponsel penerima: chat WhatsApp bertema terminal — bubble masuk saat paket tiba,
// centang ✓ → ✓✓ → ✓✓ biru (dibaca).
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { Img, staticFile, useCurrentFrame, useVideoConfig } from "remotion";
import { theme } from "../theme";
import { BEATS, PHONE, WEBHOOK, packetTimes, s, type Beat } from "../lib/layout";
import { entrance, float, outro, pop } from "../lib/anim";

// Centang status pesan: 1 = terkirim, 2 = sampai, 3 = dibaca (biru)
const Ticks: React.FC<{ state: 1 | 2 | 3 }> = ({ state }) => {
  const color = state === 3 ? theme.colors.accent : theme.colors.textDim;
  return (
    <svg width={22} height={14} viewBox="0 0 22 14" style={{ marginLeft: 6 }}>
      <path d="M1 7.5 L5 11.5 L13 2.5" fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      {state >= 2 ? (
        <path d="M8 11.5 L9 11.5 L17 2.5" fill="none" stroke={color} strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" />
      ) : null}
    </svg>
  );
};

// Ikon dokumen PDF digambar SVG (bukan emoji — warna tetap patuh palet)
const DocIcon: React.FC = () => (
  <svg width={34} height={40} viewBox="0 0 34 40">
    <path d="M3 1 H22 L32 11 V39 H3 Z" fill={theme.colors.panelHead} stroke={theme.colors.text} strokeWidth={1.6} />
    <path d="M22 1 V11 H32" fill="none" stroke={theme.colors.text} strokeWidth={1.6} />
    <text x="17.5" y="30" textAnchor="middle" fontSize="9" fontWeight={700} fill={theme.colors.text} fontFamily="monospace">
      PDF
    </text>
  </svg>
);

const Bubble: React.FC<{ beat: Beat; index: number }> = ({ beat, index }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const { arriveAt } = packetTimes(beat);
  if (frame < arriveAt - 1) return null;

  const p = pop(frame, fps, arriveAt, theme.spring.snappy);
  // Status centang mengikuti waktu: sampai → dibaca (pesan terakhir dibaca jelang webhook)
  const readAt = index === BEATS.length - 1 ? WEBHOOK.at : arriveAt + s(1.4);
  const state: 1 | 2 | 3 = frame >= readAt ? 3 : frame >= arriveAt + s(0.45) ? 2 : 1;
  const time = index === 0 ? "09:41" : "09:42";

  return (
    <div
      style={{
        alignSelf: "flex-end",
        maxWidth: 252,
        marginBottom: 14,
        transformOrigin: "100% 100%",
        ...entrance(p, 18, 0.86),
      }}
    >
      <div
        style={{
          background: theme.colors.panelHead,
          border: `1px solid ${theme.colors.line}`,
          borderRadius: "16px 16px 4px 16px",
          padding: "11px 13px 8px",
          color: theme.colors.white,
          fontFamily: theme.fonts.mono,
          fontSize: 17,
          lineHeight: "24px",
          boxShadow: "0 10px 24px -12px rgba(0,0,0,0.7)",
        }}
      >
        {beat.bubbleKind === "doc" ? (
          <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
            <DocIcon />
            <div>
              <div style={{ fontSize: 16 }}>{beat.bubble}</div>
              <div style={{ fontSize: 13, color: theme.colors.textDim }}>PDF · 214 KB</div>
            </div>
          </div>
        ) : (
          <div>{beat.bubble}</div>
        )}
        <div style={{ display: "flex", justifyContent: "flex-end", alignItems: "center", marginTop: 4, fontSize: 12, color: theme.colors.textDim }}>
          {time}
          <Ticks state={state} />
        </div>
      </div>
    </div>
  );
};

export const Phone: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const inP = pop(frame, fps, s(0.12), theme.spring.smooth);
  const out = outro(frame);
  const firstArrive = packetTimes(BEATS[0]).arriveAt;
  // Status "mengetik…" tak dipakai; header menampilkan "online" setelah pesan pertama
  const onlineP = pop(frame, fps, firstArrive, theme.spring.snappy);

  return (
    <div
      style={{
        position: "absolute",
        left: PHONE.x,
        top: PHONE.y + float(frame, 38, 4) + out.shift,
        width: PHONE.w,
        height: PHONE.h,
        opacity: inP * out.opacity,
        transform: `translateY(${(1 - inP) * 46}px) scale(${0.95 + inP * 0.05})`,
        borderRadius: 46,
        padding: 12,
        background: "#020402",
        border: `2px solid ${theme.colors.line}`,
        boxShadow: `0 50px 90px -28px rgba(0,0,0,0.8), 0 0 60px -20px ${theme.colors.glowSoft}`,
      }}
    >
      <div
        style={{
          position: "relative",
          width: "100%",
          height: "100%",
          borderRadius: 36,
          overflow: "hidden",
          background: theme.colors.bg,
          display: "flex",
          flexDirection: "column",
        }}
      >
        {/* Notch */}
        <div style={{ position: "absolute", top: 10, left: "50%", marginLeft: -48, width: 96, height: 26, borderRadius: 14, background: "#000", zIndex: 2 }} />
        {/* Header chat */}
        <div
          style={{
            paddingTop: 48,
            paddingBottom: 14,
            paddingLeft: 18,
            paddingRight: 18,
            background: theme.colors.panelHead,
            borderBottom: `1px solid ${theme.colors.line}`,
            display: "flex",
            alignItems: "center",
            gap: 12,
          }}
        >
          <div style={{ width: 44, height: 44, borderRadius: 44, background: theme.colors.panel, border: `1px solid ${theme.colors.line}`, display: "flex", alignItems: "center", justifyContent: "center" }}>
            <Img src={staticFile("logo.png")} style={{ width: 28, height: 28 }} />
          </div>
          <div style={{ fontFamily: theme.fonts.mono }}>
            <div style={{ color: theme.colors.white, fontSize: 18, fontWeight: 700 }}>SlackerSender</div>
            <div style={{ color: theme.colors.text, fontSize: 13, height: 18, opacity: 0.45 + onlineP * 0.55 }}>
              {frame >= firstArrive ? "online" : "akun bisnis"}
            </div>
          </div>
        </div>
        {/* Area pesan — pola titik halus seperti wallpaper chat */}
        <div
          style={{
            flex: 1,
            padding: "22px 16px",
            display: "flex",
            flexDirection: "column",
            justifyContent: "flex-end",
            backgroundImage: `radial-gradient(${theme.colors.lineDim} 1.2px, transparent 1.2px)`,
            backgroundSize: "18px 18px",
          }}
        >
          <div style={{ alignSelf: "center", fontFamily: theme.fonts.mono, fontSize: 12, color: theme.colors.textDim, background: theme.colors.panel, border: `1px solid ${theme.colors.lineDim}`, borderRadius: 8, padding: "4px 10px", marginBottom: 16 }}>
            HARI INI
          </div>
          {BEATS.map((b, i) => (
            <Bubble key={b.key} beat={b} index={i} />
          ))}
        </div>
        {/* Kolom input */}
        <div style={{ padding: "12px 14px 18px", display: "flex", gap: 10, alignItems: "center", borderTop: `1px solid ${theme.colors.lineDim}` }}>
          <div style={{ flex: 1, height: 40, borderRadius: 20, background: theme.colors.panel, border: `1px solid ${theme.colors.lineDim}`, fontFamily: theme.fonts.mono, fontSize: 14, color: theme.colors.textMute, display: "flex", alignItems: "center", paddingLeft: 16 }}>
            Ketik pesan
          </div>
          <div style={{ width: 40, height: 40, borderRadius: 40, background: theme.colors.line, display: "flex", alignItems: "center", justifyContent: "center" }}>
            <svg width={18} height={18} viewBox="0 0 24 24">
              <path d="M3 11.5 L21 3 L13.5 21 L11 13 Z" fill="none" stroke={theme.colors.text} strokeWidth={2} strokeLinejoin="round" />
            </svg>
          </div>
        </div>
      </div>
    </div>
  );
};
