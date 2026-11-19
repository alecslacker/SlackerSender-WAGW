// Helper animasi — membungkus interpolate/spring supaya SELALU ber-easing & ter-clamp.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import { interpolate, spring } from "remotion";
import { theme } from "../theme";
import { DURATION, OUTRO_START } from "./layout";

type EasingFn = (t: number) => number;

// interpolate dengan clamp di kedua sisi + easing wajib
export const tween = (
  frame: number,
  input: [number, number],
  output: [number, number],
  easing: EasingFn = theme.ease.out,
) =>
  interpolate(frame, input, output, {
    easing,
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
  });

// Progress spring 0→1 mulai dari frame `delay`
type SpringPreset = { damping: number; stiffness: number; mass: number };
export const pop = (
  frame: number,
  fps: number,
  delay: number,
  config: SpringPreset = theme.spring.smooth,
) => spring({ frame: frame - delay, fps, config });

// Entrance standar: opacity + translateY + scale sekaligus (bukan fade polos)
export const entrance = (p: number, rise = 26, fromScale = 0.94) => ({
  opacity: p,
  transform: `translateY(${interpolate(p, [0, 1], [rise, 0])}px) scale(${interpolate(
    p,
    [0, 1],
    [fromScale, 1],
  )})`,
});

// Exit global: elemen yang tampil di akhir loop mundur bersama sebelum frame 300
// — lebih cepat dari entrance (≈ 0.65 dtk vs ≈ 1 dtk).
export const outro = (frame: number) => {
  const p = tween(frame, [OUTRO_START, DURATION - 2], [0, 1], theme.ease.in);
  return { opacity: 1 - p, shift: -18 * p };
};

// Napas idle untuk elemen yang diam > 2 dtk
export const breathe = (frame: number, period = 22, amp = 0.012) =>
  1 + Math.sin(frame / period) * amp;

export const float = (frame: number, period = 30, amp = 3) =>
  Math.sin(frame / period) * amp;

// Jumlah karakter terketik pada frame tertentu (kecepatan per karakter)
export const typed = (frame: number, start: number, text: string, perChar = 1.6) => {
  const n = Math.floor((frame - start) / perChar);
  return text.slice(0, Math.max(0, Math.min(text.length, n)));
};
