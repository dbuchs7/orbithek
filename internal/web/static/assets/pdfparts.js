// The parts of the PDF reader (ADR-0210) that need no browser: which files
// are PDFs, the reader's address, the rectangles a selection makes on a
// page, finding a highlight's quote again on a page, the list of a
// document's highlights and its export as Markdown. pdfview.js draws the
// reader with them; jstest/pdfparts.test.mjs tests them.

// COLORS are the colours of highlights, as the server admits them
// (item.HighlightColors); the first is the colour of one without.
export const COLORS = ["yellow", "green", "blue", "pink", "purple"];

// isPdf says whether a file of an item is a PDF: told so by its bytes when
// it was stored, or named so (item.IsPDF).
export const isPdf = (a) => Boolean(a) && (a.type === "application/pdf" || /\.pdf$/i.test(a.name || ""));

// readerHref is the reader's address for a file of an item, at a page or
// at a highlight.
export function readerHref(key, file, { page = 0, comment = "" } = {}) {
  const q = comment ? "?c=" + encodeURIComponent(comment) : page > 1 ? "?page=" + page : "";
  return `#/pdf/${encodeURIComponent(key)}/${encodeURIComponent(file)}${q}`;
}

// readState reads the page and the highlight from the reader's address.
export function readState(params) {
  const page = Number.parseInt(params.get("page") || "", 10);
  return { page: page > 0 ? page : 1, comment: params.get("c") || "" };
}

// attachmentLink tells whether an address in a text leads to a file of an
// item on this site, and to which: {item, file}, or null.
export function attachmentLink(href, origin) {
  let u;
  try {
    u = new URL(href, origin);
  } catch {
    return null;
  }
  if (u.origin !== origin) return null;
  const m = /^\/api\/items\/([^/]+)\/attachments\/([0-9a-f]{32})$/.exec(u.pathname);
  return m ? { item: decodeURIComponent(m[1]), file: m[2] } : null;
}

const round = (v) => Math.round(v * 10000) / 10000;

// normRects turns the rectangles of a selection (client rectangles: left,
// top, width, height) into fractions of the page's box, clipped to the
// page, one per run of a line, top to bottom; at most max, merging lines
// when a selection runs over more (the server's budget HighlightRects).
export function normRects(rects, box, max = 200) {
  if (!(box.width > 0 && box.height > 0)) return [];
  let out = [];
  for (const r of rects) {
    let x = (r.left - box.left) / box.width, y = (r.top - box.top) / box.height;
    let right = x + r.width / box.width, bottom = y + r.height / box.height;
    x = Math.max(0, x); y = Math.max(0, y); right = Math.min(1, right); bottom = Math.min(1, bottom);
    if (right - x < 0.001 || bottom - y < 0.001) continue;
    out.push({ x, y, right, bottom });
  }
  out.sort((a, b) => a.y - b.y || a.x - b.x);
  // Runs of one line that touch or overlap become one rectangle.
  const merged = [];
  for (const r of out) {
    const last = merged.at(-1);
    const overlap = last ? Math.min(last.bottom, r.bottom) - Math.max(last.y, r.y) : 0;
    const sameLine = last && overlap > 0.5 * Math.min(last.bottom - last.y, r.bottom - r.y);
    if (sameLine && r.x <= last.right + 0.01) {
      Object.assign(last, { x: Math.min(last.x, r.x), y: Math.min(last.y, r.y), right: Math.max(last.right, r.right), bottom: Math.max(last.bottom, r.bottom) });
    } else {
      merged.push({ ...r });
    }
  }
  out = merged;
  while (out.length > max) {
    const fewer = [];
    for (let i = 0; i < out.length; i += 2) {
      const a = out[i], b = out[i + 1] || a;
      fewer.push({ x: Math.min(a.x, b.x), y: Math.min(a.y, b.y), right: Math.max(a.right, b.right), bottom: Math.max(a.bottom, b.bottom) });
    }
    out = fewer;
  }
  return out.map((r) => {
    const x = round(r.x), y = round(r.y);
    return { x, y, w: round(Math.min(r.right, 1) - x), h: round(Math.min(r.bottom, 1) - y) };
  }).filter((r) => r.w > 0 && r.h > 0 && r.x + r.w <= 1 && r.y + r.h <= 1);
}

// squash leaves the whitespace out of a text, so texts compare whatever
// breaks their lines; map[i] is where text[i] stands in raw.
export function squash(raw) {
  let text = "";
  const map = [];
  for (let i = 0; i < raw.length; i++) {
    if (/\s/.test(raw[i])) continue;
    text += raw[i];
    map.push(i);
  }
  return { text, map };
}

// locate finds a highlight's quote in the text of a page (the text layer's,
// in its order), whitespace aside: where the quote occurs more than once,
// the occurrence whose surroundings match more of the prefix and suffix.
// It answers the quote's start and end in raw, or null.
export function locate(raw, quote, prefix = "", suffix = "") {
  const q = squash(quote || "").text;
  if (!q) return null;
  const { text, map } = squash(raw);
  const pre = squash(prefix).text, suf = squash(suffix).text;
  let best = null, bestScore = -1;
  for (let at = text.indexOf(q); at >= 0; at = text.indexOf(q, at + 1)) {
    let score = 0;
    while (score < pre.length && at - score - 1 >= 0 && text[at - score - 1] === pre[pre.length - score - 1]) score++;
    let after = 0;
    while (after < suf.length && text[at + q.length + after] === suf[after]) after++;
    if (score + after > bestScore) {
      best = at;
      bestScore = score + after;
    }
  }
  if (best === null) return null;
  return { start: map[best], end: map[best + q.length - 1] + 1 };
}

// onPdf says whether a comment is a highlight in a PDF.
export const onPdf = (c) => Boolean(c?.anchor?.file);

const top = (c) => Math.min(...(c.anchor.rects || []).map((r) => r.y), 1);

// highlightsOf lists the highlights of a PDF, named by its first version,
// among an item's comments: each thread's first comment with its replies,
// by page and place on the page.
export function highlightsOf(comments, first) {
  const roots = comments.filter((c) => !c.reply && c.anchor?.file === first);
  return roots
    .map((root) => ({ root, replies: comments.filter((c) => c.reply === root.id && !c.deleted) }))
    .sort((a, b) => a.root.anchor.page - b.root.anchor.page || top(a.root) - top(b.root));
}

// mdText escapes what Markdown would read as markup in plain text.
const mdText = (s) => String(s ?? "").replace(/[\\`*_[\]<>#|]/g, (c) => "\\" + c);

// highlightsMarkdown writes a document's highlights as Markdown: a heading
// per highlight with its page, the quote as a quotation, the note with its
// author and the replies as a list. The words come from the caller, in the
// UI language.
export function highlightsMarkdown({ title, source, highlights, page, person, resolved }) {
  const lines = ["# " + mdText(title), ""];
  if (source) lines.push(mdText(source), "");
  for (const { root, replies } of highlights) {
    lines.push("## " + page(root.anchor.page) + (root.resolved ? ` (${resolved})` : ""), "");
    for (const l of String(root.anchor.quote || "").split("\n")) lines.push("> " + mdText(l));
    lines.push("");
    if (root.text) lines.push(`${mdText(root.text)} (${mdText(person(root.author))})`, "");
    if (replies.length) lines.push(...replies.map((r) => `- ${mdText(person(r.author))}: ${mdText(r.text)}`), "");
  }
  return lines.join("\n");
}

// ZOOMS are the reader's steps of zoom, as a share of the page's size.
export const ZOOMS = [0.5, 0.75, 1, 1.25, 1.5, 2, 3, 4];

export const zoomIn = (s) => ZOOMS.find((z) => z > s + 1e-6) ?? ZOOMS.at(-1);
export const zoomOut = (s) => ZOOMS.findLast((z) => z < s - 1e-6) ?? ZOOMS[0];

// clampPage keeps a page number within a document of count pages.
export const clampPage = (n, count) => (Number.isFinite(n) ? Math.min(Math.max(1, Math.trunc(n)), Math.max(1, count)) : 1);
