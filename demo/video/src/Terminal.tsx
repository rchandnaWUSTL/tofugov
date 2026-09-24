import React from 'react';
import {colors, mono, palette} from './theme';

export type Span = {text: string; fg: number | string | null; bold: number};
export type Line = Span[];
export type Cast = {cols: number; rows: number; duration: number; frames: {t: number; lines: Line[]}[]; scrollback: Line[]};

export const LINE_HEIGHT = 1.45;
export const TITLE_BAR = 44;
export const PAD = 22;

export const lineText = (l: Line | undefined) => (l ?? []).map((s) => s.text).join('');

export const frameAt = (cast: Cast, t: number) => {
  let cur = cast.frames[0];
  for (const f of cast.frames) {
    if (f.t > t) break;
    cur = f;
  }
  return cur;
};

const spanColor = (fg: Span['fg']) => (fg === null ? colors.text : typeof fg === 'number' ? palette[fg] ?? colors.text : fg);

export const Terminal: React.FC<{
  cols: number;
  rows: number;
  fontSize: number;
  title: string;
  lines: Line[];
  // Scroll position in lines; fractional values scroll smoothly.
  offset?: number;
  renderLine?: (text: string, line: Line, index: number) => React.ReactNode;
  lineStyle?: (text: string, index: number) => React.CSSProperties;
  children?: React.ReactNode;
}> = ({cols, rows, fontSize, title, lines, offset = 0, renderLine, lineStyle, children}) => {
  const lh = fontSize * LINE_HEIGHT;
  return (
    <div
      style={{
        background: colors.panel,
        border: `1px solid ${colors.border}`,
        borderRadius: 14,
        boxShadow: '0 30px 80px rgba(0,0,0,0.55)',
        overflow: 'hidden',
        fontFamily: mono,
        fontSize,
      }}
    >
      <div style={{height: TITLE_BAR, display: 'flex', alignItems: 'center', padding: '0 18px', borderBottom: `1px solid ${colors.border}`, background: '#161b22'}}>
        {['#ff5f57', '#febc2e', '#28c840'].map((c) => (
          <div key={c} style={{width: 13, height: 13, borderRadius: 7, background: c, marginRight: 8}} />
        ))}
        <div style={{flex: 1, textAlign: 'center', color: colors.muted, fontSize: 15, marginRight: 60}}>{title}</div>
      </div>
      <div style={{position: 'relative', padding: PAD, width: `${cols}ch`, height: rows * lh, boxSizing: 'content-box', overflow: 'hidden'}}>
        <div style={{transform: `translateY(${-offset * lh}px)`}}>
          {lines.map((line, i) => {
            const text = lineText(line);
            return (
              <div key={i} style={{height: lh, lineHeight: `${lh}px`, whiteSpace: 'pre', ...lineStyle?.(text, i)}}>
                {renderLine
                  ? renderLine(text, line, i)
                  : line.map((s, j) => (
                      <span key={j} style={{color: spanColor(s.fg), fontWeight: s.bold ? 700 : 400}}>
                        {s.text}
                      </span>
                    ))}
              </div>
            );
          })}
        </div>
        {children}
      </div>
    </div>
  );
};
