// Places set by hand (ADR-0180): a field that suggests places for the
// first letters typed, from the place search the server asks, and a map to
// choose a point on. Both answer a place: lat, lon and a name.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { TILE, MAX_ZOOM, pan, project, tiles, unproject, zoomAt } from "./photoparts.js";

const lang = () => (document.documentElement.lang || "").slice(0, 2);

// services tells whether places are suggested and the map has tiles, asked
// once.
let services = null;
export const placeServices = () => (services ||= get("/api/album/imports").catch(() => ({})));

let counter = 0;

// placeInput is a field that suggests places as one types; pick is told
// the place chosen. The arrow keys move through the suggestions, Enter
// takes one and Escape closes them.
export function placeInput({ value = "", label, pick, id }) {
  const listId = `place-list-${++counter}`;
  const input = h("input", { type: "text", id, value, autocomplete: "off", role: "combobox", "aria-autocomplete": "list", "aria-expanded": "false",
    "aria-controls": listId, "aria-label": label, placeholder: t("places.placeholder") });
  const list = h("ul", { class: "place-suggestions", id: listId, role: "listbox", hidden: true });
  const credit = h("p", { class: "place-credit muted", hidden: true });
  let found = [];
  let active = -1;
  let asked = 0;
  let timer = 0;
  const close = () => {
    list.hidden = credit.hidden = true;
    input.setAttribute("aria-expanded", "false");
    input.removeAttribute("aria-activedescendant");
    active = -1;
  };
  const choose = (i) => {
    const p = found[i];
    if (!p) return;
    input.value = p.label;
    close();
    pick({ lat: p.lat, lon: p.lon, name: p.label });
  };
  const mark = () => {
    [...list.children].forEach((li, i) => li.setAttribute("aria-selected", String(i === active)));
    if (active >= 0) input.setAttribute("aria-activedescendant", `${listId}-${active}`);
  };
  const show = (places, creditText) => {
    found = places;
    active = -1;
    list.replaceChildren(...places.map((p, i) => h("li", { id: `${listId}-${i}`, role: "option", "aria-selected": "false",
      onmousedown: (e) => { e.preventDefault(); choose(i); } },
      h("strong", { text: p.name }), p.label !== p.name ? h("span", { class: "muted", text: " " + p.label.slice(p.name.length + 2) }) : null)));
    if (!places.length) list.replaceChildren(h("li", { class: "muted", role: "presentation", text: t("places.none") }));
    credit.textContent = creditText || "";
    list.hidden = false;
    credit.hidden = !creditText;
    input.setAttribute("aria-expanded", "true");
  };
  input.addEventListener("input", () => {
    clearTimeout(timer);
    const q = input.value.trim();
    if (q.length < 2) return close();
    timer = setTimeout(async () => {
      const mine = ++asked;
      try {
        const r = await get(`/api/places?q=${encodeURIComponent(q)}&lang=${lang()}`);
        if (mine === asked) show(r.places, r.credit);
      } catch (err) {
        if (mine === asked) show([], "");
        list.replaceChildren(h("li", { class: "error", role: "presentation", text: err.status === 404 ? t("places.off") : err.message }));
      }
    }, 250);
  });
  input.addEventListener("keydown", (e) => {
    if (list.hidden) return;
    if (e.key === "ArrowDown" || e.key === "ArrowUp") {
      e.preventDefault();
      if (!found.length) return;
      active = (active + (e.key === "ArrowDown" ? 1 : -1) + found.length) % found.length;
      mark();
    } else if (e.key === "Enter" && active >= 0) {
      e.preventDefault();
      choose(active);
    } else if (e.key === "Escape") {
      e.preventDefault();
      e.stopPropagation();
      close();
    }
  });
  input.addEventListener("blur", () => setTimeout(close, 150));
  return { element: h("div", { class: "place-input" }, input, list, credit), input };
}

// pickPlace shows a map to choose a point on, from the place given or a
// view of the world; a click sets the point, dragging pans, the wheel and
// the buttons zoom, and the field above it moves the map to a place. It
// resolves to the place chosen, named by the place search where it can,
// or to null.
export function pickPlace(start = {}) {
  return new Promise(async (resolve) => {
    const svc = await placeServices();
    let point = start.lat != null ? { lat: start.lat, lon: start.lon } : null;
    let name = start.name || "";
    let view = point ? { ...point, zoom: 14 } : { lat: 20, lon: 0, zoom: 2 };
    const canvas = h("canvas", { class: "place-picker-canvas", tabindex: "0", "aria-label": t("places.pickerLabel") });
    const where = h("p", { class: "muted", id: "place-picker-where", "aria-live": "polite" });
    const ok = h("button", { type: "button", class: "primary", id: "place-picker-ok", text: t("places.take"), disabled: !point });
    let done = false;
    const finish = (value) => {
      if (done) return;
      done = true;
      dialog.close();
      dialog.remove();
      resolve(value);
    };
    const search = svc.places ? placeInput({ label: t("places.searchMap"), id: "place-picker-search", pick: (p) => {
      view = { lat: p.lat, lon: p.lon, zoom: Math.max(view.zoom, 13) };
      point = { lat: p.lat, lon: p.lon };
      name = p.name;
      update();
    } }).element : null;
    const zoomIn = h("button", { type: "button", class: "secondary", text: "+", "aria-label": t("photos.zoomIn") });
    const zoomOut = h("button", { type: "button", class: "secondary", text: "−", "aria-label": t("photos.zoomOut") });
    const credit = h("p", { class: "photo-map-credit", text: svc.tiles ? svc.credit || "" : "", hidden: !svc.tiles || !svc.credit });
    const dialog = h("dialog", { class: "place-picker", id: "place-picker", "aria-label": t("places.pickerTitle") },
      h("div", { class: "drawing-bar" }, h("strong", { text: t("places.pickerTitle") }),
        h("button", { type: "button", id: "place-picker-cancel", text: t("common.cancel"), onclick: () => finish(null) })),
      h("div", { class: "place-picker-body" }, search,
        h("div", { class: "photo-map" }, h("div", { class: "photo-map-zoom" }, zoomIn, zoomOut), canvas, credit),
        where, h("div", { class: "actions-row" }, ok)));
    ok.addEventListener("click", () => finish(point ? { ...point, name } : null));
    dialog.addEventListener("cancel", (e) => { e.preventDefault(); finish(null); });
    const images = new Map();
    const tile = (z, x, y) => {
      const key = `${z}/${x}/${y}`;
      let img = images.get(key);
      if (!img) {
        img = new Image();
        img.onload = () => draw();
        img.src = `/api/map/tiles/${key}`;
        images.set(key, img);
      }
      return img;
    };
    const size = () => ({ w: canvas.clientWidth || 640, h: canvas.clientHeight || 400 });
    const draw = () => {
      const { w, h: ht } = size();
      const ratio = window.devicePixelRatio || 1;
      canvas.width = Math.round(w * ratio);
      canvas.height = Math.round(ht * ratio);
      const g = canvas.getContext("2d");
      g.setTransform(ratio, 0, 0, ratio, 0, 0);
      g.fillStyle = "#eef1f4";
      g.fillRect(0, 0, w, ht);
      if (svc.tiles) {
        for (const tl of tiles(view, view.zoom, w, ht)) {
          const img = tile(tl.z, tl.x, tl.y);
          if (img.complete && img.naturalWidth) g.drawImage(img, tl.left, tl.top, TILE, TILE);
        }
      }
      if (point) {
        const c = project(view.lat, view.lon, view.zoom);
        const p = project(point.lat, point.lon, view.zoom);
        const x = p.x - c.x + w / 2;
        const y = p.y - c.y + ht / 2;
        g.beginPath();
        g.arc(x, y, 8, 0, 2 * Math.PI);
        g.fillStyle = getComputedStyle(dialog).getPropertyValue("--accent").trim() || "#2563eb";
        g.fill();
        g.lineWidth = 3;
        g.strokeStyle = "#fff";
        g.stroke();
      }
    };
    const update = () => {
      ok.disabled = !point;
      where.textContent = point ? `${name ? name + " · " : ""}${point.lat.toFixed(5)}, ${point.lon.toFixed(5)}` : t("places.clickMap");
      draw();
    };
    let asked = 0;
    const nameOf = async () => {
      if (!svc.places || !point) return;
      const mine = ++asked;
      try {
        const r = await get(`/api/places/reverse?lat=${point.lat}&lon=${point.lon}&lang=${lang()}`);
        if (mine === asked) { name = r.place?.label || ""; update(); }
      } catch { /* the point without a name */ }
    };
    const zoomTo = (next, px, py) => {
      const { w, h: ht } = size();
      view = zoomAt(view, view.zoom, Math.min(MAX_ZOOM, next), px ?? w / 2, py ?? ht / 2, w, ht);
      draw();
    };
    zoomIn.addEventListener("click", () => zoomTo(view.zoom + 1));
    zoomOut.addEventListener("click", () => zoomTo(view.zoom - 1));
    canvas.addEventListener("wheel", (e) => {
      e.preventDefault();
      const r = canvas.getBoundingClientRect();
      zoomTo(view.zoom + (e.deltaY < 0 ? 1 : -1), e.clientX - r.left, e.clientY - r.top);
    }, { passive: false });
    let drag = null;
    canvas.addEventListener("pointerdown", (e) => { drag = { x: e.clientX, y: e.clientY, moved: false }; canvas.setPointerCapture(e.pointerId); });
    canvas.addEventListener("pointermove", (e) => {
      if (!drag) return;
      const dx = e.clientX - drag.x;
      const dy = e.clientY - drag.y;
      if (Math.abs(dx) + Math.abs(dy) > 3) drag.moved = true;
      if (!drag.moved) return;
      view = { ...pan(view, view.zoom, dx, dy), zoom: view.zoom };
      drag.x = e.clientX;
      drag.y = e.clientY;
      draw();
    });
    canvas.addEventListener("pointerup", (e) => {
      const was = drag;
      drag = null;
      if (!was || was.moved) return;
      const r = canvas.getBoundingClientRect();
      const { w, h: ht } = size();
      const c = project(view.lat, view.lon, view.zoom);
      point = unproject(c.x - w / 2 + (e.clientX - r.left), c.y - ht / 2 + (e.clientY - r.top), view.zoom);
      name = "";
      update();
      nameOf();
    });
    document.body.append(dialog);
    dialog.showModal();
    update();
  });
}
