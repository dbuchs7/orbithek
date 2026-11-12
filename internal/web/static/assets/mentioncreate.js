// An item is created where it is mentioned (ADR-0204): when "@" or the
// link dialog finds no item of the title typed, the editor offers to
// create one. A dialog asks for its type, the room's page type by default,
// and its room, the edited item's by default; an item of a type with a
// text, made in the edited item's room, goes below the edited item, as a
// child page. The server decides as for any new item: the room's role,
// the restrictions of the place, and the storage, which makes nothing new
// from its critical level on (ADR-0163); what it refuses is said in the
// dialog. Nothing is created when the dialog is cancelled.
import { get, post } from "./api.js";
import { h } from "./dom.js";
import { t } from "./i18n.js";

// pageType is the type a room's pages are of: one named page, else the
// first type with a text and no workflow, else the first with a text.
export function pageType(types) {
  return (types.find((ty) => ty.name === "page") || types.find((ty) => ty.body && !ty.workflow) || types.find((ty) => ty.body) || types[0])?.name || "";
}

// creatable are the types an item can be made of from a title alone: no
// field it requires. Those that need more are made with New. A daily note
// is made from its day, never from a mention (ADR-0203).
export function creatable(types) {
  return types.filter((ty) => ty.name !== "daily-note" && !(ty.fields || []).some((f) => f.required));
}

// placeOf is where the new item goes: below the edited item when its type
// has a text and it is made in the edited item's room, else at the top of
// the room chosen. Never below a daily note (ADR-0203): a child inherits
// its parent's restriction (D9), and a note is its author's alone.
export function placeOf(host, type, space) {
  const note = (host?.labels || []).includes("daily-note");
  return type?.body && host && !note && space === host.space ? host.id : "";
}

// inherits tells whether an item made below the host is restricted as the
// host is: a restriction holds for everything below it (D9).
export function inherits(host) {
  return Boolean(host?.restriction?.read?.length || host?.restriction?.edit?.length);
}

// mentionCreator returns create(title) for the editor of an item: it asks
// for the type and room and resolves with the new item, or null. paused()
// tells that the storage makes nothing new now.
export function mentionCreator(host, paused = () => false) {
  return async (title) => {
    const [{ types }, { spaces }] = await Promise.all([get("/api/types"), get("/api/spaces")]);
    return askAndCreate(host, title, creatable(types), spaces.filter((s) => s.role === "editor" || s.role === "space-admin"), paused());
  };
}

function askAndCreate(host, title, types, spaces, paused) {
  return new Promise((resolve) => {
    const error = h("p", { class: "error", role: "alert", id: "mention-create-error" });
    const name = h("input", { name: "title", id: "mention-create-title", required: true, value: title, autocomplete: "off" });
    const typeSelect = h("select", { name: "type", id: "mention-create-type" },
      types.map((ty) => h("option", { value: ty.name, text: ty.name, selected: ty.name === pageType(types) })));
    const spaceSelect = h("select", { name: "space", id: "mention-create-space" },
      spaces.map((s) => h("option", { value: s.key, text: `${s.name} (${s.key})`, selected: s.key === host.space })));
    const where = h("p", { class: "muted", id: "mention-create-where" });
    const say = () => {
      const ty = types.find((x) => x.name === typeSelect.value);
      const below = placeOf(host, ty, spaceSelect.value);
      where.textContent = below ? t("mentionCreate.below", { title: host.title || host.key }) + (inherits(host) ? " " + t("mentionCreate.inherits") : "")
        : t("mentionCreate.top", { space: spaceSelect.value });
    };
    typeSelect.addEventListener("change", say);
    spaceSelect.addEventListener("change", say);
    say();
    const ok = h("button", { type: "submit", class: "primary", id: "mention-create-ok", text: t("mentionCreate.ok") });
    const form = h("form", { method: "dialog" },
      h("h2", { text: t("mentionCreate.title") }),
      h("label", { class: "field" }, h("span", { text: t("col.title") }), name),
      h("label", { class: "field" }, h("span", { text: t("col.type") }), typeSelect),
      h("label", { class: "field" }, h("span", { text: t("col.space") }), spaceSelect),
      where, error,
      h("div", { class: "pm-dialog-actions" },
        h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: () => close(null) }), ok));
    const el = h("dialog", { class: "pm-dialog", id: "mention-create" }, form);
    let done = false;
    const close = (v) => {
      if (done) return;
      done = true;
      el.close();
      el.remove();
      resolve(v);
    };
    if (paused) {
      error.textContent = t("storage.pausedHint");
      ok.disabled = true;
    } else if (!types.length || !spaces.length) {
      error.textContent = t("mentionCreate.nowhere");
      ok.disabled = true;
    }
    form.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      error.textContent = "";
      const ty = types.find((x) => x.name === typeSelect.value);
      const titleText = name.value.trim();
      if (!titleText || !ty) return;
      ok.disabled = true;
      try {
        const { item } = await post("/api/items", { space: spaceSelect.value, type: ty.name, parent: placeOf(host, ty, spaceSelect.value), title: titleText, properties: {} });
        close(item);
      } catch (err) {
        error.textContent = err.message;
        ok.disabled = false;
      }
    });
    el.addEventListener("cancel", (ev) => { ev.preventDefault(); close(null); });
    document.body.append(el);
    el.showModal();
    setTimeout(() => typeSelect.focus(), 0);
  });
}
