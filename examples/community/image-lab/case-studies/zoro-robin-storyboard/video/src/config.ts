// ─────────────────────────────────────────────────────────────────────────────
// IMAGE LAB — ZORO & ROBIN STORYBOARD · showcase video
// Every timing, size and string lives here. Edit, save, Studio hot-reloads.
// ─────────────────────────────────────────────────────────────────────────────

export const FPS = 30;
export const WIDTH = 1080;
export const HEIGHT = 1920;

// Safe area (video-layout rules: >=80px sides, >=100px top/bottom on 1080-wide)
export const MARGIN = 96;

// Page-to-page crossfade: outgoing drifts up, incoming rises into place.
export const TRANSITION_FRAMES = 14; // ~0.47s

// Per-scene length INCLUDING its transition overlap. Net video =
// sum(durations) - 4 * TRANSITION_FRAMES.  480 - 56 = 424 frames ≈ 14.1s
export const SCENE_FRAMES = {
  s1: 90,
  s2: 96,
  s3: 120,
  s4: 90,
  s5: 84,
} as const;

// ── the 8 real generations, in the case study's order ───────────────────────
export type Ratio = "3:2" | "16:9";
export type Shot = { file: string; label: string; model: string; ratio: Ratio };

export const SHOTS: Shot[] = [
  { file: "alternative-01.jpg", label: "A", model: "GPT Image 2", ratio: "3:2" },
  { file: "alternative-02.jpg", label: "B", model: "GPT Image 2", ratio: "3:2" },
  { file: "alternative-03.jpg", label: "C", model: "GPT Image 2", ratio: "3:2" },
  { file: "alternative-04.jpg", label: "D", model: "Nano Banana Pro", ratio: "16:9" },
  { file: "alternative-05.jpg", label: "E", model: "Nano Banana Pro", ratio: "16:9" },
  { file: "alternative-06.jpg", label: "F", model: "Nano Banana Pro", ratio: "16:9" },
  { file: "alternative-07.jpg", label: "G", model: "Nano Banana 2", ratio: "16:9" },
  { file: "alternative-08.jpg", label: "H", model: "Nano Banana 2", ratio: "16:9" },
];

export const HERO_INDEX = 2; // C — cleanest sheet; NOT "the winning model"

// ── type scale (px on a 1080-wide frame) ───────────────────────────────────
export const TYPE = {
  headline: 100, // ONE BRIEF. / 3 MODELS / PICK YOUR IMAGE.
  bignum: 172, // $0.5547
  monoPrimary: 40, // important supporting mono line
  monoSecondary: 30, // second-tier mono line
  monoCell: 22, // grid cell label
  tracking: 0.06, // letter-spacing (em) for mono uppercase
} as const;

// ── copy ───────────────────────────────────────────────────────────────────
export const COPY = {
  s1: {
    line1: "ONE BRIEF.",
    line2: "EIGHT CHOICES.",
    supporting: "ZORO & ROBIN · AI STORYBOARD",
    meta: "8 ALTERNATIVES",
  },
  s2: { headline: "3 MODELS" },
  s3: { headline: "EXPLORE." },
  s4: {
    headline: "PICK YOUR IMAGE.",
    meta: "C · GPT IMAGE 2 · $0.1696",
  },
  s5: {
    bignum: "$0.5547",
    line: "8 GENERATIONS · 3 MODELS",
    wordmark: "IMAGE LAB",
  },
} as const;
