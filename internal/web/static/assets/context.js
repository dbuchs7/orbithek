// The context of an item (ADR-0207), a section of the item's side column:
// what refers to it, each with the sentence it stands in; the items that
// share its labels; and the daily notes that name it (ADR-0205). The
// server derives it for the reader on every request, so it names only
// what they may read (D10), within the budget ContextEntries. Each entry
// opens its item, or opens it beside the current one (Shift+click, or its
// button).
import { get } from "./api.js";
import { h } from "./dom.js";
import { t } from "./i18n.js";

// kindName names how an item refers: by a mention, an embed or a link in
// its text, or by a relation of a type made by hand, named as it is.
const kindName = (kind) => ({ mentions: t("context.mentions"), embeds: t("context.embeds"), link: t("context.link") })[kind] || kind;

// contextSection returns the section, filled once the context is read;
// beside(key) opens an item beside the current one.
export function contextSection(item, beside) {
  const body = h("div", { class: "context-body" }, h("p", { class: "muted", text: t("context.loading") }));
  const section = h("section", { id: "context", class: "context" }, h("h2", { text: t("context.title") }), body);
  const entry = (it, ...extra) => h("li", { class: "context-entry", "data-key": it.key },
    h("a", { href: "#/item/" + encodeURIComponent(it.key), class: "context-link" }, h("span", { class: "context-key", text: it.key }), " ", it.title || it.key),
    h("button", { type: "button", class: "link context-beside", title: t("beside.open"), "aria-label": t("beside.openNamed", { title: it.title || it.key }),
      text: t("beside.short"), onclick: () => beside(it.key) }),
    ...extra);
  const part = (id, heading, list, more, draw) => h("div", { class: "context-part", id: "context-" + id },
    h("h3", { text: heading }),
    list.length ? h("ul", { class: "context-list" }, list.map(draw)) : h("p", { class: "muted", text: t("context.none") }),
    more ? h("p", { class: "muted", text: t("context.more") }) : null);
  get("/api/items/" + item.id + "/context").then((c) => {
    body.replaceChildren(
      part("references", t("context.references"), c.references, c.more.references, (r) => entry(r.item,
        h("span", { class: "muted context-kind", text: " · " + kindName(r.kind) + (r.count > 1 ? " " + t("context.times", { n: r.count }) : "") }),
        r.sentence ? h("q", { class: "context-sentence", text: r.sentence }) : null)),
      part("shared", t("context.shared"), c.shared, c.more.shared, (s) => entry(s.item,
        h("span", { class: "context-labels" }, s.labels.map((l) => h("span", { class: "label", text: l }))))),
      part("daily", t("context.daily"), c.daily, c.more.daily, (d) => entry(d.item,
        h("span", { class: "muted context-kind", text: " · " + d.date }),
        d.sentence ? h("q", { class: "context-sentence", text: d.sentence }) : null)));
  }).catch((err) => body.replaceChildren(h("p", { class: "error", text: err.message })));
  return section;
}
