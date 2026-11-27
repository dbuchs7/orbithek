// A search field in a text (ADR-draft-a-search-field-in-a-text): a block
// of the kind search names what it searches — a room, or a page, the page
// with the pages below it, or the pages below it only — and every reader's
// browser asks the full-text search (ADR-0033) as that reader, the hits
// below the field. The reader may choose another scope while reading;
// nothing of that is kept. A click on a hit opens it.
//
// The page shares what this module needs once (configureSearches): its
// dialog and the stars of a hit.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { SCOPES, searchable, searchBlockOf, searchTarget, searchUrl } from "./searchparts.js";

let shared = null;

// configureSearches takes what the page shares: formDialog(title, id,
// fields, submit, ok), the page's dialog, and stars(n), the stars of a
// hit as the search page draws them.
export function configureSearches(helpers) { shared = helpers; }
export const searchesReady = () => shared !== null;

// The names of the scopes, with their keys written out so the
// catalogue's tests find them (ADR-0017).
const scopeName = (s) => ({ "": t("searchblock.scopeRoom"), page: t("searchblock.scopePage"), tree: t("searchblock.scopeTree"), below: t("searchblock.scopeBelow") })[s];

// pageTitle names the page a block searches within: the item itself, or
// another page as far as the reader reads it.
async function pageTitle(block) {
  if (!block.ref) return t("searchblock.thisPage");
  try {
    const { item } = await get("/api/items/" + encodeURIComponent(block.ref));
    return `${item.key} ${item.title}`;
  } catch {
    return t("searchblock.unreadablePage");
  }
}

// searchSummary says in a line what a block searches, beside its button.
export async function searchSummary(block) {
  if (!block.scope) return t("searchblock.summaryRoom", { room: block.space || t("searchblock.ownRoom") });
  return t("searchblock.summaryPage", { scope: scopeName(block.scope), page: await pageTitle(block) });
}

// searchBlockElement draws the field a block names for the item here
// ({ id, space }), with the scope to choose and the hits below it.
export function searchBlockElement(block, here) {
  const n = Math.random().toString(36).slice(2, 8);
  const input = h("input", { type: "search", name: "q", class: "searchblock-q", id: `searchblock-q-${n}`, autocomplete: "off",
    placeholder: t("searchblock.placeholder"), "aria-label": t("searchblock.placeholder") });
  const scope = h("select", { name: "scope", class: "searchblock-scope", "aria-label": t("searchblock.scope") },
    SCOPES.map((s) => h("option", { value: s, text: scopeName(s) })));
  scope.value = block.scope || "";
  const list = h("ol", { class: "list hits searchblock-hits", "aria-live": "polite" });
  const total = h("p", { class: "muted searchblock-total" });
  const more = h("div", { class: "searchblock-more" });
  let asked = 0;
  const addHits = (page, text, target, run) => {
    if (run !== asked) return;
    for (const hit of page.hits) {
      const it = hit.item;
      list.append(h("li", {},
        hit.stars && shared?.stars ? shared.stars(hit.stars) : null,
        h("a", { href: "#/item/" + encodeURIComponent(it.key), text: it.key }), " ", h("strong", { text: it.title }),
        " ", h("span", { class: "muted", text: `${it.space} · ${it.type}${it.state ? " · " + it.state : ""}` }),
        hit.snippet ? h("p", { class: "snippet", text: hit.comment ? t("search.inComment", { text: hit.snippet }) : hit.snippet }) : null));
    }
    more.replaceChildren();
    if (page.next) {
      more.append(h("button", { type: "button", class: "link", text: t("common.more"), onclick: async () =>
        addHits(await get(searchUrl(text, target, { after: page.next })), text, target, run) }));
    }
  };
  const run = async () => {
    const text = input.value;
    const mine = ++asked;
    list.replaceChildren();
    more.replaceChildren();
    if (!searchable(text)) {
      total.textContent = "";
      return;
    }
    const target = searchTarget(block, scope.value, here || {});
    try {
      const page = await get(searchUrl(text, target));
      if (mine !== asked) return;
      total.textContent = page.total.value ? t("search.results", { n: page.total.value, q: text.trim() }) : t("search.none");
      addHits(page, text, target, mine);
    } catch (err) {
      if (mine === asked) total.textContent = err.message;
    }
  };
  let timer = null;
  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(run, 300);
  });
  scope.addEventListener("change", run);
  const form = h("form", { class: "searchblock-form", role: "search", onsubmit: (ev) => {
    ev.preventDefault();
    clearTimeout(timer);
    run();
  } }, input, scope, h("button", { type: "submit", class: "primary", text: t("searchblock.go") }));
  return h("div", { class: "searchblock" }, form, total, list, more);
}

// pickSearch asks what a search field in a text searches, starting from
// initial (a block), for the item self, and answers the block, or null
// when cancelled.
export async function pickSearch(initial = {}, self = "", helpers = shared) {
  const { spaces } = await get("/api/spaces");
  const choice = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);
  const scope = h("select", { id: "searchblock-pick-scope", name: "scope" }, SCOPES.map((s) => h("option", { value: s, text: scopeName(s) })));
  scope.value = initial.scope || "";
  const room = h("select", { id: "searchblock-pick-space", name: "space" },
    h("option", { value: "", text: t("searchblock.ownRoom") }),
    spaces.map((r) => h("option", { value: r.key, text: r.name ? `${r.key} – ${r.name}` : r.key })));
  room.value = initial.space || "";
  // The page is the item itself or another one found by typing its key
  // or a word of its title.
  let chosen = initial.ref ? { id: initial.ref } : null;
  const pageInput = h("input", { id: "searchblock-pick-page", name: "page", type: "search", autocomplete: "off", list: "searchblock-pick-pages",
    placeholder: t("searchblock.thisPage"), value: initial.ref ? await pageTitle(initial) : "" });
  const pageList = h("datalist", { id: "searchblock-pick-pages" });
  let found = [];
  pageInput.addEventListener("input", async () => {
    const typed = pageInput.value;
    chosen = found.find((it) => `${it.key} ${it.title}` === typed) || null;
    if (chosen || !searchable(typed)) return;
    const res = await get("/api/search?limit=8&q=" + encodeURIComponent(typed)).catch(() => ({ hits: [] }));
    found = (res.hits || []).map((x) => x.item);
    pageList.replaceChildren(...found.map((it) => h("option", { value: `${it.key} ${it.title}` })));
  });
  const roomField = choice(t("searchblock.room"), room);
  const pageField = h("div", {}, choice(t("searchblock.page"), pageInput), pageList);
  const shown = () => {
    roomField.hidden = scope.value !== "";
    pageField.hidden = scope.value === "";
  };
  scope.addEventListener("change", shown);
  shown();
  return helpers.formDialog(t("searchblock.insert"), "searchblock-dialog", [
    choice(t("searchblock.scope"), scope), roomField, pageField,
    h("p", { class: "muted", text: t("searchblock.hint") }),
  ], async () => {
    const typed = pageInput.value.trim();
    let ref = "";
    if (scope.value && typed) {
      if (chosen?.id) ref = chosen.id;
      else {
        // A key typed whole names its page.
        const { item } = await get("/api/items/" + encodeURIComponent(typed.split(/\s/)[0]));
        ref = item.id;
      }
    }
    return searchBlockOf({ scope: scope.value, space: room.value, ref }, self);
  }, t("searchblock.ok"));
}
