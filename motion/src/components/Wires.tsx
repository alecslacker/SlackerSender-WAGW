// Kabel data + paket yang melaju: terminal → gateway → ponsel, dan webhook balik.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { AbsoluteFill, useCurrentFrame, useVideoConfig } from "remotion";
import { theme } from "../theme";
import {
  BEATS,
  CANVAS,
  CURVE_A,
  CURVE_B,
  LEG_A,
  LEG_B,
  WEBHOOK,
  curvePath,
  packetTimes,
  pointOnCurve,
  s,
  type Curve,
} from "../lib/layout";
import { outro, pop, tween } from "../lib/anim";

// Kabel digambar memanjang (stroke-dashoffset) lalu tetap dengan arus putus-putus
const Cable: React.FC<{ curve: Curve; delay: number }> = ({ curve, delay }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const p = pop(frame, fps, delay, theme.spring.smooth);
  const LEN = 520; // panjang aproksimasi kurva (cukup untuk dasharray)
  return (
    <>
      <path d={curvePath(curve)} fill="none" stroke={theme.colors.line} strokeWidth={3} strokeLinecap="round" strokeDasharray={LEN} strokeDashoffset={LEN * (1 - p)} />
      {/* Arus data halus mengalir di atas kabel */}
      <path
        d={curvePath(curve)}
        fill="none"
        stroke={theme.colors.textMute}
        strokeWidth={2}
        strokeLinecap="round"
        strokeDasharray="4 14"
        strokeDashoffset={-frame * 1.2}
        opacity={0.7 * p}
      />
    </>
  );
};

// Satu paket bercahaya + jejak (trail) di sepanjang kurva
const Packet: React.FC<{ curve: Curve; start: number; dur: number; reverse?: boolean; color?: string }> = ({
  curve,
  start,
  dur,
  reverse = false,
  color = theme.colors.primaryBright,
}) => {
  const frame = useCurrentFrame();
  if (frame < start || frame > start + dur + 3) return null;
  const TRAIL = 6;
  return (
    <>
      {Array.from({ length: TRAIL }).map((_, i) => {
        const tRaw = tween(frame - i * 1.1, [start, start + dur], [0, 1], theme.ease.flow);
        const t = reverse ? 1 - tRaw : tRaw;
        const pt = pointOnCurve(curve, t);
        const head = i === 0;
        const r = head ? 9 : 7 - i;
        // Pudar halus saat paket masuk port tujuan
        const arrive = tween(frame, [start + dur - 2, start + dur + 3], [1, 0], theme.ease.in);
        return (
          <circle
            key={i}
            cx={pt.x}
            cy={pt.y}
            r={Math.max(r, 1.5)}
            fill={color}
            opacity={(head ? 1 : 0.55 - i * 0.08) * arrive}
            style={head ? { filter: `drop-shadow(0 0 10px ${theme.colors.glow}) drop-shadow(0 0 22px ${theme.colors.glow})` } : undefined}
          />
        );
      })}
    </>
  );
};

export const Wires: React.FC = () => {
  const frame = useCurrentFrame();
  const out = outro(frame);
  return (
    <AbsoluteFill style={{ opacity: out.opacity, transform: `translateY(${out.shift}px)` }}>
      <svg width={CANVAS.width} height={CANVAS.height} style={{ position: "absolute", inset: 0 }}>
        <Cable curve={CURVE_A} delay={s(0.35)} />
        <Cable curve={CURVE_B} delay={s(0.5)} />
        {BEATS.map((b) => {
          const t = packetTimes(b);
          return (
            <React.Fragment key={b.key}>
              <Packet curve={CURVE_A} start={t.legAStart} dur={LEG_A} />
              <Packet curve={CURVE_B} start={t.legBStart} dur={LEG_B} />
            </React.Fragment>
          );
        })}
        {/* Webhook: status dibaca kembali ke server developer (arah balik, warna aksen) */}
        <Packet curve={CURVE_B} start={WEBHOOK.at} dur={WEBHOOK.leg} reverse color={theme.colors.accent} />
        <Packet curve={CURVE_A} start={WEBHOOK.legAStart} dur={WEBHOOK.leg} reverse color={theme.colors.accent} />
      </svg>
    </AbsoluteFill>
  );
};
