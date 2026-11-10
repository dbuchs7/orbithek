// The album's map (ADR-0177): the readable pictures that tell where they
// were taken, as clustered points on a canvas. Tiles come through the
// server from the tile service the operator names; without one the map
// draws a grid of degrees. Dragging pans, the wheel and the buttons zoom,
// a click on a point shows all its pictures below the map, and a double
// click zooms in on it.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get } from "./api.js";
import { TILE, MAX_ZOOM, cluster, fit, hit, pan, project, radius, tiles, unproject, zoomAt } from "./photoparts.js";

const thumb = (p) => `/api/items/${p.item}/attachments/${p.file}/thumbnail?edge=400`;

// photoMap shows the places of the pictures the query keeps (the album's
// filters as URL parameters); open(list, index) shows the pictures of a
// point, from the one chosen.
export async function photoMap(query, open) {
  const canvas = h("canvas", { class: "photo-map-canvas", tabindex: "0", "aria-label": t("photos.mapLabel") });
  const strip = h("ul", { class: "album-grid photo-map-pictures" });
  const chosenHead = h("div", { class: "photo-map-chosen", id: "photo-map-chosen", "aria-live": "polite", hidden: true });
  const note = h("p", { class: "muted photo-map-note", role: "status" });
  const zoomIn = h("button", { type: "button", class: "secondary", text: "+", "aria-label": t("photos.zoomIn") });
  const zoomOut = h("button", { type: "button", class: "secondary", text: "−", "aria-label": t("photos.zoomOut") });
  const credit = h("p", { class: "photo-map-credit" });
  const box = h("div", { class: "photo-map" }, h("div", { class: "photo-map-zoom" }, zoomIn, zoomOut), canvas, credit);
  const root = h("div", { class: "photo-map-view" }, note, box, chosenHead, strip);

  const answer = await get("/api/album/places?" + query);
  const places = answer.places;
  // The tile service's credit, as it asks for it (ADR-0179).
  credit.textContent = answer.tiles ? answer.credit || "" : "";
  credit.hidden = !credit.textContent;
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
      const r = radius(p.count);
      // The point whose pictures are shown below carries a ring.
      if (chosen && p.places.some((q) => chosen.has(q.file))) {
        g.beginPath();
        g.arc(p.x, p.y, r + 5, 0, 2 * Math.PI);
        g.lineWidth = 3;
        g.strokeStyle = accent;
        g.stroke();
      }
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

  // show lists every picture of a point below the map, newest first, and
  // marks the point; the viewer steps through them all.
  let chosen = null;
  const show = (pt) => {
    const list = [...pt.places].sort((a, b) => String(b.taken?.at || "").localeCompare(String(a.taken?.at || "")));
    chosen = new Set(list.map((p) => p.file));
    const zoom = h("button", { type: "button", class: "link", id: "photo-map-zoom-here", text: t("photos.mapZoomHere"),
      onclick: () => zoomTo(view.zoom + 2, pt.x, pt.y) });
    // replaceChildren, unlike h, writes null as text.
    chosenHead.replaceChildren(...[h("strong", { text: t("photos.mapPlacePictures", { n: list.length }) }),
      view.zoom < MAX_ZOOM && pt.count > 1 ? zoom : null].filter(Boolean));
    chosenHead.hidden = false;
    strip.replaceChildren(...list.map((p, i) => h("li", { class: "album-card photo-map-card" },
      h("button", { type: "button", class: "album-thumb", "aria-label": p.name, onclick: () => open(list, i) },
        p.video && !p.poster ? h("span", { class: "album-video-symbol", role: "img", "aria-label": p.name }) : h("img", { src: thumb(p), alt: p.name, loading: "lazy" })),
      h("span", { class: "muted", text: p.taken?.day || "" }))));
    draw();
    chosenHead.scrollIntoView({ block: "nearest", behavior: "smooth" });
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
    const pt = hit(points, x, y);
    if (pt) show(pt);
  });
  // A double click zooms in on the point it lands on, or where it is.
  canvas.addEventListener("dblclick", (e) => {
    e.preventDefault();
    const r = canvas.getBoundingClientRect();
    const x = e.clientX - r.left;
    const y = e.clientY - r.top;
    const pt = hit(points, x, y);
    zoomTo(view.zoom + 2, pt ? pt.x : x, pt ? pt.y : y);
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
