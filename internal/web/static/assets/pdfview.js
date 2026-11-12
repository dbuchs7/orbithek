// The PDF reader (ADR-0210): a PDF file of an item drawn in the browser by
// pdf.js (vendored, run from this server), page by page as they come into
// view, each with its text laid over it so it can be selected and
// searched. A side panel shows the pages as thumbnails, the document's
// table of contents and its highlights. Whoever may comment on the item
// selects text on a page and highlights it in a colour, with a note or
// without: the highlight is a comment anchored in the PDF, by its first
// version, page, quote and rectangles on the page; the item's comments
// show it too. Nothing here turns text into markup (ADR-0014): the text
// layer is pdf.js's own, made of elements it creates.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, post, put, del } from "./api.js";
import { COLORS, readState, readerHref, normRects, locate, squash, highlightsOf, highlightsMarkdown, zoomIn, zoomOut, clampPage } from "./pdfparts.js";

// pdf.js is loaded once, when the first PDF is read; its worker comes from
// this server too, so the UI's content security policy holds.
let loaded = null;
const loadPdfjs = () => (loaded ??= import("/vendor/pdf.js").then((lib) => {
  lib.GlobalWorkerOptions.workerSrc = "/vendor/pdf.worker.js";
  return lib;
}));

// CSS_UNITS makes a zoom of 1 show a page at its printed size.
const CSS_UNITS = 96 / 72;

// colorName is a colour's name, its key written out for the catalogue's
// tests (ADR-0017).
const colorName = (c) => ({ yellow: t("pdf.colorYellow"), green: t("pdf.colorGreen"), blue: t("pdf.colorBlue"),
  pink: t("pdf.colorPink"), purple: t("pdf.colorPurple") })[c] || t("pdf.colorYellow");

// readAllComments reads every comment of an item, page by page.
async function readAllComments(path) {
  const out = [];
  let after = "";
  for (;;) {
    const page = await get(`${path}/comments?limit=500${after ? "&after=" + encodeURIComponent(after) : ""}`);
    out.push(...page.comments);
    if (!page.next) return out;
    after = page.next;
  }
}

// textNodes lists the text nodes of an element in document order.
function textNodes(el) {
  const out = [];
  const walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
  for (let n = walk.nextNode(); n; n = walk.nextNode()) out.push(n);
  return out;
}

// rangeAt makes a DOM range over the characters start to end of an
// element's text.
function rangeAt(el, start, end) {
  const range = document.createRange();
  let at = 0, began = false;
  for (const n of textNodes(el)) {
    const len = n.data.length;
    if (!began && start < at + len) {
      range.setStart(n, start - at);
      began = true;
    }
    if (began && end <= at + len) {
      range.setEnd(n, end - at);
      return range;
    }
    at += len;
  }
  return null;
}

// within is where an element stands within a scrolled container it lies
// in, in pixels from the container's top.
function within(container, el) {
  return el.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop;
}

// scrollTo scrolls a container alone, not the page around it, so that an
// element stands at its top, or at its middle when centered.
function scrollTo(container, el, centered = false) {
  const at = within(container, el);
  container.scrollTo({ top: Math.max(0, centered ? at - container.clientHeight / 2 : at - 8) });
}

// pdfView shows the reader for a file of an item. deps are the shell's:
// show mounts the view, person names a user, learnNames learns names,
// onLeave ends what the view started, here keeps the room for the top bar,
// user is the signed-in user's ID, nameOf names a user by their username,
// for the export, which others read.
export async function pdfView(ref, file, params, { show, person, nameOf, learnNames, onLeave, here, errorBox, user }) {
  const { item, editable } = await get("/api/items/" + encodeURIComponent(ref));
  here.space = item.space;
  const path = "/api/items/" + item.id;
  const pdf = await get(`${path}/pdfs/${encodeURIComponent(file)}`);
  const newest = pdf.newest;
  const fileURL = `${path}/attachments/${newest.id}`;
  const canMark = editable && item.presence === "live";
  const state = readState(params);
  let comments = await readAllComments(path);
  await learnNames(comments.map((c) => c.author)).catch(() => {});

  const status = h("div", { class: "status", role: "status" });
  const pagesEl = h("div", { class: "pdf-pages", id: "pdf-pages", tabindex: "0" });
  const side = h("aside", { class: "pdf-side", id: "pdf-side", "aria-label": t("pdf.side") });
  const pageInput = h("input", { type: "number", min: "1", value: String(state.page), id: "pdf-page", class: "pdf-page-input", "aria-label": t("pdf.page") });
  const pageCount = h("span", { class: "muted", id: "pdf-page-count" });
  const zoomText = h("span", { class: "pdf-zoom-text", id: "pdf-zoom", "aria-live": "polite" });
  const searchInput = h("input", { type: "search", id: "pdf-search", placeholder: t("pdf.search"), "aria-label": t("pdf.search") });
  const searchText = h("span", { class: "muted pdf-search-count", id: "pdf-search-count", "aria-live": "polite" });
  const button = (id, label, text, onclick) => h("button", { type: "button", class: "secondary pdf-tool", id, "aria-label": label, title: label, text, onclick });
  const toolbar = h("div", { class: "pdf-toolbar", role: "toolbar", "aria-label": t("pdf.toolbar") },
    h("span", { class: "pdf-group" },
      button("pdf-prev", t("pdf.prev"), "‹", () => go(current - 1)),
      pageInput, pageCount,
      button("pdf-next", t("pdf.next"), "›", () => go(current + 1))),
    h("span", { class: "pdf-group" },
      button("pdf-zoom-out", t("pdf.zoomOut"), "−", () => setZoom(zoomOut(zoom))),
      zoomText,
      button("pdf-zoom-in", t("pdf.zoomIn"), "+", () => setZoom(zoomIn(zoom))),
      h("button", { type: "button", class: "secondary pdf-tool", id: "pdf-fit", text: t("pdf.fitWidth"), onclick: () => setZoom(fitWidth()) })),
    h("span", { class: "pdf-group pdf-search" },
      searchInput,
      button("pdf-search-prev", t("pdf.searchPrev"), "↑", () => step(-1)),
      button("pdf-search-next", t("pdf.searchNext"), "↓", () => step(1)),
      searchText),
    h("a", { href: fileURL, download: newest.name, class: "button secondary pdf-download", id: "pdf-download", text: t("pdf.download") }));
  const selbar = h("div", { class: "pdf-selbar", id: "pdf-selbar", hidden: true, role: "group", "aria-label": t("pdf.highlight") });
  const view = h("div", { class: "pdf-reader", id: "pdf-reader" },
    h("div", { class: "pdf-head" },
      h("a", { href: "#/item/" + encodeURIComponent(item.key), class: "pdf-back", id: "pdf-back", text: "← " + (item.title || item.key) }),
      h("h1", { class: "pdf-title", text: newest.name })),
    toolbar, status,
    h("div", { class: "pdf-body" }, side, h("div", { class: "pdf-stage" }, pagesEl, selbar)));
  show(view);
  document.body.classList.add("pdf-open");
  status.textContent = t("pdf.loading");
  // What the reader starts ends when another view replaces it.
  let task = null, seen = null, thumbs = null, typing = 0;
  onLeave(() => {
    seen?.disconnect();
    thumbs?.disconnect();
    clearTimeout(typing);
    document.body.classList.remove("pdf-open");
    task?.destroy();
  });

  const lib = await loadPdfjs();
  task = lib.getDocument({
    url: fileURL, isEvalSupported: false, enableXfa: false, useWasm: false,
    wasmUrl: "/vendor/pdfjs/wasm/", cMapUrl: "/vendor/pdfjs/cmaps/", cMapPacked: true, standardFontDataUrl: "/vendor/pdfjs/standard_fonts/",
  });
  task.onPassword = (update) => {
    const pw = prompt(t("pdf.password"));
    if (pw === null) task.destroy();
    else update(pw);
  };
  let doc;
  try {
    doc = await task.promise;
  } catch (err) {
    status.replaceChildren(errorBox(new Error(t("pdf.unreadable", { error: err?.message || String(err) }))));
    return;
  }
  status.textContent = "";

  // --- pages ------------------------------------------------------------
  const count = doc.numPages;
  pageCount.textContent = t("pdf.pageOf", { n: count });
  pageInput.max = String(count);
  const first = await doc.getPage(1);
  const base = first.getViewport({ scale: CSS_UNITS });
  let zoom = 1;
  let current = 1;
  const pages = []; // per page: {el, canvas, text, marks, rendered, task, viewport}
  const fitWidth = () => Math.max(0.25, Math.min(4, (pagesEl.clientWidth - 48) / base.width));
  for (let n = 1; n <= count; n++) {
    const canvas = h("canvas", { class: "pdf-canvas" });
    const text = h("div", { class: "textLayer" });
    const marks = h("div", { class: "pdf-marks" });
    const el = h("div", { class: "pdf-page", "data-page": String(n), role: "region", "aria-label": t("pdf.pageN", { n }) }, canvas, text, marks);
    pages.push({ n, el, canvas, text, marks, rendered: false, size: null });
    pagesEl.append(el);
  }
  const sizePage = (p) => {
    const w = (p.size?.width ?? base.width) * zoom, hgt = (p.size?.height ?? base.height) * zoom;
    p.el.style.width = w + "px";
    p.el.style.height = hgt + "px";
    p.el.style.setProperty("--total-scale-factor", String(zoom * CSS_UNITS));
    p.el.style.setProperty("--scale-round-x", "1px");
    p.el.style.setProperty("--scale-round-y", "1px");
  };

  let generation = 0;
  // render draws a page once; a caller that comes while it is being drawn
  // waits for that drawing, so that what it then looks for (the marks of a
  // highlight to reveal) is there.
  function render(p) {
    if (!p.rendered) {
      p.rendered = true;
      p.drawing = drawPage(p);
    }
    return p.drawing;
  }
  async function drawPage(p) {
    const gen = generation;
    const page = await doc.getPage(p.n);
    if (gen !== generation) return; // the zoom changed meanwhile
    const viewport = page.getViewport({ scale: zoom * CSS_UNITS });
    p.size = page.getViewport({ scale: CSS_UNITS });
    sizePage(p);
    const ratio = window.devicePixelRatio || 1;
    p.canvas.width = Math.floor(viewport.width * ratio);
    p.canvas.height = Math.floor(viewport.height * ratio);
    p.canvas.style.width = viewport.width + "px";
    p.canvas.style.height = viewport.height + "px";
    try {
      p.task = page.render({ canvasContext: p.canvas.getContext("2d"), viewport, transform: ratio !== 1 ? [ratio, 0, 0, ratio, 0, 0] : null });
      await p.task.promise;
      if (gen !== generation) return;
      p.text.replaceChildren();
      await new lib.TextLayer({ textContentSource: page.streamTextContent(), container: p.text, viewport }).render();
    } catch (err) {
      if (err?.name !== "RenderingCancelledException") status.replaceChildren(errorBox(err));
      return;
    }
    if (gen !== generation) return;
    p.el.dataset.rendered = "1";
    drawMarks(p);
    drawHit(p);
  }
  seen = new IntersectionObserver((entries) => {
    for (const e of entries) if (e.isIntersecting) render(pages[Number(e.target.dataset.page) - 1]);
  }, { root: pagesEl, rootMargin: "600px 0px" });
  pages.forEach((p) => { sizePage(p); seen.observe(p.el); });

  function setZoom(next) {
    const keep = current;
    zoom = next;
    generation++;
    zoomText.textContent = Math.round(zoom * 100) + " %";
    for (const p of pages) {
      p.task?.cancel?.();
      p.rendered = false;
      delete p.el.dataset.rendered;
      p.text.replaceChildren();
      p.marks.replaceChildren();
      sizePage(p);
      seen.unobserve(p.el);
      seen.observe(p.el);
    }
    go(keep);
  }
  zoomText.textContent = "100 %";

  // go shows a page; the address follows without a new step in history.
  function go(n) {
    const to = clampPage(n, count);
    scrollTo(pagesEl, pages[to - 1].el);
    setCurrent(to);
  }
  function setCurrent(n) {
    if (n === current && pageInput.value === String(n)) return;
    current = n;
    pageInput.value = String(n);
    history.replaceState(null, "", readerHref(item.key, file, { page: n }));
    side.querySelectorAll(".pdf-thumb.current").forEach((el) => el.classList.remove("current"));
    side.querySelector(`.pdf-thumb[data-page="${n}"]`)?.classList.add("current");
  }
  pageInput.addEventListener("change", () => go(Number(pageInput.value)));
  let ticking = false;
  pagesEl.addEventListener("scroll", () => {
    if (ticking) return;
    ticking = true;
    requestAnimationFrame(() => {
      ticking = false;
      const mid = pagesEl.scrollTop + pagesEl.clientHeight / 3;
      const p = pages.find((x) => x.el.offsetTop + x.el.offsetHeight > mid) || pages.at(-1);
      setCurrent(p.n);
    });
  });

  // --- page texts, for finding quotes and searching ----------------------
  const texts = new Map();
  const pageText = (n) => {
    if (!texts.has(n)) {
      texts.set(n, doc.getPage(n).then((page) => page.getTextContent()).then((tc) => tc.items.map((i) => (i.str || "") + (i.hasEOL ? "\n" : "")).join("")));
    }
    return texts.get(n);
  };
  // rectsOf measures the characters start to end of a page's text layer
  // as rectangles in fractions of the page.
  const rectsOf = (p, start, end) => {
    const range = rangeAt(p.text, start, end);
    return range ? normRects([...range.getClientRects()], p.el.getBoundingClientRect()) : [];
  };
  // squashedRange turns a match in the squashed text of a page's text
  // layer into the layer's own offsets.
  const squashedRange = (p, at, length) => {
    const { map } = squash(p.text.textContent);
    return at + length - 1 < map.length ? { start: map[at], end: map[at + length - 1] + 1 } : null;
  };

  // --- highlights ---------------------------------------------------------
  // places holds where each highlight is in the newest version: its page,
  // and its rectangles, given or measured once its page's text is laid out;
  // found false when its quote is not in this version.
  const places = new Map();
  let highlights = [];
  async function placeAll() {
    highlights = highlightsOf(comments, pdf.first);
    for (const { root } of highlights) {
      if (places.has(root.id)) continue;
      const a = root.anchor;
      if (a.version === newest.id && a.rects?.length) {
        places.set(root.id, { page: a.page, rects: a.rects, found: true });
        continue;
      }
      // Made on another version, or by an agent without rectangles: its
      // quote is looked for on its page first, then on every page.
      const order = [a.page, ...Array.from({ length: count }, (_, i) => i + 1).filter((n) => n !== a.page)].filter((n) => n >= 1 && n <= count);
      let place = { page: a.page, rects: null, found: false };
      for (const n of order) {
        if (locate(await pageText(n), a.quote, a.prefix, a.suffix)) {
          place = { page: n, rects: null, found: true };
          break;
        }
      }
      places.set(root.id, place);
    }
  }
  function drawMarks(p) {
    if (!p.el.dataset.rendered) return;
    p.marks.querySelectorAll(".pdf-mark").forEach((m) => m.remove());
    for (const { root } of highlights) {
      const place = places.get(root.id);
      if (!place?.found || place.page !== p.n) continue;
      if (!place.rects) {
        const r = locate(p.text.textContent, root.anchor.quote, root.anchor.prefix, root.anchor.suffix);
        place.rects = r ? rectsOf(p, r.start, r.end) : [];
      }
      for (const r of place.rects) {
        const m = h("div", { class: `pdf-mark hl-${root.anchor.color || COLORS[0]}${root.resolved ? " resolved" : ""}`, "data-comment": root.id,
          title: root.text || root.anchor.quote, onclick: () => focusEntry(root.id) });
        Object.assign(m.style, { left: r.x * 100 + "%", top: r.y * 100 + "%", width: r.w * 100 + "%", height: r.h * 100 + "%" });
        p.marks.append(m);
      }
    }
  }
  const redrawMarks = () => pages.forEach(drawMarks);

  // reveal shows a highlight on its page and marks it for a moment.
  async function reveal(id) {
    const place = places.get(id);
    if (!place?.found) return;
    const p = pages[place.page - 1];
    go(place.page);
    await render(p);
    const marks = p.marks.querySelectorAll(`.pdf-mark[data-comment="${CSS.escape(id)}"]`);
    if (marks[0]) scrollTo(pagesEl, marks[0], true);
    marks.forEach((m) => m.classList.add("focus"));
    setTimeout(() => marks.forEach((m) => m.classList.remove("focus")), 1600);
  }

  // --- the side panel -----------------------------------------------------
  const outline = await doc.getOutline().catch(() => null);
  let tab = state.comment ? "highlights" : "pages";
  const panel = h("div", { class: "pdf-panel", id: "pdf-panel" });
  const tabs = h("nav", { class: "tabs pdf-tabs", "aria-label": t("pdf.side") });
  const drawTabs = () => tabs.replaceChildren(...[
    ["pages", t("pdf.tabPages")],
    outline?.length ? ["outline", t("pdf.tabOutline")] : null,
    ["highlights", t("pdf.tabHighlights", { n: highlights.length })],
  ].filter(Boolean).map(([id, label]) => h("button", { type: "button", class: "tab" + (tab === id ? " active" : ""), id: "pdf-tab-" + id, text: label,
    "aria-pressed": String(tab === id), onclick: () => { tab = id; drawSide(); } })));
  side.append(tabs, panel);

  function pagesPanel() {
    thumbs?.disconnect();
    const list = h("ol", { class: "pdf-thumbs" });
    thumbs = new IntersectionObserver((entries) => {
      for (const e of entries) {
        if (!e.isIntersecting || e.target.dataset.drawn) continue;
        e.target.dataset.drawn = "1";
        const n = Number(e.target.dataset.page);
        doc.getPage(n).then((page) => {
          const vp = page.getViewport({ scale: 1 });
          const viewport = page.getViewport({ scale: 120 / vp.width });
          const c = e.target.querySelector("canvas");
          c.width = Math.floor(viewport.width);
          c.height = Math.floor(viewport.height);
          return page.render({ canvasContext: c.getContext("2d"), viewport }).promise;
        }).catch(() => {});
      }
    }, { root: panel, rootMargin: "200px 0px" });
    for (let n = 1; n <= count; n++) {
      const li = h("li", { class: "pdf-thumb" + (n === current ? " current" : ""), "data-page": String(n) },
        h("button", { type: "button", class: "link", "aria-label": t("pdf.pageN", { n }), onclick: () => go(n) },
          h("canvas", { class: "pdf-thumb-canvas" }), h("span", { class: "muted", text: String(n) })));
      list.append(li);
      thumbs.observe(li);
    }
    return list;
  }
  function outlinePanel(entries) {
    return h("ul", { class: "pdf-outline" }, entries.map((o) => h("li", {},
      o.url
        ? h("a", { href: o.url, target: "_blank", rel: "noopener noreferrer", text: o.title })
        : h("button", { type: "button", class: "link", text: o.title, onclick: async () => {
          const dest = typeof o.dest === "string" ? await doc.getDestination(o.dest) : o.dest;
          if (Array.isArray(dest) && dest[0]) go(typeof dest[0] === "object" ? (await doc.getPageIndex(dest[0])) + 1 : dest[0] + 1);
        } }),
      o.items?.length ? outlinePanel(o.items) : null)));
  }
  function highlightsPanel() {
    const exportButton = h("button", { type: "button", class: "secondary", id: "pdf-export", text: t("pdf.export"), disabled: !highlights.length, onclick: exportMarkdown });
    if (!highlights.length) {
      return h("div", {}, h("p", { class: "empty", id: "pdf-no-highlights", text: canMark ? t("pdf.noHighlights") : t("pdf.noHighlightsRead") }));
    }
    return h("div", {}, h("p", {}, exportButton),
      h("ul", { class: "pdf-highlights", id: "pdf-highlights" }, highlights.map(entry)));
  }
  function entry({ root, replies }) {
    const place = places.get(root.id);
    const st = h("div", { class: "status" });
    const act = async (fn) => {
      try {
        await fn();
        await refresh();
      } catch (err) { st.replaceChildren(errorBox(err)); }
    };
    const mine = canMark && root.author === user;
    const replyBox = h("div");
    const reply = () => {
      const area = h("textarea", { rows: 2, required: true, "aria-label": t("comment.replyLabel") });
      replyBox.replaceChildren(h("form", { class: "comment-form", onsubmit: (ev) => {
        ev.preventDefault();
        act(() => post(path + "/comments", { text: area.value, reply: root.id }));
      } }, area, h("p", {}, h("button", { type: "submit", text: t("comment.reply") }),
        h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: () => replyBox.replaceChildren() }))));
      area.focus();
    };
    return h("li", { class: "pdf-highlight" + (root.resolved ? " resolved" : ""), "data-comment": root.id, id: "pdf-hl-" + root.id },
      h("div", { class: "pdf-highlight-head" },
        h("span", { class: `pdf-swatch hl-${root.anchor.color || COLORS[0]}`, title: colorName(root.anchor.color), "aria-label": colorName(root.anchor.color) }),
        h("span", { class: "muted", text: t("pdf.pageN", { n: place?.page || root.anchor.page }) }),
        root.resolved ? h("span", { class: "chip", text: t("pdf.resolved") }) : null),
      h("blockquote", { class: "comment-quote" },
        h("button", { type: "button", class: "link pdf-goto", text: root.anchor.quote || t("pdf.pageN", { n: root.anchor.page }), disabled: !place?.found,
          onclick: () => reveal(root.id) })),
      place?.found ? null : h("p", { class: "muted pdf-not-found", text: t("pdf.notFound") }),
      root.anchor.version && root.anchor.version !== newest.id && place?.found ? h("p", { class: "muted", text: t("pdf.earlierVersion") }) : null,
      root.text ? h("p", { class: "comment-text", text: root.text }) : null,
      h("p", { class: "muted pdf-by", text: person(root.author) }),
      replies.length ? h("ul", { class: "comments replies" }, replies.map((r) => h("li", { class: "comment" },
        h("span", { class: "muted", text: person(r.author) + ": " }), h("span", { text: r.text })))) : null,
      canMark ? h("p", { class: "thread-tools" },
        h("button", { type: "button", class: "link", text: t("comment.reply"), onclick: reply }), " · ",
        root.resolved
          ? h("button", { type: "button", class: "link", text: t("comment.reopen"), onclick: () => act(() => del(`${path}/comments/${root.id}/resolved`)) })
          : h("button", { type: "button", class: "link", text: t("comment.resolve"), onclick: () => act(() => put(`${path}/comments/${root.id}/resolved`)) }),
        mine ? [" · ", h("button", { type: "button", class: "link danger", text: t("common.delete"),
          onclick: () => confirm(t("comment.deleteConfirm")) && act(() => del(`${path}/comments/${root.id}`)) })] : null) : null,
      h("p", {}, h("a", { href: "#/item/" + encodeURIComponent(item.key), class: "pdf-discuss", text: t("pdf.discuss") })),
      replyBox, st);
  }
  function drawSide() {
    drawTabs();
    if (tab === "outline" && outline?.length) panel.replaceChildren(outlinePanel(outline));
    else if (tab === "highlights") panel.replaceChildren(highlightsPanel());
    else panel.replaceChildren(pagesPanel());
  }
  function focusEntry(id) {
    if (tab !== "highlights") {
      tab = "highlights";
      drawSide();
    }
    const el = panel.querySelector(`[data-comment="${CSS.escape(id)}"]`);
    if (!el) return;
    scrollTo(panel, el);
    el.classList.add("focus");
    setTimeout(() => el.classList.remove("focus"), 1600);
  }

  // refresh reads the comments again and redraws what shows them.
  async function refresh() {
    comments = await readAllComments(path);
    await learnNames(comments.map((c) => c.author)).catch(() => {});
    await placeAll();
    redrawMarks();
    drawSide();
  }

  function exportMarkdown() {
    const md = highlightsMarkdown({
      title: t("pdf.exportTitle", { name: newest.name }), source: `${item.key} ${item.title || ""}`.trim(), highlights,
      page: (n) => t("pdf.pageN", { n }), person: nameOf, resolved: t("pdf.resolved"),
    });
    // A blob address is set directly: h() admits only addresses of pages.
    const a = h("a", { download: newest.name.replace(/\.pdf$/i, "") + "-highlights.md" });
    a.href = URL.createObjectURL(new Blob([md], { type: "text/markdown;charset=utf-8" }));
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(a.href), 1000);
  }

  // --- making a highlight ---------------------------------------------------
  let chosen = null; // the selection being highlighted: {page, quote, prefix, suffix, rects}
  const hideBar = () => { selbar.hidden = true; chosen = null; };
  function selected() {
    const sel = document.getSelection();
    if (!sel || sel.isCollapsed || !sel.rangeCount) return null;
    const range = sel.getRangeAt(0);
    const startPage = range.startContainer.parentElement?.closest?.(".pdf-page");
    const endPage = range.endContainer.parentElement?.closest?.(".pdf-page");
    if (!startPage || !pagesEl.contains(startPage)) return null;
    if (startPage !== endPage) return { page: 0 };
    const p = pages[Number(startPage.dataset.page) - 1];
    const quote = sel.toString().replace(/[ \t]+/g, " ").trim();
    if (!quote) return null;
    const before = document.createRange();
    before.setStart(p.text, 0);
    before.setEnd(range.startContainer, range.startOffset);
    const after = document.createRange();
    after.setStart(range.endContainer, range.endOffset);
    after.setEnd(p.text, p.text.childNodes.length);
    const rects = normRects([...range.getClientRects()], p.el.getBoundingClientRect());
    return { page: p.n, quote, prefix: before.toString().slice(-32), suffix: after.toString().slice(0, 32), rects, last: range.getBoundingClientRect() };
  }
  async function save(color, text) {
    const s = chosen;
    hideBar();
    document.getSelection()?.removeAllRanges();
    try {
      const { comment } = await post(path + "/comments", { text, anchor: { file: newest.id, page: s.page, quote: s.quote, prefix: s.prefix, suffix: s.suffix, rects: s.rects, color } });
      await refresh();
      focusEntry(comment.id);
    } catch (err) { status.replaceChildren(errorBox(err)); }
  }
  function noteForm(color) {
    const s = chosen;
    let pick = color;
    const swatches = h("span", { class: "pdf-colors" }, COLORS.map((c) => h("button", { type: "button", class: `pdf-swatch hl-${c}${c === pick ? " chosen" : ""}`,
      "aria-label": colorName(c), "aria-pressed": String(c === pick), title: colorName(c), onclick: (ev) => {
        pick = c;
        swatches.querySelectorAll(".pdf-swatch").forEach((b) => { b.classList.toggle("chosen", b === ev.currentTarget); b.setAttribute("aria-pressed", String(b === ev.currentTarget)); });
      } })));
    const area = h("textarea", { rows: 3, id: "pdf-note", "aria-label": t("pdf.note"), placeholder: t("pdf.note") });
    selbar.replaceChildren(h("form", { class: "pdf-note-form", onsubmit: (ev) => {
      ev.preventDefault();
      chosen = s;
      save(pick, area.value.trim());
    } }, h("blockquote", { class: "comment-quote", text: s.quote }), swatches, area,
    h("p", {}, h("button", { type: "submit", id: "pdf-note-save", text: t("pdf.save") }),
      h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: hideBar }))));
    area.focus();
  }
  function showBar(s) {
    chosen = s;
    if (!s.page) {
      selbar.replaceChildren(h("p", { class: "muted", text: t("pdf.onePage") }));
    } else {
      selbar.replaceChildren(
        ...COLORS.map((c) => h("button", { type: "button", class: `pdf-swatch hl-${c}`, "data-color": c, "aria-label": t("pdf.highlightIn", { color: colorName(c) }),
          title: t("pdf.highlightIn", { color: colorName(c) }), onmousedown: (ev) => ev.preventDefault(), onclick: () => save(c, "") })),
        h("button", { type: "button", class: "secondary", id: "pdf-with-note", text: t("pdf.withNote"), onmousedown: (ev) => ev.preventDefault(), onclick: () => noteForm(COLORS[0]) }));
    }
    const stage = selbar.parentElement.getBoundingClientRect();
    const r = s.last || { left: stage.left + 20, bottom: stage.top + 20 };
    selbar.hidden = false;
    selbar.style.left = Math.max(4, Math.min(r.left - stage.left, stage.width - selbar.offsetWidth - 4)) + "px";
    selbar.style.top = Math.max(4, r.bottom - stage.top + 6) + "px";
  }
  if (canMark) {
    const onUp = () => setTimeout(() => {
      if (selbar.contains(document.activeElement) && !selbar.hidden && chosen) return;
      const s = selected();
      if (s) showBar(s);
      else if (!selbar.querySelector("form")) hideBar();
    }, 0);
    pagesEl.addEventListener("mouseup", onUp);
    pagesEl.addEventListener("keyup", (ev) => { if (ev.shiftKey || ev.key === "Shift") onUp(); });
    pagesEl.addEventListener("scroll", () => { if (!selbar.querySelector("form")) hideBar(); });
  }

  // --- searching ------------------------------------------------------------
  let matches = [], hit = -1, query = "";
  async function search(q) {
    query = q;
    matches = [];
    hit = -1;
    const needle = squash(q.toLowerCase()).text;
    if (needle) {
      for (let n = 1; n <= count; n++) {
        const hay = squash((await pageText(n)).toLowerCase()).text;
        if (q !== query) return; // a newer search runs
        for (let at = hay.indexOf(needle); at >= 0; at = hay.indexOf(needle, at + 1)) matches.push({ page: n, at, length: needle.length });
      }
    }
    searchText.textContent = !needle ? "" : matches.length ? "" : t("pdf.noMatches");
    pages.forEach(drawHit);
    if (matches.length) step(1);
  }
  function step(d) {
    if (!matches.length) return;
    hit = (hit + d + matches.length) % matches.length;
    searchText.textContent = t("pdf.matches", { i: hit + 1, n: matches.length });
    const m = matches[hit];
    const p = pages[m.page - 1];
    go(m.page);
    render(p).then(() => {
      pages.forEach(drawHit);
      const el = p.marks.querySelector(".pdf-hit");
      if (el) scrollTo(pagesEl, el, true);
    });
  }
  function drawHit(p) {
    p.marks.querySelectorAll(".pdf-hit").forEach((m) => m.remove());
    const m = matches[hit];
    if (!m || m.page !== p.n || !p.el.dataset.rendered) return;
    const r = squashedRange(p, m.at, m.length);
    if (!r) return;
    for (const x of rectsOf(p, r.start, r.end)) {
      const el = h("div", { class: "pdf-hit" });
      Object.assign(el.style, { left: x.x * 100 + "%", top: x.y * 100 + "%", width: x.w * 100 + "%", height: x.h * 100 + "%" });
      p.marks.append(el);
    }
  }
  searchInput.addEventListener("input", () => {
    clearTimeout(typing);
    typing = setTimeout(() => search(searchInput.value), 300);
  });
  searchInput.addEventListener("keydown", (ev) => {
    if (ev.key !== "Enter") return;
    ev.preventDefault();
    if (searchInput.value !== query) search(searchInput.value);
    else step(ev.shiftKey ? -1 : 1);
  });

  await placeAll();
  drawSide();
  // The address may name a highlight (from the comments) or a page.
  if (state.comment && places.get(state.comment)?.found) {
    reveal(state.comment);
    focusEntry(state.comment);
  } else {
    go(state.page);
  }
  pagesEl.dataset.ready = "1";
}
