// Registrasi komposisi Remotion.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import React from "react";
import { Composition } from "remotion";
import { HeroLoop } from "./scenes/HeroLoop";
import { CANVAS, DURATION, FPS } from "./lib/layout";

export const Root: React.FC = () => (
  <Composition
    id="HeroLoop"
    component={HeroLoop}
    durationInFrames={DURATION}
    fps={FPS}
    width={CANVAS.width}
    height={CANVAS.height}
  />
);
