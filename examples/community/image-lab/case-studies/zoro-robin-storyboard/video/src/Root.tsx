import "./index.css";
import { Composition } from "remotion";
import { FPS, HEIGHT, WIDTH } from "./config";
import { ImageLabShowcase, TOTAL_FRAMES } from "./ImageLabShowcase";

export const RemotionRoot: React.FC = () => {
  return (
    <Composition
      id="ImageLabShowcase"
      component={ImageLabShowcase}
      durationInFrames={TOTAL_FRAMES}
      fps={FPS}
      width={WIDTH}
      height={HEIGHT}
    />
  );
};
