import { AbsoluteFill, Easing, interpolate, useCurrentFrame } from "remotion";
import { COPY, MARGIN, TYPE } from "../config";
import { COLOR, DISPLAY, MONO } from "../theme";
import { Sheet } from "../components/Sheet";

// ONE BRIEF. EIGHT CHOICES. — the hero sheet, large and centered, one stack with
// equal air above and below. The scene rises in via the transition; only the
// image has its own motion: a slow settle then an almost-imperceptible Ken-Burns.
export const Screen1: React.FC = () => {
  const frame = useCurrentFrame();
  const IMG_W = 940;
  const IMG_H = Math.round((IMG_W * 2) / 3); // C is 3:2 → 627

  return (
    <AbsoluteFill
      style={{
        background: COLOR.bg,
        padding: `${MARGIN}px ${MARGIN}px`,
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-start",
        justifyContent: "center",
        gap: 64,
      }}
    >
      <div>
        <div
          style={{
            fontFamily: DISPLAY,
            fontSize: TYPE.headline,
            lineHeight: 1.02,
            letterSpacing: "-0.02em",
            color: COLOR.ink,
          }}
        >
          {COPY.s1.line1}
          <br />
          {COPY.s1.line2}
        </div>
        <div
          style={{
            marginTop: 24,
            fontFamily: MONO,
            fontWeight: 500,
            fontSize: TYPE.monoSecondary,
            letterSpacing: `${TYPE.tracking}em`,
            textTransform: "uppercase",
            color: COLOR.faint,
          }}
        >
          {COPY.s1.supporting}
        </div>
      </div>

      <Sheet
        file="alternative-03.jpg"
        width={IMG_W}
        height={IMG_H}
        scale={interpolate(frame, [0, 20, 90], [0.98, 1.0, 1.03], {
          extrapolateLeft: "clamp",
          extrapolateRight: "clamp",
          easing: Easing.inOut(Easing.ease),
        })}
      />

      <div
        style={{
          fontFamily: MONO,
          fontWeight: 700,
          fontSize: TYPE.monoPrimary,
          letterSpacing: `${TYPE.tracking}em`,
          textTransform: "uppercase",
          color: COLOR.ink,
        }}
      >
        {COPY.s1.meta}
      </div>
    </AbsoluteFill>
  );
};
