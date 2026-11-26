// A slide deck in a text (ADR-draft-a-slide-deck-in-a-text): a block of
// the kind slides names an album folder or collection, and the text shows
// its pictures as slides — in the order of slides of the folder or
// collection (ADR-0195), read with each reader's rights, so a reader sees
// only the pictures they read (D10). The reader steps through them in the
// text, shows them full screen, or presents them: the album's presenter's
// view and presentation window (ADR-0176), which return to the text when
// the presentation ends.
//
// The page shares what this module needs once (configureSlides): its
// dialog. The decisions that need no browser are in slidesparts.js.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { slidesOf } from "./present-parts.js";
import { startPresentation } from "./present.js";
import { CHOICE_DEPTH, MAX_CHOICES, choiceLabel, deckFilter, keyStep, slideURLs } from "./slidesparts.js";

let shared = null;

// configureSlides takes what the page shares: formDialog(title, id,
// fields, submit, ok), the page's dialog.
export function configureSlides(helpers) { shared = helpers; }
export const slidesReady = () => shared !== null;

// loadDeck reads the folder or collection a block names and its slides,
// page by page, in its order of slides. It answers null when the reader
// does not read the folder or it is gone: unreadable is non-existent.
async function loadDeck(ref) {
  let target;
  try {
    target = (await get("/api/items/" + encodeURIComponent(ref))).item;
  } catch {
    return null;
  }
  if (!target || target.presence && target.presence !== "live") return null;
  const all = [];
  let offset = 0;
  for (;;) {
    const q = deckFilter(target);
    q.set("order", "slides");
    q.set("limit", "200");
    q.set("offset", String(offset));
    const page = await get("/api/album?" + q);
    all.push(...page.pictures);
    if (!page.next) break;
    offset = page.next;
  }
  return { target, slides: slidesOf(all) };
}

// hostRef is the item whose text is shown now, which a presentation
// started from it returns to: the key or ID in the address.
function hostRef() {
  const m = /^#\/item\/([A-Za-z0-9-]{1,64})(?:[?/]|$)/.exec(location.hash);
  return m ? m[1] : "";
}

// slidesElement shows the slide deck a block names. While reading, the
// reader steps through it, shows it full screen and presents it; while
// editing (editing), the slides show without presenting, beside a button
// that changes the folder (onChange).
export function slidesElement(block, { editing = false, onChange = null } = {}) {
  const title = h("span", { class: "pm-slides-title" }, h("strong", { text: t("slides.deck") }));
  const tools = h("span", { class: "pm-slides-tools" });
  const bar = h("div", { class: "pm-slides-bar" }, title, tools);
  const stage = h("div", { class: "pm-slides-stage", tabindex: "0", role: "group", "aria-roledescription": t("slides.deck"), "aria-busy": "true" },
    h("p", { class: "muted", text: "…" }));
  const strip = h("ol", { class: "pm-slides-strip", "aria-label": t("slides.strip") });
  const caption = block.text ? h("figcaption", { class: "pm-slides-caption", text: block.text }) : null;
  const root = h("figure", { class: "pm-slides", contenteditable: "false" }, bar, stage, strip, caption);
  if (editing && onChange) {
    tools.append(h("button", { type: "button", class: "link pm-slides-change", text: t("slides.change"), onclick: onChange }));
  }
  const unavailable = (key) => {
    stage.replaceChildren(h("p", { class: "muted pm-slides-message", text: t(key) }));
    stage.removeAttribute("aria-busy");
  };

  loadDeck(block.ref).then((deck) => {
    if (!deck) return unavailable("slides.unavailable");
    const { target, slides } = deck;
    const filter = deckFilter(target);
    const albumHref = target.type === "collection" ? `#/item/${encodeURIComponent(target.key)}` : `#/album?folder=${encodeURIComponent(target.key)}`;
    title.append(" ", h("span", { class: "muted" }, t("slides.count", { n: slides.length }), " · ",
      h("a", { href: albumHref, class: "pm-slides-album", title: t("slides.openAlbum"), text: target.title || target.key })));
    if (!slides.length) return unavailable("slides.empty");
    let index = 0;
    const counter = h("span", { class: "pm-slides-counter", "aria-live": "polite" });
    const prev = h("button", { type: "button", class: "secondary pm-slides-prev", title: t("slides.prev"), "aria-label": t("slides.prev"), text: "‹", onclick: () => go(index - 1) });
    const next = h("button", { type: "button", class: "secondary pm-slides-next", title: t("slides.next"), "aria-label": t("slides.next"), text: "›", onclick: () => go(index + 1) });
    tools.prepend(prev, counter, next);
    if (!editing) {
      tools.append(
        h("button", { type: "button", class: "primary pm-slides-present", title: t("slides.presentTitle"), text: t("slides.present"),
          onclick: () => startPresentation(filter, hostRef()) }),
        h("button", { type: "button", class: "secondary pm-slides-full", title: t("slides.fullTitle"), text: t("slides.full"),
          onclick: () => (document.fullscreenElement ? document.exitFullscreen() : stage.requestFullscreen?.())?.catch?.(() => {}) }));
    }
    const thumbs = slides.map((s, i) => {
      const { thumb } = slideURLs(s);
      return h("li", {}, h("button", { type: "button", class: "pm-slides-thumb", "aria-label": t("present.goTo", { n: i + 1, name: s.name }), title: s.name, onclick: () => go(i) },
        thumb ? h("img", { src: thumb, alt: "", loading: "lazy", draggable: "false" }) : h("span", { class: "present-thumb-video", "aria-hidden": "true" }),
        h("span", { text: String(i + 1) })));
    });
    strip.replaceChildren(...thumbs);
    const show = () => {
      const s = slides[index];
      const { file, stage: picture } = slideURLs(s);
      const media = s.video
        ? h("video", { class: "pm-slides-media", src: file, controls: true, preload: "metadata", ...(s.poster ? { poster: picture } : {}), "aria-label": s.name })
        : h("img", { class: "pm-slides-media", src: picture, alt: s.name, draggable: "false" });
      stage.replaceChildren(media);
      stage.removeAttribute("aria-busy");
      counter.textContent = t("slides.counter", { n: index + 1, total: slides.length });
      prev.disabled = index === 0;
      next.disabled = index === slides.length - 1;
      thumbs.forEach((li, i) => li.firstChild.classList.toggle("on", i === index));
      thumbs[index].firstChild.scrollIntoView?.({ block: "nearest", inline: "nearest" });
    };
    const go = (i) => {
      const to = Math.min(slides.length - 1, Math.max(0, i));
      if (to !== index) { index = to; show(); }
    };
    stage.addEventListener("keydown", (e) => {
      const to = keyStep(e.key, index, slides.length);
      if (to < 0 || e.target instanceof HTMLVideoElement && e.key === " ") return;
      e.preventDefault();
      go(to);
    });
    show();
  }).catch(() => unavailable("slides.unavailable"));
  return root;
}

// folderChoices lists the folders the reader reads, down to CHOICE_DEPTH
// below the top, and the collections, each with its label, at most
// MAX_CHOICES.
async function folderChoices() {
  const out = [];
  const visit = async (parent, path, depth) => {
    if (out.length >= MAX_CHOICES) return;
    const { folders = [], collections = [] } = await get("/api/album/folders" + (parent ? "?parent=" + encodeURIComponent(parent) : ""));
    for (const c of collections) {
      if (out.length < MAX_CHOICES) out.push({ id: c.id, label: choiceLabel({ space: c.space, path: [...path, c.name], collection: true }, t("slides.collection")) });
    }
    for (const f of folders) {
      if (out.length >= MAX_CHOICES) return;
      out.push({ id: f.id, label: choiceLabel({ space: f.space, path: [...path, f.name] }) });
      if (depth + 1 < CHOICE_DEPTH) await visit(f.id, [...path, f.name], depth + 1);
    }
  };
  await visit("", [], 0);
  return out.sort((a, b) => a.label.localeCompare(b.label));
}

// pickSlides asks for the folder or collection of a slide deck and its
// caption, starting from initial (a block), and answers { ref, text }, or
// null when cancelled.
export async function pickSlides(initial = {}, helpers = shared) {
  const choices = await folderChoices();
  const folder = h("select", { id: "slides-pick-folder", name: "folder", required: true },
    h("option", { value: "", text: t("slides.chooseFolder") }),
    choices.map((c) => h("option", { value: c.id, text: c.label })));
  if (initial.ref && !choices.some((c) => c.id === initial.ref)) folder.append(h("option", { value: initial.ref, text: t("slides.current") }));
  folder.value = initial.ref || "";
  const preview = h("div", { class: "slides-pick-preview", id: "slides-pick-preview", "aria-live": "polite" });
  const showPreview = async () => {
    preview.replaceChildren();
    if (!folder.value) return;
    const deck = await loadDeck(folder.value).catch(() => null);
    if (!deck || folder.value !== deck.target.id) return;
    preview.replaceChildren(h("p", { class: "muted", text: t("slides.count", { n: deck.slides.length }) }),
      h("div", { class: "slides-pick-thumbs" }, deck.slides.slice(0, 8).map((s) => {
        const { thumb } = slideURLs(s);
        return thumb ? h("img", { src: thumb, alt: s.name, title: s.name, loading: "lazy" }) : h("span", { class: "present-thumb-video", title: s.name });
      })));
  };
  folder.addEventListener("change", showPreview);
  showPreview();
  const caption = h("input", { id: "slides-pick-caption", name: "caption", type: "text", value: initial.text || "", maxlength: 500 });
  const choice = (label, input) => h("label", { class: "field" }, h("span", { text: label }), input);
  return helpers.formDialog(t("slides.insert"), "slides-dialog", [
    choice(t("slides.folder"), folder),
    preview,
    choice(t("slides.caption"), caption),
    h("p", { class: "muted", text: t("slides.hint") }),
    choices.length ? null : h("p", { class: "muted", id: "slides-pick-none", text: t("slides.noFolders") }),
  ].filter(Boolean), async () => {
    if (!folder.value) throw new Error(t("slides.chooseFolder"));
    return { ref: folder.value, text: caption.value.trim() };
  }, t("slides.ok"));
}
