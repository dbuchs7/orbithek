// The rich-text editor (ADR-0032): ProseMirror with a schema that is exactly
// the item model's block document — paragraphs, headings (levels 1 to 3),
// bullet and ordered list items, quotes, code blocks and embeds; tasks,
// panels, tables, tables of contents and lists of child pages (ADR-0042);
// bold, italic, code, link and mention marks. Nothing outside the schema
// survives paste or input, so the editor cannot produce markup the model
// refuses.
import {
  Schema, EditorState, EditorView, Plugin, Decoration, DecorationSet, keymap, baseKeymap, toggleMark, setBlockType,
  splitBlockAs, chainCommands, history, undo, redo, inputRules, textblockTypeInputRule, TextSelection,
} from "../vendor/prosemirror.js";
import { h, safeHref } from "./dom.js";
import { t } from "./i18n.js";
import { anchorOf } from "./anchors.js";
import { openDrawing, drawingName, isDrawing } from "./drawing.js";
import { filenameOf } from "./filename.js";
import { diagramView, openModeler, pickProcess, bpmnName, drawingProcesses, processRegion } from "./bpmn.js";
import { regionsIn } from "./drawprocesses.js";
import { formView, decisionView, pickFromEngine, startFormView, pickStartLink, editPrefill } from "./engineviews.js";
import { architectureView, pickArchitecture } from "./architecture.js";
import { pickFromAlbum, pictureFile, captionOf, copiedPicture, clipMark } from "./album.js";
import { VIDEO_TYPES } from "./photoparts.js";
import { recorderControl, canRecordHere } from "./recorder.js";
import { AUDIO_TYPES } from "./recorderparts.js";
import { assist } from "./assist.js";
import { markdownToBlocks, pastesAsMarkdown } from "./markdown.js";
import {
  TONES as TABLE_TONES, MARKS as TABLE_MARKS, cellsOf, joinCells, normalMarks, normalLayout, colourOf, rangeOf, inRange, paint, formatCells,
  insertRow, insertColumn, removeRow, removeColumn, resized, filledClass, borderClass,
} from "./tableparts.js";
import { viewBlock, pickView, viewsReady } from "./itemview.js";

// Every block carries bid, its ID in a live session (ADR-0010); it is not
// stored. Text blocks keep the mentions they had without a mention mark
// (imported bodies), so saving does not drop their relations. secret names
// the protection of the whole block (ADR-0160); it is made and taken off
// from the item's page, and editing keeps it.
const bid = { bid: { default: null }, secret: { default: "" } };
const textAttrs = { ...bid, mentions: { default: [] } };
// Paragraphs, headings, quotes and panels can be aligned (ADR-0059). The
// alignment is a class, as the content security policy refuses style
// attributes; pasted text keeps an alignment it states.
const alignAttrs = { ...textAttrs, align: { default: "" } };
const alignClass = (n) => (n.attrs.align ? "pm-align-" + n.attrs.align : null);
const pastedAlign = (dom) => {
  const a = dom.style?.textAlign || dom.getAttribute("align") || "";
  return ["center", "right", "justify"].includes(a) ? a : "";
};

export const schema = new Schema({
  nodes: {
    doc: { content: "block+" },
    paragraph: { group: "block", content: "inline*", attrs: alignAttrs, parseDOM: [{ tag: "p", getAttrs: (dom) => ({ align: pastedAlign(dom) }) }],
      toDOM: (n) => ["p", { class: alignClass(n) }, 0] },
    heading: {
      group: "block", content: "inline*", defining: true,
      attrs: { ...alignAttrs, level: { default: 1 } },
      parseDOM: [1, 2, 3, 4, 5, 6].map((l) => ({ tag: "h" + l, getAttrs: (dom) => ({ level: Math.min(l, 3), align: pastedAlign(dom) }) })),
      toDOM: (n) => ["h" + (n.attrs.level + 1), { class: alignClass(n) }, 0], // h1 is the item's title
    },
    bullet: { group: "block", content: "inline*", attrs: textAttrs, parseDOM: [{ tag: "ul > li" }], toDOM: () => ["div", { class: "pm-bullet" }, 0] },
    ordered: { group: "block", content: "inline*", attrs: textAttrs, parseDOM: [{ tag: "ol > li" }], toDOM: () => ["div", { class: "pm-ordered" }, 0] },
    quote: { group: "block", content: "inline*", attrs: alignAttrs, parseDOM: [{ tag: "blockquote" }], toDOM: (n) => ["blockquote", { class: alignClass(n) }, 0] },
    code_block: { group: "block", content: "text*", marks: "", code: true, defining: true, attrs: bid, parseDOM: [{ tag: "pre", preserveWhitespace: "full" }], toDOM: () => ["pre", ["code", 0]] },
    // ADR-0042. A task's checkbox and a table's cells are drawn by node
    // views (createEditor); toDOM is what a copy carries.
    task: { group: "block", content: "inline*", attrs: { ...textAttrs, checked: { default: false } }, toDOM: (n) => ["div", { class: "pm-task", "data-checked": String(n.attrs.checked) }, 0] },
    panel: { group: "block", content: "inline*", attrs: { ...alignAttrs, tone: { default: "info" } }, toDOM: (n) => ["div", { class: ["pm-panel", alignClass(n)].filter(Boolean).join(" "), "data-tone": n.attrs.tone }, 0] },
    // A horizontal rule between blocks (ADR-0059).
    divider: { group: "block", atom: true, selectable: true, attrs: bid, parseDOM: [{ tag: "hr" }], toDOM: () => ["hr", { class: "pm-divider" }] },
    // A table's cells may be formatted, its columns and rows sized and its
    // cells coloured (ADR-0214).
    table: { group: "block", atom: true, selectable: true,
      attrs: { ...bid, text: { default: "\t\n\t" }, marks: { default: [] }, widths: { default: null }, heights: { default: null }, cells: { default: null } },
      toDOM: (n) => ["pre", { class: "pm-table" }, n.attrs.text] },
    toc: { group: "block", atom: true, selectable: true, attrs: bid, toDOM: () => ["div", { class: "pm-toc" }] },
    // Lists of the body's pictures and tables (ADR-0119), computed like a
    // table of contents.
    figures: { group: "block", atom: true, selectable: true, attrs: bid, toDOM: () => ["div", { class: "pm-figures" }] },
    tables: { group: "block", atom: true, selectable: true, attrs: bid, toDOM: () => ["div", { class: "pm-tables" }] },
    children: { group: "block", atom: true, selectable: true, attrs: bid, toDOM: () => ["div", { class: "pm-children" }] },
    // The tasks that carry a label, computed when shown (ADR-0114).
    tasklist: { group: "block", atom: true, selectable: true, attrs: { ...bid, label: { default: "" }, query: { default: "" } }, toDOM: (n) => ["div", { class: "pm-tasklist", "data-label": n.attrs.label, ...(n.attrs.query ? { "data-query": n.attrs.query } : {}) }] },
    // The items of a type as a table, a gallery or a list (ADR-0203),
    // computed when shown; the block keeps what it shows.
    itemview: { group: "block", atom: true, selectable: true, attrs: { ...bid, layout: { default: "table" }, itemType: { default: "" }, space: { default: "" }, query: { default: "" },
      group: { default: "" }, columns: { default: null }, sort: { default: "" } }, toDOM: (n) => ["div", { class: "pm-itemview", "data-layout": n.attrs.layout }] },
    // An image attachment of the item (ADR-0045), with a plain caption.
    // width is percent of the column, 0 for the picture's own size (ADR-0056).
    // align is left or right (the text flows beside it) or center (ADR-0059).
    image: { group: "block", atom: true, selectable: true, attrs: { ...bid, ref: {}, text: { default: "" }, width: { default: 0 }, align: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-image" }, ["figcaption", n.attrs.text]] },
    // A video plays in the text, sized and placed like a picture (ADR-0097).
    video: { group: "block", atom: true, selectable: true, attrs: { ...bid, ref: {}, text: { default: "" }, width: { default: 0 }, align: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-image pm-video" }, ["figcaption", n.attrs.text]] },
    // A sound plays in the text with a plain caption, such as a recording
    // (ADR-0223); it has neither a width nor a place.
    audio: { group: "block", atom: true, selectable: true, attrs: { ...bid, ref: {}, text: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-audio" }, ["figcaption", n.attrs.text]] },
    // A BPMN diagram (ADR-0129): a model kept as a file of the item, or the
    // newest version of a process of a process engine; with a plain caption.
    // view is what a diagram shows: "" the diagram, "both" the diagram and
    // its description, "text" the description only (ADR-0143).
    bpmn: { group: "block", atom: true, selectable: true, attrs: { ...bid, ref: {}, text: { default: "" }, view: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    process: { group: "block", atom: true, selectable: true, attrs: { ...bid, engine: {}, process: {}, text: { default: "" }, view: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    // A form as a process engine keeps it, and the newest deployed version
    // of a decision of a process engine (ADR-0144); with a plain caption.
    // A form may name the process whose steps name it, and its view is ""
    // the form, "both" the form and its fields, "text" the fields only
    // (ADR-0148).
    form: { group: "block", atom: true, selectable: true, attrs: { ...bid, engine: {}, form: {}, process: { default: "" }, view: { default: "" }, text: { default: "" } },
      toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    // The form of a public start link of a process engine, which starts
    // the link's process; prefill names the fields filled in beforehand
    // (ADR-0148). With a plain caption.
    startform: { group: "block", atom: true, selectable: true, attrs: { ...bid, engine: {}, link: {}, process: { default: "" }, prefill: { default: [] }, text: { default: "" } },
      toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    decision: { group: "block", atom: true, selectable: true, attrs: { ...bid, engine: {}, decision: {}, text: { default: "" } }, toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    // The architecture sketch of a release of an engine's application
    // (ADR-0146): release 0 follows the newest, layers none is the
    // engine's default; with a plain caption.
    architecture: { group: "block", atom: true, selectable: true, attrs: { ...bid, engine: {}, application: {}, release: { default: 0 }, layers: { default: [] }, text: { default: "" } },
      toDOM: (n) => ["figure", { class: "pm-diagram" }, ["figcaption", n.attrs.text]] },
    // Content an importer could not convert (ADR-0023), kept as it came and
    // shown as text; it cannot be created in the editor.
    unsupported: { group: "block", content: "text*", marks: "", code: true, atom: true, attrs: bid, toDOM: () => ["pre", { class: "pm-unsupported", title: t("editor.unsupported") }, 0] },
    embed: {
      group: "block", atom: true, selectable: true,
      attrs: { ...bid, ref: {}, label: { default: "" }, href: { default: "" } },
      toDOM: (n) => ["div", { class: "pm-embed", contenteditable: "false" },
        n.attrs.href ? ["a", { href: safeHref(n.attrs.href) }, n.attrs.label] : ["span", { class: "muted" }, n.attrs.label || t("editor.hidden")]],
    },
    // A protected block the reader may not read (ADR-0160): one dark bar
    // in place of every block of the protection, drawn by a node view.
    veiled: { group: "block", atom: true, selectable: true, attrs: bid,
      toDOM: (n) => ["div", { class: "pm-veiled", "data-secret": n.attrs.secret }, ["span", { class: "pm-veiled-bar" }, "▒▒▒▒▒▒▒▒"]] },
    mention: {
      group: "inline", inline: true, atom: true, selectable: false,
      attrs: { ref: {}, label: {} },
      toDOM: (n) => ["a", { class: "pm-mention", href: "#/item/" + encodeURIComponent(n.attrs.ref) }, n.attrs.label],
    },
    text: { group: "inline" },
  },
  marks: {
    bold: { parseDOM: [{ tag: "strong" }, { tag: "b" }], toDOM: () => ["strong", 0] },
    italic: { parseDOM: [{ tag: "em" }, { tag: "i" }], toDOM: () => ["em", 0] },
    code: { parseDOM: [{ tag: "code" }], toDOM: () => ["code", 0] },
    // ADR-0059.
    underline: { parseDOM: [{ tag: "u" }, { style: "text-decoration=underline" }], toDOM: () => ["u", 0] },
    strike: { parseDOM: [{ tag: "s" }, { tag: "del" }, { tag: "strike" }, { style: "text-decoration=line-through" }], toDOM: () => ["s", 0] },
    highlight: { parseDOM: [{ tag: "mark" }], toDOM: () => ["mark", 0] },
    link: {
      attrs: { href: {} }, inclusive: false,
      // Pasted links keep only targets the model allows (ADR-0014).
      parseDOM: [{ tag: "a[href]", getAttrs: (dom) => (isSafeLink(dom.getAttribute("href")) ? { href: dom.getAttribute("href") } : false) }],
      toDOM: (m) => ["a", { href: safeHref(m.attrs.href), rel: "noopener noreferrer nofollow", target: "_blank" }, 0],
    },
    // A protected passage (ADR-0088): ref names its protection. It is made
    // and taken off from the item's page, not typed; text typed at its end
    // stays outside it.
    secret: {
      attrs: { ref: {} }, inclusive: false, excludes: "",
      parseDOM: [{ tag: "span[data-secret]", getAttrs: (dom) => ({ ref: dom.getAttribute("data-secret") }) }],
      // A veiled one is a button that asks for access (ADR-0161).
      toDOM: (m) => {
        const cls = secretClass(m.attrs.ref);
        return ["span", { class: "pm-secret " + cls, "data-secret": m.attrs.ref, tabindex: "0", role: "button",
          "aria-label": cls === "secret-veiled" ? t("accessRequest.askLabel") : null }, 0];
      },
    },
  },
});

// secretClass says how a protected passage shows: readable or veiled for
// the reader (ADR-0088). The item's page sets it before drawing its body.
let secretClass = () => "";
export function setSecretClass(fn) { secretClass = fn || (() => ""); }

// veiledContent draws what a veiled block says beside its bar: who decides
// and how to ask (ADR-0160). The item's page sets it with setSecretClass.
let veiledContent = () => h("span", { class: "pm-veiled-text", text: t("secret.veiledBlock") });
export function setVeiledContent(fn) { veiledContent = fn || (() => h("span", { class: "pm-veiled-text", text: t("secret.veiledBlock") })); }

// blockRangeOf returns the blocks of the body a selection touches, as
// { from, to } indexes of the model's blocks, or null for none.
export function blockRangeOf(state) {
  const { from, to } = state.selection;
  let first = -1, last = -1;
  state.doc.forEach((node, offset, i) => {
    const end = offset + node.nodeSize;
    if (from < end && to > offset) {
      if (first < 0) first = i;
      last = i;
    } else if (from === to && from >= offset && from <= end && first < 0) {
      first = last = i;
    }
  });
  return first < 0 ? null : { from: first, to: last };
}

// selectionInBlock returns the selected text as a block of the body and
// characters of its text, as the model counts them, or null when the
// selection is empty or leaves one block of text.
export function selectionInBlock(state) {
  const { $from, $to, empty } = state.selection;
  if (empty || $from.depth !== 1 || $to.depth !== 1 || $from.index(0) !== $to.index(0) || !$from.parent.isTextblock) return null;
  const block = $from.parent;
  const count = (to) => cp(block.textBetween(0, to, "", (leaf) => leaf.attrs.label || ""));
  return { block: $from.index(0), start: count($from.parentOffset), end: count($to.parentOffset) };
}

export function isSafeLink(href) {
  try {
    const u = new URL(href);
    return (u.protocol === "http:" || u.protocol === "https:") ? u.host !== "" : u.protocol === "mailto:" && u.pathname !== "";
  } catch {
    return false;
  }
}

// Model block kinds and editor node names differ only for code, because a
// ProseMirror node and mark cannot share a name.
const nodeOf = (kind) => schema.nodes[kind === "code" ? "code_block" : kind] || schema.nodes.paragraph;
const kindOf = (node) => (node.type.name === "code_block" ? "code" : node.type.name);

// Offsets in the model count Unicode code points; JavaScript strings count
// UTF-16 units.
const cp = (s) => Array.from(s).length;

// --- model blocks -> editor document ----------------------------------------

export function docFromBlocks(blocks, embeds = []) {
  const nodes = blocks.map((b) => nodeFromBlock(b, embeds));
  if (nodes.length === 0) nodes.push(schema.nodes.paragraph.create());
  return schema.nodes.doc.create(null, nodes);
}

// nodeFromBlock makes the editor node of one model block; b.id becomes
// its session ID.
export function nodeFromBlock(b, embeds = []) {
  // A protected block the reader may not read is one veiled bar
  // (ADR-0160); the server sends it as a placeholder paragraph.
  if (b.secret && secretClass(b.secret) === "secret-veiled") return schema.nodes.veiled.create({ bid: b.id ?? null, secret: b.secret });
  const node = plainNode(b, embeds);
  return b.secret ? node.type.create({ ...node.attrs, secret: b.secret }, node.content, node.marks) : node;
}

function plainNode(b, embeds) {
  const id = b.id ?? null;
  if (b.kind === "table") {
    return schema.nodes.table.create({ bid: id, text: b.text || "", marks: (b.marks || []).filter((m) => TABLE_MARKS.includes(m.type)),
      widths: b.widths || null, heights: b.heights || null, cells: b.cells || null });
  }
  if (["toc", "figures", "tables", "children", "divider"].includes(b.kind)) return schema.nodes[b.kind].create({ bid: id });
  if (b.kind === "tasklist") return schema.nodes.tasklist.create({ bid: id, label: b.text || "", query: b.query || "" });
  if (b.kind === "view") return schema.nodes.itemview.create({ bid: id, layout: b.layout || "table", itemType: b.itemType || "", space: b.space || "", query: b.query || "",
    group: b.group || "", columns: b.columns?.length ? [...b.columns] : null, sort: b.sort || "" });
  if (b.kind === "image" || b.kind === "video") return schema.nodes[b.kind].create({ bid: id, ref: b.ref, text: b.text || "", width: b.width || 0, align: b.align || "" });
  if (b.kind === "bpmn") return schema.nodes.bpmn.create({ bid: id, ref: b.ref, text: b.text || "", view: b.view || "" });
  if (b.kind === "audio") return schema.nodes.audio.create({ bid: id, ref: b.ref, text: b.text || "" });
  if (b.kind === "process") return schema.nodes.process.create({ bid: id, engine: b.engine, process: b.process, text: b.text || "", view: b.view || "" });
  if (b.kind === "form") return schema.nodes.form.create({ bid: id, engine: b.engine, form: b.form, process: b.process || "", view: b.view || "", text: b.text || "" });
  if (b.kind === "startform") {
    return schema.nodes.startform.create({ bid: id, engine: b.engine, link: b.link, process: b.process || "", prefill: (b.prefill || []).map((p) => ({ field: p.field, from: p.from })), text: b.text || "" });
  }
  if (b.kind === "decision") return schema.nodes.decision.create({ bid: id, engine: b.engine, decision: b.decision, text: b.text || "" });
  if (b.kind === "architecture") return schema.nodes.architecture.create({ bid: id, engine: b.engine, application: b.application, release: b.release || 0, layers: b.layers || [], text: b.text || "" });
  if (b.kind === "embed") {
    const v = embeds.find((e) => e.ref === b.ref);
    return schema.nodes.embed.create({
      bid: id,
      ref: b.ref,
      label: v && !v.restricted ? `${v.key} ${v.title}` : "",
      href: v && !v.restricted ? "#/item/" + encodeURIComponent(v.key) : "",
    });
  }
  const type = nodeOf(b.kind);
  const plain = type === schema.nodes.code_block || type === schema.nodes.unsupported;
  const attrs = plain ? { bid: id } : { bid: id, mentions: b.mentions || [] };
  if (type === schema.nodes.heading) attrs.level = b.level || 1;
  if (type === schema.nodes.task) attrs.checked = Boolean(b.checked);
  if (type === schema.nodes.panel) attrs.tone = b.tone || "info";
  if (type.spec.attrs?.align) attrs.align = b.align || "";
  return type.create(attrs, inlineContent(b.text || "", plain ? [] : b.marks || []));
}

function inlineContent(text, marks) {
  const chars = Array.from(text);
  const cuts = new Set([0, chars.length]);
  for (const m of marks) cuts.add(m.start).add(m.end);
  const points = [...cuts].sort((a, b) => a - b);
  const out = [];
  let i = 0;
  while (i < chars.length) {
    const mention = marks.find((m) => m.type === "mention" && m.start === i);
    if (mention) {
      out.push(schema.nodes.mention.create({ ref: mention.ref, label: chars.slice(mention.start, mention.end).join("") }));
      i = mention.end;
      continue;
    }
    const next = points.find((p) => p > i) ?? chars.length;
    const pm = marks
      .filter((m) => m.type !== "mention" && m.start <= i && m.end >= next)
      .filter((m) => schema.marks[m.type])
      .map((m) => (m.type === "link" ? schema.marks.link.create({ href: m.href })
        : m.type === "secret" ? schema.marks.secret.create({ ref: m.ref }) : schema.marks[m.type].create()));
    out.push(schema.text(chars.slice(i, next).join(""), pm));
    i = next;
  }
  return out;
}

// --- editor document -> model blocks ----------------------------------------

export function blocksFromDoc(doc) {
  const blocks = [];
  doc.forEach((node) => blocks.push(blockFromNode(node)));
  return blocks;
}

// blockFromNode is the model block of one editor node.
export function blockFromNode(node) {
  const b = plainBlock(node);
  if (node.attrs?.secret) b.secret = node.attrs.secret;
  return b;
}

function plainBlock(node) {
  if (node.type === schema.nodes.veiled) return { kind: "paragraph", text: "▒▒▒▒▒▒▒▒" };
  if (node.type === schema.nodes.embed) return { kind: "embed", ref: node.attrs.ref };
  if (node.type === schema.nodes.table) {
    const a = node.attrs;
    return { kind: "table", text: a.text, ...(a.marks?.length ? { marks: a.marks } : {}), ...(a.widths ? { widths: a.widths } : {}),
      ...(a.heights ? { heights: a.heights } : {}), ...(a.cells ? { cells: a.cells } : {}) };
  }
  if (["toc", "figures", "tables", "children", "divider"].includes(node.type.name)) return { kind: node.type.name };
  if (node.type === schema.nodes.tasklist) return node.attrs.query ? { kind: "tasklist", query: node.attrs.query } : { kind: "tasklist", text: node.attrs.label };
  if (node.type === schema.nodes.itemview) {
    const a = node.attrs;
    const b = { kind: "view", layout: a.layout };
    for (const k of ["itemType", "space", "query", "group", "sort"]) if (a[k]) b[k] = a[k];
    if (a.columns?.length) b.columns = [...a.columns];
    return b;
  }
  if (node.type === schema.nodes.bpmn) return { kind: "bpmn", text: node.attrs.text, ref: node.attrs.ref, ...(node.attrs.view ? { view: node.attrs.view } : {}) };
  if (node.type === schema.nodes.process) return { kind: "process", text: node.attrs.text, engine: node.attrs.engine, process: node.attrs.process, ...(node.attrs.view ? { view: node.attrs.view } : {}) };
  if (node.type === schema.nodes.form) {
    const a = node.attrs;
    return { kind: "form", text: a.text, engine: a.engine, form: a.form, ...(a.process ? { process: a.process } : {}), ...(a.view ? { view: a.view } : {}) };
  }
  if (node.type === schema.nodes.startform) {
    const a = node.attrs;
    return { kind: "startform", text: a.text, engine: a.engine, link: a.link, ...(a.process ? { process: a.process } : {}),
      ...(a.prefill?.length ? { prefill: a.prefill.map((p) => ({ field: p.field, from: p.from })) } : {}) };
  }
  if (node.type === schema.nodes.decision) return { kind: "decision", text: node.attrs.text, engine: node.attrs.engine, decision: node.attrs.decision };
  if (node.type === schema.nodes.architecture) {
    const a = node.attrs;
    return { kind: "architecture", text: a.text, engine: a.engine, application: a.application, ...(a.release ? { release: a.release } : {}), ...(a.layers?.length ? { layers: [...a.layers] } : {}) };
  }
  if (node.type === schema.nodes.audio) return { kind: "audio", text: node.attrs.text, ref: node.attrs.ref };
  if (node.type === schema.nodes.image || node.type === schema.nodes.video) {
    const b = { kind: node.type.name, text: node.attrs.text, ref: node.attrs.ref };
    if (node.attrs.width) b.width = node.attrs.width;
    if (node.attrs.align) b.align = node.attrs.align;
    return b;
  }
  let text = "";
  const marks = [];
  const mentions = new Set((node.attrs && node.attrs.mentions) || []);
  node.forEach((child) => {
    const start = cp(text);
    if (child.type === schema.nodes.mention) {
      text += child.attrs.label;
      marks.push({ type: "mention", start, end: cp(text), ref: child.attrs.ref });
      mentions.add(child.attrs.ref);
      return;
    }
    text += child.text;
    for (const m of child.marks) {
      const mark = { type: m.type.name, start, end: cp(text) };
      if (m.type.name === "link") mark.href = m.attrs.href;
      if (m.type.name === "secret") mark.ref = m.attrs.ref;
      const prev = marks.findLast((x) => x.type === mark.type && x.href === mark.href && x.ref === mark.ref && x.end === start);
      if (prev) prev.end = mark.end; // one mark across adjacent text nodes
      else marks.push(mark);
    }
  });
  const b = { kind: kindOf(node), text };
  if (node.type === schema.nodes.heading) b.level = node.attrs.level;
  if (marks.length) b.marks = marks;
  if (mentions.size) b.mentions = [...mentions];
  if (node.type === schema.nodes.task && node.attrs.checked) b.checked = true;
  if (node.type === schema.nodes.panel) b.tone = node.attrs.tone;
  if (node.attrs.align) b.align = node.attrs.align;
  return b;
}

// --- Markdown (ADR-0133) ----------------------------------------------------------

// markdownInsertion converts Markdown into blocks of the body (markdown.js)
// and returns the transaction that puts them at the selection as a paste
// does, replacing what is selected: { tr, pictures, dropped }, the last two
// counting what was simplified. One paragraph goes into the text at the
// cursor; other blocks split the block there or follow it. Without a
// transaction, reason says why: "empty" (nothing to insert), "protected"
// (the selection touches a protected passage, ADR-0088) or "code" (the
// cursor is in a code block, where a paste stays text unless explicit).
export function markdownInsertion(state, text, explicit = false) {
  const { $from, from, to } = state.selection;
  const secret = schema.marks.secret;
  if (state.doc.rangeHasMark(from, to, secret) || secret.isInSet($from.marks())) return { reason: "protected" };
  const inCode = Boolean($from.parent.type.spec.code);
  if (inCode && !explicit) return { reason: "code" };
  const { blocks, pictures, dropped } = markdownToBlocks(text, isSafeLink);
  if (blocks.length === 0) return { reason: "empty" };
  const doc = docFromBlocks(blocks);
  const inline = blocks.length === 1 && blocks[0].kind === "paragraph" && $from.parent.isTextblock && !inCode;
  const slice = inline ? doc.slice(1, doc.content.size - 1) : doc.slice(0, doc.content.size);
  const tr = state.tr.replaceSelection(slice).scrollIntoView().setMeta("paste", true).setMeta("uiEvent", "paste");
  return { tr, pictures, dropped };
}

// --- the editor -----------------------------------------------------------------

// Blocks that continue on Enter: list items, tasks (a new one unchecked)
// and panels (in the same tone).
const continued = () => [schema.nodes.bullet, schema.nodes.ordered, schema.nodes.task, schema.nodes.panel];

const listItem = (state, dispatch) => {
  const { $from, empty } = state.selection;
  const node = $from.parent;
  if (!empty || !continued().includes(node.type)) return false;
  // Enter on an empty item ends the list; otherwise the next item keeps the kind.
  if (node.content.size === 0) return setBlockType(schema.nodes.paragraph)(state, dispatch);
  return splitBlockAs((n) => ({ type: n.type, attrs: n.type === schema.nodes.panel ? { tone: n.attrs.tone } : {} }))(state, dispatch);
};

function plugins() {
  const block = (type, attrs) => (re) => textblockTypeInputRule(re, type, attrs);
  return [
    inputRules({
      rules: [
        textblockTypeInputRule(/^(#{1,3})\s$/, schema.nodes.heading, (m) => ({ level: m[1].length })),
        block(schema.nodes.bullet)(/^[-*]\s$/),
        block(schema.nodes.ordered)(/^1[.)]\s$/),
        block(schema.nodes.quote)(/^>\s$/),
        block(schema.nodes.code_block)(/^```$/),
        block(schema.nodes.task)(/^\[\s?\]\s$/),
        block(schema.nodes.task, { checked: true })(/^\[x\]\s$/i),
      ],
    }),
    history(),
    keepSecrets,
    secretBlocks,
    keymap({
      "Mod-z": undo, "Mod-y": redo, "Shift-Mod-z": redo,
      "Mod-b": toggleMark(schema.marks.bold), "Mod-i": toggleMark(schema.marks.italic), "Mod-`": toggleMark(schema.marks.code),
      "Mod-u": toggleMark(schema.marks.underline), "Mod-Shift-x": toggleMark(schema.marks.strike), "Mod-Shift-h": toggleMark(schema.marks.highlight),
      "Mod-Shift-l": setAlign(""), "Mod-Shift-e": setAlign("center"), "Mod-Shift-r": setAlign("right"), "Mod-Shift-j": setAlign("justify"),
      Enter: chainCommands(listItem, baseKeymap.Enter),
    }),
    keymap(baseKeymap),
  ];
}

// keepSecrets keeps a block's protection when its kind changes (a heading
// made a paragraph, "# " typed at its start): ProseMirror makes a new
// node then, with its own attributes (ADR-0160).
const keepSecrets = new Plugin({
  appendTransaction(trs, before, after) {
    if (!trs.some((tr) => tr.steps.some((st) => "gapFrom" in st)) || before.doc.childCount !== after.doc.childCount) return null;
    let tr = null;
    after.doc.forEach((node, offset, i) => {
      const was = before.doc.child(i).attrs.secret;
      if (was && "secret" in node.attrs && !node.attrs.secret) tr = (tr || after.tr).setNodeMarkup(offset, null, { ...node.attrs, secret: was });
    });
    return tr;
  },
});

// secretBlocks marks the protected blocks a reader reads (ADR-0160).
const secretBlocks = new Plugin({
  props: {
    decorations: (state) => {
      const decos = [];
      state.doc.forEach((node, offset) => {
        if (node.attrs.secret && node.type !== schema.nodes.veiled) decos.push(Decoration.node(offset, offset + node.nodeSize, { class: "pm-block-secret", "data-secret": node.attrs.secret }));
      });
      return DecorationSet.create(state.doc, decos);
    },
  },
});

// setAlign aligns the blocks of the selection that can be aligned
// (ADR-0059); "" is the default, left.
export function setAlign(align) {
  return (state, dispatch) => {
    const { from, to } = state.selection;
    const tr = state.tr;
    state.doc.nodesBetween(from, to, (node, pos) => {
      if (node.isTextblock && "align" in node.attrs && node.attrs.align !== align) tr.setNodeMarkup(pos, null, { ...node.attrs, align });
      return !node.isTextblock;
    });
    if (!tr.docChanged) return false;
    if (dispatch) dispatch(tr);
    return true;
  };
}

// alignOf is the alignment of the block the selection starts in, or null
// for a block that cannot be aligned.
function alignOf(state) {
  const node = state.selection.$from.parent;
  return node.isTextblock && "align" in node.attrs ? node.attrs.align : null;
}

// markActive reports whether a mark applies at the selection.
function markActive(state, type) {
  const { from, $from, to, empty } = state.selection;
  if (empty) return Boolean(type.isInSet(state.storedMarks || $from.marks()));
  return state.doc.rangeHasMark(from, to, type);
}

// --- node views (ADR-0042) -----------------------------------------------------

// A node view's own controls are not editor content: the editor neither
// handles their events nor reads their changes as edits. A click elsewhere
// on the block selects it, like any block, so it can be cut, moved or
// deleted.
const selectable = { stopEvent: (ev) => Boolean(ev.target.closest?.("a, button, input, select")), ignoreMutation: () => true };

// taskView draws a task's checkbox; ticking it changes the task.
function taskView(node, view, getPos) {
  const box = h("input", { type: "checkbox", "aria-label": t("editor.taskDone") });
  const text = h("span", { class: "pm-task-text" });
  const dom = h("div", { class: "pm-task" }, h("span", { class: "pm-task-box", contenteditable: "false" }, box), text);
  const show = () => {
    box.checked = node.attrs.checked;
    box.disabled = !view.editable;
    dom.dataset.checked = String(node.attrs.checked);
  };
  box.addEventListener("change", () => {
    const pos = getPos();
    if (pos !== undefined) view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, checked: box.checked }));
    show(); // a refused change (someone else holds the task) shows as it is
  });
  show();
  return {
    dom, contentDOM: text,
    update: (n) => { if (n.type !== node.type) return false; node = n; show(); return true; },
    stopEvent: (ev) => ev.target === box,
    ignoreMutation: (m) => m.type === "attributes" && (m.target === box || m.target === dom),
  };
}

// --- tables (ADR-0042, ADR-0214) ------------------------------------------------

// A cell is edited in an editor of its own, one at a time, whose schema
// holds text with the formats a cell may carry.
const cellSchema = new Schema({
  nodes: { doc: { content: "text*" }, text: {} },
  marks: Object.fromEntries(TABLE_MARKS.map((m) => [m, schema.spec.marks.get(m)])),
});
let cellAids = null;

// toneName names a tone of a coloured cell, as a kanban names its tones.
const toneName = (x) => ({ blue: t("tone.blue"), green: t("tone.green"), yellow: t("tone.yellow"), orange: t("tone.orange"),
  red: t("tone.red"), purple: t("tone.purple"), teal: t("tone.teal"), gray: t("tone.gray") })[x] || x;

// cellDoc is a cell's editor document; cellOf reads a cell back.
function cellDoc(cell) {
  const chars = Array.from(cell.text);
  const cuts = [...new Set([0, chars.length, ...cell.marks.flatMap((m) => [m.start, m.end])])].sort((x, y) => x - y);
  const nodes = [];
  for (let i = 0; i + 1 < cuts.length; i++) {
    if (cuts[i] >= cuts[i + 1]) continue;
    const marks = cell.marks.filter((m) => m.start <= cuts[i] && m.end >= cuts[i + 1] && cellSchema.marks[m.type])
      .map((m) => (m.type === "link" ? cellSchema.marks.link.create({ href: m.href }) : cellSchema.marks[m.type].create()));
    nodes.push(cellSchema.text(chars.slice(cuts[i], cuts[i + 1]).join(""), marks));
  }
  return cellSchema.nodes.doc.create(null, nodes);
}

function cellOf(doc) {
  let text = "";
  const marks = [];
  doc.forEach((n) => {
    const start = cp(text);
    text += n.text;
    for (const m of n.marks) marks.push({ type: m.type.name, start, end: cp(text), ...(m.type.name === "link" ? { href: m.attrs.href } : {}) });
  });
  return { text, marks: normalMarks(marks) };
}

// cellContent draws a cell's text with its formats, for reading.
function cellContent(cell) {
  const chars = Array.from(cell.text);
  const cuts = [...new Set([0, chars.length, ...cell.marks.flatMap((m) => [m.start, m.end])])].sort((x, y) => x - y);
  const out = [];
  for (let i = 0; i + 1 < cuts.length; i++) {
    if (cuts[i] >= cuts[i + 1]) continue;
    let el = chars.slice(cuts[i], cuts[i + 1]).join("");
    for (const m of cell.marks.filter((x) => x.start <= cuts[i] && x.end >= cuts[i + 1])) {
      const tag = { bold: "strong", italic: "em", underline: "u", strike: "s", highlight: "mark", code: "code" }[m.type];
      if (tag) el = h(tag, {}, el);
      else if (m.type === "link" && isSafeLink(m.href)) el = h("a", { href: safeHref(m.href), rel: "noopener noreferrer nofollow", target: "_blank" }, el);
    }
    out.push(el);
  }
  return out;
}

// tableView draws a table: its cells with their formats, its column
// widths and row heights and its coloured cells; the text wraps within
// a cell. An editor edits a cell in place, formats it, chooses several
// cells with Shift and a click to colour or format them at once, drags a
// cell's right or lower edge to size its column or row, and adds and
// removes rows and columns at the chosen cell.
function tableView(node, view, getPos) {
  const dom = h("div", { class: "pm-table", contenteditable: "false" });
  let shown = null;
  let tds = [];
  let anchor = null, head = null; // the chosen cells, { r, c }
  let active = null; // the cell being edited: { r, c, view }
  let tools = null;
  const locked = () => dom.classList.contains("pm-locked");
  const editable = () => view.editable && !locked();
  const keyOf = (a) => JSON.stringify([a.text, a.marks, a.widths, a.heights, a.cells, view.editable]);
  const read = () => ({ rows: cellsOf(node.attrs.text, node.attrs.marks),
    layout: { widths: node.attrs.widths, heights: node.attrs.heights, cells: node.attrs.cells } });
  const range = () => (anchor ? rangeOf(anchor, head || anchor) : null);
  // commit writes rows and layout into the block; keep leaves the cell
  // being edited as it is, since it shows the change already.
  const commit = (rows, layout, keep) => {
    const pos = getPos();
    if (pos === undefined || locked()) { render(true); return; }
    const { text, marks } = joinCells(rows);
    const l = normalLayout(layout, rows.length, rows[0].length);
    const attrs = { ...node.attrs, text, marks, widths: l.widths, heights: l.heights, cells: l.cells };
    if (keep) shown = keyOf(attrs);
    view.dispatch(view.state.tr.setNodeMarkup(pos, null, attrs));
  };
  const close = () => {
    if (!active) return;
    const { r, c, view: inner } = active;
    active = null;
    inner.destroy();
    const td = tds[r]?.[c];
    if (td) td.querySelector(".pm-cell-text").replaceChildren(...cellContent(read().rows[r][c]));
  };
  const choose = (r, c, extend) => {
    if (extend && anchor) head = { r, c };
    else { anchor = { r, c }; head = null; }
    const rg = range();
    tds.forEach((row, i) => row.forEach((td, j) => td.classList.toggle("pm-cell-chosen", inRange(rg, i, j) && Boolean(head))));
    showTools();
  };
  // move edits the cell dr rows and dc columns from the one being edited,
  // across the end of a row to the next.
  const move = (dr, dc) => {
    if (!active) return false;
    const rows = read().rows;
    let r = active.r + dr, c = active.c + dc;
    if (c >= rows[0].length) { c = 0; r++; }
    if (c < 0) { c = rows[0].length - 1; r--; }
    if (r < 0 || r >= rows.length) return true;
    open(r, c);
    return true;
  };
  const format = (type) => () => {
    if (!editable()) return;
    if (active && !head) {
      const inner = active.view;
      if (type === "link") { (cellAids ||= assist(cellSchema, isSafeLink)).openLink(inner); return; }
      toggleMark(cellSchema.marks[type])(inner.state, inner.dispatch);
      inner.focus();
      return;
    }
    const rg = range();
    if (!rg || type === "link") return;
    close();
    const { rows, layout } = read();
    commit(formatCells(rows, rg, type), layout);
  };
  const open = (r, c, ev) => {
    if (active && active.r === r && active.c === c) return;
    close();
    choose(r, c, false);
    if (!editable()) return;
    const holder = tds[r][c].querySelector(".pm-cell-text");
    holder.replaceChildren();
    const keys = {
      "Mod-z": undo, "Mod-y": redo, "Shift-Mod-z": redo,
      "Mod-b": toggleMark(cellSchema.marks.bold), "Mod-i": toggleMark(cellSchema.marks.italic), "Mod-`": toggleMark(cellSchema.marks.code),
      "Mod-u": toggleMark(cellSchema.marks.underline), "Mod-Shift-x": toggleMark(cellSchema.marks.strike), "Mod-Shift-h": toggleMark(cellSchema.marks.highlight),
      "Mod-k": () => { format("link")(); return true; },
      Tab: () => move(0, 1), "Shift-Tab": () => move(0, -1), Enter: () => move(1, 0), "Shift-Enter": () => true,
      Escape: () => { close(); view.focus(); return true; },
    };
    const inner = new EditorView(holder, {
      state: EditorState.create({ doc: cellDoc(read().rows[r][c]), plugins: [history(), keymap(keys), keymap(baseKeymap)] }),
      editable,
      attributes: { class: "pm-cell-editor", "aria-label": t("editor.cell", { row: r + 1, col: c + 1 }) },
      // Tabs and line breaks would split the cell; pasted text has spaces.
      transformPastedText: (text) => text.replace(/[\t\r\n]+/g, " "),
      dispatchTransaction: (tr) => {
        if (!active || active.view !== inner) return;
        inner.updateState(inner.state.apply(tr));
        if (!tr.docChanged) return;
        const { rows, layout } = read();
        rows[r][c] = cellOf(inner.state.doc);
        commit(rows, layout, true);
      },
    });
    active = { r, c, view: inner };
    inner.focus();
    const at = ev && inner.posAtCoords({ left: ev.clientX, top: ev.clientY });
    if (at) inner.dispatch(inner.state.tr.setSelection(TextSelection.create(inner.state.doc, at.pos)));
  };
  // A drag of a cell's right or lower edge sizes its column or row; the
  // table shows the size while it moves and keeps it on release.
  const resize = (what, i, ev) => {
    ev.preventDefault();
    ev.stopPropagation();
    if (!editable()) return;
    close();
    const table = dom.querySelector("table");
    const cols = [...table.querySelectorAll("col")];
    const measured = what === "widths" ? tds[0].map((td) => td.getBoundingClientRect().width) : tds.map((row) => row[0].getBoundingClientRect().height);
    const start = what === "widths" ? ev.clientX : ev.clientY;
    const from = measured[i];
    let layout = read().layout;
    const show = (l) => {
      if (what === "widths") {
        const widths = l.widths;
        cols.forEach((col, j) => { col.style.width = widths[j] ? widths[j] + "px" : ""; });
        table.classList.add("pm-table-sized");
        table.style.width = widths.every((w) => w) ? widths.reduce((x, y) => x + y, 0) + "px" : "100%";
      } else tds[i][0].parentElement.style.height = l.heights[i] + "px";
    };
    const onMove = (e) => {
      layout = resized(read().layout, what, i, from + (what === "widths" ? e.clientX : e.clientY) - start, measured);
      show(layout);
    };
    const onUp = (e) => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      onMove(e);
      commit(read().rows, layout);
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  };
  // The table's tools act on the chosen cells, or on the cell being
  // edited: formats, colours, rows and columns.
  const showTools = () => {
    if (!tools) return;
    const rg = range();
    for (const el of tools.querySelectorAll("[data-needs-cell]")) el.disabled = !rg;
    const { layout } = read();
    const here = rg ? colourOf(layout, rg.r0, rg.c0) : { fill: "", border: "" };
    tools.querySelector(".pm-table-fill").value = here.fill;
    tools.querySelector(".pm-table-border").value = here.border;
  };
  const toolbar = (rows, layout) => {
    const nRows = rows.length, nCols = rows[0].length;
    const change = (fn) => () => {
      close();
      const rg = range();
      const at = rg ? { r: rg.r1, c: rg.c1 } : { r: nRows - 1, c: nCols - 1 };
      const next = fn(read().rows, read().layout, at);
      anchor = head = null;
      commit(next.rows, next.layout);
    };
    // A button of the tools keeps the cell being edited, so a format
    // applies to its selection.
    const keepFocus = (ev) => ev.preventDefault();
    const button = (label, title, onclick, needsCell, cls) => h("button", { type: "button", class: cls || "link", title, "aria-label": title,
      onmousedown: keepFocus, onclick, ...(needsCell ? { "data-needs-cell": "" } : {}) }, label);
    const tone = (cls, label, what) => {
      const select = h("select", { class: cls, title: label, "aria-label": label, "data-needs-cell": "" },
        h("option", { value: "", text: t("editor.tableNoColour") }),
        ...TABLE_TONES.map((x) => h("option", { value: x, text: toneName(x) })));
      select.addEventListener("change", () => {
        const rg = range();
        if (!rg || !editable()) return showTools();
        close();
        const { rows: rs, layout: l } = read();
        commit(rs, paint(l, rs.length, rs[0].length, rg, what, select.value));
      });
      return h("label", { class: "pm-table-tone" }, h("span", { text: label }), select);
    };
    return h("div", { class: "pm-table-tools" },
      h("span", { class: "pm-group" },
        button(h("strong", { text: "B" }), t("editor.bold"), format("bold"), true, "pm-table-format"),
        button(h("em", { text: "I" }), t("editor.italic"), format("italic"), true, "pm-table-format"),
        button(h("u", { text: "U" }), t("editor.underline"), format("underline"), true, "pm-table-format"),
        button(h("s", { text: "S" }), t("editor.strike"), format("strike"), true, "pm-table-format"),
        button(h("mark", { text: "ab" }), t("editor.highlight"), format("highlight"), true, "pm-table-format"),
        button("</>", t("editor.code"), format("code"), true, "pm-table-format"),
        button(t("editor.link"), t("editor.tableLink"), format("link"), true, "pm-table-format")),
      tone("pm-table-fill", t("editor.tableFill"), "fill"),
      tone("pm-table-border", t("editor.tableBorder"), "border"),
      h("span", { class: "pm-group" },
        button(t("editor.addRow"), t("editor.addRowTitle"), change((rs, l, at) => insertRow(rs, l, at.r))),
        button(t("editor.addColumn"), t("editor.addColumnTitle"), change((rs, l, at) => insertColumn(rs, l, at.c))),
        nRows > 1 ? button(t("editor.removeRow"), t("editor.removeRowTitle"), change((rs, l, at) => removeRow(rs, l, at.r))) : null,
        nCols > 1 ? button(t("editor.removeColumn"), t("editor.removeColumnTitle"), change((rs, l, at) => removeColumn(rs, l, at.c))) : null,
        layout.widths || layout.heights ? button(t("editor.tableResetSizes"), t("editor.tableResetSizes"),
          change((rs, l) => ({ rows: rs, layout: { ...l, widths: null, heights: null } }))) : null),
    );
  };
  const render = (force) => {
    const key = keyOf(node.attrs);
    if (!force && shown === key) return;
    shown = key;
    close();
    const edit = view.editable;
    const { rows, layout } = read();
    if (anchor && (anchor.r >= rows.length || anchor.c >= rows[0].length)) anchor = head = null;
    if (head && (head.r >= rows.length || head.c >= rows[0].length)) head = null;
    const colgroup = h("colgroup", {}, rows[0].map(() => h("col")));
    const table = h("table", {}, colgroup);
    if (layout.widths) {
      table.classList.add("pm-table-sized");
      [...colgroup.children].forEach((col, j) => { if (layout.widths[j]) col.style.width = layout.widths[j] + "px"; });
      table.style.width = layout.widths.every((w) => w) ? layout.widths.reduce((x, y) => x + y, 0) + "px" : "100%";
    }
    tds = rows.map((row, i) => row.map((cell, j) => {
      const { fill, border } = colourOf(layout, i, j);
      const td = h(i === 0 ? "th" : "td", { class: [filledClass(fill), borderClass(border)].filter(Boolean).join(" ") || null },
        h("div", { class: "pm-cell-text" }, ...cellContent(cell)));
      if (edit) {
        td.classList.add("pm-cell-editable");
        td.tabIndex = 0;
        td.setAttribute("aria-label", t("editor.cell", { row: i + 1, col: j + 1 }));
        td.append(h("span", { class: "pm-col-handle", title: t("editor.tableDragColumn"), "aria-hidden": "true", onpointerdown: (ev) => resize("widths", j, ev) }),
          h("span", { class: "pm-row-handle", title: t("editor.tableDragRow"), "aria-hidden": "true", onpointerdown: (ev) => resize("heights", i, ev) }));
        td.addEventListener("mousedown", (ev) => {
          if (ev.target.closest(".pm-col-handle, .pm-row-handle")) return;
          if (ev.shiftKey) { ev.preventDefault(); close(); choose(i, j, true); return; }
          if (active && active.r === i && active.c === j) return;
          ev.preventDefault();
          open(i, j, ev);
        });
        td.addEventListener("focus", (ev) => { if (ev.target === td && !(active && active.r === i && active.c === j)) open(i, j); });
      }
      return td;
    }));
    const body = h("tbody");
    tds.forEach((row, i) => {
      const tr = h("tr", {}, ...row);
      if (layout.heights?.[i]) tr.style.height = layout.heights[i] + "px";
      body.append(tr);
    });
    table.append(body);
    tools = edit ? toolbar(rows, layout) : null;
    dom.replaceChildren(...[tools, h("div", { class: "pm-table-scroll" }, table)].filter(Boolean)); // replaceChildren writes a null out as "null"
    if (anchor) choose(anchor.r, anchor.c, false);
    if (head) choose(head.r, head.c, true);
    showTools();
  };
  render();
  return {
    dom,
    // Events in the table's own controls and in the cell being edited
    // are the table's, not the editor's.
    stopEvent: (ev) => Boolean(ev.target.closest?.("a, button, input, select, .pm-cell-editable")),
    ignoreMutation: () => true,
    update: (n) => { if (n.type !== node.type) return false; node = n; render(); return true; },
    destroy: () => close(),
  };
}

// tocView lists the body's headings; a click scrolls to one. tocs holds
// the refresh functions of an editor's tables of contents, which a plugin
// calls after every change of the document.
function tocView(node, view, tocs) {
  const dom = h("nav", { class: "pm-toc", contenteditable: "false", "aria-label": t("editor.toc") });
  const refresh = () => {
    const heads = [];
    view.state.doc.forEach((n, offset) => {
      if (n.type === schema.nodes.heading && n.textContent.trim()) heads.push({ level: n.attrs.level, text: n.textContent, offset });
    });
    dom.replaceChildren(h("p", { class: "pm-box-title", text: t("editor.toc") }), heads.length
      ? h("ul", {}, heads.map((x) => h("li", { class: "pm-toc-" + x.level }, h("button", { type: "button", class: "link", text: x.text,
        onclick: () => view.nodeDOM(x.offset)?.scrollIntoView({ block: "start" }) }))))
      : h("p", { class: "muted", text: t("editor.tocEmpty") }));
  };
  tocs.add(refresh);
  refresh();
  return { dom, ...selectable, update: (n) => n.type === node.type, destroy: () => tocs.delete(refresh) };
}

// listView lists the body's pictures (figures) or tables (tables),
// numbered in their order, each with its caption or a table's header row;
// a click scrolls to one (ADR-0119). It is refreshed like a table of
// contents.
function listView(node, view, tocs) {
  const figures = node.type.name === "figures";
  const title = figures ? t("editor.figures") : t("editor.tables");
  const dom = h("nav", { class: figures ? "pm-figures" : "pm-tables", contenteditable: "false", "aria-label": title });
  const refresh = () => {
    const entries = [];
    view.state.doc.forEach((n, offset) => {
      if (figures && n.type === schema.nodes.image) {
        const label = t("editor.figure", { n: entries.length + 1 });
        entries.push({ text: n.attrs.text.trim() ? `${label}: ${n.attrs.text.trim()}` : label, offset });
      } else if (!figures && n.type === schema.nodes.table) {
        const label = t("editor.tableN", { n: entries.length + 1 });
        const head = n.attrs.text.split("\n")[0].split("\t").map((c) => c.trim()).filter(Boolean).join(", ");
        entries.push({ text: head ? `${label}: ${head}` : label, offset });
      }
    });
    dom.replaceChildren(h("p", { class: "pm-box-title", text: title }), entries.length
      ? h("ul", {}, entries.map((x) => h("li", {}, h("button", { type: "button", class: "link", text: x.text,
        onclick: () => view.nodeDOM(x.offset)?.scrollIntoView({ block: "start" }) }))))
      : h("p", { class: "muted", text: figures ? t("editor.figuresEmpty") : t("editor.tablesEmpty") }));
  };
  tocs.add(refresh);
  refresh();
  return { dom, ...selectable, update: (n) => n.type === node.type, destroy: () => tocs.delete(refresh) };
}

// embedView shows an embedded item whole (ADR-0106): a heading that leads
// to it, then what load(ref) returns, its content read-only, or nothing
// where the reader may not see it. Events inside the content stay there.
function embedView(node, load) {
  const dom = h("div", { class: "pm-embed", contenteditable: "false" });
  const visible = Boolean(node.attrs.href);
  dom.append(visible ? h("a", { class: "pm-embed-title", href: safeHref(node.attrs.href), text: node.attrs.label })
    : h("span", { class: "muted", text: node.attrs.label || t("editor.hidden") }));
  const body = h("div", { class: "pm-embed-body", "aria-busy": "true" });
  if (visible && load) {
    dom.append(body);
    load(node.attrs.ref).then((el) => body.replaceChildren(...(el ? [el] : [])))
      .catch(() => body.replaceChildren(h("p", { class: "muted", text: t("embed.failed") })))
      .finally(() => body.removeAttribute("aria-busy"));
  }
  return { dom, ignoreMutation: () => true, stopEvent: (ev) => body.contains(ev.target) };
}

// taskListView shows the tasks that carry a label, or that a query
// selects (ADR-0151), from tasks.list, which the view of the item
// supplies: sortable and filterable (ADR-0114).
function taskListView(node, tasks) {
  const { label, query } = node.attrs;
  const dom = h("section", { class: "pm-tasklist", contenteditable: "false", "data-label": label, "data-query": query || null },
    h("p", { class: "pm-box-title", text: query ? t("editor.taskListQuery", { query }) : t("editor.taskListOf", { label }) }));
  const body = h("div", { "aria-busy": "true" }, h("p", { class: "muted", text: "…" }));
  dom.append(body);
  Promise.resolve(tasks?.list ? tasks.list(query ? { query } : { label }) : null)
    .then((el) => body.replaceChildren(el || h("p", { class: "muted", text: query ? t("editor.taskListQueryNone") : t("editor.taskListNone") })))
    .catch(() => body.replaceChildren(h("p", { class: "muted", text: t("embed.failed") })))
    .finally(() => body.removeAttribute("aria-busy"));
  return { dom, ignoreMutation: () => true, stopEvent: (ev) => body.contains(ev.target),
    update: (n) => n.type === node.type && n.attrs.label === label && n.attrs.query === query };
}

// itemViewView shows the items a view block names (ADR-0203), as far as
// the reader reads them; while editing, a button changes what it shows.
function itemViewView(node, v, getPos) {
  const attrs = node.attrs;
  const dom = h("section", { class: "pm-itemview", contenteditable: "false", "data-layout": attrs.layout });
  const body = h("div", { "aria-busy": "true" }, h("p", { class: "muted", text: "…" }));
  const block = blockFromNode(node);
  if (v.editable && viewsReady()) {
    dom.append(h("p", { class: "pm-box-title" }, h("button", { type: "button", class: "link pm-itemview-edit", text: t("views.change"), onclick: async () => {
      const next = await pickView({ layout: block.layout, type: block.itemType || "", space: block.space || "", query: block.query || "", group: block.group || "",
        columns: block.columns || [], sort: block.sort || "" });
      const pos = getPos();
      if (!next || typeof pos !== "number") return;
      v.dispatch(v.state.tr.setNodeMarkup(pos, null, { ...node.attrs, layout: next.layout, itemType: next.itemType || "", space: next.space || "", query: next.query || "",
        group: next.group || "", columns: next.columns?.length ? next.columns : null, sort: next.sort || "" }));
    } })));
  }
  dom.append(body);
  Promise.resolve(viewsReady() ? viewBlock(block) : null)
    .then((el) => body.replaceChildren(el || h("p", { class: "muted", text: t("views.none") })))
    .catch(() => body.replaceChildren(h("p", { class: "muted", text: t("embed.failed") })))
    .finally(() => body.removeAttribute("aria-busy"));
  return { dom, ignoreMutation: () => true, stopEvent: (ev) => dom.contains(ev.target),
    update: (n) => n.type === node.type && JSON.stringify(n.attrs) === JSON.stringify(attrs) };
}

// childrenView lists the child pages of the item, from childPages().
function childrenView(node, childPages) {
  const dom = h("nav", { class: "pm-children", contenteditable: "false", "aria-label": t("editor.children") },
    h("p", { class: "pm-box-title", text: t("editor.children") }));
  const list = h("div", {}, h("p", { class: "muted", text: "…" }));
  dom.append(list);
  Promise.resolve(childPages ? childPages() : []).then((kids) => {
    list.replaceChildren(kids.length
      ? h("ul", {}, kids.map((c) => h("li", {}, h("a", { href: "#/item/" + encodeURIComponent(c.key), text: c.title || c.key }))))
      : h("p", { class: "muted", text: t("editor.childrenNone") }));
  }).catch(() => list.replaceChildren(h("p", { class: "muted", text: t("editor.childrenNone") })));
  return { dom, ...selectable, update: (n) => n.type === node.type };
}

// imageSizes are the widths offered besides dragging, in percent of the
// column; 0 is the picture's own size (ADR-0056).
const imageSizes = [0, 25, 50, 75, 100];

// imageView shows an image attachment; an editor edits its caption and
// its width, and a drawing (ADR-0053) in the drawing editor.
function imageView(node, view, getPos, files) {
  // A video block plays its file with the browser's own controls (ADR-0097).
  const video = node.type.name === "video";
  const img = video
    ? h("video", { src: files ? files.url(node.attrs.ref) : "", controls: true, preload: "metadata", playsinline: true, title: node.attrs.text })
    : h("img", { src: files ? files.url(node.attrs.ref) : "", alt: node.attrs.text, loading: "lazy" });
  const handle = h("span", { class: "pm-resize", title: t("editor.imageResize"), "aria-hidden": "true" });
  // The processes of engines in a drawing are shown live over its picture
  // (ADR-0142), from the regions the picture lists in its first bytes. A
  // picture whose name is not known here, as in an embedded text or one
  // saved by someone else since, is asked too.
  const live = h("span", { class: "pm-live" });
  const frame = h("span", { class: "pm-image-frame" }, img, live, handle);
  let liveRef = null;
  let regions = [];
  const showLive = async () => {
    const ref = !video && files && (files.isDrawing?.(node.attrs.ref) || !files.knows?.(node.attrs.ref)) ? node.attrs.ref : null;
    if (ref === liveRef) return;
    liveRef = ref;
    regions.forEach((r) => r.destroy());
    regions = [];
    live.replaceChildren();
    if (!ref) return;
    try {
      const res = await fetch(files.url(ref), { credentials: "same-origin", headers: { Range: "bytes=0-65535" } });
      if (!res.ok) return;
      const found = regionsIn(new Uint8Array(await res.arrayBuffer()));
      if (liveRef !== ref) return;
      regions = found.map(processRegion);
      live.replaceChildren(...regions.map((r) => r.dom));
    } catch {
      // The picture shows the processes as they were saved.
    }
  };
  const caption = h("figcaption");
  const dom = h("figure", { class: video ? "pm-image pm-video" : "pm-image", contenteditable: "false" }, frame, caption);
  let shown = null;
  // The width is set through the style object, which the content security
  // policy allows (unlike a style attribute). A width in percent applies to
  // the frame, which is a share of the column; the picture fills it.
  // A picture floated left or right takes its width (half the column by
  // default) as a whole figure, so the text flows beside it (ADR-0059).
  const showWidth = (w) => {
    const floats = node.attrs.align === "left" || node.attrs.align === "right";
    dom.style.width = floats ? `${w || 50}%` : "";
    frame.style.width = floats ? "100%" : w ? `${w}%` : "";
    img.style.width = w || floats ? "100%" : "";
  };
  const setWidth = (w) => {
    const pos = getPos();
    if (pos === undefined || dom.classList.contains("pm-locked")) return showWidth(node.attrs.width);
    view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, width: w }));
  };
  // Left and right let the text flow beside the picture (ADR-0059).
  const setImageAlign = (align) => {
    const pos = getPos();
    if (pos !== undefined && !dom.classList.contains("pm-locked")) view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, align }));
  };
  // Dragging the corner sets the width in steps of 5 percent of the column.
  handle.addEventListener("pointerdown", (e) => {
    if (!view.editable) return;
    e.preventDefault();
    handle.setPointerCapture(e.pointerId);
    const column = dom.getBoundingClientRect().width;
    const start = frame.getBoundingClientRect().width;
    const x0 = e.clientX;
    let w = node.attrs.width;
    const move = (ev) => {
      w = Math.min(100, Math.max(10, Math.round(((start + ev.clientX - x0) / column) * 20) * 5));
      showWidth(w);
    };
    const up = () => {
      handle.removeEventListener("pointermove", move);
      handle.removeEventListener("pointerup", up);
      if (w !== node.attrs.width) setWidth(w);
    };
    handle.addEventListener("pointermove", move);
    handle.addEventListener("pointerup", up);
  });
  // A drawing is edited from its picture and saved as a new attachment; the
  // block then points at it. The former picture stays, as earlier versions
  // of the body show it (ADR-0043).
  const redraw = async (status) => {
    status.textContent = "";
    try {
      // A drawing open as a live whiteboard is worked on there: the drawing
      // editor could not save it (ADR-0172).
      const working = await files.live?.(node.attrs.ref, true);
      if (working) {
        status.replaceChildren(t("draw.busyLive", { n: working }), " ",
          ...(files.whiteboard ? [h("a", { href: files.whiteboard(node.attrs.ref), text: t("draw.joinWhiteboard", { n: working }) })] : []));
        return;
      }
      const res = await fetch(files.url(node.attrs.ref), { credentials: "same-origin" });
      if (!res.ok) throw new Error(res.statusText);
      // The new version keeps the drawing's name, which may have been
      // renamed in the album (ADR-0180); only a file that names none is
      // named after the caption.
      const kept = filenameOf(res.headers.get("Content-Disposition"));
      const fileName = isDrawing(kept) ? kept : drawingName(node.attrs.text);
      // The new picture is a version of the one edited (ADR-0055), so its
      // copies elsewhere learn of it. The dialog stays open until it is
      // stored, so the block cannot be left before it points at it.
      const a = await openDrawing(await res.blob(), (png) =>
        files.upload(new File([png], fileName, { type: "image/png" }), { item: files.item, attachment: node.attrs.ref }), drawingProcesses(files));
      if (!a) return;
      const pos = getPos();
      if (pos === undefined || dom.classList.contains("pm-locked")) throw new Error(t("draw.locked"));
      view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, ref: a.id }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // A copy whose original was changed since takes the newer version over:
  // copied again, from the same source (ADR-0055).
  const takeOver = async (status, origin) => {
    status.textContent = "";
    try {
      const src = { item: origin.item.id, attachment: origin.newer.id };
      const res = await fetch(`/api/items/${src.item}/attachments/${src.attachment}`, { credentials: "same-origin" });
      if (!res.ok) throw new Error(res.statusText);
      const a = await files.upload(new File([await res.blob()], origin.newer.name, { type: origin.newer.type }), src);
      const pos = getPos();
      if (pos === undefined || dom.classList.contains("pm-locked")) throw new Error(t("draw.locked"));
      view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, ref: a.id }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // openWith offers a drawing as a live whiteboard and, to one who may
  // edit, alone in the drawing editor; while people work on it live, it
  // says so and offers to join (ADR-0172).
  const openWith = () => {
    const ref = node.attrs.ref;
    const board = h("a", { class: "pm-whiteboard", href: files.whiteboard(ref), text: t("draw.openWhiteboard") });
    const alone = files.drawHref ? h("a", { class: "pm-draw-alone", href: files.drawHref(ref), text: t("draw.openDrawing") }) : null;
    const box = h("span", { class: "pm-drawing-open" }, board, ...(alone ? [" · ", alone] : []));
    files.live?.(ref).then((n) => {
      if (!n || node.attrs.ref !== ref) return;
      board.textContent = t("draw.joinWhiteboard", { n });
      alone?.replaceWith(h("span", { class: "muted", text: t("draw.aloneWhileLive") }));
    });
    return box;
  };
  const render = () => {
    const key = node.attrs.ref + node.attrs.text + node.attrs.width + node.attrs.align + view.editable;
    if (shown === key) return;
    shown = key;
    dom.dataset.ref = node.attrs.ref;
    const src = files ? files.url(node.attrs.ref) : "";
    // A video keeps its place in the playback while only its caption changes.
    if (img.getAttribute("src") !== src) img.src = src;
    if (video) img.title = node.attrs.text;
    else img.alt = node.attrs.text;
    showWidth(node.attrs.width);
    showLive();
    dom.classList.toggle("pm-sizable", view.editable);
    for (const a of ["left", "center", "right"]) dom.classList.toggle("pm-image-" + a, node.attrs.align === a);
    const drawing = !video && files?.isDrawing?.(node.attrs.ref) && files.whiteboard;
    if (!view.editable) return caption.replaceChildren(...[node.attrs.text, ...(drawing ? [" ", openWith()] : [])]);
    const size = h("select", { class: "pm-image-size", "aria-label": t("editor.imageSize"), onchange: (e) => setWidth(Number(e.target.value)) },
      [...new Set([...imageSizes, node.attrs.width])].sort((a, b) => a - b).map((w) =>
        h("option", { value: String(w), text: w ? `${w} %` : t("editor.imageOwnSize"), selected: w === node.attrs.width })));
    const input = h("input", { value: node.attrs.text, placeholder: t("editor.caption"), "aria-label": t("editor.caption") });
    input.addEventListener("change", () => {
      const pos = getPos();
      if (pos !== undefined && !dom.classList.contains("pm-locked")) {
        view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, text: input.value.replace(/[\r\n]+/g, " ") }));
      } else render(shown = null);
    });
    const status = h("span", { class: "error", role: "status" });
    const origin = files?.origin?.(node.attrs.ref);
    // replaceChildren takes nodes and strings only: no arrays, no null.
    const place = h("select", { class: "pm-image-align", "aria-label": t("editor.imageAlign"), onchange: (e) => setImageAlign(e.target.value) },
      [["", t("editor.imageAlignDefault")], ["left", t("editor.imageAlignLeft")], ["center", t("editor.imageAlignCenter")], ["right", t("editor.imageAlignRight")]]
        .map(([v, label]) => h("option", { value: v, text: label, selected: v === node.attrs.align })));
    const remove = h("button", { type: "button", class: "link pm-image-remove", text: t("editor.imageRemove"), onclick: () => {
      const pos = getPos();
      if (pos === undefined || dom.classList.contains("pm-locked")) return;
      view.dispatch(view.state.tr.delete(pos, pos + node.nodeSize));
      view.focus(); // the button is gone; the text goes on where the picture was
    } });
    caption.replaceChildren(...[input, " ", size, " ", place, " ", remove,
      ...(files?.isDrawing?.(node.attrs.ref) ? [" ", h("button", { type: "button", class: "link pm-redraw", text: t("draw.edit"), onclick: () => redraw(status) })] : []),
      ...(drawing ? [" ", h("a", { class: "link pm-whiteboard", href: files.whiteboard(node.attrs.ref), text: t("draw.openWhiteboard") })] : []),
      ...(origin?.newer ? [" ", h("button", { type: "button", class: "link pm-take-over", text: t("att.takeOver", { key: origin.item.key }), onclick: () => takeOver(status, origin) })] : []),
      status]);
  };
  render();
  // A click on a picture selects its block, so it can be deleted, cut or
  // moved like text; a video keeps its clicks for its controls.
  const stopEvent = (ev) => caption.contains(ev.target) || handle.contains(ev.target) || live.contains(ev.target) || (video && img.contains(ev.target));
  return {
    dom, ignoreMutation: () => true, stopEvent,
    update: (n) => { if (n.type !== node.type) return false; node = n; render(); return true; },
    destroy: () => regions.forEach((r) => r.destroy()),
  };
}

// playsElsewhere reports whether another block of the text plays the same
// sound file.
function playsElsewhere(doc, ref, pos) {
  let found = false;
  doc.descendants((n, at) => {
    if (at !== pos && n.type === schema.nodes.audio && n.attrs.ref === ref) found = true;
    return !found;
  });
  return found;
}

// audioView plays a sound file of the item with the browser's own controls
// (ADR-0223); an editor edits its caption and removes
// it. Removing the last block that plays a file offers to delete the file
// too, so a recording does not stay behind unseen; that removal cannot be
// undone, as the text could otherwise get back a file that is gone. report
// shows what failed. The player keeps its clicks, so playing does not
// select the block.
function audioView(node, view, getPos, files, report = () => {}) {
  const player = h("audio", { class: "pm-audio-player", controls: true, preload: "metadata" });
  const caption = h("figcaption");
  const dom = h("figure", { class: "pm-audio", contenteditable: "false" }, player, caption);
  let shown = null;
  const change = (attrs) => {
    const pos = getPos();
    if (pos === undefined || dom.classList.contains("pm-locked")) return false;
    view.dispatch(view.state.tr.setNodeMarkup(pos, null, { ...node.attrs, ...attrs }));
    return true;
  };
  const render = () => {
    const key = node.attrs.ref + node.attrs.text + view.editable;
    if (shown === key) return;
    shown = key;
    dom.dataset.ref = node.attrs.ref;
    const src = files ? files.url(node.attrs.ref) : "";
    // The player keeps its place while only the caption changes.
    if (player.getAttribute("src") !== src) player.src = src;
    player.setAttribute("aria-label", node.attrs.text || t("editor.audio"));
    if (!view.editable) return caption.replaceChildren(node.attrs.text);
    const input = h("input", { value: node.attrs.text, placeholder: t("editor.caption"), "aria-label": t("editor.caption") });
    input.addEventListener("change", () => {
      if (!change({ text: input.value.replace(/[\r\n]+/g, " ") })) render(shown = null);
    });
    const take = (deleteFile) => {
      const pos = getPos();
      if (pos === undefined || dom.classList.contains("pm-locked")) return;
      const ref = node.attrs.ref;
      const tr = view.state.tr.delete(pos, pos + node.nodeSize);
      if (deleteFile) tr.setMeta("addToHistory", false);
      view.dispatch(tr);
      view.focus();
      if (deleteFile) files.remove(ref).catch((err) => report(t("editor.audioNotDeleted", { problem: err.message })));
    };
    const remove = h("button", { type: "button", class: "link pm-audio-remove", text: t("editor.audioRemove"), onclick: () => {
      const pos = getPos();
      if (pos === undefined || dom.classList.contains("pm-locked")) return;
      if (!files?.remove || playsElsewhere(view.state.doc, node.attrs.ref, pos)) return take(false);
      // The last block that plays the file: keep it or delete it too.
      caption.replaceChildren(h("span", { class: "pm-audio-ask", role: "group", "aria-label": t("editor.audioOrphan") },
        h("span", { text: t("editor.audioOrphan") }), " ",
        h("button", { type: "button", class: "danger pm-audio-delete-file", text: t("editor.audioDeleteFile"), onclick: () => take(true) }), " ",
        h("button", { type: "button", class: "secondary pm-audio-keep-file", text: t("editor.audioKeepFile"), onclick: () => take(false) }), " ",
        h("button", { type: "button", class: "link pm-audio-cancel", text: t("common.cancel"), onclick: () => render(shown = null) })));
      caption.querySelector(".pm-audio-keep-file").focus();
    } });
    caption.replaceChildren(input, " ", remove);
  };
  render();
  return {
    dom, ignoreMutation: () => true,
    stopEvent: (ev) => caption.contains(ev.target) || player.contains(ev.target),
    update: (n) => { if (n.type !== node.type) return false; node = n; render(); return true; },
    destroy: () => player.pause(),
  };
}

// createEditor mounts an editor for blocks and returns its element and a
// function reading the blocks back. With editable false it is a viewer.
// resolve(keyOrId) returns an item (for mentions and embeds) or throws;
// childPages() the item's child pages as [{key, title}], for a list of
// child pages. files, for an item's attachments (ADR-0045), has url(id),
// where a file is served, and upload(file), which stores a file and returns
// the attachment. extra adds plugins, such as a live session's (live.js);
// viewProps adds editor view properties. comment(anchor), if given, starts
// an inline comment on the selected text (anchors.js, ADR-0047); note(anchor)
// a personal note on it (notes.js, ADR-0107).
// embedContent(ref), if given, returns the content of an embedded item,
// shown in its block (ADR-0106). markdown.insert(address), if given, makes
// a page from a Markdown file of a repository below the item and returns
// it, or null when cancelled (ADR-0138). weblinks.save(address), if given,
// saves the page a link leads to as a web link item and returns {item,
// problem}, or null when cancelled (ADR-0208).
export function createEditor({ blocks, embeds, editable, resolve, childPages, files, extra = [], viewProps = {}, comment, protect, note, actions = [], embedContent, tasks, markdown, weblinks, create }) {
  const mount = h("div", { class: "pm-editor" });
  const tocs = new Set();
  const tocPlugin = new Plugin({ view: () => ({ update: (v, prev) => { if (!v.state.doc.eq(prev.doc)) tocs.forEach((f) => f()); } }) });
  // The toolbar shows what applies at the selection, after every change,
  // one's own or another's in a live session (ADR-0059).
  let onUpdate = () => {};
  const statePlugin = new Plugin({ view: () => ({ update: (v) => onUpdate(v.state) }) });
  // Links, "@" and embedding as in a word processor (ADR-0096); a link to
  // a Markdown file offers to show the file instead (ADR-0138).
  let showFile = null;
  let saveLink = null;
  const aids = assist(schema, isSafeLink, { showFile: markdown ? (href) => showFile(href) : null, saveLink: weblinks ? (href) => saveLink(href) : null, create });
  const aidKeys = keymap({ "Mod-k": (state, dispatch, v) => { aids.openLink(v); return true; } });
  // Pictures pasted into the text (ADR-0108): one copied in the album
  // becomes a copy of its original file, any other image an upload.
  // Markdown becomes formatted text (ADR-0133), unless Ctrl+Shift+V
  // (Cmd+Shift+V) pastes plain text.
  let onPaste = () => false;
  let plain = false;
  const pastePlugin = new Plugin({ props: {
    handleKeyDown: (v, ev) => { plain = ev.shiftKey && (ev.ctrlKey || ev.metaKey) && ev.key.toLowerCase() === "v"; return false; },
    handlePaste: (v, ev) => { const asText = plain; plain = false; return onPaste(ev, asText); },
  } });
  const view = new EditorView(mount, {
    state: EditorState.create({ doc: docFromBlocks(blocks, embeds), plugins: [...extra, ...(editable ? [pastePlugin, ...aids.plugins, aidKeys] : []), ...plugins(), tocPlugin, statePlugin] }),
    editable: () => editable,
    attributes: { "aria-label": t("editor.body"), role: "textbox", "aria-multiline": "true" },
    nodeViews: {
      task: taskView,
      table: tableView,
      toc: (node, v) => tocView(node, v, tocs),
      figures: (node, v) => listView(node, v, tocs),
      tables: (node, v) => listView(node, v, tocs),
      children: (node) => childrenView(node, childPages),
      image: (node, v, getPos) => imageView(node, v, getPos, files),
      video: (node, v, getPos) => imageView(node, v, getPos, files),
      audio: (node, v, getPos) => audioView(node, v, getPos, files, (problem) => { status.textContent = problem; }),
      bpmn: (node, v, getPos) => diagramView(node, v, getPos, files),
      process: (node, v, getPos) => diagramView(node, v, getPos, files),
      form: (node, v, getPos) => formView(node, v, getPos),
      startform: (node, v, getPos) => startFormView(node, v, getPos, files),
      decision: (node, v, getPos) => decisionView(node, v, getPos),
      architecture: (node, v, getPos) => architectureView(node, v, getPos),
      embed: (node) => embedView(node, embedContent),
      tasklist: (node) => taskListView(node, tasks),
      itemview: (node, v, getPos) => itemViewView(node, v, getPos),
      veiled: (node) => ({ dom: h("div", { class: "pm-veiled", "data-secret": node.attrs.secret, contenteditable: "false" },
        h("span", { class: "pm-veiled-bar", "aria-hidden": "true", text: "▒▒▒▒▒▒▒▒" }), veiledContent(node.attrs.secret)), ...selectable }),
    },
    ...viewProps,
  });
  const run = (cmd) => () => { cmd(view.state, view.dispatch); view.focus(); };
  const kindSelect = h("select", { "aria-label": t("editor.blockKind"), onchange: (e) => {
    const [kind, arg] = e.target.value.split(":");
    const attrs = kind === "heading" ? { level: Number(arg) } : kind === "panel" ? { tone: arg } : null;
    run(setBlockType(schema.nodes[kind], attrs))();
  } }, [
    ["paragraph", t("editor.paragraph")], ["heading:1", t("editor.heading", { n: 1 })], ["heading:2", t("editor.heading", { n: 2 })], ["heading:3", t("editor.heading", { n: 3 })],
    ["bullet", t("editor.bullet")], ["ordered", t("editor.ordered")], ["task", t("editor.task")], ["quote", t("editor.quote")], ["code_block", t("editor.codeBlock")],
    ["panel:info", t("editor.panelInfo")], ["panel:note", t("editor.panelNote")], ["panel:warning", t("editor.panelWarning")],
    ["panel:tip", t("editor.panelTip")], ["panel:error", t("editor.panelError")],
  ].map(([v, label]) => h("option", { value: v, text: label })));
  // insertBlock puts a block after the one the cursor is in.
  const insertBlock = (node) => {
    const { $to } = view.state.selection;
    view.dispatch(view.state.tr.insert($to.depth >= 1 ? $to.after(1) : $to.pos, node));
    view.focus();
  };
  const insertSelect = h("select", { "aria-label": t("editor.insert"), onchange: (e) => {
    const kind = e.target.value;
    e.target.value = "";
    if (kind === "table") insertBlock(schema.nodes.table.create({ text: `${t("editor.heading", { n: 1 })}\t${t("editor.heading", { n: 2 })}\n\t` }));
    else if (kind === "image") picker.click();
    else if (kind === "video") videoPicker.click();
    else if (kind === "audio") audioPicker.click();
    else if (kind === "record") recordSound();
    else if (kind === "drawing") draw();
    else if (kind === "bpmnnew") newModel();
    else if (kind === "bpmnfile") modelPicker.click();
    else if (kind === "process") pickedProcess();
    else if (kind === "form" || kind === "decision") pickedFromEngine(kind);
    else if (kind === "startform") pickedStartLink();
    else if (kind === "architecture") pickedArchitecture();
    else if (kind === "album") fromAlbum();
    else if (kind === "clip") fromClip();
    else if (kind === "embed") embed();
    else if (kind === "markdown") markdownDialog();
    else if (kind === "newtask") newTask();
    else if (kind === "tasklist") taskList();
    else if (kind === "itemview") itemView();
    else if (kind === "markdownfile") markdownFile("");
    else if (kind) insertBlock(schema.nodes[kind].create());
  } }, [["", t("editor.insert")], ["embed", t("assist.embedItem")], ["markdown", t("editor.markdown")], markdown ? ["markdownfile", t("editor.markdownFile")] : null, tasks?.create ? ["newtask", t("editor.newTask")] : null, tasks?.pickLabel ? ["tasklist", t("editor.taskList")] : null, viewsReady() ? ["itemview", t("views.insert")] : null, ["table", t("editor.table")], ["divider", t("editor.divider")], ["toc", t("editor.toc")], ["figures", t("editor.figures")], ["tables", t("editor.tables")], ["children", t("editor.children")],
    files ? ["image", t("editor.image")] : null, files ? ["video", t("editor.video")] : null, files ? ["audio", t("editor.audio")] : null, files && canRecordHere() ? ["record", t("editor.record")] : null, files ? ["drawing", t("draw.insert")] : null, files ? ["album", t("album.fromAlbum")] : null, files ? ["clip", t("editor.copiedPicture")] : null,
    files ? ["bpmnnew", t("bpmn.insertNew")] : null, files ? ["bpmnfile", t("bpmn.insertFile")] : null, ["process", t("bpmn.insertProcess")],
    ["form", t("form.insert")], ["startform", t("start.insert")], ["decision", t("decision.insert")], ["architecture", t("arch.insert")]].filter(Boolean)
    .map(([v, label]) => h("option", { value: v, text: label })));
  // An image is uploaded as an attachment of the item, then shown.
  const picker = h("input", { type: "file", accept: "image/png,image/jpeg,image/gif,image/webp", hidden: true, "aria-label": t("editor.image") });
  const uploadImage = async (file) => {
    status.textContent = "";
    try {
      const a = await files.upload(file);
      if (!a.inline) throw new Error(t("editor.notAnImage"));
      insertBlock(schema.nodes.image.create({ ref: a.id, text: a.name.replace(/\.[^.]*$/, "") }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  picker.addEventListener("change", () => {
    const file = picker.files[0];
    picker.value = "";
    if (file) uploadImage(file);
  });
  // A video is uploaded as an attachment of the item, then played in the
  // text (ADR-0097); the server reads its format from its bytes.
  const videoPicker = h("input", { type: "file", accept: "video/mp4,video/webm,video/quicktime,.mov", hidden: true, "aria-label": t("editor.video") });
  videoPicker.addEventListener("change", async () => {
    const file = videoPicker.files[0];
    videoPicker.value = "";
    if (!file) return;
    status.textContent = t("editor.uploading");
    try {
      const a = await files.upload(file);
      if (!VIDEO_TYPES.includes(a.type)) throw new Error(t("editor.notAVideo"));
      status.textContent = "";
      insertBlock(schema.nodes.video.create({ ref: a.id, text: a.name.replace(/\.[^.]*$/, "") }));
    } catch (err) {
      status.textContent = err.message;
    }
  });
  // A sound is uploaded as an attachment of the item, then played in the
  // text (ADR-0223); the server reads its format from
  // its bytes (ADR-0219).
  const placeSound = async (file) => {
    status.textContent = t("editor.uploading");
    const a = await files.upload(file);
    if (!AUDIO_TYPES.includes(a.type)) throw new Error(t("editor.notASound"));
    status.textContent = "";
    insertBlock(schema.nodes.audio.create({ ref: a.id, text: a.name.replace(/\.[^.]*$/, "") }));
  };
  const audioPicker = h("input", { type: "file", accept: "audio/*,.webm,.ogg,.opus,.m4a,.mp3,.wav", hidden: true, "aria-label": t("editor.audio") });
  audioPicker.addEventListener("change", async () => {
    const file = audioPicker.files[0];
    audioPicker.value = "";
    if (!file) return;
    try {
      await placeSound(file);
    } catch (err) {
      status.textContent = err.message;
    }
  });
  // Sound recorded here is uploaded and played in the text: a dialog
  // records at once, and closing it discards what is not attached.
  const recordSound = () => {
    status.textContent = "";
    const problem = h("div", { class: "pm-record-status" });
    const dialog = h("dialog", { class: "pm-dialog pm-record", "aria-labelledby": "pm-record-title" });
    const close = () => { if (dialog.open) dialog.close(); dialog.remove(); view.focus(); };
    const recorder = recorderControl(async (file) => {
      try {
        await placeSound(file);
        close();
      } catch (err) {
        status.textContent = "";
        problem.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
      }
    }, problem, { prefix: "pm-", autostart: true });
    dialog.append(h("h2", { id: "pm-record-title", text: t("editor.recordTitle") }), recorder, problem,
      h("div", { class: "pm-dialog-actions" }, h("button", { type: "button", class: "link", id: "pm-record-close", text: t("common.cancel"), onclick: close })));
    dialog.addEventListener("cancel", (ev) => { ev.preventDefault(); close(); });
    document.body.append(dialog);
    dialog.showModal();
  };
  // A drawing (ADR-0053) is made in the drawing editor, stored as an
  // attachment that carries its scene, and shown like an image.
  const draw = async () => {
    status.textContent = "";
    try {
      const a = await openDrawing(null, (png) => files.upload(new File([png], drawingName(""), { type: "image/png" })), drawingProcesses(files));
      if (!a) return;
      insertBlock(schema.nodes.image.create({ ref: a.id, text: "" }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // A BPMN model (ADR-0129) is drawn in the modeler or uploaded as a file,
  // stored as a file of the item and shown as a diagram; a process of a
  // process engine is named, and read from the engine when shown.
  const newModel = async () => {
    status.textContent = "";
    try {
      const a = await openModeler("", (xml) => files.upload(new File([xml], bpmnName(""), { type: "application/xml" })));
      if (a) insertBlock(schema.nodes.bpmn.create({ ref: a.id, text: "" }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  const modelPicker = h("input", { type: "file", accept: ".bpmn,.xml,application/xml,text/xml", hidden: true, "aria-label": t("bpmn.insertFile") });
  modelPicker.addEventListener("change", async () => {
    const file = modelPicker.files[0];
    modelPicker.value = "";
    if (!file) return;
    status.textContent = t("editor.uploading");
    try {
      const a = await files.upload(file);
      if (!/^text\/(xml|plain);/.test(a.type)) throw new Error(t("bpmn.notAModel"));
      status.textContent = "";
      insertBlock(schema.nodes.bpmn.create({ ref: a.id, text: a.name.replace(/\.[^.]*$/, "") }));
    } catch (err) {
      status.textContent = err.message;
    }
  });
  const pickedProcess = async () => {
    status.textContent = "";
    const p = await pickProcess();
    if (p) insertBlock(schema.nodes.process.create({ engine: p.engine, process: p.process, text: p.name || "" }));
  };
  // A form or a decision of a process engine (ADR-0144) is named, and read
  // from the engine when shown.
  const pickedFromEngine = async (kind) => {
    status.textContent = "";
    const p = await pickFromEngine(kind === "form" ? "forms" : "decisions");
    if (p) insertBlock(schema.nodes[kind].create({ engine: p.engine, [kind]: p.id, ...(p.process ? { process: p.process } : {}), text: p.name || "" }));
  };
  // A start form (ADR-0148) names a public start link; what it fills in
  // beforehand is chosen when it is placed, from what its fields hint at.
  const pickedStartLink = async () => {
    status.textContent = "";
    const p = await pickStartLink();
    if (!p) return;
    const prefill = await editPrefill({ engine: p.engine, link: p.link, prefill: [] }, true);
    insertBlock(schema.nodes.startform.create({ engine: p.engine, link: p.link, process: p.process || "", prefill: prefill || [], text: p.name || "" }));
  };
  // The architecture sketch of a release (ADR-0146) is named, and read
  // from the engine when shown.
  const pickedArchitecture = async () => {
    status.textContent = "";
    const p = await pickArchitecture();
    if (p) insertBlock(schema.nodes.architecture.create(p));
  };
  // A picture of the album is copied into this item with its source
  // (ADR-0055), unless it is one of its own files already.
  const placeAlbum = async (p) => {
    const a = p.item.id === files.item ? p.attachment
      : await files.upload(await pictureFile(p), { item: p.item.id, attachment: p.attachment.id });
    // A video of the album plays in the text (ADR-0097, ADR-0189).
    const node = VIDEO_TYPES.includes(p.attachment.type) ? schema.nodes.video : schema.nodes.image;
    insertBlock(node.create({ ref: a.id, text: captionOf(p.attachment.name) }));
  };
  // A picture copied in the album (ADR-0108), named by the clipboard's
  // marker; its name and type come from this browser's copy or the item.
  const pasteAlbum = async (item, attachment) => {
    status.textContent = "";
    try {
      let p = copiedPicture();
      if (p?.item.id !== item || p?.attachment.id !== attachment) {
        const { attachments } = await (await fetch(`/api/items/${item}/attachments`, { credentials: "same-origin" })).json();
        const a = (attachments || []).find((x) => x.id === attachment);
        if (!a) throw new Error(t("editor.clipGone"));
        p = { item: { id: item }, attachment: a };
      }
      await placeAlbum(p);
    } catch (err) {
      status.textContent = err.message;
    }
  };
  const fromClip = () => {
    const p = copiedPicture();
    if (!p) status.textContent = t("editor.noClip");
    else pasteAlbum(p.item.id, p.attachment.id);
  };
  onPaste = (ev, asText) => {
    const html = ev.clipboardData?.getData("text/html") || "";
    const m = files && html.match(clipMark);
    if (m) {
      pasteAlbum(m[1], m[2]);
      return true;
    }
    const file = files && [...(ev.clipboardData?.files || [])].find((f) => /^image\/(png|jpeg|gif|webp)$/.test(f.type));
    if (file) {
      uploadImage(file);
      return true;
    }
    const text = ev.clipboardData?.getData("text/plain") || "";
    if (asText || !pastesAsMarkdown(text, html)) return false;
    // In a code block or a protected passage the paste stays as it is.
    const reason = insertMarkdown(text, false);
    if (reason === "code" || reason === "protected" || reason === "empty") return false;
    if (reason) status.textContent = markdownError(reason);
    return true;
  };
  // insertMarkdown puts Markdown at the selection (ADR-0133) and tells in
  // the status what was simplified; it returns why nothing was inserted,
  // or null. A transaction refused because someone else holds a block
  // there (live.js, ADR-0010) changes nothing.
  const insertMarkdown = (text, explicit) => {
    if (!view.editable) return "refused";
    const r = markdownInsertion(view.state, text, explicit);
    if (!r.tr) return r.reason;
    const before = view.state.doc;
    view.dispatch(r.tr);
    if (view.state.doc === before) return "refused";
    status.textContent = [r.pictures ? t("editor.markdownPictures", { n: r.pictures }) : "",
      r.dropped ? t("editor.markdownDropped", { n: r.dropped }) : ""].filter(Boolean).join(" ");
    return null;
  };
  const markdownError = (reason) => (reason === "empty" ? t("editor.markdownEmpty")
    : reason === "protected" ? t("editor.markdownProtected") : t("editor.markdownRefused"));
  // Markdown pasted into a dialog, for text the paste does not recognise
  // as Markdown; it goes where the cursor was. Ctrl+Enter inserts it too.
  const markdownDialog = async () => {
    status.textContent = "";
    const area = h("textarea", { name: "markdown", class: "code", rows: "12", spellcheck: "false", "aria-describedby": "pm-markdown-hint" });
    area.addEventListener("keydown", (ev) => {
      if (ev.key === "Enter" && (ev.ctrlKey || ev.metaKey)) area.form.requestSubmit();
    });
    const done = aids.dialog(t("editor.markdownTitle"), [
      h("label", { class: "field" }, h("span", { text: t("editor.markdownText") }), area),
      h("p", { class: "muted", id: "pm-markdown-hint", text: t("editor.markdownHint") }),
    ], { ok: t("editor.markdownInsert"), submit: () => {
      const reason = insertMarkdown(area.value, true);
      if (reason) throw new Error(markdownError(reason));
      return true;
    } });
    setTimeout(() => area.focus(), 0);
    await done;
    view.focus();
  };
  // A picture from the album (ADR-0054) is copied into this item, unless
  // it is one of its own files already.
  const fromAlbum = async () => {
    status.textContent = "";
    try {
      const p = await pickFromAlbum();
      if (!p) return;
      await placeAlbum(p);
    } catch (err) {
      status.textContent = err.message;
    }
  };
  const status = h("span", { class: "error", role: "status" });
  // An item shown in the text, chosen by searching (ADR-0096).
  const embed = async () => {
    status.textContent = "";
    const it = await aids.pickItem(t("assist.embedItem"));
    if (!it) return view.focus();
    insertBlock(schema.nodes.embed.create({ ref: it.id, label: `${it.key} ${it.title}`, href: "#/item/" + encodeURIComponent(it.key) }));
  };
  // A new task, made as a child of this item, goes into the text as a
  // link or shown whole (ADR-0114).
  const newTask = async () => {
    status.textContent = "";
    try {
      const made = await tasks.create();
      if (!made) return view.focus();
      const { item: it, display } = made;
      const label = `${it.key} ${it.title}`;
      insertBlock(display === "full"
        ? schema.nodes.embed.create({ ref: it.id, label, href: "#/item/" + encodeURIComponent(it.key) })
        : schema.nodes.paragraph.create(null, schema.nodes.mention.create({ ref: it.id, label })));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // An item made from a link takes the place of a paragraph that holds
  // the link alone, shown whole; elsewhere it goes after the paragraph.
  const placeEmbed = (it, address) => {
    const node = schema.nodes.embed.create({ ref: it.id, label: `${it.key} ${it.title}`, href: "#/item/" + encodeURIComponent(it.key) });
    const { $from } = view.state.selection;
    const block = $from.depth >= 1 ? $from.node(1) : null;
    // The paragraph holds the link alone when all its text but spaces
    // is the link.
    let alone = Boolean(address && block?.isTextblock && block.childCount);
    block?.forEach((n) => {
      if (!n.isText || (n.text.trim() && !n.marks.some((m) => m.type === schema.marks.link && m.attrs.href === address))) alone = false;
    });
    if (alone) {
      view.dispatch(view.state.tr.replaceWith($from.before(1), $from.after(1), node));
      view.focus();
    } else {
      insertBlock(node);
    }
  };
  // A Markdown file of a repository becomes a page below this item, which
  // the text shows whole (ADR-0138). From a link's bubble, it takes the
  // place of a paragraph that holds the link alone.
  const markdownFile = async (address) => {
    status.textContent = "";
    try {
      const it = await markdown.insert(address);
      if (!it) return view.focus();
      placeEmbed(it, address);
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // The page a link leads to becomes a web link item, which the text
  // shows as its card (ADR-0208), in place of a paragraph that holds the
  // link alone.
  saveLink = async (address) => {
    status.textContent = "";
    try {
      const made = await weblinks.save(address);
      if (!made) return view.focus();
      placeEmbed(made.item, address);
      if (made.problem) status.textContent = made.problem;
    } catch (err) {
      status.textContent = err.message;
    }
  };
  showFile = markdownFile;
  // A list of the tasks that carry a label (ADR-0114), or that a query
  // selects (ADR-0151).
  const taskList = async () => {
    status.textContent = "";
    const spec = await tasks.pickLabel();
    if (!spec) return view.focus();
    insertBlock(schema.nodes.tasklist.create(spec));
  };
  // A view of items (ADR-0203): what it shows is chosen as at #/view.
  const itemView = async () => {
    status.textContent = "";
    try {
      const spec = await pickView({});
      if (!spec) return view.focus();
      insertBlock(schema.nodes.itemview.create({ layout: spec.layout, itemType: spec.itemType || "", space: spec.space || "", query: spec.query || "",
        group: spec.group || "", columns: spec.columns?.length ? spec.columns : null, sort: spec.sort || "" }));
    } catch (err) {
      status.textContent = err.message;
    }
  };
  // Commenting on a passage works while reading too (ADR-0047).
  const commentButton = comment ? h("button", { type: "button", id: "comment-selection", text: t("editor.comment"), onclick: () => {
    status.textContent = "";
    const anchor = anchorOf(view.state);
    if (!anchor) status.textContent = t("editor.commentNeedsSelection");
    else comment(anchor);
  } }) : null;
  // A personal note on a passage, seen by its maker alone (ADR-0107).
  const noteButton = note ? h("button", { type: "button", id: "note-selection", text: t("editor.note"), title: t("editor.noteTitle"), onclick: () => {
    status.textContent = "";
    const anchor = anchorOf(view.state);
    if (!anchor) status.textContent = t("editor.noteNeedsSelection");
    else note(anchor);
  } }) : null;
  // Protecting starts from the text read (ADR-0088): the words selected,
  // the blocks the selection touches, or the block last clicked, such as a
  // picture (ADR-0160).
  let clicked = null;
  if (protect) {
    view.dom.addEventListener("mousedown", (ev) => {
      clicked = null;
      view.state.doc.forEach((node, offset, i) => {
        const dom = view.nodeDOM(offset);
        if (dom && dom.contains?.(ev.target)) clicked = { from: i, to: i };
      });
    });
  }
  const protectButton = protect ? h("button", { type: "button", id: "protect-selection", text: t("editor.protect"), onclick: () => {
    status.textContent = "";
    const passage = selectionInBlock(view.state);
    const range = view.state.selection.empty ? clicked : blockRangeOf(view.state);
    if (!passage && !range) status.textContent = t("editor.protectNeedsSelection");
    else protect({ passage, range });
  } }) : null;
  // The toolbar's groups; buttons that show a state (a mark, an alignment)
  // are pressed while it applies at the selection (ADR-0059).
  const group = (...els) => h("span", { class: "pm-group" }, ...els.filter(Boolean));
  const toggle = (title, label, cmd, isOn, cls) => {
    const el = h("button", { type: "button", title, "aria-label": title, "aria-pressed": "false", class: cls || null, onclick: run(cmd) }, label);
    return { el, isOn };
  };
  const markButton = (name, title, label) => toggle(title, label, toggleMark(schema.marks[name]), (st) => markActive(st, schema.marks[name]));
  const markButtons = [
    markButton("bold", t("editor.bold"), h("strong", { text: "B" })),
    markButton("italic", t("editor.italic"), h("em", { text: "I" })),
    markButton("underline", t("editor.underline"), h("u", { text: "U" })),
    markButton("strike", t("editor.strike"), h("s", { text: "S" })),
    markButton("highlight", t("editor.highlight"), h("mark", { text: "ab" })),
    markButton("code", t("editor.code"), "</>"),
  ];
  // An alignment icon: four lines, drawn by the stylesheet.
  const alignIcon = (a) => h("span", { class: "pm-ico pm-ico-" + a, "aria-hidden": "true" }, h("i"), h("i"), h("i"), h("i"));
  const alignButtons = [["", t("editor.alignLeft"), "left"], ["center", t("editor.alignCenter"), "center"], ["right", t("editor.alignRight"), "right"], ["justify", t("editor.alignJustify"), "justify"]]
    .map(([a, title, icon]) => toggle(title, alignIcon(icon), setAlign(a), (st) => alignOf(st) === a));
  const listButton = (kind, label, title) => h("button", { type: "button", title, "aria-label": title, text: label, onclick: () => {
    const node = schema.nodes[kind];
    const inIt = view.state.selection.$from.parent.type === node;
    run(setBlockType(inIt ? schema.nodes.paragraph : node))();
  } });
  const stateful = [...markButtons, ...alignButtons];
  // The block kind shows the kind of the block the selection starts in, so
  // choosing a kind always changes the choice and applies at once; a block
  // the choice does not list, such as a picture, leaves it empty.
  const kindOf = (st) => {
    const node = st.selection.$from.parent;
    const name = node.type.name;
    return name === "heading" ? `heading:${node.attrs.level}` : name === "panel" ? `panel:${node.attrs.tone}` : name;
  };
  const refresh = (st) => {
    const kind = kindOf(st);
    if ([...kindSelect.options].some((o) => o.value === kind)) kindSelect.value = kind;
    else kindSelect.selectedIndex = -1;
    for (const b of stateful) b.el.setAttribute("aria-pressed", String(Boolean(b.isOn(st))));
    const canAlign = alignOf(st) !== null;
    for (const b of alignButtons) b.el.disabled = !canAlign;
  };
  const toolbar = !editable
    // The text's own actions, such as editing it, stand beside them.
    ? (commentButton || noteButton || protectButton || actions.length ? h("div", { class: "pm-toolbar pm-readbar", role: "toolbar" }, commentButton, noteButton, protectButton, ...actions, status) : null)
    : h("div", { class: "pm-toolbar", role: "toolbar", "aria-label": t("editor.toolbar") },
    group(kindSelect),
    group(...markButtons.map((b) => b.el)),
    group(...alignButtons.map((b) => b.el)),
    group(listButton("bullet", "•", t("editor.bullet")), listButton("ordered", "1.", t("editor.ordered")), listButton("task", "☑", t("editor.task"))),
    group(
      h("button", { type: "button", id: "pm-link", title: t("assist.linkTitle"), "aria-label": t("assist.linkTitle"), text: t("editor.link"), onclick: () => aids.openLink(view) }),
      h("button", { type: "button", id: "pm-at-button", title: t("assist.mentionTitle"), "aria-label": t("assist.mentionTitle"), text: "@", onclick: () => {
        const { from, to } = view.state.selection;
        const before = from > 0 ? view.state.doc.textBetween(from - 1, from, null, " ") : "";
        view.dispatch(view.state.tr.insertText(before && !/\s/.test(before) ? " @" : "@", from, to));
        view.focus();
      } })),
    group(insertSelect, picker, videoPicker, audioPicker, modelPicker),
    commentButton,
    h("button", { type: "button", title: t("editor.undo"), text: "↶", onclick: run(undo) }),
    h("button", { type: "button", title: t("editor.redo"), text: "↷", onclick: run(redo) }),
    // The text's own actions, such as ending the editing, stand where
    // they stand while reading (ADR-0091).
    actions.length ? h("span", { class: "pm-actions" }, ...actions) : null,
    status,
  );
  if (editable) {
    onUpdate = refresh;
    refresh(view.state);
  }
  return { element: h("div", { class: "pm-wrap" }, toolbar, mount), blocks: () => blocksFromDoc(view.state.doc), view };
}
