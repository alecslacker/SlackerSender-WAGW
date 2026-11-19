// Chip metrik kecil di bawah kabel: latensi & jumlah pesan (counter tabular-nums).
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { interpolate, useCurrentFrame, useVideoConfig } from "remotion";
import { theme } from "../theme";
import { BEATS, GATEWAY, packetTimes, s } from "../lib/layout";
import { entrance, float, outro, pop } from "../lib/anim";

const Chip: React.FC<{ x: number; y: number; delay: number; label: string; children: React.ReactNode }> = ({
  x,
  y,
  delay,
  label,
  children,
}) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const p = pop(frame, fps, delay, theme.spring.snappy);
  const out = outro(frame);
  if (frame < delay - 1) return null;
  return (
    <div
      style={{
        position: "absolute",
        left: x,
        top: y + float(frame + delay, 32, 3) + out.shift,
        opacity: out.opacity,
      }}
    >
      <div
        style={{
          ...entrance(p, 16, 0.9),
          background: theme.colors.panel,
          border: `1px solid ${theme.colors.line}`,
          borderRadius: 12,
          padding: "10px 14px",
          fontFamily: theme.fonts.mono,
          boxShadow: "0 16px 30px -18px rgba(0,0,0,0.8)",
        }}
      >
        <div style={{ fontSize: 12, letterSpacing: 2, color: theme.colors.textDim }}>{label}</div>
        <div style={{ fontSize: 22, color: theme.colors.white, fontWeight: 700, fontVariantNumeric: "tabular-nums" }}>
          {children}
        </div>
      </div>
    </div>
  );
};

export const StatChips: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const firstArrive = packetTimes(BEATS[0]).arriveAt;
  const secondArrive = packetTimes(BEATS[1]).arriveAt;

  // Counter pesan hari ini: naik saat tiap pesan tiba
  const c1 = pop(frame, fps, firstArrive, theme.spring.counter);
  const c2 = pop(frame, fps, secondArrive, theme.spring.counter);
  const count = Math.round(1238 + interpolate(c1, [0, 1], [0, 1]) + interpolate(c2, [0, 1], [0, 1]));

  return (
    <>
      <Chip x={GATEWAY.cx - 238} y={GATEWAY.cy + 196} delay={firstArrive + s(0.15)} label="LATENSI">
        86 ms
      </Chip>
      <Chip x={GATEWAY.cx + 48} y={GATEWAY.cy + 196} delay={firstArrive + s(0.3)} label="TERKIRIM HARI INI">
        {count.toLocaleString("id-ID")}
      </Chip>
    </>
  );
};
