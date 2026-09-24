// Replays asciicast v3 recordings through a headless xterm and writes one
// screen snapshot per output event, so Remotion can render any frame exactly.
import {readFileSync, writeFileSync, mkdirSync} from 'node:fs';
import xterm from '@xterm/headless';

const {Terminal} = xterm;
const casts = ['old', 'new'];

const write = (term, data) => new Promise((resolve) => term.write(data, resolve));

function snapshot(term, from = term.buffer.active.viewportY, count = term.rows) {
  const buf = term.buffer.active;
  const cell = buf.getNullCell();
  const lines = [];
  for (let y = 0; y < count; y++) {
    const line = buf.getLine(from + y);
    const spans = [];
    for (let x = 0; line && x < term.cols; x++) {
      line.getCell(x, cell);
      if (cell.getWidth() === 0) continue;
      const text = cell.getChars() || ' ';
      let fg = null;
      if (cell.isFgPalette()) fg = cell.getFgColor();
      else if (cell.isFgRGB()) fg = '#' + cell.getFgColor().toString(16).padStart(6, '0');
      const bold = cell.isBold() ? 1 : 0;
      const last = spans[spans.length - 1];
      if (last && last.fg === fg && last.bold === bold) last.text += text;
      else spans.push({text, fg, bold});
    }
    const lastSpan = spans[spans.length - 1];
    if (lastSpan) lastSpan.text = lastSpan.text.replace(/\s+$/, '');
    lines.push(spans.filter((s) => s.text.length > 0));
  }
  return lines;
}

mkdirSync('src/casts', {recursive: true});
for (const name of casts) {
  const [header, ...events] = readFileSync(`public/${name}.cast`, 'utf8').trim().split('\n');
  const {term: size} = JSON.parse(header);
  const term = new Terminal({cols: size.cols, rows: size.rows, scrollback: 2000, allowProposedApi: true});

  const frames = [];
  let t = 0;
  for (const line of events) {
    const [interval, type, data] = JSON.parse(line);
    t += interval;
    if (type !== 'o') continue;
    await write(term, data.replace(/\n/g, '\r\n').replace(/\r\r\n/g, '\r\n'));
    frames.push({t: +t.toFixed(3), lines: snapshot(term)});
  }
  const buf = term.buffer.active;
  const scrollback = snapshot(term, 0, buf.viewportY + term.rows);
  writeFileSync(`src/casts/${name}.json`, JSON.stringify({cols: size.cols, rows: size.rows, duration: t, frames, scrollback}));
  console.log(`${name}: ${frames.length} snapshots, ${t.toFixed(1)}s`);
}
