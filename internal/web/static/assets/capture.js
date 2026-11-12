// Capturing into the wiki from anywhere (ADR-0207) without a browser:
// reading what a share, a bookmarklet or a link passed on, the blocks a
// capture adds to a body, the targets it may go to, and the bookmarklet
// itself.
//
// A share arrives as title, text and url. Many phones pass a page's
// address inside the text, so an address that stands alone at the text's
// end is taken as the url when none came.
//
// Plain functions over plain data, so node --test runs them
// (internal/web/jstest/capture.test.mjs). The view is captureview.js.

// LIMITS bound what a capture keeps of each part, in characters; the
// bookmarklet cuts the selection the same way, so an address stays short.
export const LIMITS = { title: 300, text: 4000, url: 2048 };

const webAddress = /^https?:\/\/[^\s]+$/i;
const trailingAddress = /(?:^|\s)(https?:\/\/[^\s]+)\s*$/i;

// isWebAddress reports whether s is an http or https address.
export function isWebAddress(s) {
  if (!webAddress.test(s || "")) return false;
  try {
    const u = new URL(s);
    return (u.protocol === "http:" || u.protocol === "https:") && Boolean(u.host);
  } catch {
    return false;
  }
}

const cut = (s, n) => {
  const chars = [...String(s || "")];
  return chars.length > n ? chars.slice(0, n).join("") : chars.join("");
};

// readCapture takes what the address of the capture view holds: title,
// text and url, trimmed and bounded. An address at the end of the text
// becomes the url when none was given.
export function readCapture(params) {
  const get = (k) => (params && typeof params.get === "function" ? params.get(k) : params?.[k]) || "";
  let title = cut(get("title").trim(), LIMITS.title);
  let text = get("text").replace(/\r\n/g, "\n").trim();
  let url = get("url").trim();
  if (!isWebAddress(url)) url = "";
  if (!url) {
    const m = text.match(trailingAddress);
    if (m && isWebAddress(m[1])) {
      url = m[1];
      text = text.slice(0, m.index).trim();
    }
  }
  // A text that is only the title again says nothing more.
  if (text === title) text = "";
  return { title, text: cut(text, LIMITS.text), url: cut(url, LIMITS.url) };
}

// paragraphs splits a text into its paragraphs, at empty lines.
export function paragraphs(text) {
  return String(text || "").replace(/\r\n/g, "\n").split(/\n\s*\n/).map((p) => p.replace(/\s+/g, " ").trim()).filter(Boolean);
}

// captureBlocks are the blocks a capture adds to a body: a title without
// an address first, as a paragraph of its own; the text as quotes, one per
// paragraph; then, with an address, a paragraph that names where it comes
// from: sourceLabel ("Source:") and the title, or the address, linked to
// the address.
export function captureBlocks(capture, sourceLabel) {
  const blocks = [];
  if (capture.title && !capture.url) blocks.push({ kind: "paragraph", text: capture.title });
  blocks.push(...paragraphs(capture.text).map((text) => ({ kind: "quote", text })));
  if (capture.url) {
    const name = capture.title || capture.url;
    const before = sourceLabel ? sourceLabel + " " : "";
    const start = [...before].length;
    blocks.push({ kind: "paragraph", text: before + name, marks: [{ type: "link", start, end: start + [...name].length, href: capture.url }] });
  }
  return blocks;
}

// TARGETS are where a capture may go, in the order offered; each says
// whether it can take a capture. The view gives each its fields and its
// action (captureview.js). Another target, such as one's note of the day,
// is one more entry here and there.
export const TARGETS = [
  // A page of the web becomes a web link item (ADR-0206).
  { id: "weblink", takes: (c) => Boolean(c.url) },
  // Paragraphs added to one's daily note of today (ADR-0203).
  { id: "daily", takes: (c) => Boolean(c.title || c.text || c.url) },
  // A new item of a chosen type in a chosen room, the capture its text.
  { id: "item", takes: (c) => Boolean(c.title || c.text || c.url) },
  // The capture added at the end of an item found by searching.
  { id: "append", takes: (c) => Boolean(c.title || c.text || c.url) },
];

// targetsFor lists the targets that take a capture, the first being the
// one chosen at first.
export function targetsFor(capture, targets = TARGETS) {
  return targets.filter((tg) => tg.takes(capture));
}

// captureAddress is the address of the capture view for a capture.
export function captureAddress(capture) {
  const q = new URLSearchParams();
  for (const k of ["title", "text", "url"]) if (capture[k]) q.set(k, capture[k]);
  const s = q.toString();
  return "#/capture" + (s ? "?" + s : "");
}

// bookmarklet is the code of a bookmark that opens the capture view of
// origin in a small window, with the page's title, address and the text
// selected on it. It is a constant written here around origin, which is
// this site's own address, quoted as a JavaScript string.
export function bookmarklet(origin) {
  const target = JSON.stringify(String(origin).replace(/\/+$/, "") + "/#/capture?");
  return "javascript:(()=>{const q=new URLSearchParams({title:document.title.slice(0," + LIMITS.title + "),url:location.href,text:String(getSelection()).slice(0," + LIMITS.text + ")});" +
    "window.open(" + target + "+q,'orbithek-capture','width=640,height=720')})()";
}
