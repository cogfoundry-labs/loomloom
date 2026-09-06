import { Img, staticFile } from "remotion";
import type { Ratio } from "../config";
import { COLOR } from "../theme";

// One storyboard sheet, framed tight to the image (no letterboxing — the border
// hugs the picture). Source JPEGs are exactly 3:2 or 16:9.
export const ratioOf = (r: Ratio) => (r === "3:2" ? 2 / 3 : 9 / 16);

// Fit a sheet of the given ratio into a box, returning the drawn image size.
export const fit = (r: Ratio, maxW: number, maxH: number) => {
  const ar = ratioOf(r);
  let w = maxW;
  let h = w * ar;
  if (h > maxH) {
    h = maxH;
    w = h / ar;
  }
  return { w: Math.round(w), h: Math.round(h) };
};

export const Sheet: React.FC<{
  file: string;
  width: number;
  height: number;
  border?: number;
  opacity?: number;
  scale?: number;
}> = ({ file, width, height, border = 2, opacity = 1, scale = 1 }) => {
  return (
    <div
      style={{
        width,
        height,
        opacity,
        scale,
        border: `${border}px solid ${COLOR.frame}`,
        background: "#ffffff",
        flexShrink: 0,
      }}
    >
      <Img src={staticFile(file)} style={{ width: "100%", height: "100%", display: "block" }} />
    </div>
  );
};
