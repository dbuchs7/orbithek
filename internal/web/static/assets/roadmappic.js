// The picture of a roadmap or a Gantt chart (ADR-draft-roadmap-one-listing-and-pictures): the chart as a PNG or
// JPEG file, drawn on a canvas in this browser from what the chart shows.
// Nothing new is asked of the server: the picture holds exactly the items
// the reader sees, as the chart in the page does (D10).
//
// The picture is drawn from the chart's model, not copied from the page:
// it holds the whole frame and every row, also those scrolled out of
// sight, looks the same in every browser, and needs no library. The
// decisions — where each head, tick and bar stands in pixels, how far a
// picture may be scaled before a browser refuses it, what the file is
// called — are plain functions tested in Node (jstest/roadmappic.test.mjs);
// the drawing needs a browser and is in e2e/smoke.cjs.
import { FORMATS, JPEG_QUALITY, exportName, save } from "./picexport.js";

// pictureFormats are the formats the chart exports in, in the order of
// the menu.
export const pictureFormats = ["png", "jpeg"];

// SIZES are the measures of the picture in CSS pixels, as the chart in
// the page has them (style.css: .tl): the head column, a row, the axis,
// the title above it and the margin around everything.
export const SIZES = { head: 224, row: 30, axis: 28, title: 36, margin: 16, minBar: 6, diamond: 12 };

// LIMITS bound the canvas a picture is drawn on. Browsers refuse a canvas
// beyond a side or an area of their own — the smallest common ones are
// about 16384 pixels a side and 125 million pixels in all — and give no
// picture then. A picture is drawn twice as sharp as the page where it
// fits, smaller where it does not, but never so small that its text
// cannot be read.
export const LIMITS = { side: 16384, area: 120e6, sharp: 2, least: 0.5 };

// pictureLayout places everything of the picture in CSS pixels: spec is
// the chart's model in words (roadmap.js), with its frame, the captions of
// its ticks, its rows with their heads and bars, a title and a date. Each
// unit of the axis is spec.unit pixels wide, as the chart's least width.
export function pictureLayout(spec, sizes = SIZES) {
  const { head, row, axis, title, margin, minBar } = sizes;
  const fr = spec.frame;
  const track = fr.ticks.length * spec.unit;
  const left = margin + head;
  const xOf = (day) => left + ((day - fr.from) / fr.days) * track;
  const axisY = margin + title;
  const bodyY = axisY + axis;
  const width = left + track + margin;
  const height = bodyY + spec.rows.length * row + margin;
  const ticks = fr.ticks.map((tick, i) => ({ x: xOf(tick.day), w: xOf(tick.end) - xOf(tick.day), text: spec.ticks[i] ?? "" }));
  const rows = [];
  const bars = [];
  spec.rows.forEach((r, i) => {
    const y = bodyY + i * row;
    // A Gantt chart parts every row from the one above, a roadmap every
    // lane: as the chart's borders do.
    rows.push({ y, h: row, head: r.head || null, rule: i > 0 && (spec.gantt || r.first) });
    for (const b of r.bars) {
      const base = { y: y + 4, h: row - 8, text: b.text || "", done: Boolean(b.done), waiting: Boolean(b.waiting), inverted: Boolean(b.inverted) };
      if (b.milestone) bars.push({ ...base, milestone: true, x: (xOf(b.from) + xOf(b.to + 1)) / 2, w: 0 });
      else bars.push({ ...base, milestone: false, x: xOf(b.from), w: Math.max(minBar, xOf(b.to + 1) - xOf(b.from)) });
    }
  });
  return {
    width, height, margin, head, left, track, axisY, axisH: axis, bodyY, bodyH: spec.rows.length * row,
    title: spec.title || "", date: spec.date || "", ticks, rows, bars,
    today: fr.today === null || fr.today === undefined ? null : (xOf(fr.today) + xOf(fr.today + 1)) / 2,
  };
}

// pictureScale is how many canvas pixels a CSS pixel of a picture of
// width × height takes: LIMITS.sharp where the canvas fits, less where it
// would exceed a side or the area, and 0 when even LIMITS.least would.
export function pictureScale(width, height, limits = LIMITS) {
  const scale = Math.min(limits.sharp, limits.side / width, limits.side / height, Math.sqrt(limits.area / (width * height)));
  return scale >= limits.least ? scale : 0;
}

// pictureName is the file a picture is saved as: what the view shows,
// such as "Gantt chart PLAT release 2026-11-12", with the format's
// extension; characters file systems refuse become a dash, as for the
// pictures of the album.
export function pictureName(base, format) {
  const f = FORMATS[format];
  if (!f || !pictureFormats.includes(format)) throw new Error(`unknown format ${format}`);
  return exportName(String(base ?? "") + f.ext, format);
}

// The rest needs a browser.

// TOKENS are the look's colours the picture is drawn in (style.css), so
// it looks as the chart does, in the light look or the dark.
const TOKENS = ["surface", "surface-2", "border", "border-strong", "text", "muted", "accent", "accent-soft", "success", "success-soft", "warning", "danger"];

// colours reads the look's colours as the browser computes them; a probe
// resolves each token, whatever colour function the look writes it in.
function colours(h) {
  const probe = h("span", { hidden: true });
  document.body.append(probe);
  const out = {};
  for (const name of TOKENS) {
    probe.style.color = `var(--${name})`;
    out[name] = getComputedStyle(probe).color;
  }
  probe.remove();
  return out;
}

// fit shortens a text to a width with an ellipsis, as the chart does.
function fit(ctx, text, width) {
  if (width <= 0 || !text) return "";
  if (ctx.measureText(text).width <= width) return text;
  let lo = 0, hi = text.length;
  while (lo < hi) {
    const mid = Math.ceil((lo + hi) / 2);
    if (ctx.measureText(text.slice(0, mid) + "…").width <= width) lo = mid;
    else hi = mid - 1;
  }
  return lo ? text.slice(0, lo) + "…" : "";
}

function roundRect(ctx, x, y, w, h, r) {
  const rr = Math.min(r, w / 2, h / 2);
  ctx.beginPath();
  ctx.moveTo(x + rr, y);
  ctx.arcTo(x + w, y, x + w, y + h, rr);
  ctx.arcTo(x + w, y + h, x, y + h, rr);
  ctx.arcTo(x, y + h, x, y, rr);
  ctx.arcTo(x, y, x + w, y, rr);
  ctx.closePath();
}

function line(ctx, x1, y1, x2, y2, colour, width = 1) {
  ctx.strokeStyle = colour;
  ctx.lineWidth = width;
  ctx.beginPath();
  ctx.moveTo(x1, y1);
  ctx.lineTo(x2, y2);
  ctx.stroke();
}

// hatch is the pattern of a finished bar: stripes of the success colour on
// the surface, as style.css draws .tl-done.
function hatch(ctx, c, h) {
  const tile = h("canvas", { width: 10, height: 10 });
  const t = tile.getContext("2d");
  t.fillStyle = c.surface;
  t.fillRect(0, 0, 10, 10);
  t.strokeStyle = c["success-soft"];
  t.lineWidth = 4;
  t.beginPath();
  for (const o of [-10, 0, 10]) {
    t.moveTo(o, 10);
    t.lineTo(o + 10, 0);
  }
  t.stroke();
  return ctx.createPattern(tile, "repeat");
}

// paint draws a layout on a canvas at a scale.
function paint(L, c, font, scale, h) {
  const canvas = h("canvas", { width: Math.ceil(L.width * scale), height: Math.ceil(L.height * scale) });
  const ctx = canvas.getContext("2d");
  if (!ctx) return null;
  ctx.scale(scale, scale);
  ctx.textBaseline = "middle";
  ctx.fillStyle = c.surface;
  ctx.fillRect(0, 0, L.width, L.height);
  const right = L.left + L.track;
  const bottom = L.bodyY + L.bodyH;

  // The title and the day it shows.
  ctx.fillStyle = c.text;
  ctx.font = `600 15px ${font}`;
  ctx.textAlign = "right";
  ctx.fillText(L.date, right, L.margin + 12);
  const dateW = ctx.measureText(L.date).width;
  ctx.textAlign = "left";
  ctx.fillText(fit(ctx, L.title, L.track + L.head - dateW - 24), L.margin, L.margin + 12);

  // The axis with its units.
  ctx.fillStyle = c["surface-2"];
  ctx.fillRect(L.margin, L.axisY, right - L.margin, L.axisH);
  ctx.font = `12px ${font}`;
  for (const tick of L.ticks) {
    line(ctx, tick.x + 0.5, L.axisY, tick.x + 0.5, L.axisY + L.axisH, c.border);
    ctx.fillStyle = c.muted;
    ctx.fillText(fit(ctx, tick.text, tick.w - 12), tick.x + 6, L.axisY + L.axisH / 2);
  }
  line(ctx, L.margin, L.axisY + L.axisH - 0.5, right, L.axisY + L.axisH - 0.5, c["border-strong"]);

  // The lines of the units and the rules between rows or lanes.
  for (const tick of L.ticks) line(ctx, tick.x + 0.5, L.bodyY, tick.x + 0.5, bottom, c.border);
  for (const r of L.rows) if (r.rule) line(ctx, L.margin, r.y + 0.5, right, r.y + 0.5, c.border);

  // The heads: a label as a chip, the lane without one in italics, an
  // item of a Gantt chart by its key and title.
  ctx.save();
  ctx.beginPath();
  ctx.rect(L.margin, L.bodyY, L.head - 12, L.bodyH);
  ctx.clip();
  for (const r of L.rows) {
    if (!r.head) continue;
    const x = L.margin + 12, y = r.y + r.h / 2;
    if (r.head.label !== undefined) {
      ctx.font = `12px ${font}`;
      const text = fit(ctx, r.head.label, L.head - 48);
      const w = ctx.measureText(text).width + 16;
      ctx.fillStyle = c["accent-soft"];
      roundRect(ctx, x, y - 11, w, 22, 11);
      ctx.fill();
      ctx.fillStyle = c.text;
      ctx.fillText(text, x + 8, y);
    } else if (r.head.none !== undefined) {
      ctx.font = `italic 14px ${font}`;
      ctx.fillStyle = c.text;
      ctx.fillText(fit(ctx, r.head.none, L.head - 36), x, y);
    } else {
      ctx.font = `14px ${font}`;
      ctx.fillStyle = c.accent;
      const key = r.head.key || "";
      ctx.fillText(key, x, y);
      const kw = ctx.measureText(key + " ").width;
      ctx.fillStyle = c.muted;
      ctx.fillText(fit(ctx, r.head.text || "", L.head - 36 - kw), x + kw, y);
    }
  }
  ctx.restore();
  line(ctx, L.left - 0.5, L.axisY, L.left - 0.5, bottom, c.border);

  // The bars and milestones, kept to the tracks.
  ctx.save();
  ctx.beginPath();
  ctx.rect(L.left, L.bodyY, L.track, L.bodyH);
  ctx.clip();
  const done = hatch(ctx, c, h);
  ctx.font = `12px ${font}`;
  for (const b of L.bars) {
    const dashed = b.waiting || b.inverted;
    ctx.setLineDash(dashed ? [4, 3] : []);
    if (b.milestone) {
      const s = SIZES.diamond, cy = b.y + b.h / 2;
      ctx.save();
      ctx.translate(b.x, cy);
      ctx.rotate(Math.PI / 4);
      ctx.fillStyle = dashed ? c.surface : b.done ? c["success-soft"] : c.accent;
      ctx.strokeStyle = dashed ? c.warning : b.done ? c.success : c.accent;
      ctx.lineWidth = 1;
      roundRect(ctx, -s / 2, -s / 2, s, s, 2);
      ctx.fill();
      ctx.stroke();
      ctx.restore();
      if (b.text) {
        ctx.fillStyle = b.done ? c.muted : c.text;
        ctx.fillText(b.text, b.x + 11, cy);
      }
      continue;
    }
    roundRect(ctx, b.x + 0.5, b.y + 0.5, b.w - 1, b.h - 1, 6);
    ctx.fillStyle = b.done ? done : c["accent-soft"];
    ctx.fill();
    ctx.strokeStyle = b.waiting ? c.warning : b.done ? c.success : c.accent;
    ctx.lineWidth = 1;
    ctx.stroke();
    const text = fit(ctx, b.text, b.w - 12);
    if (text) {
      ctx.fillStyle = b.done ? c.muted : c.text;
      ctx.fillText(text, b.x + 6, b.y + b.h / 2);
    }
  }
  ctx.setLineDash([]);
  ctx.restore();

  // Today, over everything in the tracks.
  if (L.today !== null) line(ctx, L.today, L.bodyY, L.today, bottom, c.danger, 2);
  return canvas;
}

const toBlob = (canvas, type) => new Promise((resolve) => {
  try {
    canvas.toBlob(resolve, type, type === FORMATS.jpeg.type ? JPEG_QUALITY : undefined);
  } catch {
    resolve(null);
  }
});

// exportChart draws the chart of spec as a picture of format and saves it
// under a name made of base; it answers the file's name. A browser that
// refuses a canvas of the size asked gets one half as large, as long as
// its text stays readable; beyond that the error is "too-large", and the
// view is narrowed by its labels, type or room, or drawn by a coarser
// scale. h is dom.js's, passed in so this module loads in Node.
export async function exportChart(spec, format, base, h) {
  const name = pictureName(base, format);
  const L = pictureLayout(spec);
  const c = colours(h);
  const font = getComputedStyle(document.body).fontFamily || "sans-serif";
  for (let scale = pictureScale(L.width, L.height); scale >= LIMITS.least; scale /= 2) {
    const canvas = paint(L, c, font, scale, h);
    const blob = canvas && (await toBlob(canvas, FORMATS[format].type));
    if (blob) {
      save(blob, name, h);
      return name;
    }
  }
  throw new Error("too-large");
}
