// The dialog that moves a page with its child pages (ADR-0067,
// ADR-draft-a-page-is-moved-into-another-room-from-its-head-and-its-tree):
// to the top of a room, or below a page there, chosen in the room's page
// tree or found by its title or key. A move into another room says first
// what changes. It is opened from the page's head and from the page tree;
// the server decides, as for every move.
import { get, post } from "./api.js";
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { moveRooms, treeURL, findURL, belowItself, placeTrail, crossesRoom } from "./moveparts.js";

// moveDialog asks where item ({id, key, title, space}) goes and moves it.
// spaces are the rooms the reader holds a role in, as /api/spaces lists
// them. It answers the moved item, with its key in its new room, or null
// when cancelled.
export function moveDialog(item, spaces) {
  const rooms = moveRooms(spaces);
  const roomName = (key) => rooms.find((s) => s.key === key)?.name || key;
  return new Promise((resolve) => {
    let result = null;
    // chosen is the page it goes below, with its trail: the page and the
    // pages above it, nearest first; asked is the newest trail asked for.
    let chosen = null;
    let asked = 0;
    let finding = null;

    const roomSelect = h("select", { name: "space", id: "move-space" },
      rooms.map((s) => h("option", { value: s.key, text: `${s.name} (${s.key})`, selected: s.key === item.space })));
    const top = h("input", { type: "radio", name: "where", value: "top", id: "move-top", checked: true });
    const below = h("input", { type: "radio", name: "where", value: "below", id: "move-below" });
    const find = h("input", { type: "search", id: "move-find", autocomplete: "off", placeholder: t("move.findPlaceholder"), "aria-label": t("move.find") });
    const pages = h("ul", { class: "move-pages", id: "move-pages", role: "group", "aria-label": t("move.pages") });
    const pagesBox = h("div", { class: "move-pages-box", hidden: true }, find, pages);
    const place = h("p", { class: "move-place", id: "move-place", "aria-live": "polite" });
    const warning = h("p", { class: "notice", id: "move-warning", hidden: true });
    const status = h("p", { class: "error", role: "alert", hidden: true });
    const ok = h("button", { type: "submit", class: "primary", id: "move-ok", text: t("move.submit") });

    const room = () => roomSelect.value;
    const fail = (err) => { status.textContent = err.message; status.hidden = false; };

    // describe says where the page goes and, for another room, what that
    // changes; and names the button accordingly.
    const describe = () => {
      const name = roomName(room());
      if (below.checked && !chosen) place.textContent = t("move.choosePage");
      else if (below.checked && chosen.trail) place.textContent = t("move.goesTo", { place: placeTrail(name, chosen.trail.map((p) => p.title)) });
      else if (below.checked) place.textContent = t("move.goesTo", { place: placeTrail(name, [chosen.title]) });
      else place.textContent = t("move.goesTo", { place: placeTrail(name, []) });
      const other = crossesRoom(item, room());
      warning.hidden = !other;
      warning.textContent = other ? t("move.otherRoom", { title: item.title, room: name }) : "";
      ok.textContent = other ? t("move.submitInto", { room: name }) : t("move.submit");
    };

    // choose takes a page as the target and reads the pages above it, to
    // name the place and to tell a place below the page itself.
    const choose = async (id, title) => {
      chosen = { id, title };
      status.hidden = true;
      describe();
      const mine = ++asked;
      const trail = [];
      for (let p = id; p && trail.length < 50; ) {
        try {
          const { item: it } = await get("/api/items/" + encodeURIComponent(p));
          trail.push({ id: it.id, title: it.title });
          p = it.parent;
        } catch {
          break;
        }
      }
      if (mine !== asked || chosen?.id !== id) return;
      chosen.trail = trail;
      describe();
      if (belowItself(item.id, trail.map((p) => p.id))) fail(new Error(t("move.belowItself")));
    };

    const pageChoice = (n) => h("label", { class: "move-page" },
      h("input", { type: "radio", name: "parent", value: n.id, onchange: () => choose(n.id, n.title || n.key).catch(fail) }),
      " ", h("span", { text: n.title || n.key }), " ", h("span", { class: "muted", text: `(${n.key})` }));

    // level lists one level of the room's tree below parent, page by page;
    // the page being moved is left out, and with it everything below it.
    const level = async (parent, ul, after = "") => {
      const res = await get(treeURL(room(), parent, after));
      const shown = res.items.filter((n) => n.id !== item.id);
      if (!parent && !after && !shown.length) ul.append(h("li", { class: "muted", text: t("move.noPages") }));
      for (const n of shown) {
        const kids = h("ul", { hidden: true });
        let loaded = false;
        const toggle = n.hasChildren
          ? h("button", { type: "button", class: "tree-toggle", "aria-expanded": "false", "aria-label": t("tree.expand", { title: n.title }), text: "▸" })
          : h("span", { class: "tree-toggle" });
        if (n.hasChildren) {
          toggle.addEventListener("click", async () => {
            if (!loaded) { loaded = true; await level(n.id, kids).catch(fail); }
            kids.hidden = !kids.hidden;
            toggle.textContent = kids.hidden ? "▸" : "▾";
            toggle.setAttribute("aria-expanded", String(!kids.hidden));
            toggle.setAttribute("aria-label", t(kids.hidden ? "tree.expand" : "tree.collapse", { title: n.title }));
          });
        }
        ul.append(h("li", { "data-id": n.id }, h("div", { class: "move-row" }, toggle, pageChoice(n)), kids));
      }
      if (res.next) {
        const more = h("li", {}, h("button", { type: "button", class: "link", text: t("common.more"), onclick: () => { more.remove(); level(parent, ul, res.next).catch(fail); } }));
        ul.append(more);
      }
    };

    // fill shows the room's tree, or the pages found by the words typed.
    const fill = async () => {
      const words = find.value.trim();
      const mine = ++asked;
      chosen = null;
      describe();
      if (!words) {
        const ul = h("ul", { class: "move-level" });
        await level("", ul);
        if (mine === asked) pages.replaceChildren(h("li", {}, ul));
        return;
      }
      const res = await get(findURL(room(), words));
      if (mine !== asked) return;
      const found = res.items.filter((it) => it.id !== item.id);
      pages.replaceChildren(...(found.length
        ? found.map((it) => h("li", { "data-id": it.id }, h("div", { class: "move-row" }, pageChoice(it), " ", h("span", { class: "muted", text: it.type }))))
        : [h("li", { class: "muted", text: t("move.noneFound") })]));
    };

    const where = () => {
      pagesBox.hidden = !below.checked;
      status.hidden = true;
      if (below.checked && !pages.childElementCount) fill().catch(fail);
      describe();
    };
    top.addEventListener("change", where);
    below.addEventListener("change", where);
    roomSelect.addEventListener("change", () => {
      pages.replaceChildren();
      find.value = "";
      status.hidden = true;
      if (below.checked) fill().catch(fail);
      describe();
    });
    find.addEventListener("input", () => {
      clearTimeout(finding);
      finding = setTimeout(() => fill().catch(fail), 250);
    });
    // Enter in the search field finds at once instead of moving.
    find.addEventListener("keydown", (ev) => {
      if (ev.key !== "Enter") return;
      ev.preventDefault();
      clearTimeout(finding);
      fill().catch(fail);
    });

    const f = h("form", { method: "dialog", class: "note-form move-form", id: "move-form" },
      h("label", { class: "field" }, h("span", { text: t("col.space") }), roomSelect),
      h("fieldset", { class: "move-where" }, h("legend", { text: t("move.place") }),
        h("label", { class: "check" }, top, " ", t("move.top")),
        h("label", { class: "check" }, below, " ", t("move.belowPage"))),
      pagesBox, place, warning,
      h("p", { class: "muted", text: t("move.note") }),
      status,
      h("div", { class: "actions-row" }, ok,
        h("button", { type: "button", class: "link", id: "move-cancel", text: t("common.cancel"), onclick: () => d.close() })));
    f.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      status.hidden = true;
      if (below.checked && !chosen) return fail(new Error(t("move.choosePage")));
      if (below.checked && chosen.trail && belowItself(item.id, chosen.trail.map((p) => p.id))) return fail(new Error(t("move.belowItself")));
      ok.disabled = true;
      try {
        const res = await post(`/api/items/${encodeURIComponent(item.id)}/move`, { space: room(), parent: below.checked ? chosen.id : "", before: "" });
        result = res.item;
        d.close();
      } catch (err) {
        fail(err);
      } finally {
        ok.disabled = false;
      }
    });
    const title = t("move.title", { title: item.title });
    const d = h("dialog", { class: "note-dialog move-dialog", id: "move-dialog", "aria-label": title },
      h("div", { class: "drawing-bar" }, h("strong", { text: title })), f);
    d.addEventListener("close", () => { clearTimeout(finding); d.remove(); resolve(result); });
    document.body.append(d);
    describe();
    d.showModal();
    roomSelect.focus();
  });
}
