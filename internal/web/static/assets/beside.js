// An item beside another (ADR-0207): a pane at the right of the item or
// daily note shown, holding a second item to read, and to edit where the
// reader may, as on its own page. Its item is kept in the address
// (?beside=KEY), so a link and the browser's back button bring the pair
// back. Shift+click on a link to an item, or Open beside, opens it there;
// Escape or the close button closes it. On a phone the pane follows the
// content instead of standing beside it (ADR-0116).
//
// The pane reads the item through the API like any view: an item the
// reader may not read is not found (D10). Its text is drawn by the
// editor as a reader sees it; Edit joins the item's live session
// (ADR-0010), with the same writing aids as on its page.
import { get } from "./api.js";
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { createEditor } from "./editor.js";
import { liveBody } from "./live.js";

// BESIDE is the parameter of the address that names the item beside.
export const BESIDE = "beside";

// withBeside is the current address with the item beside set, or taken
// away when key is empty. Other parameters stay.
export function withBeside(hash, key) {
  const [path, query = ""] = String(hash || "").replace(/^#/, "").split("?");
  const params = new URLSearchParams(query);
  if (key) params.set(BESIDE, key);
  else params.delete(BESIDE);
  // A bare edit stays bare, as the item view writes it (#/item/X?edit).
  const q = params.toString().replace(/(^|&)edit=(?=&|$)/, "$1edit");
  return "#" + path + (q ? "?" + q : "");
}

// hostsBeside tells whether a view takes an item beside it: an item's page
// and a daily note.
export function hostsBeside(hash) {
  return /^#\/(item|daily)(\/|\?|$)/.test(String(hash || ""));
}

// installBeside makes Shift+click on a link to an item open it beside the
// view, where the view takes one; a text being edited keeps Shift+click
// for its selection. Escape closes the pane when no dialog is open and no
// text is being edited.
export function installBeside() {
  document.addEventListener("click", (ev) => {
    if (!ev.shiftKey || ev.ctrlKey || ev.metaKey || ev.altKey || ev.button !== 0 || !hostsBeside(location.hash)) return;
    const a = ev.target instanceof Element ? ev.target.closest("a[href^='#/item/']") : null;
    if (!a || a.closest("[contenteditable=true]")) return;
    const key = decodeURIComponent(a.getAttribute("href").slice("#/item/".length).split("?")[0]);
    if (!key) return;
    ev.preventDefault();
    ev.stopPropagation();
    location.hash = withBeside(location.hash, key);
  }, true);
  document.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape" || ev.defaultPrevented || document.querySelector("dialog[open]")) return;
    if (!document.getElementById("beside-pane")) return;
    if (ev.target instanceof Element && ev.target.closest("[contenteditable=true], input, textarea, select")) return;
    if (document.documentElement.classList.contains("reading-mode")) return;
    ev.preventDefault();
    location.hash = withBeside(location.hash, "");
  });
}

// openBeside opens an item beside the current view.
export function openBeside(key) {
  location.hash = withBeside(location.hash, key);
}

// besidePane returns the pane for the item named by params, or null when
// none is. tools are the item page's helpers for a text: files(item,
// attachments, canEdit), childPages(item), embeds(chain), tasks(item,
// editing), markdown(item), create(item) for creating an item from a
// mention, and onLeave(fn) to end a live session when the view goes.
export function besidePane(params, tools) {
  const key = params.get(BESIDE);
  if (!key) return null;
  const body = h("div", { class: "beside-body" }, h("p", { class: "muted", text: t("beside.loading") }));
  const close = h("button", { type: "button", class: "icon-btn beside-close", id: "beside-close", title: t("beside.close"), "aria-label": t("beside.close"),
    text: "×", onclick: () => { location.hash = withBeside(location.hash, ""); } });
  const head = h("div", { class: "beside-head" }, h("span", { class: "muted", text: t("beside.title") }), close);
  const pane = h("aside", { class: "beside-pane", id: "beside-pane", "aria-label": t("beside.title"), tabindex: "-1" }, head, body);
  let session = null;
  tools.onLeave(() => { if (session) session.finish().catch(() => {}); });

  const draw = async (editing) => {
    const { item, editable } = await get("/api/items/" + encodeURIComponent(key));
    const path = "/api/items/" + item.id;
    const [{ types }, { embeds }, { attachments }] = await Promise.all([get("/api/types"), get(path + "/embeds"), get(path + "/attachments")]);
    const type = types.find((ty) => ty.name === item.type) || { fields: [], body: false };
    const canEdit = item.presence === "live" && editable && item.readsAllSecrets !== false && type.body;
    const resolve = async (ref) => (await get("/api/items/" + encodeURIComponent(ref))).item;
    const href = "#/item/" + encodeURIComponent(item.key);
    let text;
    if (editing && canEdit) {
      session = liveBody({ ref: item.id, embeds, resolve, childPages: tools.childPages(item), files: tools.files(item, attachments, true),
        embedContent: tools.embeds([item.id]), tasks: tools.tasks(item, true), markdown: tools.markdown(item), create: tools.create(item) });
      text = session.element;
    } else if (type.body) {
      text = createEditor({ blocks: item.body, embeds, editable: false, resolve, childPages: tools.childPages(item), files: tools.files(item, attachments),
        embedContent: tools.embeds([item.id]), tasks: tools.tasks(item, false) }).element;
    } else {
      text = h("p", { class: "muted", text: t("item.noBody") });
    }
    const toggle = canEdit ? h("button", { type: "button", id: "beside-edit", class: editing ? "primary" : "", text: editing ? t("item.done") : t("item.edit"),
      onclick: async () => {
        toggle.disabled = true;
        if (session) {
          const s = session;
          session = null;
          await s.finish().catch(() => {});
        }
        draw(!editing).catch((err) => body.replaceChildren(h("p", { class: "error", text: err.message })));
      } }) : null;
    const props = type.fields.filter((f) => item.properties[f.name] !== undefined && item.properties[f.name] !== null);
    body.replaceChildren(
      h("h2", { class: "beside-title" }, h("a", { href, text: item.title || item.key })),
      h("p", { class: "muted beside-meta" }, `${item.key} · ${item.type}`, item.state ? " · " + item.state : "",
        " · ", h("a", { href, id: "beside-open", text: t("beside.openFull") })),
      props.length ? h("dl", { class: "beside-props" }, props.flatMap((f) => [h("dt", { text: f.name }), h("dd", { text: String(item.properties[f.name]).replace(/T00:00:00Z$/, "") })])) : null,
      item.labels?.length ? h("p", { class: "beside-labels" }, item.labels.map((l) => h("span", { class: "label", text: l }))) : null,
      toggle ? h("div", { class: "pm-toolbar pm-readbar beside-bar", role: "toolbar" }, toggle) : null,
      h("div", { class: "beside-text " + (editing ? "editing" : "reading") }, text));
  };
  draw(false).catch((err) => body.replaceChildren(h("p", { class: "error", role: "alert", text: err.status === 404 ? t("beside.notFound") : err.message })));
  return pane;
}
