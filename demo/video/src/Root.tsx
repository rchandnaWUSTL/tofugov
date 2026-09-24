import {Composition} from 'remotion';
import {Demo, DEMO_FRAMES, FPS} from './Demo';

export const Root = () => (
  <>
    <Composition id="Demo" component={Demo} defaultProps={{captions: true}} durationInFrames={DEMO_FRAMES} fps={FPS} width={1920} height={1080} />
    <Composition id="DemoNoCaptions" component={Demo} defaultProps={{captions: false}} durationInFrames={DEMO_FRAMES} fps={FPS} width={1920} height={1080} />
  </>
);
