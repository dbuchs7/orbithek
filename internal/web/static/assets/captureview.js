// The capture view (ADR-0207): what a share to the installed app, the
// bookmarklet or a link passed on, shown to be saved into the wiki. A
// capture goes to one of its targets: a new web link item when it names a
// page (ADR-0206), a new item of a chosen type in a chosen room, or the
// end of an item found by searching. Nothing is saved before the person
// chooses and saves; every change goes through the API with their rights.
//
// The targets are data: TARGETS (capture.js) says which take a capture,
// TARGET_VIEWS here gives each its label, fields and action. Another
// target is one entry in each.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, post } from "./api.js";
import { TARGETS, captureBlocks, isWebAddress, readCapture } from "./capture.js";
import { appendToDailyNote } from "./daily.js";

const field = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);

// placeFields chooses a room one may create in and, if wished, an item
// there to go below.
function placeFields(ctx, prefix) {
  const space = h("select", { name: "space", id: prefix + "-space", required: true },
    ctx.spaces.map((s) => h("option", { value: s.key, text: `${s.name} (${s.key})`, selected: s.key === ctx.space })));
  const parent = h("select", { name: "parent", id: prefix + "-parent" });
  const fill = async () => {
    parent.replaceChildren(h("option", { value: "", text: t("capture.top") }));
    if (!space.value) return;
    const { items } = await get(`/api/items?space=${encodeURIComponent(space.value)}&limit=500`).catch(() => ({ items: [] }));
    parent.append(...items.slice().sort((a, b) => a.title.localeCompare(b.title)).map((it) => h("option", { value: it.id, text: `${it.title} (${it.key})` })));
  };
  space.addEventListener("change", fill);
  fill();
  return { elements: [field(t("capture.space"), space), field(t("capture.below"), parent)], values: () => ({ space: space.value, parent: parent.value }) };
}

// TARGET_VIEWS give each target its label, fields and action. run answers
// {item, problem}.
export const TARGET_VIEWS = {
  weblink: {
    label: () => t("capture.toWebLink"),
    fields(ctx) {
      const place = placeFields(ctx, "capture-weblink");
      const text = ctx.previews ? h("input", { type: "checkbox", name: "pagetext", id: "capture-pagetext" }) : null;
      return {
        elements: [...place.elements, text ? h("label", { class: "check" }, text, " ", t("capture.pageText")) : null],
        values: () => ({ ...place.values(), text: Boolean(text?.checked) }),
      };
    },
    run: (c, v) => post("/api/weblinks", { url: c.url, title: c.title, quote: c.text, space: v.space, parent: v.parent, text: v.text }),
  },
  // Today's daily note, made when missing in the room chosen for daily
  // notes; without that room the server says so (ADR-0203).
  daily: {
    label: () => t("capture.toDaily"),
    fields: () => ({ elements: [], values: () => ({}) }),
    run: (c) => appendToDailyNote([c.title, c.text, c.url].filter(Boolean).join("\n")).then(({ item }) => ({ item })),
  },
  item: {
    label: () => t("capture.toItem"),
    fields(ctx) {
      // Types whose items take a text and need no field filled in.
      const usable = ctx.types.filter((ty) => ty.body && !(ty.fields || []).some((f) => f.required));
      const type = h("select", { name: "type", id: "capture-type", required: true },
        usable.map((ty) => h("option", { value: ty.name, text: ty.name, selected: ty.name === "page" })));
      const place = placeFields(ctx, "capture-item");
      return { elements: [field(t("capture.type"), type), ...place.elements], values: () => ({ ...place.values(), type: type.value }) };
    },
    run: (c, v) => {
      // The title names the item; it stands in the body only as the name
      // of the source.
      let title = c.title;
      if (!title && c.url) try { title = new URL(c.url).hostname.replace(/^www\./, ""); } catch { /* no address */ }
      if (!title) title = [...c.text.replace(/\s+/g, " ")].slice(0, 80).join("").trim() || t("capture.untitled");
      const blocks = captureBlocks({ ...c, title: c.url ? c.title : "" }, t("capture.source"));
      return post("/api/items", { space: v.space, type: v.type, parent: v.parent, title, ...(blocks.length ? { body: blocks } : {}) });
    },
  },
  append: {
    label: () => t("capture.toAppend"),
    fields() {
      let chosen = null;
      const results = h("div", { class: "capture-results", id: "capture-results", role: "radiogroup", "aria-label": t("capture.results") });
      const search = h("input", { type: "search", name: "q", id: "capture-search", placeholder: t("capture.searchHint"), autocomplete: "off" });
      let timer = null;
      const find = async () => {
        const q = search.value.trim();
        if (!q) return results.replaceChildren();
        const { items } = await get(`/api/items?q=${encodeURIComponent(q)}&limit=10`).catch(() => ({ items: [] }));
        if (search.value.trim() !== q) return;
        chosen = null;
        results.replaceChildren(...(items.length ? items.map((it) => h("label", { class: "check capture-result" },
          h("input", { type: "radio", name: "append-to", value: it.id, onchange: () => { chosen = it; } }), " ", `${it.title} (${it.key})`))
          : [h("p", { class: "muted", text: t("capture.noResults") })]));
      };
      search.addEventListener("input", () => { clearTimeout(timer); timer = setTimeout(find, 250); });
      search.addEventListener("keydown", (ev) => { if (ev.key === "Enter") { ev.preventDefault(); clearTimeout(timer); find(); } });
      return {
        elements: [field(t("capture.searchItem"), search), results],
        values: () => {
          if (!chosen) throw new Error(t("capture.chooseItem"));
          return { item: chosen };
        },
      };
    },
    run: (c, v) => post(`/api/items/${encodeURIComponent(v.item.id)}/body/blocks`, { blocks: captureBlocks(c, t("capture.source")) }),
  },
};

// captureTargets are the targets with their views, in the order offered.
export function captureTargets() {
  return TARGETS.filter((tg) => TARGET_VIEWS[tg.id]).map((tg) => ({ ...tg, ...TARGET_VIEWS[tg.id] }));
}

// captureView shows a capture to be saved. ctx holds the rooms one may
// create in (spaces, favourites first), the room shown before (space),
// the types and whether the server reads pages (previews). done(item) is
// told what was saved.
export function captureView(initial, ctx, done = () => {}) {
  const title = h("input", { name: "title", id: "capture-title", value: initial.title, maxlength: 300 });
  const url = h("input", { name: "url", id: "capture-url", type: "url", value: initial.url, maxlength: 2048, placeholder: "https://" });
  const text = h("textarea", { name: "text", id: "capture-text", rows: "6", text: initial.text });
  text.value = initial.text;
  const preview = h("div", { class: "capture-preview muted", id: "capture-preview", role: "status" });
  const choices = h("fieldset", { class: "capture-targets", id: "capture-targets" }, h("legend", { text: t("capture.where") }));
  const fieldsBox = h("div", { class: "capture-fields", id: "capture-fields" });
  const status = h("div", { class: "status", role: "status" });
  const result = h("div", { id: "capture-done" });
  const save = h("button", { type: "submit", class: "primary", id: "capture-save", text: t("capture.save") });
  let current = null;
  let fields = null;
  const capture = () => readCapture({ title: title.value, text: text.value, url: url.value });

  // The targets follow what the capture holds; the one chosen stays while
  // it can.
  const offer = () => {
    const c = capture();
    const open = captureTargets().filter((tg) => tg.takes(c));
    const keep = open.find((tg) => tg.id === current?.id) || open[0] || null;
    choices.replaceChildren(h("legend", { text: t("capture.where") }), ...open.map((tg) => h("label", { class: "check" },
      h("input", { type: "radio", name: "target", value: tg.id, id: "capture-target-" + tg.id, checked: tg.id === keep?.id, onchange: () => choose(tg) }), " ", tg.label())));
    if (!open.length) choices.append(h("p", { class: "muted", text: t("capture.empty") }));
    if (keep?.id !== current?.id) choose(keep);
    save.disabled = !keep;
  };
  const choose = (tg) => {
    current = tg;
    fields = tg ? tg.fields(ctx) : null;
    fieldsBox.replaceChildren(...(fields ? fields.elements.filter(Boolean) : []));
  };

  // With previews on, a page's own title fills an empty title.
  let previewed = "";
  const lookUp = async () => {
    const address = url.value.trim();
    if (!ctx.previews || !isWebAddress(address) || address === previewed) return;
    previewed = address;
    preview.textContent = t("capture.reading");
    try {
      const p = await post("/api/linkpreview", { url: address });
      if (url.value.trim() !== address) return;
      if (!title.value.trim() && p.title) title.value = p.title;
      preview.replaceChildren(...[p.site, p.description].filter(Boolean).map((s) => h("span", { text: s })));
      offer();
    } catch (err) {
      preview.textContent = err.message;
    }
  };
  for (const el of [title, text]) el.addEventListener("input", offer);
  url.addEventListener("input", offer);
  url.addEventListener("change", lookUp);

  const formEl = h("form", { id: "capture-form", class: "capture-form" },
    field(t("capture.title"), title), field(t("capture.url"), url), preview, field(t("capture.text"), text),
    choices, fieldsBox, h("p", { class: "actions-row" }, save), status);
  formEl.addEventListener("submit", async (ev) => {
    ev.preventDefault();
    status.replaceChildren();
    if (!current) return;
    save.disabled = true;
    try {
      const res = await current.run(capture(), fields.values());
      const it = res.item;
      result.replaceChildren(h("section", { class: "notice success", id: "capture-saved" },
        h("p", {}, t("capture.saved"), " ", h("a", { href: "#/item/" + encodeURIComponent(it.key), id: "capture-open", text: `${it.key} ${it.title}` })),
        res.problem ? h("p", { class: "muted", id: "capture-problem", text: t("capture.problem", { problem: res.problem }) }) : null,
        // A window the bookmarklet opened may be closed again.
        window.opener ? h("button", { type: "button", id: "capture-close", text: t("capture.close"), onclick: () => window.close() }) : null));
      formEl.hidden = true;
      done(it);
    } catch (err) {
      status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
      save.disabled = false;
    }
  });
  offer();
  lookUp();
  return h("section", { class: "card capture", id: "capture" }, h("h1", { text: t("capture.heading") }),
    h("p", { class: "muted", text: t("capture.intro") }), formEl, result);
}
