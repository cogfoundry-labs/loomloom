import { AbsoluteFill, Easing, interpolate, useCurrentFrame } from "remotion";
import { COPY, MARGIN, SHOTS, TYPE } from "../config";
import { COLOR, DISPLAY } from "../theme";
import { Sheet } from "../components/Sheet";

// EXPLORE. — the 8 alternatives as one tall editorial strip, ONE slow continuous
// vertical move. It eases to a near-stop at three points so the eye can register
// each image, then rolls on. No cuts, no flips.
const STRIP_W = 700;
const GAP = 60;
const H_32 = (STRIP_W * 2) / 3; // 467
const H_169 = (STRIP_W * 9) / 16; // 394
const BAND_H = 210;

export const Screen3: React.FC = () => {
  const frame = useCurrentFrame();

  // Four legs, each Easing.inOut → a soft deceleration into three ~10-frame holds
  // (the equal-value pairs) and a soft acceleration back out.
  const y = interpolate(
    frame,
    [10, 34, 44, 66, 76, 96, 106, 114],
    [140, -410, -410, -960, -960, -1510, -1510, -1971],
    {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: Easing.inOut(Easing.ease),
    },
  );

  return (
    <AbsoluteFill style={{ background: COLOR.bg, overflow: "hidden" }}>
      <div
        style={{
          position: "absolute",
          left: "50%",
          top: 0,
          translate: `-50% ${y}px`,
          display: "flex",
          flexDirection: "column",
          gap: GAP,
        }}
      >
        {SHOTS.map((shot) => (
          <Sheet
            key={shot.label}
            file={shot.file}
            width={STRIP_W}
            height={shot.ratio === "3:2" ? H_32 : H_169}
            border={2}
          />
        ))}
      </div>

      <div style={{ position: "absolute", top: 0, left: 0, right: 0, height: BAND_H, background: COLOR.bg, zIndex: 2 }} />
      <div
        style={{
          position: "absolute",
          left: MARGIN,
          top: MARGIN,
          fontFamily: DISPLAY,
          fontSize: TYPE.headline,
          letterSpacing: "-0.02em",
          color: COLOR.ink,
          zIndex: 3,
        }}
      >
        {COPY.s3.headline}
      </div>
    </AbsoluteFill>
  );
};
