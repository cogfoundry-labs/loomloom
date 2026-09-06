import { AbsoluteFill } from "remotion";
import { COPY, MARGIN, SHOTS, TYPE } from "../config";
import { COLOR, DISPLAY, MONO } from "../theme";
import { cellRect } from "../grid";
import { Sheet } from "../components/Sheet";

// 3 MODELS — all 8 real generations in a clean 2x4 grid. The whole block rides
// the scene transition in as one composition (no per-cell stagger).
export const Screen2: React.FC = () => {
  return (
    <AbsoluteFill style={{ background: COLOR.bg }}>
      <div
        style={{
          position: "absolute",
          left: MARGIN,
          top: MARGIN + 30,
          fontFamily: DISPLAY,
          fontSize: TYPE.headline,
          letterSpacing: "-0.02em",
          color: COLOR.ink,
        }}
      >
        {COPY.s2.headline}
      </div>

      {SHOTS.map((shot, i) => {
        const r = cellRect(i);
        return (
          <div key={shot.label} style={{ position: "absolute", left: r.imgX, top: r.y, width: r.imgW }}>
            <Sheet file={shot.file} width={r.imgW} height={r.imgH} border={1} />
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
          </div>
        );
      })}
    </AbsoluteFill>
  );
};
