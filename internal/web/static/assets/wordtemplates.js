// Word templates of types (ADR-0167): a page of such a type read as its
// template's document, and the templates kept under Administration.
// Every element is made with h(); the frame holds text and checked values
// only (ADR-0014).

import { ApiError, del, get, post, put } from "./api.js";
import { h, when } from "./dom.js";
import { t } from "./i18n.js";
import { chromeOf, fieldText, knownClass, pictureSource, points, runStyle, tabSegments } from "./wordframe.js";

// ensureStylesheet links a template version's stylesheet once.
function ensureStylesheet(href) {
  if (!/^\/api\/word-templates\/[a-z][a-z0-9-]*\/versions\/\d+\/style\.css$/.test(href || "")) return;
  if ([...document.querySelectorAll("link[data-word-template]")].some((l) => l.dataset.wordTemplate === href)) return;
  document.head.append(h("link", { rel: "stylesheet", href, "data-word-template": href }));
}

function renderRun(r, ctx) {
  if (r.image) {
    const src = pictureSource(r.image.src);
    if (!src) return null;
    const img = h("img", { src, alt: "", class: "wt-img" });
    const w = points(r.image.width);
    const hh = points(r.image.height);
    if (w && hh) {
      img.style.width = w;
      img.style.height = hh;
    }
    if (r.image.align === "right" || r.image.align === "left") img.style.float = r.image.align;
    return img;
  }
  if (r.br) return h("br");
  if (r.field) return document.createTextNode(fieldText(r, ctx.sheet, ctx.sheets));
  const style = runStyle(r);
  if (!Object.keys(style).length) return document.createTextNode(r.text || "");
  const span = h("span", { text: r.text || "" });
  Object.assign(span.style, style);
  return span;
}

function renderPara(p, ctx) {
  const cls = knownClass(p.class);
  const segs = tabSegments(p.runs);
  const el = h("p", { class: "wt-p" + (cls ? " wt-" + cls : "") + (segs.length > 1 ? " wt-tabs" : "") });
  if (["left", "center", "right", "justify"].includes(p.align)) el.style.textAlign = p.align;
  if (segs.length > 1) {
    for (const seg of segs) el.append(h("span", { class: "wt-seg" }, seg.map((r) => renderRun(r, ctx))));
  } else {
    el.append(...(p.runs || []).map((r) => renderRun(r, ctx)).filter(Boolean));
  }
  if (!(p.runs || []).length) el.append(h("br"));
  return el;
}

function renderTable(tb, ctx) {
  return h("table", { class: "wt-table" + (tb.borders ? " wt-borders" : "") },
    h("tbody", {}, (tb.rows || []).map((row) => h("tr", {}, row.map((c) => {
      const td = h("td", { colspan: Number.isInteger(c.span) && c.span > 1 ? c.span : undefined }, renderNodes(c.nodes, ctx));
      const w = points(c.width);
      if (w) td.style.width = w;
      if (/^#[0-9A-Fa-f]{6}$/.test(c.fill || "")) td.style.backgroundColor = c.fill;
      return td;
    })))));
}

function renderNodes(nodes, ctx) {
  return (nodes || []).map((n) => {
    if (n.body) {
      ctx.placed = true;
      return h("div", { class: "wt-body" }, ctx.body);
    }
    if (n.table) return renderTable(n.table, ctx);
    if (n.p) return renderPara(n.p, ctx);
    return null;
  });
}

// documentView frames a page's text (the reading view's section) as its
// template's document: a sheet per page of the template's content, each
// with its header and footer, the text on the sheet the template places it.
// The text's bar of buttons stays above the sheets. canEdit offers to move
// the page to the template's newest version.
export function documentView(doc, body, { canEdit = false, onUpgrade } = {}) {
  ensureStylesheet(doc.stylesheet);
  const f = doc.frame || {};
  const sheets = (f.sheets && f.sheets.length) ? f.sheets : [[{ body: true }]];
  const bar = body.querySelector(".pm-readbar");
  const scope = /^wt-t-[a-z][a-z0-9-]*-\d+$/.test(doc.scope || "") ? doc.scope : "";
  const root = h("div", { class: "wt-doc " + scope, id: "word-document", role: "document", "aria-label": t("wt.document", { name: doc.template.name || doc.template.id }) });
  const ctx = { sheets: sheets.length, body, placed: false };
  sheets.forEach((nodes, i) => {
    ctx.sheet = i + 1;
    const { header, footer } = chromeOf(f, i);
    root.append(h("div", { class: "wt-sheet" },
      h("div", { class: "wt-head" }, renderNodes(header, ctx)),
      h("div", { class: "wt-main" }, renderNodes(nodes, ctx)),
      h("div", { class: "wt-foot" }, renderNodes(footer, ctx))));
  });
  if (!ctx.placed) root.lastChild?.querySelector(".wt-main")?.append(h("div", { class: "wt-body" }, body));
  const tpl = doc.template;
  const newer = tpl.newest > tpl.version;
  const note = h("p", { class: "muted wt-note", id: "word-template-note" },
    t("wt.basedOn", { name: tpl.name || tpl.id, version: tpl.version }),
    newer ? " " + t("wt.newer", { version: tpl.newest }) : "",
    newer && canEdit && onUpgrade ? [" ", h("button", { type: "button", class: "link", id: "word-template-upgrade", text: t("wt.upgrade", { version: tpl.newest }), onclick: onUpgrade })] : null);
  return h("div", { class: "wt-wrap" }, bar, note, root);
}

// uploadTemplate adds a template, or a new version of it.
async function uploadTemplate(id, name, file) {
  const q = new URLSearchParams({ file: file.name });
  if (name) q.set("name", name);
  const res = await fetch(`/api/word-templates/${encodeURIComponent(id)}?${q}`, {
    method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream" }, body: file,
  });
  let data = null;
  try { data = await res.json(); } catch { /* not JSON */ }
  if (!res.ok) throw new ApiError(res.status, (data && data.error) || res.statusText);
  return data;
}

// uploaded says which version was kept and what cleaning took out.
function uploaded(r) {
  const removed = r.template?.info?.removed || [];
  return t("wt.uploaded", { version: r.version }) + (removed.length ? " " + t("wt.removed", { list: removed.join(", ") }) : "");
}

// wordTemplatesCard lists the templates for administrators: their
// versions, what the newest holds and what uses them, with a new version,
// renaming, downloading and removing; and adds new ones.
export async function wordTemplatesCard() {
  const box = h("div", { id: "word-templates" });
  const status = h("div", { class: "status", id: "word-templates-status", role: "status" });
  const act = async (fn) => {
    status.replaceChildren();
    try {
      const done = await fn();
      await load();
      if (done) status.replaceChildren(h("p", { class: "notice success", text: done }));
    } catch (err) {
      status.replaceChildren(h("p", { class: "notice warning", text: err.message }));
    }
  };
  const entry = (tp) => {
    const newest = tp.versions[tp.versions.length - 1] || {};
    const info = tp.info;
    const used = tp.types.length > 0 || tp.items > 0;
    const file = h("input", { type: "file", accept: ".dotx,.docx", class: "wt-file", "aria-label": t("wt.newVersion") });
    file.addEventListener("change", () => file.files[0] && act(async () => {
      return uploaded(await uploadTemplate(tp.id, "", file.files[0]));
    }));
    return h("article", { class: "wt-entry", "data-template": tp.id },
      h("h3", {}, tp.name, " ", h("span", { class: "muted", text: tp.id })),
      h("p", { class: "muted", text: t("wt.versions", { n: tp.versions.length, file: newest.file || "", when: when(newest.at) }) }),
      tp.error ? h("p", { class: "notice warning", text: tp.error }) : null,
      info ? h("ul", { class: "wt-info" },
        h("li", { text: info.controls.length ? t("wt.controls", { list: info.controls.join(", ") }) : t("wt.noControls") }),
        h("li", { text: info.body ? t("wt.bodyMarked") : t("wt.bodyAtEnd") }),
        h("li", { text: [info.header ? t("wt.header") : t("wt.noHeader"), info.footer ? t("wt.footer") : t("wt.noFooter")].join(" · ") }),
        info.removed.length ? h("li", { text: t("wt.removed", { list: info.removed.join(", ") }) }) : null) : null,
      h("p", { class: "muted", text: t("wt.used", { types: tp.types.join(", ") || "–", items: tp.items }) }),
      h("div", { class: "actions" },
        h("label", { class: "wt-upload" }, h("span", { class: "muted", text: t("wt.newVersion") }), file),
        h("button", { type: "button", class: "link", text: t("wt.rename"), onclick: () => {
          const name = prompt(t("wt.renamePrompt"), tp.name);
          if (name) act(async () => { await put(`/api/word-templates/${encodeURIComponent(tp.id)}`, { name }); });
        } }),
        newest.version ? h("a", { href: `/api/word-templates/${encodeURIComponent(tp.id)}/versions/${newest.version}/file`, download: "", text: t("wt.download") }) : null,
        h("button", { type: "button", class: "link danger", text: t("wt.remove"), disabled: used, title: used ? t("wt.inUse") : undefined, onclick: () =>
          confirm(t("wt.removeConfirm", { name: tp.name })) && act(async () => { await del(`/api/word-templates/${encodeURIComponent(tp.id)}`); }) })));
  };
  const load = async () => {
    const data = await get("/api/word-templates");
    const idInput = h("input", { name: "id", required: true, pattern: "[a-z][a-z0-9\\-]{0,39}", title: t("wt.idHint") });
    const nameInput = h("input", { name: "name", required: true, maxlength: 80 });
    const fileInput = h("input", { name: "file", type: "file", accept: ".dotx,.docx", required: true });
    const add = h("form", { id: "word-template-new", class: "wt-new" },
      h("h3", { text: t("wt.add") }),
      h("label", { class: "field" }, h("span", { text: t("wt.id") }), idInput),
      h("label", { class: "field" }, h("span", { text: t("wt.name") }), nameInput),
      h("label", { class: "field" }, h("span", { text: t("wt.file", { mb: Math.round(data.maxBytes / (1 << 20)) }) }), fileInput),
      h("button", { type: "submit", text: t("wt.upload") }));
    add.addEventListener("submit", (ev) => {
      ev.preventDefault();
      act(async () => {
        return uploaded(await uploadTemplate(idInput.value.trim(), nameInput.value.trim(), fileInput.files[0]));
      });
    });
    box.replaceChildren(
      h("p", { class: "muted", text: t("wt.help", { prefix: data.controlPrefix }) }),
      h("p", { class: "muted", text: t("wt.helpType") }),
      data.templates.length ? h("div", { class: "wt-list" }, data.templates.map(entry)) : h("p", { class: "empty", text: t("wt.none") }),
      add);
  };
  await load();
  return h("div", {}, box, status);
}

// upgradeTemplate moves a page to its template's newest version.
export function upgradeTemplate(path) {
  return post(path + "/template");
}
