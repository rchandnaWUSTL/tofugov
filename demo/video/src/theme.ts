import {loadFont as loadMono} from '@remotion/google-fonts/JetBrainsMono';
import {loadFont as loadSans} from '@remotion/google-fonts/Inter';

export const mono = loadMono('normal', {weights: ['400', '700'], subsets: ['latin']}).fontFamily;
export const sans = loadSans('normal', {weights: ['400', '600', '800'], subsets: ['latin']}).fontFamily;

export const colors = {
  bg: '#0b0f17',
  panel: '#0d1117',
  border: '#2a3140',
  text: '#e6edf3',
  muted: '#8b949e',
  red: '#ff6b6b',
  amber: '#f2b544',
  green: '#3fb950',
  blue: '#58a6ff',
};

// xterm's 16-color palette, tuned for a dark background.
export const palette = [
  '#484f58', '#ff7b72', '#3fb950', '#d29922', '#58a6ff', '#bc8cff', '#39c5cf', '#b1bac4',
  '#6e7681', '#ffa198', '#56d364', '#e3b341', '#79c0ff', '#d2a8ff', '#56d4dd', '#f0f6fc',
];

export const bandColor: Record<string, string> = {
  HIGH: colors.red,
  MEDIUM: colors.amber,
  LOW: colors.green,
  UNSCORED: colors.muted,
};
