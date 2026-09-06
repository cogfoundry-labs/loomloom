import { AbsoluteFill, Easing, interpolate, useCurrentFrame } from "remotion";
import { COPY, TYPE } from "../config";
import { COLOR, DISPLAY, MONO } from "../theme";

// The number. No image. A quiet proof point: real generations, real models,
// real cost. End on the figure. "IMAGE LAB" is the only green on the whole video.
export const Screen5: React.FC = () => {
  const frame = useCurrentFrame();

  const rise = (a: number, b: number) => ({
    opacity: interpolate(frame, [a, b], [0, 1], {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: Easing.out(Easing.ease),
    }),
    translate: `0px ${interpolate(frame, [a, b], [16, 0], {
      extrapolateLeft: "clamp",
      extrapolateRight: "clamp",
      easing: Easing.out(Easing.ease),
    })}px`,
  });

  return (
    <AbsoluteFill
      style={{
        background: COLOR.bg,
        display: "flex",
        flexDirection: "column",
        alignItems: "center",
        justifyContent: "center",
      }}
    >
      <div
        style={{
          fontFamily: DISPLAY,
          fontSize: TYPE.bignum,
          letterSpacing: "-0.03em",
          color: COLOR.ink,
          ...rise(6, 22),
        }}
      >
        {COPY.s5.bignum}
      </div>
      <div
        style={{
          marginTop: 28,
          fontFamily: MONO,
          fontWeight: 500,
          fontSize: TYPE.monoPrimary,
          letterSpacing: `${TYPE.tracking}em`,
          textTransform: "uppercase",
          color: COLOR.muted,
          ...rise(16, 32),
        }}
      >
        {COPY.s5.line}
      </div>

      <div
        style={{
          position: "absolute",
          bottom: 150,
          fontFamily: MONO,
          fontWeight: 700,
          fontSize: 34,
          letterSpacing: "0.14em",
          textTransform: "uppercase",
          color: COLOR.accent,
          ...rise(28, 44),
        }}
      >
        {COPY.s5.wordmark}
      </div>
    </AbsoluteFill>
  );
};
