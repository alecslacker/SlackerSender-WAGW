// Lapisan dasar: mesh glow bergerak + grid terminal ber-mask (bukan latar flat).
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { AbsoluteFill, useCurrentFrame } from "remotion";
import { theme } from "../theme";
import { DURATION } from "../lib/layout";

export const Backdrop: React.FC = () => {
  const frame = useCurrentFrame();
  // Gerak siklus penuh dalam satu loop → frame 0 == frame 300 (mulus)
  const phase = (frame / DURATION) * Math.PI * 2;
  const d1 = Math.sin(phase) * 60;
  const d2 = Math.cos(phase) * 45;

  return (
    <AbsoluteFill style={{ background: theme.colors.bg }}>
      <div
        style={{
          position: "absolute",
          width: 1100,
          height: 1100,
          borderRadius: "50%",
          left: 260 + d1,
          top: -420,
          filter: "blur(60px)",
          background: `radial-gradient(circle, ${theme.colors.glowSoft}, transparent 62%)`,
        }}
      />
      <div
        style={{
          position: "absolute",
          width: 900,
          height: 900,
          borderRadius: "50%",
          right: -260 - d2,
          bottom: -460,
          filter: "blur(80px)",
          background: `radial-gradient(circle, ${theme.colors.glowSoft}, transparent 65%)`,
        }}
      />
      {/* Grid terminal 40px, fokus di tengah lalu memudar ke tepi */}
      <AbsoluteFill
        style={{
          backgroundImage: `linear-gradient(${theme.colors.lineDim} 1px, transparent 1px), linear-gradient(90deg, ${theme.colors.lineDim} 1px, transparent 1px)`,
          backgroundSize: "40px 40px",
          backgroundPosition: `0px ${(frame * 0.4) % 40}px`,
          maskImage: "radial-gradient(ellipse 72% 70% at 50% 48%, black 28%, transparent 80%)",
          WebkitMaskImage:
            "radial-gradient(ellipse 72% 70% at 50% 48%, black 28%, transparent 80%)",
          opacity: 0.85,
        }}
      />
    </AbsoluteFill>
  );
};
