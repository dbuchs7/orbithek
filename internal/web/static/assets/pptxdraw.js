// pptxdraw draws the scenes of assets/pptxscene.js onto a canvas and makes
// a picture of each slide (ADR-0186): backgrounds, shapes with their fills
// and lines, pictures, tables and text, wrapped, aligned, numbered and
// shrunk as the presentation asks. It is an approximation of what the
// presentation program draws; fonts the browser lacks are replaced by
// metric-compatible ones where it has them. The text layout is a plain
// function with the measuring passed in, so node --test runs it; drawing
// needs a canvas.
import { css, geometry, mul, slideScene } from "./pptxscene.js";

// SLIDE_EDGE is the length in pixels of a slide picture's longer side.
export const SLIDE_EDGE = 1920;
const EMU_PT = 12700;
// LINE is the height of a single-spaced line, as a multiple of its font
// size, as presentation programs set it.
const LINE = 1.2;

const SERIF = /times|georgia|cambria|garamond|palatino|book antiqua|serif/i;
const MONO = /courier|consolas|mono|lucida console/i;
// family names a font with the fonts that stand in for it.
export function family(font) {
  const f = String(font || "Calibri").replace(/["\\]/g, "");
  const extra = /^calibri/i.test(f) ? ', "Carlito"' : /^cambria/i.test(f) ? ', "Caladea"' : "";
  const fallback = MONO.test(f) ? '"Liberation Mono", "Courier New", monospace'
    : SERIF.test(f) ? '"Liberation Serif", "Times New Roman", serif' : '"Liberation Sans", Arial, Helvetica, sans-serif';
  return `"${f}"${extra}, ${fallback}`;
}
// fontCSS is a run's canvas font at px pixels.
export const fontCSS = (run, px) => `${run.italic ? "italic " : ""}${run.bold ? "bold " : ""}${Math.max(1, px).toFixed(2)}px ${family(run.font)}`;

// layoutText lays out a text body in a box of w × h pixels, k pixels per
// EMU, with measure(font, text) giving a text's width in pixels. scale
// shrinks the fonts and reduce the line spacing, as autofit does. It gives
// the lines, each with its baseline, its pieces and its bullet, and the
// height the text takes.
export function layoutText(text, w, h, k, measure, scale = 1, reduce = 0) {
  const b = text.body;
  const left = b.lIns * k, right = b.rIns * k, top = b.tIns * k, bottom = b.bIns * k;
  const width = b.wrap ? Math.max(1, w - left - right) : Infinity;
  const lines = [];
  let y = 0;
  let firstPara = true;
  for (const p of text.paragraphs) {
    const pxOf = (pt) => pt * EMU_PT * k * scale;
    const endPx = pxOf(p.endSize || p.runs[0]?.size || 18);
    const spacing = (s, px) => (s.pts !== undefined ? s.pts * EMU_PT * k * scale : s.pct * px * LINE);
    const marL = p.marL * k, indent = p.indent * k;
    const pieces = [];
    for (const r of p.runs) {
      if (r.br) { pieces.push({ br: true, run: r }); continue; }
      for (const word of r.text.match(/\s+|[^\s]+/g) || []) pieces.push({ text: word, run: r, space: /^\s+$/.test(word) });
    }
    const paraLines = [];
    let line = { items: [], width: 0 };
    let x0 = marL + (p.bullet ? 0 : indent);
    const avail = () => width - (paraLines.length === 0 ? x0 : marL);
    const push = () => {
      while (line.items.length && line.items[line.items.length - 1].space) line.width -= line.items.pop().w;
      paraLines.push(line);
      line = { items: [], width: 0 };
    };
    for (const piece of pieces) {
      if (piece.br) {
        line.brPx = pxOf(piece.run.size);
        push();
        continue;
      }
      const px = pxOf(piece.run.size) * (piece.run.baseline ? 0.65 : 1);
      const font = fontCSS(piece.run, px);
      let wd = measure(font, piece.text);
      if (piece.space && !line.items.length && paraLines.length) continue;
      if (!piece.space && line.width + wd > avail() && line.items.length) push();
      if (!piece.space && wd > avail()) {
        // A word longer than the line breaks between its letters.
        let part = "";
        for (const ch of piece.text) {
          const cw = measure(font, part + ch);
          if (line.width + cw > avail() && (part || line.items.length)) {
            if (part) line.items.push({ text: part, run: piece.run, px, font, w: measure(font, part) });
            line.width += part ? measure(font, part) : 0;
            push();
            part = ch;
          } else part += ch;
        }
        piece.text = part;
        wd = measure(font, part);
      }
      line.items.push({ text: piece.text, run: piece.run, px, font, w: wd, space: piece.space });
      line.width += wd;
    }
    if (line.items.length || !paraLines.length) push();
    // Spacing before the first paragraph of a text does not count.
    const firstPx = Math.max(endPx, ...paraLines[0].items.map((i) => i.px / (i.run.baseline ? 0.65 : 1)));
    if (!firstPara) y += spacing(p.spcBef, firstPx);
    firstPara = false;
    paraLines.forEach((ln, i) => {
      const maxPx = Math.max(...ln.items.map((it) => it.px / (it.run.baseline ? 0.65 : 1)), ln.items.length ? 0 : ln.brPx || endPx);
      const lh = p.lnSpc.pts !== undefined ? p.lnSpc.pts * EMU_PT * k : maxPx * LINE * p.lnSpc.pct * (1 - reduce);
      const start = i === 0 ? x0 : marL;
      const room = (Number.isFinite(width) ? width : ln.width + start) - start;
      let x = start;
      if (p.align === "ctr") x = start + (room - ln.width) / 2;
      else if (p.align === "r") x = start + room - ln.width;
      const items = [];
      for (const it of ln.items) {
        items.push({ text: it.text, x, px: it.px, font: it.font, run: it.run, w: it.w });
        x += it.w;
      }
      let bullet = null;
      if (i === 0 && p.bullet) {
        const bpx = p.bullet.size * EMU_PT * k * scale;
        bullet = { text: p.bullet.text, x: marL + indent, px: bpx, font: fontCSS({ font: p.bullet.font }, bpx), color: p.bullet.color };
      }
      // The descent sits at the bottom of the line; extra spacing goes above.
      lines.push({ baseline: y + lh - maxPx * 0.25, items, bullet, height: lh });
      y += lh;
    });
    y += spacing(p.spcAft, firstPx);
  }
  const height = y;
  const room = h - top - bottom;
  const offset = top + (b.anchor === "ctr" ? (room - height) / 2 : b.anchor === "b" ? room - height : 0);
  for (const ln of lines) ln.baseline += offset;
  return { lines, height, overflow: height > room + 0.5, left, width: Number.isFinite(width) ? width : w - left - right };
}

// fitText lays out a text body as its autofit asks: with the scale the
// presentation stored, or, when it asks to shrink text that does not fit
// and stored none, shrunk step by step until it fits.
export function fitText(text, w, h, k, measure) {
  const af = text.body.autofit;
  if (af?.kind === "norm" && af.fontScale) return layoutText(text, w, h, k, measure, af.fontScale, af.lnSpcReduction || 0);
  let out = layoutText(text, w, h, k, measure);
  if (af?.kind !== "norm") return out;
  for (let s = 0.95, r = 0.05; out.overflow && s >= 0.25; s -= 0.05, r = Math.min(0.2, r + 0.05)) out = layoutText(text, w, h, k, measure, s, r);
  return out;
}

// ---- drawing ----

const toCanvas = (c, m) => c.setTransform(m[0], m[1], m[2], m[3], m[4], m[5]);

function trace(c, cmds) {
  c.beginPath();
  for (const cmd of cmds) {
    switch (cmd.op) {
      case "M": c.moveTo(cmd.x, cmd.y); break;
      case "L": c.lineTo(cmd.x, cmd.y); break;
      case "C": c.bezierCurveTo(cmd.x1, cmd.y1, cmd.x2, cmd.y2, cmd.x, cmd.y); break;
      case "Q": c.quadraticCurveTo(cmd.x1, cmd.y1, cmd.x, cmd.y); break;
      case "E": c.ellipse(cmd.cx, cmd.cy, Math.max(0, cmd.rx), Math.max(0, cmd.ry), 0, cmd.a0, cmd.a1, Boolean(cmd.ccw)); break;
      case "Z": c.closePath(); break;
      default:
    }
  }
}

// paint sets a fill style for a box of w × h; false when there is nothing
// to fill.
function paint(c, fill, w, h, images) {
  if (!fill || fill.type === "none") return false;
  if (fill.type === "solid") { c.fillStyle = css(fill.color); return fill.color.a > 0; }
  if (fill.type === "grad" && fill.stops.length) {
    let g;
    if (fill.radial) g = c.createRadialGradient(w / 2, h / 2, 0, w / 2, h / 2, Math.hypot(w, h) / 2);
    else {
      const a = (fill.angle * Math.PI) / 180;
      const half = Math.abs((w / 2) * Math.cos(a)) + Math.abs((h / 2) * Math.sin(a));
      g = c.createLinearGradient(w / 2 - Math.cos(a) * half, h / 2 - Math.sin(a) * half, w / 2 + Math.cos(a) * half, h / 2 + Math.sin(a) * half);
    }
    for (const s of fill.stops) g.addColorStop(Math.min(1, Math.max(0, s.pos)), css(s.color));
    c.fillStyle = g;
    return true;
  }
  if (fill.type === "blip" && images.get(fill.media)) return "blip";
  return false;
}

function drawImage(c, img, crop, w, h, alpha = 1) {
  const iw = img.width, ih = img.height;
  const sx = crop.l * iw, sy = crop.t * ih;
  const sw = Math.max(1, iw * (1 - crop.l - crop.r)), sh = Math.max(1, ih * (1 - crop.t - crop.b));
  c.save();
  c.globalAlpha *= alpha;
  c.drawImage(img, sx, sy, sw, sh, 0, 0, w, h);
  c.restore();
}

function strokeLine(c, line, scale) {
  if (!line || !line.color || line.color.a === 0) return false;
  const width = line.width || 9525;
  c.strokeStyle = css(line.color);
  c.lineWidth = Math.max(width, 0.75 / scale);
  c.setLineDash(line.dash ? line.dash.map((d) => d * width) : []);
  c.lineJoin = "round";
  return true;
}

// arrow draws an arrow head at the end of a line from (x0, y0) to (x, y).
function arrow(c, x0, y0, x, y, width) {
  const a = Math.atan2(y - y0, x - x0), len = width * 4.5, half = width * 2.5;
  c.beginPath();
  c.moveTo(x, y);
  c.lineTo(x - len * Math.cos(a) + half * Math.sin(a), y - len * Math.sin(a) - half * Math.cos(a));
  c.lineTo(x - len * Math.cos(a) - half * Math.sin(a), y - len * Math.sin(a) + half * Math.cos(a));
  c.closePath();
  c.fillStyle = c.strokeStyle;
  c.fill();
}

function drawShape(c, item, base, images) {
  const k = base[0];
  toCanvas(c, mul(base, item.m));
  const g = geometry(item.geom, item.w, item.h);
  for (const path of g.paths) {
    trace(c, path.cmds);
    if (path.fill) {
      const p = paint(c, item.fill, item.w, item.h, images);
      if (p === "blip") {
        c.save();
        c.clip(path.evenodd ? "evenodd" : "nonzero");
        drawImage(c, images.get(item.fill.media), item.fill.crop, item.w, item.h, item.fill.alpha);
        c.restore();
      } else if (p) c.fill(path.evenodd ? "evenodd" : "nonzero");
    }
    if (path.stroke && strokeLine(c, item.line, k)) {
      trace(c, path.cmds);
      c.stroke();
      const pts = path.cmds.filter((x) => x.op === "M" || x.op === "L");
      if (!path.fill && pts.length >= 2) {
        const width = Math.max(item.line.width || 9525, 0.75 / k);
        if (item.line.tailEnd) arrow(c, pts.at(-2).x, pts.at(-2).y, pts.at(-1).x, pts.at(-1).y, width);
        if (item.line.headEnd) arrow(c, pts[1].x, pts[1].y, pts[0].x, pts[0].y, width);
      }
    }
  }
  if (item.text) {
    const box = item.textBox || { x: 0, y: 0, w: item.w, h: item.h };
    drawText(c, item.text, mul(base, item.tm), box);
  }
}

// drawText draws a text body into a box of its shape, laid out in pixels
// along the shape's axes.
function drawText(c, text, m, box) {
  const s = Math.sqrt(Math.abs(m[0] * m[3] - m[1] * m[2])) || 1;
  let mm = mul(m, [1, 0, 0, 1, box.x, box.y]);
  let w = box.w * s, h = box.h * s;
  if (text.body.vert) {
    mm = mul(mm, [0, 1, -1, 0, box.w, 0]);
    [w, h] = [h, w];
  }
  toCanvas(c, mul(mm, [1 / s, 0, 0, 1 / s, 0, 0]));
  const measure = (font, str) => { c.font = font; return c.measureText(str).width; };
  const out = fitText(text, w, h, s, measure);
  c.textBaseline = "alphabetic";
  for (const ln of out.lines) {
    if (ln.bullet) {
      c.font = ln.bullet.font;
      c.fillStyle = css(ln.bullet.color);
      c.fillText(ln.bullet.text, out.left + ln.bullet.x, ln.baseline);
    }
    for (const it of ln.items) {
      if (it.run.color?.a === 0) continue;
      c.font = it.font;
      c.fillStyle = css(it.run.color);
      const y = ln.baseline - (it.run.baseline || 0) * it.px * 1.5;
      c.fillText(it.text, out.left + it.x, y);
      if (it.run.underline || it.run.strike) {
        c.fillRect(out.left + it.x, it.run.underline ? y + it.px * 0.12 : y - it.px * 0.3, it.w, Math.max(1, it.px * 0.06));
      }
    }
  }
}

function drawTable(c, item, base) {
  const m = mul(base, item.m);
  const xs = [0];
  for (const w of item.cols) xs.push(xs.at(-1) + w);
  const ys = [0];
  for (const r of item.rows) ys.push(ys.at(-1) + r.h);
  const k = base[0];
  item.rows.forEach((row, ri) => {
    row.cells.forEach((cell, ci) => {
      if (cell.merged) return;
      const x = xs[ci], y = ys[ri];
      const w = xs[Math.min(xs.length - 1, ci + cell.span)] - x, h = ys[Math.min(ys.length - 1, ri + cell.rowSpan)] - y;
      toCanvas(c, mul(m, [1, 0, 0, 1, x, y]));
      c.beginPath();
      c.rect(0, 0, w, h);
      if (paint(c, cell.fill, w, h, new Map()) === true) c.fill();
      if (cell.text) {
        const text = { ...cell.text, body: { ...cell.text.body, lIns: cell.margins.l, rIns: cell.margins.r, tIns: cell.margins.t, bIns: cell.margins.b, anchor: cell.anchor } };
        drawText(c, text, mul(m, [1, 0, 0, 1, x, y]), { x: 0, y: 0, w, h });
      }
      toCanvas(c, mul(m, [1, 0, 0, 1, x, y]));
      for (const [side, x0, y0, x1, y1] of [["t", 0, 0, w, 0], ["b", 0, h, w, h], ["l", 0, 0, 0, h], ["r", w, 0, w, h]]) {
        if (!strokeLine(c, cell.borders[side], k)) continue;
        c.beginPath();
        c.moveTo(x0, y0);
        c.lineTo(x1, y1);
        c.stroke();
      }
    });
  });
}

function drawMissing(c, item, base) {
  toCanvas(c, mul(base, item.m));
  c.fillStyle = "rgba(127,127,127,0.08)";
  c.fillRect(0, 0, item.w, item.h);
  c.strokeStyle = "rgba(127,127,127,0.5)";
  c.lineWidth = 1 / base[0];
  c.setLineDash([6 / base[0], 4 / base[0]]);
  c.strokeRect(0, 0, item.w, item.h);
  c.setLineDash([]);
}

// drawSlide draws a scene onto a canvas of the slide's proportions; images
// maps each picture's part to its decoded image.
export function drawSlide(scene, images, canvas) {
  const c = canvas.getContext("2d");
  const k = canvas.width / scene.width;
  const base = [k, 0, 0, k, 0, 0];
  c.setTransform(1, 0, 0, 1, 0, 0);
  c.fillStyle = "#ffffff";
  c.fillRect(0, 0, canvas.width, canvas.height);
  toCanvas(c, base);
  const bg = paint(c, scene.background, scene.width, scene.height, images);
  if (bg === "blip") drawImage(c, images.get(scene.background.media), scene.background.crop, scene.width, scene.height);
  else if (bg) c.fillRect(0, 0, scene.width, scene.height);
  for (const item of scene.items) {
    c.save();
    try {
      if (item.kind === "shape") drawShape(c, item, base, images);
      else if (item.kind === "picture") {
        toCanvas(c, mul(base, item.m));
        const img = images.get(item.media);
        if (img) {
          c.save();
          trace(c, geometry(item.geom, item.w, item.h).paths[0].cmds);
          c.clip();
          drawImage(c, img, item.crop, item.w, item.h, item.alpha);
          c.restore();
        } else drawMissing(c, item, base);
        if (strokeLine(c, item.line, k)) { trace(c, geometry(item.geom, item.w, item.h).paths[0].cmds); c.stroke(); }
      } else if (item.kind === "table") drawTable(c, item, base);
      else drawMissing(c, item, base);
    } finally {
      c.restore();
    }
  }
}

const MIME = { png: "image/png", jpg: "image/jpeg", jpeg: "image/jpeg", gif: "image/gif", bmp: "image/bmp", webp: "image/webp", ico: "image/x-icon" };
let fonts = null;
// loadFonts makes the vendored Liberation Sans known to the canvas, for
// browsers that have no Arial, once.
function loadFonts() {
  if (!fonts) {
    fonts = (async () => {
      try {
        if (typeof FontFace === "undefined" || !document.fonts) return;
        const f = new FontFace("Liberation Sans", "url(/vendor/fonts/Liberation/LiberationSans-Regular.woff2)");
        document.fonts.add(await f.load());
      } catch { /* the browser's own sans-serif stands in */ }
    })();
  }
  return fonts;
}

// renderSlide makes a picture of one slide of a deck read by readDeck:
// a PNG blob, or a JPEG when the PNG would be large, and what in it could
// only be approximated.
export async function renderSlide(deck, slide, cache = new Map()) {
  await loadFonts();
  const scene = await slideScene(deck.pkg, deck, slide.part, slide.number);
  const images = new Map();
  for (const part of scene.media) {
    if (!cache.has(part)) {
      cache.set(part, (async () => {
        try {
          const ext = (part.match(/\.([^.]+)$/)?.[1] || "").toLowerCase();
          const blob = await deck.pkg.file(part, MIME[ext] || "");
          return blob ? await createImageBitmap(blob) : null;
        } catch { return null; }
      })());
    }
    const img = await cache.get(part);
    if (img) images.set(part, img);
    else scene.missing.add("picture");
  }
  const canvas = document.createElement("canvas");
  const landscape = scene.width >= scene.height;
  canvas.width = landscape ? SLIDE_EDGE : Math.round(SLIDE_EDGE * (scene.width / scene.height));
  canvas.height = landscape ? Math.round(SLIDE_EDGE * (scene.height / scene.width)) : SLIDE_EDGE;
  drawSlide(scene, images, canvas);
  const toBlob = (type, q) => new Promise((done, fail) => canvas.toBlob((b) => (b ? done(b) : fail(new Error("pptx: the slide could not be drawn"))), type, q));
  let blob = await toBlob("image/png");
  // Photographs make large PNGs; past 4 MB a JPEG keeps the slide small.
  if (blob.size > 4 << 20) blob = await toBlob("image/jpeg", 0.92);
  canvas.width = canvas.height = 0;
  return { blob, missing: [...scene.missing] };
}
