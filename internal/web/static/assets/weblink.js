// A web link shown as a card (ADR-0206): the picture its page names, its
// title linked to the page, the site, author and day, and the page's
// description. A web link is an item; the card is how a page or an embed
// shows any item whose url field holds a web address, so a link card in a
// text is an embed of the web link item, with nothing copied into the
// text (D4).
//
// webLinkParts and pictureOf are plain functions, run by node --test
// (internal/web/jstest/weblink.test.mjs); webLinkCard draws.
import { h } from "./dom.js";
import { isWebAddress } from "./capture.js";

// PICTURE is the name a page's picture has as a file of its item, before
// its extension (internal/server, api_weblinks.go).
export const PICTURE = "preview";

const pictureTypes = ["image/png", "image/jpeg", "image/gif", "image/webp"];

// pictureOf is the item's newest picture named as a page's, or null.
export function pictureOf(attachments) {
  const list = (attachments || []).filter((a) => pictureTypes.includes(a.type) && String(a.name).startsWith(PICTURE + "."));
  return list.length ? list[list.length - 1] : null;
}

// isWebLink reports whether an item is shown as a web link: its url
// field holds a web address.
export function isWebLink(item) {
  return isWebAddress(item?.properties?.url);
}

// webLinkParts are what a card shows of an item: its address, title,
// site, author, day of publication, description and picture.
export function webLinkParts(item, attachments) {
  const p = item?.properties || {};
  const text = (v) => (typeof v === "string" ? v.trim() : "");
  let host = "";
  try { host = new URL(p.url).hostname.replace(/^www\./, ""); } catch { /* no address */ }
  return {
    url: isWebAddress(p.url) ? p.url : "",
    title: item?.title || host,
    site: text(p.site) || host,
    author: text(p.author),
    published: text(p.published).slice(0, 10),
    description: text(p.description),
    picture: pictureOf(attachments),
  };
}

// webLinkCard draws a card for an item; actions stand below it.
export function webLinkCard(item, attachments, { actions = [], id } = {}) {
  const parts = webLinkParts(item, attachments);
  const meta = [parts.site, parts.author, parts.published].filter(Boolean).join(" · ");
  return h("div", { class: "weblink-card", id, "data-url": parts.url },
    parts.picture ? h("img", { class: "weblink-picture", src: `/api/items/${encodeURIComponent(item.id)}/attachments/${encodeURIComponent(parts.picture.id)}`, alt: "", loading: "lazy" }) : null,
    h("div", { class: "weblink-text" },
      meta ? h("p", { class: "weblink-site muted", text: meta }) : null,
      parts.url ? h("a", { class: "weblink-title", href: parts.url, target: "_blank", rel: "noopener noreferrer", text: parts.title }) : h("span", { class: "weblink-title", text: parts.title }),
      parts.description ? h("p", { class: "weblink-description", text: parts.description }) : null,
      actions.length ? h("p", { class: "actions-row weblink-actions" }, ...actions) : null));
}
