// pptx reads a presentation file (.pptx, .ppsx) in the browser for the
// album's slide import (ADR-0186): the order of its slides, which are
// hidden, each slide's title and its speaker's notes. A presentation file
// holds no pictures of its slides, so the pictures come from the slides'
// export as PNG or JPEG and are paired with the slides here by their
// numbers. Nothing of the file reaches the server but the pictures and
// the notes. It needs no browser — the XML is read by a small reader of
// its own — so node --test runs it.
import { entries as zipEntries, read as zipRead } from "./zipread.js";
import { baseName } from "./photoparts.js";

// isDeck reports whether a file is a presentation the import reads.
export const isDeck = (name) => /\.(pptx|ppsx)$/i.test(name || "");

const ENTITIES = { lt: "<", gt: ">", amp: "&", quot: "\"", apos: "'" };
const unescape = (s) => s.replace(/&(#x[0-9a-fA-F]+|#[0-9]+|[a-zA-Z]+);/g, (m, e) => {
  if (e[0] !== "#") return ENTITIES[e] ?? m;
  const n = e[1] === "x" || e[1] === "X" ? parseInt(e.slice(2), 16) : parseInt(e.slice(1), 10);
  return n > 0 && n <= 0x10ffff ? String.fromCodePoint(n) : m;
});
const localName = (q) => q.slice(q.indexOf(":") + 1);

// parseXML reads an XML document into a tree of elements: name without
// its prefix, attributes by their qualified names, and children, which
// are elements or strings. It reads what Office writes — elements,
// attributes, text, CDATA, comments, processing instructions — and
// refuses a document whose elements do not nest.
export function parseXML(text) {
  const root = { name: "#document", attrs: {}, children: [] };
  const stack = [root];
  const top = () => stack[stack.length - 1];
  const skip = (from, end) => {
    const at = text.indexOf(end, from);
    if (at < 0) throw new Error("xml: unterminated markup");
    return at + end.length;
  };
  let i = 0;
  while (i < text.length) {
    const lt = text.indexOf("<", i);
    if (lt < 0) {
      if (text.slice(i).trim()) top().children.push(unescape(text.slice(i)));
      break;
    }
    if (lt > i) top().children.push(unescape(text.slice(i, lt)));
    if (text.startsWith("<!--", lt)) { i = skip(lt, "-->"); continue; }
    if (text.startsWith("<![CDATA[", lt)) {
      i = skip(lt, "]]>");
      top().children.push(text.slice(lt + 9, i - 3));
      continue;
    }
    if (text.startsWith("<?", lt)) { i = skip(lt, "?>"); continue; }
    if (text.startsWith("<!", lt)) { i = skip(lt, ">"); continue; }
    let j = lt + 1;
    let quote = "";
    for (; j < text.length; j++) {
      const c = text[j];
      if (quote) { if (c === quote) quote = ""; } else if (c === "\"" || c === "'") quote = c;
      else if (c === ">") break;
    }
    if (j >= text.length) throw new Error("xml: unterminated tag");
    const body = text.slice(lt + 1, j);
    i = j + 1;
    if (body[0] === "/") {
      const name = localName(body.slice(1).trim());
      if (stack.length < 2 || top().name !== name) throw new Error("xml: elements do not nest");
      stack.pop();
      continue;
    }
    const closed = body.endsWith("/");
    const inner = closed ? body.slice(0, -1) : body;
    const qname = inner.match(/^[^\s/>]+/)?.[0];
    if (!qname) throw new Error("xml: element without a name");
    const attrs = {};
    for (const m of inner.slice(qname.length).matchAll(/([^\s=]+)\s*=\s*(?:"([^"]*)"|'([^']*)')/g)) {
      attrs[m[1]] = unescape(m[2] ?? m[3]);
    }
    const node = { name: localName(qname), attrs, children: [] };
    top().children.push(node);
    if (!closed) stack.push(node);
  }
  if (stack.length !== 1) throw new Error("xml: unclosed element");
  return root;
}

const kids = (node, name) => node.children.filter((c) => typeof c !== "string" && (!name || c.name === name));
const child = (node, name) => kids(node, name)[0] || null;
// all lists the elements of a name below a node, in document order.
function all(node, name, out = []) {
  for (const c of kids(node)) {
    if (c.name === name) out.push(c);
    all(c, name, out);
  }
  return out;
}
const rootOf = (doc) => kids(doc)[0] || null;
// attrOf reads an attribute by its local name; prefixed says whether the
// one with a prefix is meant, as r:id beside id on a slide's entry.
const attrOf = (node, name, prefixed = false) => {
  for (const [k, v] of Object.entries(node?.attrs || {})) {
    if (localName(k) === name && k.includes(":") === prefixed) return v;
  }
  return undefined;
};

// paragraphText is the text of a paragraph (a:p): its runs and fields,
// a line break as brk.
function paragraphText(p, brk) {
  let s = "";
  const walk = (n) => {
    for (const c of kids(n)) {
      if (c.name === "t") s += c.children.filter((x) => typeof x === "string").join("");
      else if (c.name === "br") s += brk;
      else walk(c);
    }
  };
  walk(p);
  return s;
}

// placeholderOf is the placeholder type of a shape (p:sp), "" when it is
// none; a placeholder without a type is a body or content placeholder.
function placeholderOf(sp) {
  const ph = all(child(sp, "nvSpPr") || { children: [] }, "ph")[0];
  if (!ph) return "";
  return attrOf(ph, "type") || "body";
}

// titleOf is the text of a slide's title placeholder, on one line.
export function titleOf(slide) {
  for (const sp of all(slide, "sp")) {
    if (!["title", "ctrTitle"].includes(placeholderOf(sp))) continue;
    const body = child(sp, "txBody");
    if (!body) continue;
    const text = kids(body, "p").map((p) => paragraphText(p, " ")).join(" ").replace(/\s+/g, " ").trim();
    if (text) return text;
  }
  return "";
}

// NOT_NOTES are the placeholders of a notes page that are not what the
// speaker wrote: the picture of the slide, its number, header, footer and
// date.
const NOT_NOTES = new Set(["sldImg", "sldNum", "hdr", "ftr", "dt", "title", "ctrTitle"]);

// notesOf is what a notes page holds: the text of its shapes other than
// the slide's picture, number, header, footer and date, a line per
// paragraph.
export function notesOf(notesSlide) {
  const out = [];
  for (const sp of all(notesSlide, "sp")) {
    if (NOT_NOTES.has(placeholderOf(sp))) continue;
    const body = child(sp, "txBody");
    if (!body) continue;
    const text = kids(body, "p").map((p) => paragraphText(p, "\n").replace(/[ \t]+$/gm, "")).join("\n").trim();
    if (text) out.push(text);
  }
  return out.join("\n\n").replace(/\n{3,}/g, "\n\n");
}

// relsPath is where the relationships of a part are kept.
const relsPath = (part) => {
  const at = part.lastIndexOf("/");
  return `${part.slice(0, at + 1)}_rels/${part.slice(at + 1)}.rels`;
};

// resolve finds the part a relationship of a part names: relative to the
// part's folder, or from the package's root when it starts with a slash.
export function resolve(part, target) {
  const segs = target.startsWith("/") ? [] : part.split("/").slice(0, -1);
  for (const s of target.replace(/^\/+/, "").split("/")) {
    if (s === "..") segs.pop();
    else if (s && s !== ".") segs.push(s);
  }
  return segs.join("/");
}

// relationships reads the relationships of a part: ID to type and part.
// Links outside the package are left out.
function relationships(doc, part) {
  const out = new Map();
  const root = rootOf(doc);
  for (const r of root ? kids(root, "Relationship") : []) {
    if (attrOf(r, "TargetMode") === "External" || !r.attrs.Id || !r.attrs.Target) continue;
    let target = r.attrs.Target;
    try { target = decodeURI(target); } catch { /* kept as written */ }
    out.set(r.attrs.Id, { type: r.attrs.Type || "", part: resolve(part, target) });
  }
  return out;
}
const isType = (rel, kind) => rel.type.endsWith("/" + kind);

// MAX_PART bounds the XML read of one part; slides and notes are far
// smaller, so a larger part is a damaged or hostile file.
export const MAX_PART = 16 << 20;

// readDeck reads a presentation: its slides in their order, each with its
// position (from 1), whether it is hidden, its title and its notes.
export async function readDeck(blob) {
  let list;
  try {
    list = await zipEntries(blob);
  } catch {
    throw new Error("pptx: not a presentation");
  }
  const byName = new Map();
  for (const e of list) {
    const name = e.name.replace(/^\/+/, "");
    byName.set(name, e);
    if (!byName.has(name.toLowerCase())) byName.set(name.toLowerCase(), e);
  }
  const xml = async (part, needed = true) => {
    const e = byName.get(part) || byName.get(part.toLowerCase());
    if (!e) {
      if (needed) throw new Error(`pptx: ${part} is missing`);
      return null;
    }
    if (e.size > MAX_PART) throw new Error(`pptx: ${part} is too large`);
    return parseXML(await (await zipRead(blob, e)).text());
  };
  const pkg = await xml("_rels/.rels", false);
  const main = (pkg && [...relationships(pkg, "").values()].find((r) => isType(r, "officeDocument"))?.part) || "ppt/presentation.xml";
  const presentation = rootOf(await xml(main));
  if (!presentation || presentation.name !== "presentation") throw new Error("pptx: not a presentation");
  const rels = relationships(await xml(relsPath(main)), main);
  const order = all(presentation, "sldIdLst")[0];
  const slides = [];
  for (const entry of order ? kids(order, "sldId") : []) {
    const rel = rels.get(attrOf(entry, "id", true));
    if (!rel || !isType(rel, "slide")) continue;
    const slide = rootOf(await xml(rel.part));
    if (!slide) continue;
    const own = await xml(relsPath(rel.part), false);
    const notesRel = own ? [...relationships(own, rel.part).values()].find((r) => isType(r, "notesSlide")) : null;
    const notesDoc = notesRel ? await xml(notesRel.part, false) : null;
    const show = attrOf(slide, "show");
    slides.push({
      number: slides.length + 1,
      hidden: show === "0" || show === "false",
      title: titleOf(slide),
      notes: notesDoc ? notesOf(notesDoc) : "",
    });
  }
  if (!slides.length) throw new Error("pptx: no slides");
  return { slides };
}

// numberOf is the slide number a picture's file name tells: the last
// number in it, as in Slide3.PNG or Folie12.png; null when it has none.
export function numberOf(name) {
  const stem = baseName(name).replace(/\.[^.]*$/, "");
  const m = stem.match(/(\d+)\D*$/);
  return m ? Number(m[1]) : null;
}

// pair matches the pictures of an export with the slides they show.
// Hidden slides are left out, as a slide show leaves them out. The
// pictures are matched by their numbers when they number the slides —
// every slide, or the shown ones by their positions — and in order when
// they number the shown slides one after the other, as an export that
// left the hidden ones out does. It gives the pairs in the slides' order
// and how many hidden slides were left out, or throws an error whose
// code is "mismatch" when the pictures fit neither way.
export function pair(slides, pictures) {
  const shown = slides.filter((s) => !s.hidden);
  const hidden = slides.length - shown.length;
  const numbered = pictures.map((p) => ({ picture: p, n: numberOf(p.name) }));
  const ns = numbered.map((x) => x.n);
  const unique = ns.every((n) => n !== null) && new Set(ns).size === ns.length;
  const fail = () => Object.assign(new Error("pptx: the pictures do not match the slides"),
    { code: "mismatch", slides: shown.length, pictures: pictures.length });
  if (!shown.length || !pictures.length) throw fail();
  // A single slide takes a single picture, whatever its name.
  if (shown.length === 1 && pictures.length === 1 && !unique) return { pairs: [{ slide: shown[0], picture: pictures[0] }], hidden };
  if (!unique) throw fail();
  const bySlide = new Map(numbered.map((x) => [x.n, x.picture]));
  const shownNumbers = shown.map((s) => s.number);
  const matchesShown = shownNumbers.every((n) => bySlide.has(n));
  if (pictures.length === slides.length && ns.every((n) => n >= 1 && n <= slides.length) && matchesShown) {
    return { pairs: shown.map((s) => ({ slide: s, picture: bySlide.get(s.number) })), hidden };
  }
  if (pictures.length === shown.length) {
    if (matchesShown) return { pairs: shown.map((s) => ({ slide: s, picture: bySlide.get(s.number) })), hidden };
    const sorted = [...numbered].sort((a, b) => a.n - b.n);
    if (sorted.every((x, i) => x.n === sorted[0].n + i)) {
      return { pairs: shown.map((s, i) => ({ slide: s, picture: sorted[i].picture })), hidden };
    }
  }
  throw fail();
}

// FILE_NAME is the most of a slide's title its picture's name takes.
const FILE_NAME = 80;

// clean makes a title fit a file name: no slashes, control characters or
// characters systems refuse in file names, on one line, not too long.
const clean = (s) => s.replace(/[\\/:*?"<>|\p{Cc}]/gu, " ").replace(/\s+/g, " ").trim()
  .slice(0, FILE_NAME).trim().replace(/[. ]+$/, "");

// slideName names a slide's picture so the album orders them as the
// slides were: its place in the deck, zero-padded to width, then its
// title, then the picture's own extension — "03 Our goals.png".
export function slideName(position, width, title, picture) {
  const ext = (picture.match(/\.([^./\\]+)$/)?.[1] || "png").toLowerCase();
  const n = String(position).padStart(width, "0");
  const t = clean(title || "");
  return `${n}${t ? " " + t : ""}.${ext === "jpeg" ? "jpg" : ext}`;
}

// widthOf is how many digits the positions of count slides take, two at
// least.
export const widthOf = (count) => Math.max(2, String(count).length);

// folderName names the folder a presentation's slides go into: its file
// name without the extension, else fallback.
export const folderName = (file, fallback) => clean(baseName(file).replace(/\.[^.]*$/, "")) || fallback;
