// The parts of a live whiteboard (ADR-0169, ADR-0170) that need no
// browser: which elements are held by whom, what changed, what a
// selection locks, the frames in reading order, the time left, the result
// of a vote, the comments by element and where a badge goes on the
// screen. whiteboard.js uses them in the drawing page; jstest tests them.

import { KEY } from "./drawprocesses.js";

// NOTE_COLOURS are the colours of notes, by the names the server knows.
export const NOTE_COLOURS = {
  yellow: "#ffec99", green: "#b2f2bb", blue: "#a5d8ff", pink: "#fcc2d7", orange: "#ffd8a8", violet: "#d0bfff",
};

// PEOPLE_COLOURS are the participants' colours for cursors and selections,
// by the number the server gives each person.
export const PEOPLE_COLOURS = [
  "#e03131", "#1971c2", "#2f9e44", "#f08c00", "#9c36b5", "#0c8599", "#e8590c", "#5f3dc4", "#c2255c", "#66a80f",
  "#1864ab", "#d9480f", "#087f5b", "#862e9c", "#a61e4d", "#364fc7", "#5c940d", "#c92a2a", "#0b7285", "#495057",
];

export const colourOf = (n) => PEOPLE_COLOURS[(Number.isInteger(n) ? n : 0) % PEOPLE_COLOURS.length];

// isNote tells a note (ADR-0170).
export const isNote = (el) => el?.type === "rectangle" && el.customData?.[KEY]?.kind === "note";

// VOTABLE are the kinds one may vote on, as the server takes them.
const VOTABLE = new Set(["rectangle", "diamond", "ellipse", "image"]);
export const votable = (el) => !!el && !el.isDeleted && VOTABLE.has(el.type);

// byIndex orders elements as Excalidraw stacks them: by their fractional
// index, those without one where they were.
export function byIndex(elements) {
  return elements.map((el, i) => [el, i]).sort(([a, i], [b, j]) => {
    const x = a.index || "", y = b.index || "";
    if (x === y) return i - j;
    return x < y ? -1 : 1;
  }).map(([el]) => el);
}

// shown is an element as one participant sees it: held by someone else, it
// is locked, so it cannot be selected (ADR-0169). heldBy maps element IDs
// to the client holding them.
export function shown(el, heldBy, me) {
  const holder = heldBy.get(el.id);
  return holder && holder !== me && !el.locked ? { ...el, locked: true } : el;
}

// relocked are the elements to show anew when the locks of ids change:
// those whose lock is shown otherwise than it should be now, as they are
// on the server. The others stay as the scene has them; that matters for
// what one holds oneself: it may be newer than the server's while its
// change is on the way, and Excalidraw may still be working on the very
// object, as on a picture being added, whose file is set only once it is
// read. Replaced, the work would go to an object no longer shown and
// never be sent. scene maps element IDs to what the scene shows.
export function relocked(ids, scene, truth, heldBy, me) {
  const out = [];
  for (const id of ids) {
    const el = truth.get(id);
    if (!el) continue;
    const want = shown(el, heldBy, me);
    const now = scene.get(id);
    if (now && !!now.locked === !!want.locked) continue;
    out.push(want);
  }
  return out;
}

// sendable is an element as it is sent: the lock shown for another's hold
// is not the element's own, and an image this browser could not load is
// not broken for the others.
export function sendable(el, truth, heldBy, me) {
  const holder = heldBy.get(el.id);
  let out = el;
  if (holder && holder !== me) out = { ...out, locked: truth.get(el.id)?.locked ?? false };
  if (out.type === "image" && out.status === "error") out = { ...out, status: truth.get(el.id)?.status ?? "saved" };
  return out;
}

// pictures are the files the image elements among els show, each once.
export const pictures = (els) => [...new Set(els.filter((el) => el.type === "image" && el.fileId && !el.isDeleted).map((el) => el.fileId))];

// pointerTool is the tool a cursor is sent with: the laser pointer, or the
// plain pointer.
export const pointerTool = (tool) => (tool === "laser" ? "laser" : "pointer");

// changed lists the elements of the scene newer than the server's.
export function changed(scene, truth) {
  return scene.filter((el) => {
    const known = truth.get(el.id);
    return !known || el.version > known.version;
  });
}

// newer tells whether an element from the server replaces what one holds.
export const newer = (el, known) => !known || el.version > known.version ||
  (el.version === known.version && el.versionNonce !== known.versionNonce);

// lockSet is what a selection locks: the selected elements, the text bound
// to each and the container of a selected text.
export function lockSet(selectedIds, elements) {
  const byId = new Map(elements.map((el) => [el.id, el]));
  const out = new Set();
  for (const id of selectedIds) {
    const el = byId.get(id);
    if (!el || el.isDeleted) continue;
    out.add(id);
    for (const b of el.boundElements || []) if (b.type === "text") out.add(b.id);
    if (el.containerId) out.add(el.containerId);
  }
  return out;
}

// difference returns what to lock and what to release to go from one set
// of held elements to another.
export function difference(held, wanted) {
  return { lock: [...wanted].filter((id) => !held.has(id)), release: [...held].filter((id) => !wanted.has(id)) };
}

// textOf is the text an element shows: its own, its bound text's or a
// frame's name.
export function textOf(el, byId) {
  if (!el) return "";
  if (el.type === "text") return el.originalText || el.text || "";
  if (el.type === "frame" || el.type === "magicframe") return el.name || "";
  for (const b of el.boundElements || []) {
    const t = b.type === "text" ? byId.get(b.id) : null;
    if (t && !t.isDeleted) return textOf(t, byId);
  }
  return "";
}

// frames are the frames in reading order, as the server steps through them
// (ADR-0170).
export function frames(elements) {
  const fs = elements.filter((el) => !el.isDeleted && (el.type === "frame" || el.type === "magicframe"));
  return fs.map((f, i) => [f, i]).sort(([a, i], [b, j]) => {
    const half = Math.min(a.height, b.height) / 2;
    const d = a.y - b.y;
    if (d < -half || d > half) return a.y - b.y;
    return a.x - b.x || i - j;
  }).map(([f]) => f);
}

// inFrame are a frame and what lies in it, for fitting the view to it.
export const inFrame = (frame, elements) => elements.filter((el) => !el.isDeleted && (el.id === frame.id || el.frameId === frame.id));

// timeLeft is the countdown's milliseconds left (ADR-0170): offset is the
// server's clock minus this one's.
export function timeLeft(timer, now, offset = 0) {
  if (!timer?.running) return null;
  if (timer.paused) return Math.max(0, timer.left || 0);
  return Math.max(0, timer.ends - (now + offset));
}

// clock writes milliseconds as m:ss, or h:mm:ss from an hour on.
export function clock(ms) {
  const s = Math.ceil(Math.max(0, ms) / 1000);
  const h = Math.floor(s / 3600), m = Math.floor((s % 3600) / 60), r = s % 60;
  const two = (n) => String(n).padStart(2, "0");
  return h ? `${h}:${two(m)}:${two(r)}` : `${m}:${two(r)}`;
}

// ranking is a closed vote's result, most votes first: the element, its
// text and its votes.
export function ranking(results, elements) {
  const byId = new Map(elements.map((el) => [el.id, el]));
  return Object.entries(results || {}).filter(([id, n]) => n > 0 && byId.has(id))
    .map(([id, votes]) => ({ id, votes, text: textOf(byId.get(id), byId) }))
    .sort((a, b) => b.votes - a.votes || a.text.localeCompare(b.text));
}

// threadsByElement groups the open threads on the elements of one drawing
// by element (ADR-0171).
export function threadsByElement(comments, drawing) {
  const out = new Map();
  for (const c of comments || []) {
    if (c.reply || c.deleted || c.resolved || c.anchor?.drawing !== drawing || !c.anchor?.element) continue;
    out.set(c.anchor.element, [...(out.get(c.anchor.element) || []), c]);
  }
  return out;
}

// onScreen is where a scene point shows in the canvas, for the badges of
// votes and comments.
export function onScreen(x, y, appState) {
  const zoom = appState?.zoom?.value || 1;
  return { left: (x + (appState?.scrollX || 0)) * zoom, top: (y + (appState?.scrollY || 0)) * zoom };
}

// viewOf is the scene point in the middle of what is in sight, with the
// zoom, as a presenter's view is followed; scrollFor is the scroll that
// puts such a point in the middle of another's view.
export function viewOf(appState) {
  const zoom = appState.zoom?.value || 1;
  return { x: appState.width / 2 / zoom - appState.scrollX, y: appState.height / 2 / zoom - appState.scrollY, zoom };
}

export function scrollFor(view, appState) {
  return { scrollX: appState.width / 2 / view.zoom - view.x, scrollY: appState.height / 2 / view.zoom - view.y };
}

// throttle runs fn at most every ms milliseconds, the last call winning.
export function throttle(fn, ms, timers = { setTimeout: (f, d) => setTimeout(f, d) }) {
  let waiting = null, last = null;
  return (...args) => {
    last = args;
    if (waiting) return;
    waiting = timers.setTimeout(() => { waiting = null; fn(...last); }, ms);
  };
}
