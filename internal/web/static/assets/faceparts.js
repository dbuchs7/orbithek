// The parts of faces in the album (ADR-0194) that need no browser: boxes
// as fractions of a picture, keeping names when faces are found again,
// and how a person's face is cut out for a cover. They are tested in Node
// (jstest/faces.test.mjs); faces.js finds faces with them.

// MIN_FACE is the smallest box kept, as a fraction of the picture, as the
// server takes it.
export const MIN_FACE = 0.005;

const clamp01 = (v) => Math.min(1, Math.max(0, v));
const round = (v) => Math.round(v * 1e4) / 1e4;

// boxOf turns a box in pixels of a picture of width w and height h into
// fractions within the picture; null when it is too small or outside.
export function boxOf(box, w, h) {
  if (!(w > 0 && h > 0) || !box) return null;
  const x = clamp01(box.x / w), y = clamp01(box.y / h);
  const x2 = clamp01((box.x + box.width) / w), y2 = clamp01((box.y + box.height) / h);
  const out = { x: round(x), y: round(y), w: round(x2 - x), h: round(y2 - y) };
  if (out.w < MIN_FACE || out.h < MIN_FACE) return null;
  // Rounding must not push the box past the picture's edge.
  out.w = Math.min(out.w, round(1 - out.x));
  out.h = Math.min(out.h, round(1 - out.y));
  return out;
}

// overlap is the share of two boxes that they have in common: the area
// of their intersection over the area of their union, from 0 to 1.
export function overlap(a, b) {
  const ix = Math.max(0, Math.min(a.x + a.w, b.x + b.w) - Math.max(a.x, b.x));
  const iy = Math.max(0, Math.min(a.y + a.h, b.y + b.h) - Math.max(a.y, b.y));
  const inter = ix * iy;
  const union = a.w * a.h + b.w * b.h - inter;
  return union > 0 ? inter / union : 0;
}

// mergeFaces keeps the names of the faces known when faces are found
// again: a face found where a named one was takes its name. Named faces
// nothing was found at stay, so a search never drops a name.
export function mergeFaces(known, found) {
  const left = (known || []).filter((f) => f.person);
  const out = (found || []).map((f) => {
    let best = -1, score = 0.3;
    left.forEach((k, i) => { const s = overlap(k, f); if (s > score) { score = s; best = i; } });
    if (best < 0) return { ...f };
    const [k] = left.splice(best, 1);
    return { ...f, person: k.person };
  });
  return [...out, ...left.map((k) => ({ x: k.x, y: k.y, w: k.w, h: k.h, person: k.person }))];
}

// facesBody is the faces as the server takes them: the box, the person,
// and a vector only where there is one.
export function facesBody(faces) {
  return { faces: (faces || []).map((f) => ({ x: f.x, y: f.y, w: f.w, h: f.h, ...(f.person ? { person: f.person } : {}),
    ...(f.vector && f.vector.length ? { vector: Array.from(f.vector) } : {}) })) };
}

// coverCut is how a picture of width w and height h is drawn in a square
// of size so that a face fills it with some room around it: the scale and
// the offset of the picture's top left corner.
export function coverCut(face, w, h, size, room = 1.8) {
  const fw = face.w * w, fh = face.h * h;
  const side = Math.max(fw, fh) * room || Math.max(w, h);
  const scale = size / side;
  const cx = (face.x + face.w / 2) * w, cy = (face.y + face.h / 2) * h;
  return { scale, left: size / 2 - cx * scale, top: size / 2 - cy * scale };
}

// personNames are the distinct names of faces, in the order given.
export function personNames(faces) {
  const seen = new Set();
  const out = [];
  for (const f of faces || []) {
    const k = (f.person || "").toLowerCase();
    if (k && !seen.has(k)) { seen.add(k); out.push(f.person); }
  }
  return out;
}
