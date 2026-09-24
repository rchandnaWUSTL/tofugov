import React from 'react';
import {AbsoluteFill, Easing, Sequence, interpolate, spring, useCurrentFrame, useVideoConfig} from 'remotion';
import oldCastJson from './casts/old.json';
import newCastJson from './casts/new.json';
import {Cast, LINE_HEIGHT, Line, PAD, TITLE_BAR, Terminal, frameAt, lineText} from './Terminal';
import {bandColor, colors, mono, sans} from './theme';

const oldCast = oldCastJson as Cast;
const newCast = newCastJson as Cast;

export const FPS = 30;
const S = (s: number) => Math.round(s * FPS);

// ------------------------------------------------------------------ pacing
// BBC subtitle guidance is 160-180 words per minute. We use the low end
// because viewers are also watching the picture.
const WPM = 160;
// Time to notice new text and move the eyes to it before reading starts.
const NOTICE = 1.0;
// Time to find the thing a caption points at (a boxed line, a table row).
const LOOK = 0.8;
const FADE = 0.3;
const wordCount = (...texts: string[]) => texts.reduce((n, t) => n + t.trim().split(/\s+/).length, 0);
const readSecs = (...texts: string[]) => NOTICE + wordCount(...texts) / (WPM / 60);

const COPY = {
  oldChip: 'Upgrading without tofugov',
  oldCaption: 'Workspace 6 of 200 rotates the database password',
  newChip: 'Upgrading with tofugov',
  newCaption: 'The same change, flagged without reading a plan',
  newFallback: 'Riskiest workspaces first',
  ctaTitle: 'Try it on your workspaces',
  ctaCommands: [
    'go install github.com/rchandnaWUSTL/tofugov/cmd/tofugov@latest',
    'tofugov upgrade --provider hashicorp/aws=6.0.0 ./workspaces/*',
  ],
  ctaUrl: 'github.com/rchandnaWUSTL/tofugov',
};

// Old scene: type `tofu plan`, stream the output, zoom onto the password
// change, then shrink into a grid of 200 workspaces.
const oldTypedIdx = oldCast.frames.findIndex((fr) => lineText(fr.lines[0]).trim() === '$ tofu plan');
const OLD = (() => {
  const castStart = 0.5;
  const streamStart = castStart + oldCast.frames[Math.max(oldTypedIdx, 0)].t + 0.3;
  const streamEnd = streamStart + 1.8;
  const zoom = streamEnd + 0.3;
  const caption = zoom + 0.3;
  const shrink = caption + FADE + readSecs(COPY.oldCaption) + LOOK;
  const captionEnd = shrink + 0.7 + 1.2 + FADE;
  return {castStart, streamStart, streamEnd, zoom, caption, shrink, captionEnd, end: captionEnd + 0.1};
})();

// New scene: type the command, show the table, point at the password row.
const NEW = (() => {
  const castStart = 0.5;
  const tableShown = castStart + newCast.duration + 0.2;
  const caption = tableShown + 1.0;
  const captionEnd = caption + FADE + readSecs(COPY.newCaption) + LOOK + FADE;
  return {castStart, tableShown, caption, captionEnd, end: captionEnd + 0.1};
})();

const CLOSE = (() => {
  const end = 0.1 + FADE + readSecs(COPY.ctaTitle, ...COPY.ctaCommands, COPY.ctaUrl) + 0.35 + 0.1;
  return {end};
})();

const DUR = {old: S(OLD.end), new: S(NEW.end), close: S(CLOSE.end)};
export const DEMO_FRAMES = DUR.old + DUR.new + DUR.close;

// ------------------------------------------------------------------ helpers

const CLAMP = {extrapolateLeft: 'clamp', extrapolateRight: 'clamp'} as const;
const ease = Easing.bezier(0.33, 0, 0.2, 1);
const ramp = (f: number, start: number, dur = S(0.4)) => interpolate(f, [start, start + dur], [0, 1], {...CLAMP, easing: ease});
const sceneOpacity = (f: number, dur: number) => ramp(f, 0, S(0.35)) * (1 - ramp(f, dur - S(0.35), S(0.35)));

const Background: React.FC = () => (
  <AbsoluteFill style={{background: `radial-gradient(1200px 700px at 50% 35%, #152033 0%, ${colors.bg} 70%)`}} />
);

// Off for the captionless cut: only the terminal footage carries the story.
const CaptionsOn = React.createContext(true);

const Chip: React.FC<{label: string; color: string}> = ({label, color}) =>
  !React.useContext(CaptionsOn) ? null : (
  <div
    style={{
      position: 'absolute', zIndex: 10, top: 44, left: 64, padding: '10px 22px', borderRadius: 999,
      border: `2px solid ${color}`, color, fontFamily: sans, fontWeight: 600, fontSize: 30, background: colors.bg, boxShadow: '0 8px 24px rgba(0,0,0,0.5)',
    }}
  >
    {label}
  </div>
);

// from/to are in seconds and include the fades.
const Caption: React.FC<{from: number; to: number; children: React.ReactNode}> = ({from, to, children}) => {
  const f = useCurrentFrame();
  const o = ramp(f, S(from), S(FADE)) * (1 - ramp(f, S(to - FADE), S(FADE)));
  if (o === 0 || !React.useContext(CaptionsOn)) return null;
  return (
    <div style={{position: 'absolute', bottom: 64, left: 0, right: 0, display: 'flex', justifyContent: 'center', opacity: o, transform: `translateY(${(1 - o) * 16}px)`}}>
      <div
        style={{
          fontFamily: sans, fontSize: 40, fontWeight: 600, color: colors.text, background: 'rgba(22,27,34,0.94)',
          border: `1px solid ${colors.border}`, borderRadius: 16, padding: '20px 36px', boxShadow: '0 12px 40px rgba(0,0,0,0.5)',
        }}
      >
        {children}
      </div>
    </div>
  );
};

// ------------------------------------------------------------------ old way

const OLD_FONT = 16;
const GRID = {cols: 20, rows: 10, w: 64, h: 44, gap: 14};

const OldScene: React.FC = () => {
  const f = useCurrentFrame();
  const {fps} = useVideoConfig();
  const t = f / fps;
  const {cols, rows, scrollback} = oldCast;
  const lh = OLD_FONT * LINE_HEIGHT;
  const total = scrollback.length;
  const finalOffset = total - rows;

  let lines: Line[];
  let offset = 0;
  if (t < OLD.streamStart) {
    lines = frameAt(oldCast, Math.max(0, t - OLD.castStart)).lines;
  } else {
    const p = interpolate(t, [OLD.streamStart, OLD.streamEnd], [0, 1], {...CLAMP, easing: Easing.inOut(Easing.quad)});
    const revealed = 1 + p * (total - 1);
    lines = scrollback.slice(0, Math.ceil(revealed));
    offset = Math.max(0, revealed - rows);
  }

  const hi = scrollback.findIndex((l) => lineText(l).includes('random_password.db_master must be replaced'));
  const hiEnd = scrollback.findIndex((l) => lineText(l).includes('"rotation"'));
  const boxRows = hiEnd - hi + 1;
  const viewRow = hi - finalOffset;

  const winW = cols * OLD_FONT * 0.6 + 2 * PAD;
  const winH = TITLE_BAR + 2 * PAD + rows * lh;
  const winTop = (1080 - winH) / 2 + 20;
  const winLeft = (1920 - winW) / 2;
  const ox = winW * 0.32;
  const oy = TITLE_BAR + PAD + (viewRow + boxRows / 2) * lh;

  const zoomIn = spring({frame: f - S(OLD.zoom), fps, config: {damping: 200}});
  const shrink = ramp(f, S(OLD.shrink), S(0.7));
  const scale = (1 + 0.9 * zoomIn) * (1 - 0.9 * shrink);
  const dx = (960 - (winLeft + ox)) * zoomIn;
  const dy = (480 - (winTop + oy)) * zoomIn;

  return (
    <AbsoluteFill style={{opacity: sceneOpacity(f, DUR.old)}}>
      <Chip label={COPY.oldChip} color={colors.muted} />
      <div
        style={{
          position: 'absolute', left: winLeft, top: winTop, transformOrigin: `${ox}px ${oy}px`,
          transform: `translate(${dx}px, ${dy}px) scale(${scale})`, opacity: (1 - shrink) * ramp(f, 0, S(0.4)),
        }}
      >
        <Terminal cols={cols} rows={rows} fontSize={OLD_FONT} title="~/fixtures/workspaces/ws-06-db-credentials" lines={lines} offset={offset}>
          <div
            style={{
              position: 'absolute', left: PAD - 10, right: PAD - 10, top: PAD + viewRow * lh - 4, height: boxRows * lh + 8,
              border: `3px solid ${colors.red}`, borderRadius: 8, background: 'rgba(255,107,107,0.10)',
              boxShadow: `0 0 40px rgba(255,107,107,0.35)`, opacity: ramp(f, S(OLD.zoom + 0.2), S(0.4)),
            }}
          />
        </Terminal>
      </div>

      <TileGrid start={S(OLD.shrink + 0.2)} />

      <Caption from={OLD.caption} to={OLD.captionEnd}>
        Workspace 6 of 200 rotates the <span style={{color: colors.red}}>database password</span>
      </Caption>
    </AbsoluteFill>
  );
};

const TileGrid: React.FC<{start: number}> = ({start}) => {
  const f = useCurrentFrame();
  if (f < start) return null;
  const width = GRID.cols * (GRID.w + GRID.gap) - GRID.gap;
  const pulse = 0.5 + 0.5 * Math.sin((f - start) / 5);
  return (
    <div style={{position: 'absolute', top: 150, left: (1920 - width) / 2, display: 'grid', gridTemplateColumns: `repeat(${GRID.cols}, ${GRID.w}px)`, gap: GRID.gap}}>
      {Array.from({length: GRID.cols * GRID.rows}, (_, i) => {
        const o = ramp(f, start + i * 0.12, S(0.25));
        const target = i === 5;
        return (
          <div
            key={i}
            style={{
              width: GRID.w, height: GRID.h, borderRadius: 7, opacity: o, transform: `scale(${0.6 + 0.4 * o})`,
              background: target ? 'rgba(255,107,107,0.18)' : colors.panel,
              border: `${target ? 2 : 1}px solid ${target ? colors.red : colors.border}`,
              boxShadow: target ? `0 0 ${14 + 16 * pulse}px rgba(255,107,107,0.6)` : undefined,
              padding: '9px 8px', boxSizing: 'border-box', display: 'flex', flexDirection: 'column', gap: 5,
            }}
          >
            {[34, 44, 24].map((w, j) => (
              <div key={j} style={{height: 4, width: w, borderRadius: 2, background: target && j === 1 ? colors.red : '#2f3746'}} />
            ))}
          </div>
        );
      })}
    </div>
  );
};

// ------------------------------------------------------------------ new way

const NEW_FONT = 18;
const ROW_RE = /^\s*\d+\s+ws-/;
const BAND_RE = /\b(HIGH|MEDIUM|LOW|UNSCORED)\b/;

const NewScene: React.FC = () => {
  const f = useCurrentFrame();
  const {fps} = useVideoConfig();
  const t = f / fps;
  const {cols, rows} = newCast;
  const lh = NEW_FONT * LINE_HEIGHT;
  const lines = frameAt(newCast, Math.max(0, t - NEW.castStart)).lines;
  const texts = lines.map(lineText);
  const dimLow = ramp(f, S(NEW.tableShown + 0.3), S(0.5));

  const pwRow = texts.findIndex((l) => ROW_RE.test(l) && l.includes('ws-06-db-credentials'));
  const pwBand = pwRow >= 0 ? texts[pwRow].match(BAND_RE)?.[1] : undefined;
  const showPw = pwBand !== undefined && pwBand !== 'LOW';

  const winW = cols * NEW_FONT * 0.6 + 2 * PAD;
  const winH = TITLE_BAR + 2 * PAD + rows * lh;

  return (
    <AbsoluteFill style={{opacity: sceneOpacity(f, DUR.new)}}>
      <Chip label={COPY.newChip} color={colors.blue} />
      <div style={{position: 'absolute', left: (1920 - winW) / 2, top: (1080 - winH) / 2 - 50, opacity: ramp(f, 0, S(0.4))}}>
        <Terminal
          cols={cols}
          rows={rows}
          fontSize={NEW_FONT}
          title="~/fixtures/workspaces"
          lines={lines}
          lineStyle={(text) => {
            const band = text.match(BAND_RE)?.[1];
            if (!ROW_RE.test(text) || !band) return {};
            if (band === 'LOW') return {opacity: 1 - 0.62 * dimLow};
            return {background: `${bandColor[band]}${Math.round(dimLow * 36).toString(16).padStart(2, '0')}`};
          }}
          renderLine={(text, line) => {
            if (text.startsWith('ID ') || text.startsWith('Run ')) return <span style={{color: colors.muted, fontWeight: text.startsWith('ID') ? 700 : 400}}>{text}</span>;
            const m = ROW_RE.test(text) ? text.match(BAND_RE) : null;
            if (!m || m.index === undefined) {
              return line.map((s, j) => <span key={j} style={{color: s.fg === 2 ? colors.green : colors.text}}>{s.text}</span>);
            }
            return (
              <>
                <span style={{color: colors.text}}>{text.slice(0, m.index)}</span>
                <span style={{background: bandColor[m[1]], color: '#0b0f17', fontWeight: 700, borderRadius: 4}}>{m[1]}</span>
                <span style={{color: colors.text}}>{text.slice(m.index + m[1].length)}</span>
              </>
            );
          }}
        >
          {showPw && (
            <div
              style={{
                position: 'absolute', left: PAD - 10, right: PAD - 10, top: PAD + pwRow * lh - 3, height: lh + 6,
                border: `3px solid ${colors.red}`, borderRadius: 8, boxShadow: '0 0 36px rgba(255,107,107,0.45)', opacity: ramp(f, S(NEW.caption), S(0.4)),
              }}
            />
          )}
        </Terminal>
      </div>

      <Caption from={NEW.caption} to={NEW.captionEnd}>
        {showPw ? COPY.newCaption : COPY.newFallback}
      </Caption>
    </AbsoluteFill>
  );
};

// ------------------------------------------------------------------ close

const CloseScene: React.FC = () => {
  const f = useCurrentFrame();
  return (
    <AbsoluteFill style={{opacity: sceneOpacity(f, DUR.close), alignItems: 'center', justifyContent: 'center'}}>
      {React.useContext(CaptionsOn) && (
        <div style={{fontFamily: sans, fontWeight: 800, fontSize: 64, color: colors.text, opacity: ramp(f, S(0.1))}}>
          {COPY.ctaTitle}
        </div>
      )}
      <div
        style={{
          marginTop: 56, padding: '30px 40px', borderRadius: 16, background: colors.panel, border: `1px solid ${colors.border}`,
          fontFamily: mono, fontSize: 30, lineHeight: 1.8, color: colors.text, opacity: ramp(f, S(0.6)),
          transform: `translateY(${(1 - ramp(f, S(0.6))) * 14}px)`,
        }}
      >
        {COPY.ctaCommands.map((c) => (
          <div key={c}>
            <span style={{color: colors.green}}>$ </span>
            {c}
          </div>
        ))}
      </div>
      <div style={{marginTop: 44, fontFamily: sans, fontSize: 38, fontWeight: 600, color: colors.blue, opacity: ramp(f, S(1.2))}}>
        {COPY.ctaUrl}
      </div>
    </AbsoluteFill>
  );
};

export const Demo: React.FC<{captions: boolean}> = ({captions}) => (
  <CaptionsOn.Provider value={captions}>
  <AbsoluteFill>
    <Background />
    <Sequence from={0} durationInFrames={DUR.old}>
      <OldScene />
    </Sequence>
    <Sequence from={DUR.old} durationInFrames={DUR.new}>
      <NewScene />
    </Sequence>
    <Sequence from={DUR.old + DUR.new} durationInFrames={DUR.close}>
      <CloseScene />
    </Sequence>
  </AbsoluteFill>
  </CaptionsOn.Provider>
);
