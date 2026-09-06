// Palette + type, lifted from the Image Lab case-study page.
// Accent (#007a5c) is used exactly ONCE in the video — the "IMAGE LAB" wordmark
// on the final screen. Everything else is ink / muted / hairline on the near-
// white canvas.

import { loadFont as loadDisplay } from "@remotion/google-fonts/ArchivoBlack";
import { loadFont as loadMono } from "@remotion/google-fonts/IBMPlexMono";

// "Arial Black" does not render in Remotion's headless Chromium; Archivo Black is
// the heavy uppercase display face the redesign-lab case studies actually use.
const display = loadDisplay();
const mono = loadMono("normal", { weights: ["400", "500", "700"] });

export const DISPLAY = display.fontFamily; // "Archivo Black"
export const MONO = mono.fontFamily; // "IBM Plex Mono"

export const COLOR = {
  bg: "#f4f4f0",
  ink: "#0b0b0b",
  muted: "#4a4a46",
  faint: "#68675e",
  hairline: "#d8d6ce",
  frame: "#0b0b0b",
  accent: "#007a5c", // final screen only
} as const;
