// The album's map (ADR-0175): the readable pictures that tell where they
// were taken, as clustered points on a canvas. Tiles come through the
// server from the tile service the operator names; without one the map
// draws a grid of degrees. Dragging pans, the wheel and the buttons zoom,
// and a click on a point shows its pictures below the map.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { TILE, MAX_ZOOM, cluster, fit, pan, project, tiles, unproject, zoomAt } from "./photoparts.js";

const thumb = (p) => `/api/items/${p.item}/attachments/${p.file}/thumbnail?edge=400`;

// photoMap shows the places of the pictures the query keeps (the album's
// filters as URL parameters); open shows one picture.
export async function photoMap(query, open) {
  const canvas = h("canvas", { class: "photo-map-canvas", tabindex: "0", "aria-label": t("photos.mapLabel") });
  const strip = h("ul", { class: "album-grid photo-map-pictures", "aria-live": "polite" });
  const note = h("p", { class: "muted photo-map-note", role: "status" });
  const zoomIn = h("button", { type: "button", class: "secondary", text: "+", "aria-label": t("photos.zoomIn") });
  const zoomOut = h("button", { type: "button", class: "secondary", text: "−", "aria-label": t("photos.zoomOut") });
  const box = h("div", { class: "photo-map" }, h("div", { class: "photo-map-zoom" }, zoomIn, zoomOut), canvas);
  const root = h("div", { class: "photo-map-view" }, note, box, strip);

  const answer = await get("/api/album/places?" + query);
  const places = answer.places;
  note.textContent = !places.length ? t("photos.mapNone")
    : [t("photos.mapCount", { n: answer.total }), answer.truncated ? t("photos.mapTruncated", { n: places.length }) : "", answer.tiles ? "" : t("photos.mapNoTiles")].filter(Boolean).join(" ");

  let view = { lat: 20, lon: 0, zoom: 2 };
  let fitted = false;
  let points = [];
  const images = new Map();
  const size = () => ({ w: canvas.clientWidth || 800, h: canvas.clientHeight || 480 });

  // tile is the image of a tile, loading; it draws the map again once it
  // has arrived.
  const tile = (z, x, y) => {
    const key = `${z}/${x}/${y}`;
    let img = images.get(key);
    if (!img) {
      img = new Image();
      img.onload = () => draw();
      img.src = `/api/map/tiles/${key}`;
      images.set(key, img);
      if (images.size > 400) images.delete(images.keys().next().value);
    }
    return img;
  };

  const style = (name, fallback) => getComputedStyle(root).getPropertyValue(name).trim() || fallback;

  const draw = () => {
    const { w, h: ht } = size();
    const ratio = window.devicePixelRatio || 1;
    if (canvas.width !== Math.round(w * ratio) || canvas.height !== Math.round(ht * ratio)) {
      canvas.width = Math.round(w * ratio);
      canvas.height = Math.round(ht * ratio);
    }
    if (!fitted && places.length) {
      view = fit(places, w, ht);
      fitted = true;
    }
    const g = canvas.getContext("2d");
    g.setTransform(ratio, 0, 0, ratio, 0, 0);
    g.fillStyle = style("--surface-2", "#eef1f4");
    g.fillRect(0, 0, w, ht);
    if (answer.tiles) {
      for (const tl of tiles(view, view.zoom, w, ht)) {
        const img = tile(tl.z, tl.x, tl.y);
        if (img.complete && img.naturalWidth) g.drawImage(img, tl.left, tl.top, TILE, TILE);
      }
    } else {
      // A grid of degrees, every 30° at a world view, finer deeper in.
      const step = view.zoom < 4 ? 30 : view.zoom < 7 ? 5 : view.zoom < 10 ? 1 : 0.1;
      const c = project(view.lat, view.lon, view.zoom);
      const left = c.x - w / 2;
      const top = c.y - ht / 2;
      const a = unproject(left, top, view.zoom);
      const b = unproject(left + w, top + ht, view.zoom);
      g.strokeStyle = style("--border", "#c9d1d9");
      g.lineWidth = 1;
      g.beginPath();
      for (let lon = Math.floor(a.lon / step) * step; lon <= b.lon; lon += step) {
        const x = project(0, lon, view.zoom).x - left;
        g.moveTo(x, 0); g.lineTo(x, ht);
      }
      for (let lat = Math.floor(b.lat / step) * step; lat <= a.lat; lat += step) {
        const y = project(lat, 0, view.zoom).y - top;
        g.moveTo(0, y); g.lineTo(w, y);
      }
      g.stroke();
    }
    points = cluster(places, view, view.zoom, w, ht);
    const accent = style("--accent", "#2563eb");
    for (const p of points) {
      const r = p.count === 1 ? 7 : Math.min(26, 10 + Math.log2(p.count) * 3);
      g.beginPath();
      g.arc(p.x, p.y, r, 0, 2 * Math.PI);
      g.fillStyle = accent;
      g.globalAlpha = 0.85;
      g.fill();
      g.globalAlpha = 1;
      g.lineWidth = 2;
      g.strokeStyle = "#fff";
      g.stroke();
      if (p.count > 1) {
        g.fillStyle = "#fff";
        g.font = "600 12px system-ui, sans-serif";
        g.textAlign = "center";
        g.textBaseline = "middle";
        g.fillText(String(p.count), p.x, p.y);
      }
    }
  };

  // show lists the pictures of a point below the map.
  const show = (pt) => {
    strip.replaceChildren(...pt.places.slice(0, 200).map((p) => h("li", { class: "album-card photo-map-card" },
      h("button", { type: "button", class: "album-thumb", "aria-label": p.name, onclick: () => open(p) },
        h("img", { src: thumb(p), alt: p.name, loading: "lazy" })),
      h("span", { class: "muted", text: p.taken?.day || "" }))));
  };

  const zoomTo = (next, px, py) => {
    const { w, h: ht } = size();
    view = zoomAt(view, view.zoom, next, px ?? w / 2, py ?? ht / 2, w, ht);
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
  canvas.addEventListener("pointerdown", (e) => {
    drag = { x: e.clientX, y: e.clientY, moved: false };
    canvas.setPointerCapture(e.pointerId);
  });
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
    const x = e.clientX - r.left;
    const y = e.clientY - r.top;
    const hit = points.find((p) => Math.hypot(p.x - x, p.y - y) <= (p.count === 1 ? 10 : 26));
    if (!hit) return;
    // A point of many places at a shallow zoom opens up; otherwise its
    // pictures are shown.
    if (hit.count > 1 && view.zoom < MAX_ZOOM - 3 && new Set(hit.places.map((p) => `${p.lat.toFixed(4)},${p.lon.toFixed(4)}`)).size > 1) zoomTo(view.zoom + 2, hit.x, hit.y);
    else show(hit);
  });
  canvas.addEventListener("keydown", (e) => {
    const step = 80;
    const moves = { ArrowLeft: [step, 0], ArrowRight: [-step, 0], ArrowUp: [0, step], ArrowDown: [0, -step] };
    if (moves[e.key]) {
      e.preventDefault();
      view = { ...pan(view, view.zoom, ...moves[e.key]), zoom: view.zoom };
      draw();
    } else if (e.key === "+" || e.key === "=") zoomTo(view.zoom + 1);
    else if (e.key === "-") zoomTo(view.zoom - 1);
  });
  new ResizeObserver(() => draw()).observe(canvas);
  requestAnimationFrame(draw);
  return root;
}
