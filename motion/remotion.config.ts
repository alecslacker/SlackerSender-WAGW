// Konfigurasi render Remotion — animasi hero SlackerSender.
// Author: Mas Wondho — Duta Corpora Indonesia (DCI)
// Copyright © 2026 Duta Corpora Indonesia
import { Config } from "@remotion/cli/config";

// PNG menjaga detail teks terminal kecil (JPEG bikin huruf mono "kabur").
Config.setVideoImageFormat("png");
Config.setOverwriteOutput(true);
