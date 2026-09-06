import { AbsoluteFill } from "remotion";
import {
  linearTiming,
  TransitionSeries,
  type TransitionPresentation,
  type TransitionPresentationComponentProps,
} from "@remotion/transitions";
import { SCENE_FRAMES, TRANSITION_FRAMES } from "./config";
import { Screen1 } from "./scenes/Screen1";
import { Screen2 } from "./scenes/Screen2";
import { Screen3 } from "./scenes/Screen3";
import { Screen4 } from "./scenes/Screen4";
import { Screen5 } from "./scenes/Screen5";

// Page-to-page: a 0.47s crossfade — outgoing content drifts up ~24px as it
// fades, incoming rises ~24px into place. That is the only page transition.
const FadeUp: React.FC<TransitionPresentationComponentProps<Record<string, unknown>>> = ({
  children,
  presentationProgress,
  presentationDirection,
}) => {
  const entering = presentationDirection === "entering";
  const opacity = entering ? presentationProgress : 1 - presentationProgress;
  const ty = entering ? (1 - presentationProgress) * 24 : -presentationProgress * 24;
  return <AbsoluteFill style={{ opacity, translate: `0px ${ty}px` }}>{children}</AbsoluteFill>;
};

const fadeUp = (): TransitionPresentation<Record<string, unknown>> => ({
  component: FadeUp,
  props: {},
});

const t = () => (
  <TransitionSeries.Transition
    presentation={fadeUp()}
    timing={linearTiming({ durationInFrames: TRANSITION_FRAMES })}
  />
);

export const ImageLabShowcase: React.FC = () => {
  return (
    <TransitionSeries>
      <TransitionSeries.Sequence durationInFrames={SCENE_FRAMES.s1}>
        <Screen1 />
      </TransitionSeries.Sequence>
      {t()}
      <TransitionSeries.Sequence durationInFrames={SCENE_FRAMES.s2}>
        <Screen2 />
      </TransitionSeries.Sequence>
      {t()}
      <TransitionSeries.Sequence durationInFrames={SCENE_FRAMES.s3}>
        <Screen3 />
      </TransitionSeries.Sequence>
      {t()}
      <TransitionSeries.Sequence durationInFrames={SCENE_FRAMES.s4}>
        <Screen4 />
      </TransitionSeries.Sequence>
      {t()}
      <TransitionSeries.Sequence durationInFrames={SCENE_FRAMES.s5}>
        <Screen5 />
      </TransitionSeries.Sequence>
    </TransitionSeries>
  );
};

// net frames = sum(scene durations) - 4 * TRANSITION_FRAMES
export const TOTAL_FRAMES =
  SCENE_FRAMES.s1 +
  SCENE_FRAMES.s2 +
  SCENE_FRAMES.s3 +
  SCENE_FRAMES.s4 +
  SCENE_FRAMES.s5 -
  4 * TRANSITION_FRAMES;
