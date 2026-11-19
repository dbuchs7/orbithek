// The page tree's width (ADR-0247):
// a handle at the tree's right edge widens or narrows it by dragging, by
// the arrow keys, Home and End; a double-click forgets the width, and the
// tree takes its default again. The width is kept in this browser, for
// every page, as folding is (ADR-0100).
//
// The arithmetic needs no browser and is tested on its own; treeHandle and
// applyTreeWidth touch the document only when they are called.

// TREE_MIN is the narrowest tree in pixels, TREE_MAX_SHARE the widest as a
// share of the window; the stylesheet clamps --tree-w to the same bounds,
// so a width kept on a wide screen still fits a narrower one.
export const TREE_MIN = 180;
export const TREE_MAX_SHARE = 0.5;
// TREE_STEP is how far an arrow key moves the edge, TREE_BIG_STEP how far
// with Shift.
export const TREE_STEP = 16;
export const TREE_BIG_STEP = 64;

const KEY = "ot.tree.width";

// treeMax is the widest tree a window of the given width allows.
export function treeMax(viewport) {
  return Math.max(TREE_MIN, Math.floor(viewport * TREE_MAX_SHARE));
}

// clampWidth keeps a width between TREE_MIN and treeMax, whole pixels.
export function clampWidth(px, viewport) {
  return Math.min(treeMax(viewport), Math.max(TREE_MIN, Math.round(px)));
}

// parseWidth reads a kept width: a positive number of pixels, or 0 for
// none (nothing kept, or something that is no width).
export function parseWidth(stored) {
  if (stored === null || stored === undefined || stored === "") return 0;
  const n = Number(stored);
  return Number.isFinite(n) && n > 0 ? Math.round(n) : 0;
}

// keyWidth is the width a key gives the tree, or null for a key that does
// not move the edge.
export function keyWidth(key, shift, current, viewport) {
  const step = shift ? TREE_BIG_STEP : TREE_STEP;
  switch (key) {
    case "ArrowLeft": return clampWidth(current - step, viewport);
    case "ArrowRight": return clampWidth(current + step, viewport);
    case "Home": return TREE_MIN;
    case "End": return treeMax(viewport);
    default: return null;
  }
}

// storedWidth is the width kept in this browser, or 0.
function storedWidth() {
  try { return parseWidth(localStorage.getItem(KEY)); } catch { return 0; }
}

// keepWidth keeps a width, or forgets it for 0.
function keepWidth(px) {
  try {
    if (px) localStorage.setItem(KEY, String(px));
    else localStorage.removeItem(KEY);
  } catch { /* for this page only */ }
}

// setWidth sets the tree's width on the document, or takes it away for 0.
function setWidth(px) {
  const root = document.documentElement.style;
  if (px) root.setProperty("--tree-w", px + "px");
  else root.removeProperty("--tree-w");
}

// applyTreeWidth sets the kept width before the first view is drawn.
export function applyTreeWidth() {
  setWidth(storedWidth());
}

// treeHandle returns the handle for the tree column col: a vertical
// separator that can be focused, labelled label and titled hint.
export function treeHandle(col, label, hint) {
  const handle = document.createElement("div");
  handle.className = "tree-resizer";
  handle.id = "tree-resizer";
  handle.tabIndex = 0;
  handle.title = hint;
  handle.setAttribute("role", "separator");
  handle.setAttribute("aria-orientation", "vertical");
  handle.setAttribute("aria-label", label);
  const width = () => Math.round(col.getBoundingClientRect().width);
  const show = () => {
    handle.setAttribute("aria-valuemin", String(TREE_MIN));
    handle.setAttribute("aria-valuemax", String(treeMax(window.innerWidth)));
    handle.setAttribute("aria-valuenow", String(width()));
  };
  const set = (px, keep) => {
    setWidth(px);
    if (keep) keepWidth(px);
    show();
  };
  let start = null;
  handle.addEventListener("pointerdown", (e) => {
    if (e.button !== 0) return;
    e.preventDefault();
    start = { x: e.clientX, w: width(), moved: false };
    handle.setPointerCapture(e.pointerId);
    document.documentElement.classList.add("tree-resizing");
  });
  handle.addEventListener("pointermove", (e) => {
    if (!start || (e.clientX === start.x && !start.moved)) return;
    start.moved = true;
    set(clampWidth(start.w + e.clientX - start.x, window.innerWidth), false);
  });
  const end = (e) => {
    if (!start) return;
    // A click without a move keeps nothing, so the default stays the default.
    const moved = start.moved;
    start = null;
    if (handle.hasPointerCapture(e.pointerId)) handle.releasePointerCapture(e.pointerId);
    document.documentElement.classList.remove("tree-resizing");
    if (moved) keepWidth(width());
  };
  handle.addEventListener("pointerup", end);
  handle.addEventListener("pointercancel", end);
  handle.addEventListener("dblclick", () => set(0, true));
  handle.addEventListener("keydown", (e) => {
    const px = keyWidth(e.key, e.shiftKey, width(), window.innerWidth);
    if (px === null) return;
    e.preventDefault();
    set(px, true);
  });
  handle.addEventListener("focus", show);
  handle.addEventListener("pointerenter", show);
  return handle;
}
