// How a picture looks, as the album compares it (ADR-0195): a difference
// hash of the picture turned upright, the same the server makes with
// internal/photo/fingerprint.go. The picture is reduced by area averaging
// to 9×8 cells of grey; each of the 64 bits says whether a cell is
// brighter than its right neighbour. It survives scaling, a new encoding
// and a removed place, which all change the file's bytes.
//
// This module runs without a browser: it works on pixels. The shared test
// vector (internal/web/jstest/fingerprint-vector.json) holds it to the
// server's computation.

export const GRID_COLS = 9;
export const GRID_ROWS = 8;
// MIN_CONTRAST is the least difference of grey between the brightest and
// the darkest cell: a flatter picture has no fingerprint.
export const MIN_CONTRAST = 3;

// spans maps each of n source pixels onto m ≤ n cells: pixel j covers
// cell idx[j] with the share weight[j] and the next cell with the rest.
function spans(n, m) {
  const idx = new Array(n);
  const weight = new Array(n);
  for (let j = 0; j < n; j++) {
    const d = Math.floor((j * m) / n);
    idx[j] = d;
    weight[j] = 1;
    const b = (d + 1) * n;
    if ((j + 1) * m > b) weight[j] = (b - j * m) / m;
  }
  return { idx, weight };
}

// greyGrid reduces RGBA pixels (as a canvas gives them, not premultiplied)
// to cols×rows cells of grey, a transparent pixel counting as black.
export function greyGrid(rgba, width, height, cols = GRID_COLS, rows = GRID_ROWS) {
  const x = spans(width, cols);
  const y = spans(height, rows);
  const line = new Float64Array(cols);
  const g = new Float64Array(cols * rows);
  for (let row = 0; row < height; row++) {
    line.fill(0);
    for (let col = 0; col < width; col++) {
      const i = 4 * (row * width + col);
      const a = rgba[i + 3];
      let r = rgba[i], gr = rgba[i + 1], b = rgba[i + 2];
      if (a !== 255) {
        r = Math.round((r * a) / 255);
        gr = Math.round((gr * a) / 255);
        b = Math.round((b * a) / 255);
      }
      const v = 0.299 * r + 0.587 * gr + 0.114 * b;
      const d = x.idx[col];
      const w = x.weight[col];
      line[d] += v * w;
      if (w !== 1) line[d + 1] += v * (1 - w);
    }
    const d = y.idx[row];
    const w = y.weight[row];
    for (let c = 0; c < cols; c++) {
      g[d * cols + c] += line[c] * w;
      if (w !== 1) g[(d + 1) * cols + c] += line[c] * (1 - w);
    }
  }
  const norm = (cols * rows) / (width * height);
  for (let i = 0; i < g.length; i++) g[i] *= norm;
  return g;
}

// gridFingerprint writes the bits of a 9×8 grid as 16 hexadecimal digits,
// or "" for a flat grid.
export function gridFingerprint(g) {
  let lo = g[0], hi = g[0];
  for (const v of g) { lo = Math.min(lo, v); hi = Math.max(hi, v); }
  if (hi - lo < MIN_CONTRAST) return "";
  let out = "", nibble = 0, n = 0;
  for (let r = 0; r < GRID_ROWS; r++) {
    for (let c = 0; c < GRID_COLS - 1; c++) {
      nibble = (nibble << 1) | (g[r * GRID_COLS + c] > g[r * GRID_COLS + c + 1] ? 1 : 0);
      if (++n === 4) {
        out += nibble.toString(16);
        nibble = 0;
        n = 0;
      }
    }
  }
  return out;
}

// fingerprintOf is the fingerprint of upright RGBA pixels, or "" for a
// picture smaller than the grid or flat.
export function fingerprintOf(rgba, width, height) {
  if (width < GRID_COLS || height < GRID_ROWS) return "";
  return gridFingerprint(greyGrid(rgba, width, height));
}

const BITS = [0, 1, 1, 2, 1, 2, 2, 3, 1, 2, 2, 3, 2, 3, 3, 4];
const FP = /^[0-9a-f]{16}$/;

// distance is how many bits two fingerprints differ in, or -1.
export function distance(a, b) {
  if (!FP.test(a || "") || !FP.test(b || "")) return -1;
  let n = 0;
  for (let i = 0; i < 16; i++) n += BITS[parseInt(a[i], 16) ^ parseInt(b[i], 16)];
  return n;
}

// sameShape: two pictures have the same proportions, as far as known.
export function sameShape(w1, h1, w2, h2) {
  if (!(w1 > 0 && h1 > 0 && w2 > 0 && h2 > 0)) return true;
  const a = w1 / h1, b = w2 / h2;
  return Math.abs(a - b) / Math.max(a, b) <= 0.02;
}

// lookAlike: two files read ({fingerprint, width, height}) look the same,
// their fingerprints at most bits apart and their proportions equal.
export function lookAlike(a, b, bits) {
  const d = distance(a.fingerprint, b.fingerprint);
  return d >= 0 && d <= bits && sameShape(a.width, a.height, b.width, b.height);
}

// earlierTwins finds the files chosen twice: for each file read
// ({sha256, fingerprint, width, height}) the index of an earlier one that
// is the same file or looks the same, or -1.
export function earlierTwins(files, bits) {
  return files.map((f, i) => {
    for (let j = 0; j < i; j++) {
      const o = files[j];
      if ((f.sha256 && f.sha256 === o.sha256) || lookAlike(f, o, bits)) return j;
    }
    return -1;
  });
}

// fitWithin is the size w×h scaled so its longer edge is at most edge,
// never enlarged, each side at least one pixel.
export function fitWithin(w, h, edge) {
  if (w <= edge && h <= edge) return { width: w, height: h };
  const s = edge / Math.max(w, h);
  return { width: Math.max(1, Math.round(w * s)), height: Math.max(1, Math.round(h * s)) };
}
