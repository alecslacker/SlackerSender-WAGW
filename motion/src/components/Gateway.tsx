// Node gateway di tengah: logo SlackerSender + cincin putar + denyut tiap paket lewat.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { Img, staticFile, useCurrentFrame, useVideoConfig } from "remotion";
import { theme } from "../theme";
import { BEATS, DURATION, GATEWAY, packetTimes, s } from "../lib/layout";
import { breathe, outro, pop, tween } from "../lib/anim";

// Gelombang denyut saat paket diproses
const Pulse: React.FC<{ at: number }> = ({ at }) => {
  const frame = useCurrentFrame();
  const t = frame - at;
  if (t < 0 || t > s(0.9)) return null;
  const grow = tween(frame, [at, at + s(0.9)], [0, 1], theme.ease.out);
  return (
    <div
      style={{
        position: "absolute",
        left: GATEWAY.cx,
        top: GATEWAY.cy,
        width: GATEWAY.r * 2,
        height: GATEWAY.r * 2,
        marginLeft: -GATEWAY.r,
        marginTop: -GATEWAY.r,
        borderRadius: "50%",
        border: `2px solid ${theme.colors.primary}`,
        transform: `scale(${1 + grow * 1.15})`,
        opacity: 0.8 * (1 - grow),
      }}
    />
  );
};

export const Gateway: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const inP = pop(frame, fps, s(0.18), theme.spring.bouncy);
  const out = outro(frame);
  // Rotasi siklus penuh per loop → mulus di batas loop
  const rot = (frame / DURATION) * 360;
  // Kedip terang saat paket tiba di gateway
  const flash = BEATS.reduce((acc, b) => {
    const { gwAt } = packetTimes(b);
    const d = frame - gwAt;
    return d >= 0 && d < s(0.4) ? Math.max(acc, 1 - d / s(0.4)) : acc;
  }, 0);

  const size = GATEWAY.r * 2;

  return (
    <>
      {BEATS.map((b) => (
        <Pulse key={b.key} at={packetTimes(b).gwAt} />
      ))}
      <div
        style={{
          position: "absolute",
          left: GATEWAY.cx - GATEWAY.r,
          top: GATEWAY.cy - GATEWAY.r + out.shift,
          width: size,
          height: size,
          opacity: inP * out.opacity,
          transform: `scale(${(0.6 + inP * 0.4) * breathe(frame, 26, 0.015) * (1 + flash * 0.06)})`,
        }}
      >
        {/* Cincin putus-putus berputar = gateway aktif */}
        <svg width={size + 40} height={size + 40} style={{ position: "absolute", left: -20, top: -20, transform: `rotate(${rot}deg)` }}>
          <circle
            cx={(size + 40) / 2}
            cy={(size + 40) / 2}
            r={GATEWAY.r + 14}
            fill="none"
            stroke={theme.colors.line}
            strokeWidth={2}
            strokeDasharray="10 9"
          />
        </svg>
        {/* Inti gateway */}
        <div
          style={{
            position: "absolute",
            inset: 0,
            borderRadius: "50%",
            background: `radial-gradient(circle at 50% 38%, ${theme.colors.panelHead}, ${theme.colors.panel})`,
            border: `2px solid ${theme.colors.primary}`,
            boxShadow: `0 0 ${40 + flash * 50}px ${theme.colors.glow}, inset 0 0 30px rgba(34,197,94,0.12)`,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
          }}
        >
          <Img src={staticFile("logo.png")} style={{ width: size * 0.56, height: size * 0.56 }} />
        </div>
      </div>
      {/* Label di bawah node */}
      <div
        style={{
          position: "absolute",
          left: GATEWAY.cx - 160,
          width: 320,
          top: GATEWAY.cy + GATEWAY.r + 34 + out.shift,
          textAlign: "center",
          fontFamily: theme.fonts.display,
          fontSize: 34,
          letterSpacing: 4,
          color: theme.colors.text,
          opacity: pop(frame, fps, s(0.5), theme.spring.smooth) * out.opacity,
        }}
      >
        SLACKERSENDER
        <div style={{ fontFamily: theme.fonts.mono, fontSize: 15, letterSpacing: 3, color: theme.colors.textDim, marginTop: 2 }}>
          GATEWAY · ONLINE
        </div>
      </div>
    </>
  );
};
