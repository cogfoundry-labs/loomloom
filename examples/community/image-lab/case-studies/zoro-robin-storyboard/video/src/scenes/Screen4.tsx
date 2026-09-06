import { AbsoluteFill, Easing, interpolate, useCurrentFrame } from "remotion";
import { COPY, HERO_INDEX, MARGIN, SHOTS, TYPE } from "../config";
import { COLOR, DISPLAY, MONO } from "../theme";
import { cellRect } from "../grid";
import { Sheet } from "../components/Sheet";

// PICK YOUR IMAGE. — opens on Screen 2's exact grid (visual callback), then the
// seven other alternatives recede to 12% and C lifts forward to become dominant.
// No "winner" badge, no ranking language — a human choosing, not a benchmark.
const HERO = { x: 40, y: 626, w: 1000, h: 667 }; // centered, 3:2

const ease = (frame: number, w: [number, number], out: [number, number]) =>
  interpolate(frame, w, out, {
    extrapolateLeft: "clamp",
    extrapolateRight: "clamp",
    easing: Easing.out(Easing.ease),
  });

export const Screen4: React.FC = () => {
  const frame = useCurrentFrame();

  return (
    <AbsoluteFill style={{ background: COLOR.bg }}>
      <div
        style={{
          position: "absolute",
          left: MARGIN,
          top: MARGIN + 30,
          fontFamily: DISPLAY,
          fontSize: TYPE.headline,
          lineHeight: 1.02,
          letterSpacing: "-0.02em",
          color: COLOR.ink,
          zIndex: 3,
        }}
      >
        PICK YOUR
        <br />
        IMAGE.
      </div>

      {SHOTS.map((shot, i) => {
        const r = cellRect(i);
        const hero = i === HERO_INDEX;
        const x = hero ? ease(frame, [10, 46], [r.imgX, HERO.x]) : r.imgX;
        const y = hero ? ease(frame, [10, 46], [r.y, HERO.y]) : r.y;
        const w = hero ? ease(frame, [10, 46], [r.imgW, HERO.w]) : r.imgW;
        const h = hero ? ease(frame, [10, 46], [r.imgH, HERO.h]) : r.imgH;
        const opacity = hero
          ? 1
          : interpolate(frame, [10, 34], [1, 0.09], {
              extrapolateLeft: "clamp",
              extrapolateRight: "clamp",
              easing: Easing.inOut(Easing.ease),
            });
        return (
          <div key={shot.label} style={{ position: "absolute", left: x, top: y, opacity, zIndex: hero ? 2 : 1 }}>
            <Sheet file={shot.file} width={w} height={h} border={hero ? 2 : 1} />
            {!hero && (
              <div
                style={{
                  marginTop: 14,
                  fontFamily: MONO,
                  fontWeight: 700,
                  fontSize: TYPE.monoCell,
                  letterSpacing: `${TYPE.tracking}em`,
                  textTransform: "uppercase",
                  color: COLOR.faint,
                }}
              >
                <span style={{ color: COLOR.ink }}>{shot.label}</span>
                {"  ·  "}
                {shot.model}
              </div>
            )}
          </div>
        );
      })}

      <div
        style={{
          position: "absolute",
          left: HERO.x,
          top: HERO.y + HERO.h + 22,
          width: HERO.w,
          textAlign: "center",
          fontFamily: MONO,
          fontWeight: 700,
          fontSize: TYPE.monoSecondary,
          letterSpacing: `${TYPE.tracking}em`,
          textTransform: "uppercase",
          color: COLOR.ink,
          opacity: ease(frame, [42, 58], [0, 1]),
          zIndex: 3,
        }}
      >
        {COPY.s4.meta}
      </div>
    </AbsoluteFill>
  );
};
