import { SHOTS, WIDTH } from "./config";
import { fit } from "./components/Sheet";

// The 2-column x 4-row grid, shared by Screen 2 (all 8) and Screen 4 (which
// lifts the hero out of it). Reading order A B / C D / E F / G H — grid
// position does not encode model grouping; the per-cell labels do.
export const GRID = {
  cols: 2,
  sideMargin: 80,
  gutterX: 40,
  cellMaxH: 244, // sheets fit within this; rows align on it
  labelH: 42,
  gutterY: 40,
  topY: 380,
} as const;

export const CELL_W = (WIDTH - GRID.sideMargin * 2 - GRID.gutterX) / GRID.cols; // 440
export const ROW_H = GRID.cellMaxH + GRID.labelH + GRID.gutterY;

// The 3:2 sheets are narrower than the 16:9 ones. Rather than centre each in an
// invisible cell (ragged both edges), align column 0 to the left margin and
// column 1 to the right margin — the grid's outer edges stay clean.
export const cellRect = (i: number) => {
  const col = i % GRID.cols;
  const row = Math.floor(i / GRID.cols);
  const { w: imgW, h: imgH } = fit(SHOTS[i].ratio, CELL_W, GRID.cellMaxH);
  const imgX = col === 0 ? GRID.sideMargin : WIDTH - GRID.sideMargin - imgW;
  return { imgX, y: GRID.topY + row * ROW_H, imgW, imgH };
};
