// The parts of presenting pictures of the album (ADR-0176) that need no
// browser: the order of the slides, the geometry of zooming and of the
// laser pointer, the keys and the messages between the presenter's window
// and the presentation window. They are tested in Node
// (jstest/present.test.mjs); present.js draws with them.

const DRAWING = ".drawing.png";

// MAX_ZOOM is the closest a slide is shown, as a multiple of the size that
// fits the window; ZOOM_STEP is one step of the keys and buttons.
export const MAX_ZOOM = 8;
export const ZOOM_STEP = 1.25;

// LASER_TRAIL is how long, in milliseconds, the laser pointer leaves a
// fading trail behind it.
export const LASER_TRAIL = 350;

// captionOf is a slide's name: its file name without the extension or a
// drawing's marker, as the album shows it.
export const captionOf = (name) => (name.endsWith(DRAWING) ? name.slice(0, -DRAWING.length) : name.replace(/\.[^.]*$/, ""));

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

// slidesOf makes the slides of a presentation from the album's pictures:
// the newest version of each, ordered by name so that "2 Plan" comes
// before "10 Outlook", and of two with the same name by the item and the
// file, so the order is the same in every window.
export function slidesOf(pictures) {
  return (pictures || []).map((p) => ({
    item: { id: p.item.id, key: p.item.key, title: p.item.title },
    attachment: { id: p.attachment.id, name: p.attachment.name, type: p.attachment.type },
    name: captionOf(p.attachment.name),
    notes: p.notes || "",
    editable: !!p.editable,
    drawing: !!p.drawing,
  })).sort((a, b) => collator.compare(a.name, b.name) || collator.compare(a.item.key, b.item.key) ||
    (a.attachment.id < b.attachment.id ? -1 : a.attachment.id > b.attachment.id ? 1 : 0));
}

// DECK_FILTERS are the album's filters a presentation is chosen by, a
// collection (ADR-0175) among them.
export const DECK_FILTERS = ["folder", "collection", "space", "q", "kind", "label"];

// deckParams keeps only the album's filters of a set of parameters, the
// empty ones left out.
export function deckParams(params) {
  const out = new URLSearchParams();
  for (const k of DECK_FILTERS) {
    const v = params.get(k);
    if (v) out.set(k, v);
  }
  return out;
}

// step moves from one slide by delta, staying within the deck.
export const step = (index, count, delta) => (count ? Math.min(count - 1, Math.max(0, index + delta)) : 0);

// fit is the scale at which an image of size image fits a view, whole.
export function fit(view, image) {
  if (!(image.w > 0 && image.h > 0 && view.w > 0 && view.h > 0)) return 1;
  return Math.min(view.w / image.w, view.h / image.h);
}

const clamp = (v, lo, hi) => Math.min(hi, Math.max(lo, v));

// clampView keeps a zoom within its bounds and its centre where the image
// still fills the view along each axis it is larger than the view; along
// an axis it fits in, the image stays centred. The centre is a point of
// the image, from 0 to 1 across and down.
export function clampView(view, image, zoom, center) {
  const z = clamp(Number.isFinite(zoom) ? zoom : 1, 1, MAX_ZOOM);
  const scale = fit(view, image) * z;
  const axis = (c, viewLen, imgLen) => {
    const half = viewLen / (2 * imgLen * scale);
    return half >= 0.5 ? 0.5 : clamp(Number.isFinite(c) ? c : 0.5, half, 1 - half);
  };
  return { zoom: z, center: { x: axis(center?.x, view.w, image.w), y: axis(center?.y, view.h, image.h) } };
}

// placement is where an image is drawn in a view: its scale and the
// position of its top left corner, for a zoom and a centre.
export function placement(view, image, zoom, center) {
  const v = clampView(view, image, zoom, center);
  const scale = fit(view, image) * v.zoom;
  return { scale, left: view.w / 2 - v.center.x * image.w * scale, top: view.h / 2 - v.center.y * image.h * scale };
}

// toImage turns a point of the view into a point of the image, from 0 to
// 1 across and down; outside the image it is below 0 or above 1. The
// laser pointer is sent this way, so each window shows it on the same
// spot of the slide, whatever its size.
export function toImage(view, image, zoom, center, point) {
  const p = placement(view, image, zoom, center);
  return { x: (point.x - p.left) / (image.w * p.scale), y: (point.y - p.top) / (image.h * p.scale) };
}

// toView turns a point of the image back into a point of the view.
export function toView(view, image, zoom, center, point) {
  const p = placement(view, image, zoom, center);
  return { x: p.left + point.x * image.w * p.scale, y: p.top + point.y * image.h * p.scale };
}

// zoomAt zooms by factor about a point of the view, which stays over the
// same spot of the image as far as the bounds allow.
export function zoomAt(view, image, zoom, center, point, factor) {
  const at = toImage(view, image, zoom, center, point);
  const z = clamp(zoom * factor, 1, MAX_ZOOM);
  const scale = fit(view, image) * z;
  return clampView(view, image, z, {
    x: at.x - (point.x - view.w / 2) / (image.w * scale),
    y: at.y - (point.y - view.h / 2) / (image.h * scale),
  });
}

// panBy moves a zoomed slide by dx and dy pixels of the view.
export function panBy(view, image, zoom, center, dx, dy) {
  const v = clampView(view, image, zoom, center);
  const scale = fit(view, image) * v.zoom;
  return clampView(view, image, v.zoom, { x: v.center.x - dx / (image.w * scale), y: v.center.y - dy / (image.h * scale) });
}

// keyAction is what a key does in either window. The keys of presentation
// remotes (page up and down, B for a black screen) are among them.
export function keyAction(key) {
  switch (key) {
    case "ArrowRight": case "ArrowDown": case "PageDown": case " ": case "Enter": case "n": case "N":
      return "next";
    case "ArrowLeft": case "ArrowUp": case "PageUp": case "Backspace": case "p": case "P":
      return "prev";
    case "Home": return "first";
    case "End": return "last";
    case "+": case "=": return "zoomIn";
    case "-": case "_": return "zoomOut";
    case "0": return "zoomReset";
    case "l": case "L": return "laser";
    case "b": case "B": case ".": return "black";
    case "f": case "F": return "fullscreen";
    case "Escape": return "escape";
    default: return null;
  }
}

// clock writes a duration in milliseconds as m:ss, or h:mm:ss from an
// hour on.
export function clock(ms) {
  const s = Math.max(0, Math.floor(ms / 1000));
  const two = (n) => String(n).padStart(2, "0");
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h ? `${h}:${two(m)}:${two(s % 60)}` : `${m}:${two(s % 60)}`;
}

// channelName is the name of the channel the two windows of one
// presentation talk on; a presentation's id is letters and digits only.
export const channelName = (id) => "ot-present-" + String(id).replace(/[^A-Za-z0-9]/g, "");

// newId makes the id of a presentation from random bytes.
export function newId(bytes) {
  return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
}

// The messages between the windows. The presenter's window holds the
// state and sends all of it on every change ("state"); the presentation
// window asks for it when it opens ("hello"), sends what its keys and
// pointer do ("command"), and both send the laser pointer's spot on the
// slide, or null when it left ("laser"). "bye" ends the presentation.
export const MESSAGES = ["state", "hello", "command", "laser", "bye"];

// validMessage reports whether something received on the channel is a
// message of a presentation, so a stray one changes nothing.
export function validMessage(m) {
  if (!m || typeof m !== "object" || !MESSAGES.includes(m.type)) return false;
  if (m.type === "command") return typeof m.action === "string";
  if (m.type === "laser") return m.at === null || (Number.isFinite(m.at?.x) && Number.isFinite(m.at?.y));
  if (m.type === "state") return Array.isArray(m.state?.slides) && Number.isInteger(m.state?.index);
  return true;
}

// apply works out the state after a command: moving between slides
// starts each slide unzoomed; zooming by the keys zooms about the centre.
export function apply(state, action, view, image) {
  const n = state.slides.length;
  const s = { ...state };
  const go = (i) => { if (i !== s.index) Object.assign(s, { index: i, zoom: 1, center: { x: 0.5, y: 0.5 } }); };
  switch (action) {
    case "next": go(step(s.index, n, 1)); break;
    case "prev": go(step(s.index, n, -1)); break;
    case "first": go(0); break;
    case "last": go(Math.max(0, n - 1)); break;
    case "zoomIn": case "zoomOut": {
      const mid = { x: view.w / 2, y: view.h / 2 };
      Object.assign(s, zoomAt(view, image, s.zoom, s.center, mid, action === "zoomIn" ? ZOOM_STEP : 1 / ZOOM_STEP));
      break;
    }
    case "zoomReset": Object.assign(s, { zoom: 1, center: { x: 0.5, y: 0.5 } }); break;
    case "laser": s.laser = !s.laser; break;
    case "black": s.black = !s.black; break;
    default: return state;
  }
  return s;
}
