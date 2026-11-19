// Panel terminal developer: mengetik request API Slackersender, lalu respons gateway.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { useCurrentFrame, useVideoConfig } from "remotion";
import { theme } from "../theme";
import { BEATS, TERMINAL, WEBHOOK, packetTimes, s } from "../lib/layout";
import { breathe, entrance, float, outro, pop, tween, typed } from "../lib/anim";

const LINE_H = 30;

// Satu blok perintah + respons per beat
const CommandBlock: React.FC<{ index: number }> = ({ index }) => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();
  const beat = BEATS[index];
  const { gwAt } = packetTimes(beat);

  const cmd = typed(frame, beat.typeStart, beat.cmd, 1.5);
  const typingDone = cmd.length === beat.cmd.length;
  const caretOn = !typingDone && frame >= beat.typeStart && Math.floor(frame / 8) % 2 === 0;

  // Respons muncul ketika paket sampai di gateway (bukti sinkron)
  const respP = pop(frame, fps, gwAt, theme.spring.snappy);
  const visible = frame >= beat.typeStart - 2;

  if (!visible) return null;
  return (
    <div style={{ marginBottom: 18 }}>
      <div style={{ display: "flex", gap: 12, height: LINE_H, alignItems: "center" }}>
        <span style={{ color: theme.colors.primary }}>$</span>
        <span style={{ color: theme.colors.white }}>
          {cmd}
          <span style={{ opacity: caretOn ? 1 : 0, color: theme.colors.text }}>▍</span>
        </span>
      </div>
      <div
        style={{
          display: "flex",
          gap: 12,
          height: LINE_H,
          alignItems: "center",
          paddingLeft: 24,
          ...entrance(respP, 10, 0.98),
        }}
      >
        <span style={{ color: theme.colors.textMute }}>→</span>
        <span style={{ color: theme.colors.text }}>{beat.resp}</span>
        <span
          style={{
            marginLeft: 6,
            fontSize: 17,
            color: theme.colors.text,
            border: `1px solid ${theme.colors.textDim}`,
            borderRadius: 4,
            padding: "0px 7px",
            fontWeight: 700,
            lineHeight: "24px",
          }}
        >
          200 OK
        </span>
      </div>
    </div>
  );
};

export const Terminal: React.FC = () => {
  const frame = useCurrentFrame();
  const { fps } = useVideoConfig();

  const inP = pop(frame, fps, s(0.05), theme.spring.smooth);
  const out = outro(frame);
  const hookP = pop(frame, fps, WEBHOOK.arriveTerminal, theme.spring.snappy);
  // Tepi panel menyala sesaat saat webhook masuk (bukti event diterima)
  const hookGlow = tween(frame, [WEBHOOK.arriveTerminal, WEBHOOK.arriveTerminal + s(0.8)], [1, 0], theme.ease.out) * (frame >= WEBHOOK.arriveTerminal ? 1 : 0);

  return (
    <div
      style={{
        position: "absolute",
        left: TERMINAL.x,
        top: TERMINAL.y + float(frame, 34, 3) + out.shift,
        width: TERMINAL.w,
        height: TERMINAL.h,
        opacity: inP * out.opacity,
        transform: `translateY(${(1 - inP) * 40}px) scale(${(0.95 + inP * 0.05) * breathe(frame, 40, 0.004)})`,
        borderRadius: 18,
        background: theme.colors.panel,
        border: `1px solid ${theme.colors.line}`,
        boxShadow: `0 40px 80px -24px rgba(0,0,0,0.75), 0 0 0 1px rgba(56,189,248,${0.55 * hookGlow}), 0 0 ${40 * hookGlow}px rgba(56,189,248,${0.25 * hookGlow})`,
        overflow: "hidden",
        fontFamily: theme.fonts.mono,
      }}
    >
      {/* Bilah judul jendela */}
      <div
        style={{
          height: 52,
          background: theme.colors.panelHead,
          borderBottom: `1px solid ${theme.colors.line}`,
          display: "flex",
          alignItems: "center",
          padding: "0 20px",
          gap: 10,
        }}
      >
        {[theme.colors.red, theme.colors.amber, theme.colors.primary].map((c) => (
          <span key={c} style={{ width: 13, height: 13, borderRadius: 13, background: c, opacity: 0.85 }} />
        ))}
        <span style={{ marginLeft: 14, fontSize: 18, color: theme.colors.textDim, letterSpacing: 0.4 }}>
          slackersender — api
        </span>
      </div>

      <div style={{ padding: "26px 28px", fontSize: 22, lineHeight: `${LINE_H}px` }}>
        <div style={{ color: theme.colors.textMute, marginBottom: 14, height: LINE_H }}>
          # kirim lewat gateway sendiri
        </div>
        {BEATS.map((_, i) => (
          <CommandBlock key={i} index={i} />
        ))}
        {/* Webhook balik dari gateway */}
        {frame >= WEBHOOK.arriveTerminal - 1 ? (
          <div style={{ display: "flex", gap: 12, height: LINE_H, alignItems: "center", ...entrance(hookP, 10, 0.98) }}>
            <span style={{ color: theme.colors.accent }}>⇠</span>
            <span style={{ color: theme.colors.textDim }}>webhook</span>
            <span style={{ color: theme.colors.white }}>{WEBHOOK.event}</span>
          </div>
        ) : null}
      </div>
    </div>
  );
};
