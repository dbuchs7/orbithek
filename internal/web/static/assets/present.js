// Presenting pictures of the album (ADR-0176): the pictures the album
// shows — a folder, narrowed by its filters — become slides, in the order
// of the folder or collection set by hand and else by name; whoever may
// change that order drags the slides of the strip (ADR-0195). The presenter's window shows the slide, the next one, the
// speaker's notes, the time and every slide; a second window, opened from
// it and moved to the projector, shows the slide alone. The two talk over
// a BroadcastChannel of this browser: nothing goes through the server but
// the pictures and notes the presenter may read anyway. Both windows step
// through the slides, zoom and point with a laser pointer; the presenter's
// window holds the state and sends it to the other on every change.
//
// The decisions that need no browser are in present-parts.js, tested in
// Node (jstest/present.test.mjs).
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { get, put } from "./api.js";
import { moveWithin, orderDrop } from "./photoparts.js";
import {
  LASER_TRAIL, ZOOM_STEP, slidesOf, orderOf, clampView, deckParams, placement, toImage, toView, zoomAt, panBy, keyAction, clock,
  channelName, newId, validMessage, apply,
} from "./present-parts.js";

const fileURL = (s) => `/api/items/${s.item.id}/attachments/${s.attachment.id}`;
// posterURL is the picture of a video slide shown until it plays (ADR-0192).
const posterURL = (s) => `${fileURL(s)}/thumbnail?edge=1600`;

// showHref is the address of the presentation window of a presentation.
const showHref = (id) => `${location.pathname}#/show?s=${encodeURIComponent(id)}`;

// openWindow opens the presentation window of a presentation, or brings
// it forward. A browser lets a page open a window only on a click, so the
// album opens it before the presenter's window loads the slides.
const openWindow = (id) => window.open(showHref(id), "ot-present-" + id, "popup,width=1024,height=640");

// started is the presentation window the album opened last, for the
// presenter's window that follows.
let started = null;

// startPresentation opens the presentation window for the album's filters
// and leads this window to the presenter's view. It is called on a click.
export function startPresentation(filters) {
  const id = newId(crypto.getRandomValues(new Uint8Array(8)));
  started = { id, win: openWindow(id) };
  const q = deckParams(filters);
  q.set("s", id);
  location.hash = "#/present?" + q;
}

// loadDeck reads every picture the filters select, page by page, in the
// order of slides (ADR-0195), and whether one may change that order.
async function loadDeck(filters) {
  const all = [];
  let offset = 0;
  for (;;) {
    const q = deckParams(filters);
    q.set("order", "slides");
    q.set("limit", "200");
    q.set("offset", String(offset));
    const page = await get("/api/album?" + q);
    all.push(...page.pictures);
    if (!page.next) return { slides: slidesOf(all), orderable: !!page.orderable };
    offset = page.next;
  }
}

// stage shows one slide, fitted to its box and zoomed about a point of it,
// with the laser pointer's trail on a canvas above it. Its box may have
// any size: the slide's spot, the zoom and the pointer are points of the
// picture, from 0 to 1, so every window shows the same part of it.
//
// A video slide (ADR-0192) plays in a video element as the state says,
// with sound only where sound is set; a small stage shows its poster.
// onEnded is told when it played to its end.
function stage({ id, onPoint, onZoom, onPan, onEnded, small = false, sound = false }) {
  const img = h("img", { class: "present-img", alt: "", draggable: "false" });
  const vid = h("video", { class: "present-video", playsinline: true, preload: "metadata", hidden: true });
  vid.muted = !sound;
  vid.addEventListener("ended", () => onEnded?.());
  const canvas = h("canvas", { class: "present-laser", "aria-hidden": "true" });
  const black = h("div", { class: "present-black", hidden: true });
  const hush = h("p", { class: "present-sound", hidden: true, text: t("present.soundHint") });
  const box = h("div", { class: "present-stage" + (small ? " present-stage-small" : ""), id }, img, vid, canvas, black, hush);
  let state = null;
  let current = null;
  const playingVideo = () => !small && !!current?.video;
  let trail = [];
  let frame = 0;
  const view = () => ({ w: box.clientWidth, h: box.clientHeight });
  const image = () => ({ w: img.naturalWidth, h: img.naturalHeight });
  const ready = () => img.complete && img.naturalWidth > 0;

  const draw = () => {
    if (!state || !ready() || playingVideo()) return;
    const p = placement(view(), image(), small ? 1 : state.zoom, state.center);
    img.style.width = image().w * p.scale + "px";
    img.style.height = image().h * p.scale + "px";
    img.style.transform = `translate(${p.left}px, ${p.top}px)`;
    drawLaser();
  };
  const drawLaser = () => {
    cancelAnimationFrame(frame);
    const v = view();
    const ratio = window.devicePixelRatio || 1;
    if (canvas.width !== Math.round(v.w * ratio) || canvas.height !== Math.round(v.h * ratio)) {
      canvas.width = Math.round(v.w * ratio);
      canvas.height = Math.round(v.h * ratio);
    }
    const g = canvas.getContext("2d");
    g.setTransform(ratio, 0, 0, ratio, 0, 0);
    g.clearRect(0, 0, v.w, v.h);
    const now = performance.now();
    trail = trail.filter((p) => p.at === null ? false : now - p.t < LASER_TRAIL);
    if (!state || !ready() || !trail.length) return;
    const z = small ? 1 : state.zoom;
    trail.forEach((p, i) => {
      const at = toView(v, image(), z, state.center, p.at);
      const last = i === trail.length - 1;
      const fade = last ? 1 : Math.max(0, 1 - (now - p.t) / LASER_TRAIL);
      g.beginPath();
      g.arc(at.x, at.y, last ? 9 : 6 * fade + 1, 0, Math.PI * 2);
      g.fillStyle = `rgba(255, 32, 32, ${last ? 0.95 : 0.55 * fade})`;
      if (last) { g.shadowColor = "rgba(255, 0, 0, 0.9)"; g.shadowBlur = 14; } else g.shadowBlur = 0;
      g.fill();
    });
    // The trail fades by itself; the dot stays where it is.
    if (trail.length > 1) frame = requestAnimationFrame(drawLaser);
  };
  img.addEventListener("load", draw);
  if ("ResizeObserver" in window) new ResizeObserver(draw).observe(box);

  // The pointer over the slide: the laser's spot while the laser is on,
  // dragging a zoomed slide otherwise; the wheel zooms about the pointer.
  const local = (e) => { const r = box.getBoundingClientRect(); return { x: e.clientX - r.left, y: e.clientY - r.top }; };
  let drag = null;
  if (!small) {
    box.addEventListener("pointermove", (e) => {
      if (!state || !ready() || playingVideo()) return;
      if (state.laser) return onPoint?.(toImage(view(), image(), state.zoom, state.center, local(e)));
      if (drag) {
        const p = local(e);
        onPan?.(panBy(view(), image(), state.zoom, state.center, p.x - drag.x, p.y - drag.y));
        drag = p;
      }
    });
    box.addEventListener("pointerleave", () => { if (state?.laser) onPoint?.(null); });
    box.addEventListener("pointerdown", (e) => {
      if (!state || state.laser || state.zoom <= 1) return;
      drag = local(e);
      box.setPointerCapture(e.pointerId);
      box.classList.add("dragging");
    });
    const end = () => { drag = null; box.classList.remove("dragging"); };
    box.addEventListener("pointerup", end);
    box.addEventListener("pointercancel", end);
    box.addEventListener("wheel", (e) => {
      if (!state || !ready() || playingVideo()) return;
      e.preventDefault();
      onZoom?.(zoomAt(view(), image(), state.zoom, state.center, local(e), e.deltaY < 0 ? ZOOM_STEP : 1 / ZOOM_STEP));
    }, { passive: false });
    box.addEventListener("dblclick", () => { if (!playingVideo()) onZoom?.({ zoom: 1, center: { x: 0.5, y: 0.5 } }); });
  }

  return {
    el: box,
    view, image,
    // show draws a slide in a state; the picture is loaded only when the
    // slide changes.
    show(slide, s) {
      state = s;
      // The laser's spot goes when the laser is put away.
      if (!s.laser && trail.length) { trail = []; drawLaser(); }
      box.classList.toggle("laser-on", !!s.laser);
      box.classList.toggle("zoomed", !small && s.zoom > 1);
      black.hidden = !s.black || small;
      current = slide;
      box.classList.toggle("video-slide", !!slide?.video);
      if (playingVideo()) {
        img.hidden = true;
        vid.hidden = false;
        const src = fileURL(slide);
        if (vid.getAttribute("src") !== src) {
          trail = [];
          drawLaser();
          if (slide.poster) vid.poster = posterURL(slide);
          else vid.removeAttribute("poster");
          vid.setAttribute("aria-label", slide.name);
          vid.src = src;
        }
        if (s.playing && !s.black) {
          // A browser may refuse sound before a click in this window; the
          // video then plays without, and a click turns it on.
          vid.play().catch(() => {
            if (vid.muted) return;
            vid.muted = true;
            hush.hidden = false;
            vid.play().catch(() => {});
          });
        } else vid.pause();
        return;
      }
      if (vid.hasAttribute("src")) { vid.pause(); vid.removeAttribute("src"); vid.load(); }
      vid.hidden = true;
      img.hidden = false;
      if (!slide) { img.removeAttribute("src"); img.alt = ""; return; }
      const src = slide.video ? (slide.poster ? posterURL(slide) : "") : fileURL(slide);
      if (!src) { img.removeAttribute("src"); img.alt = slide.name; return; }
      if (img.getAttribute("src") !== src) {
        trail = [];
        img.alt = slide.name;
        img.src = src;
      }
      draw();
    },
    // unmute turns on the sound a browser held back until a click; it
    // reports whether it did.
    unmute() {
      if (!sound || hush.hidden) return false;
      vid.muted = false;
      hush.hidden = true;
      return true;
    },
    // laser shows the laser's spot, or takes it away with null.
    laser(at) {
      if (at === null) trail = [];
      else trail.push({ at, t: performance.now() });
      drawLaser();
    },
  };
}

// fullscreen asks for the whole screen for an element, or leaves it.
function fullscreen(el) {
  if (document.fullscreenElement) document.exitFullscreen?.().catch(() => {});
  else el.requestFullscreen?.().catch(() => {});
}

// typing reports whether a key goes into a field, where it is no command.
const typing = (e) => e.target instanceof Element && e.target.closest("input, textarea, select, [contenteditable]");

// presenterView is the presenter's window: it loads the deck from the
// album's filters in params and leads the presentation window the album
// opened, or one opened from here.
export async function presenterView({ params, show, onLeave, done }) {
  const { slides, orderable } = await loadDeck(params);
  // The folder or collection whose order the slides follow (ADR-0195).
  const container = orderOf(params);
  if (!slides.length) {
    show(h("div", { class: "present-empty" }, h("p", { text: t("present.empty") }),
      h("button", { type: "button", class: "secondary", text: t("present.back"), onclick: done })));
    return;
  }
  // The presentation keeps its id over a reload of this window; the window
  // the album opened for it is taken over.
  const id = /^[0-9a-f]{16}$/.test(params.get("s") || "") ? params.get("s") : newId(crypto.getRandomValues(new Uint8Array(8)));
  const channel = new BroadcastChannel(channelName(id));
  let state = { slides, index: 0, zoom: 1, center: { x: 0.5, y: 0.5 }, laser: false, black: false };
  const begun = Date.now();
  let audience = started?.id === id ? started.win : null;
  started = null;
  let audienceSeen = false;

  const send = (m) => channel.postMessage(m);
  const main = stage({
    id: "present-current",
    onPoint: (at) => { main.laser(at); send({ type: "laser", at }); },
    onZoom: (v) => set({ ...state, ...v }),
    onPan: (v) => set({ ...state, ...v }),
    onEnded: () => command("ended"),
  });
  const next = stage({ id: "present-next", small: true });
  const counter = h("span", { class: "present-counter", id: "present-counter" });
  const name = h("strong", { class: "present-name", id: "present-name" });
  const notesBox = h("div", { class: "present-notes", id: "present-notes", "aria-live": "polite" });
  const elapsed = h("span", { class: "present-time", id: "present-elapsed", title: t("present.elapsed") });
  const now = h("span", { class: "present-time muted", id: "present-clock", title: t("present.clock") });
  const linkState = h("span", { class: "present-link muted", id: "present-link", role: "status" });
  const strip = h("ol", { class: "present-strip", id: "present-strip", "aria-label": t("present.slides") });
  // The strip shows every slide; where one may change the order, a slide
  // is dragged before another, or after it onto its latter half, and the
  // order is kept for the folder or collection (ADR-0195).
  const canOrder = orderable && !!container;
  let draggingSlide = null;
  const fillStrip = () => strip.replaceChildren(...state.slides.map((s, i) => {
    const b = h("button", { type: "button", class: "present-thumb", "data-index": String(i), title: s.name, draggable: canOrder ? "true" : "false",
      "aria-label": t("present.goTo", { n: i + 1, name: s.name }), onclick: () => set({ ...state, index: i, zoom: 1, center: { x: 0.5, y: 0.5 } }) },
    s.video && !s.poster ? h("span", { class: "present-thumb-video", "aria-hidden": "true" }) : h("img", { src: s.video ? posterURL(s) : fileURL(s), alt: "", loading: "lazy", draggable: "false" }),
    h("span", { text: String(i + 1) }));
    if (canOrder) {
      const after = (e) => { const r = b.getBoundingClientRect(); return e.clientX > r.left + r.width / 2; };
      b.addEventListener("dragstart", (e) => {
        draggingSlide = s.first;
        e.dataTransfer.effectAllowed = "move";
        e.dataTransfer.setData("text/plain", s.name);
      });
      b.addEventListener("dragend", () => {
        draggingSlide = null;
        for (const el of strip.querySelectorAll(".drop-before, .drop-after")) el.classList.remove("drop-before", "drop-after");
      });
      const over = (e) => {
        if (!draggingSlide) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        const a = after(e);
        b.classList.toggle("drop-after", a);
        b.classList.toggle("drop-before", !a);
      };
      b.addEventListener("dragenter", over);
      b.addEventListener("dragover", over);
      b.addEventListener("dragleave", (e) => { if (!b.contains(e.relatedTarget)) b.classList.remove("drop-before", "drop-after"); });
      b.addEventListener("drop", (e) => {
        b.classList.remove("drop-before", "drop-after");
        if (!draggingSlide) return;
        e.preventDefault();
        const moved = draggingSlide;
        draggingSlide = null;
        const firsts = state.slides.map((x) => x.first);
        placeSlide(moved, orderDrop(firsts, [moved], i, after(e), true));
      });
    }
    return h("li", {}, b);
  }));
  // placeSlide moves a slide before another ("" for the end), keeps the
  // order for the folder or collection, and stays on the slide shown.
  const placeSlide = async (moved, before) => {
    const current = state.slides[state.index];
    const slides = moveWithin(state.slides, (x) => x.first, [moved], before);
    if (slides.every((x, i) => x === state.slides[i])) return;
    try {
      await put(`/api/album/order/${encodeURIComponent(container)}`, { pictures: [moved], before });
    } catch (err) {
      linkState.textContent = err.message;
      return;
    }
    set({ ...state, slides, index: Math.max(0, slides.indexOf(current)) });
    fillStrip();
    render();
  };
  const btn = (id, text, title, onclick, cls = "secondary") => h("button", { type: "button", id, class: cls, text, title, "aria-label": title, onclick });
  const laserBtn = btn("present-laser", t("present.laser"), t("present.laserTitle"), () => command("laser"));
  const blackBtn = btn("present-blackout", t("present.black"), t("present.blackTitle"), () => command("black"));
  const zoomText = h("span", { class: "present-zoom muted", id: "present-zoom" });

  // notes shows the speaker's notes of the slide; one who may edit its
  // picture writes them here, and they are kept with it (ADR-0176).
  let editing = false;
  const showNotes = () => {
    const s = state.slides[state.index];
    if (editing) return;
    const text = s.notes ? h("div", { class: "present-notes-text", text: s.notes }) : h("p", { class: "muted", text: t("present.noNotes") });
    notesBox.replaceChildren(text, s.editable ? h("button", { type: "button", class: "link", id: "present-notes-edit",
      text: s.notes ? t("present.notesEdit") : t("present.notesAdd"), onclick: editNotes }) : null);
  };
  const editNotes = () => {
    const s = state.slides[state.index];
    editing = true;
    const area = h("textarea", { id: "present-notes-input", rows: "8", "aria-label": t("present.notesOf", { name: s.name }), value: s.notes });
    const status = h("p", { class: "status", role: "status" });
    const close = () => { editing = false; showNotes(); };
    const save = h("button", { type: "button", class: "primary", id: "present-notes-save", text: t("present.notesSave"), onclick: async () => {
      try {
        await put(`/api/items/${s.item.id}/attachments/${s.attachment.id}/notes`, { notes: area.value });
        s.notes = area.value.replace(/\r\n?/g, "\n").trim();
        close();
        send({ type: "state", state });
      } catch (err) { status.replaceChildren(h("span", { class: "error", text: err.message })); }
    } });
    area.addEventListener("keydown", (e) => { if (e.key === "Escape") { e.preventDefault(); e.stopPropagation(); close(); } });
    notesBox.replaceChildren(area, h("div", { class: "actions-row" }, save,
      h("button", { type: "button", class: "link", text: t("common.cancel"), onclick: close })), status);
    area.focus();
  };

  const render = () => {
    const s = state.slides[state.index];
    main.show(s, state);
    next.show(state.slides[state.index + 1] || null, { ...state, zoom: 1, laser: false, black: false });
    counter.textContent = t("present.counter", { n: state.index + 1, total: state.slides.length });
    name.textContent = s.name;
    zoomText.textContent = state.zoom > 1 ? t("present.zoomed", { n: Math.round(state.zoom * 100) }) : "";
    laserBtn.setAttribute("aria-pressed", String(!!state.laser));
    blackBtn.setAttribute("aria-pressed", String(!!state.black));
    for (const b of strip.querySelectorAll(".present-thumb")) {
      const on = Number(b.dataset.index) === state.index;
      b.classList.toggle("current", on);
      if (on) b.setAttribute("aria-current", "true"); else b.removeAttribute("aria-current");
    }
    strip.querySelector(".present-thumb.current")?.scrollIntoView?.({ block: "nearest", inline: "nearest" });
    if (!editing) showNotes();
  };
  const set = (s) => {
    if (!s) return;
    const changed = s.index !== state.index;
    state = s;
    if (changed) { editing = false; main.laser(null); }
    render();
    send({ type: "state", state });
  };
  const command = (action, m = {}) => {
    if (action === "escape" || action === "fullscreen") return;
    // The presentation window zoomed or moved the slide with its pointer.
    if (action === "view") return set({ ...state, ...clampView(main.view(), main.image(), Number(m.zoom), m.center) });
    set(apply(state, action, main.view(), main.image()));
  };

  channel.onmessage = (e) => {
    const m = e.data;
    if (!validMessage(m)) return;
    if (m.type === "hello") { audienceSeen = true; showLink(); send({ type: "state", state }); }
    if (m.type === "command") command(m.action, m);
    if (m.type === "laser") main.laser(m.at);
    if (m.type === "bye") { audienceSeen = false; showLink(); }
  };

  const showLink = () => {
    linkState.textContent = audienceSeen ? t("present.connected") : t("present.notConnected");
    linkState.classList.toggle("ok", audienceSeen);
  };
  // openAudience opens the presentation window, or brings it forward.
  const openAudience = () => {
    if (audience && !audience.closed) return audience.focus();
    audience = openWindow(id);
    if (!audience) linkState.textContent = t("present.blocked");
  };

  const onKey = (e) => {
    if (typing(e) || e.ctrlKey || e.metaKey || e.altKey) return;
    const action = keyAction(e.key, !!state.slides[state.index].video);
    if (!action) return;
    e.preventDefault();
    if (action === "escape") return state.laser ? command("laser") : null;
    command(action);
  };
  document.addEventListener("keydown", onKey);
  const tick = setInterval(() => {
    elapsed.textContent = clock(Date.now() - begun);
    now.textContent = new Date().toLocaleTimeString(document.documentElement.lang || undefined, { hour: "2-digit", minute: "2-digit" });
  }, 500);
  onLeave(() => {
    send({ type: "bye" });
    channel.close();
    clearInterval(tick);
    document.removeEventListener("keydown", onKey);
    if (audience && !audience.closed) audience.close();
  });

  show(h("div", { class: "present-console", id: "present-console" },
    h("header", { class: "present-bar" },
      name, counter,
      h("span", { class: "present-tools" },
        btn("present-prev", "‹", t("present.prev"), () => command("prev")),
        btn("present-next-slide", "›", t("present.next"), () => command("next")),
        btn("present-zoom-out", "−", t("present.zoomOut"), () => command("zoomOut")),
        btn("present-zoom-reset", "1:1", t("present.zoomReset"), () => command("zoomReset")),
        btn("present-zoom-in", "+", t("present.zoomIn"), () => command("zoomIn")),
        zoomText, laserBtn, blackBtn),
      h("span", { class: "present-tools" },
        elapsed, now, linkState,
        btn("present-open", t("present.open"), t("present.openTitle"), openAudience, "primary"),
        btn("present-end", t("present.end"), t("present.endTitle"), done, "secondary"))),
    h("div", { class: "present-body" },
      h("section", { class: "present-main", "aria-label": t("present.current") }, main.el),
      h("aside", { class: "present-side" },
        h("h2", { class: "present-label", text: t("present.nextSlide") }), next.el,
        h("h2", { class: "present-label", text: t("present.notes") }), notesBox,
        h("p", { class: "muted present-keys", text: t("present.keys") }))),
    strip));
  fillStrip();
  render();
  showLink();
  elapsed.textContent = clock(0);
  // A window opened before the slides were loaded waits for them.
  send({ type: "state", state });
  if (!audience) linkState.textContent = t("present.openHint");
}

// audienceView is the presentation window: the slide alone, as the
// presenter's window says. Its keys and laser pointer go to that window,
// which decides.
export function audienceView({ params, show, onLeave }) {
  const id = params.get("s") || "";
  const channel = new BroadcastChannel(channelName(id));
  let state = null;
  const send = (m) => channel.postMessage(m);
  const main = stage({
    id: "show-stage",
    onPoint: (at) => { main.laser(at); send({ type: "laser", at }); },
    onZoom: (v) => send({ type: "command", action: "view", ...v }),
    onPan: (v) => send({ type: "command", action: "view", ...v }),
    onEnded: () => send({ type: "command", action: "ended" }),
    sound: true,
  });
  const waiting = h("p", { class: "present-waiting", id: "show-waiting", text: t("present.waiting") });
  // Only a gesture in this window may take the whole screen.
  const full = h("button", { type: "button", class: "present-full", id: "show-fullscreen", text: t("present.fullscreen"),
    title: t("present.fullscreenTitle"), onclick: (e) => { e.stopPropagation(); fullscreen(root); } });
  const root = h("div", { class: "present-show", id: "present-show" }, main.el, waiting, full);

  channel.onmessage = (e) => {
    const m = e.data;
    if (!validMessage(m)) return;
    if (m.type === "state") {
      state = m.state;
      waiting.hidden = true;
      main.show(state.slides[state.index], state);
      document.title = state.slides[state.index]?.name || document.title;
    }
    if (m.type === "laser") main.laser(m.at);
    if (m.type === "bye") window.close();
  };
  const video = () => !!state?.slides[state.index]?.video;
  const onKey = (e) => {
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    const action = keyAction(e.key, video());
    if (!action) return;
    e.preventDefault();
    if (action === "fullscreen") return fullscreen(root);
    if (action === "escape") return;
    send({ type: "command", action });
  };
  // A click moves on, as a remote's button does, unless the laser is on
  // or the slide is zoomed, when the pointer moves it. On a video it plays
  // and pauses, or first turns on the sound a browser held back.
  main.el.addEventListener("click", (e) => {
    if (!state || e.detail !== 1 || main.unmute()) return;
    if (video()) return send({ type: "command", action: "play" });
    if (!state.laser && state.zoom <= 1) send({ type: "command", action: "next" });
  });
  document.addEventListener("fullscreenchange", () => root.classList.toggle("full", document.fullscreenElement === root));
  document.addEventListener("keydown", onKey);
  onLeave(() => {
    send({ type: "bye" });
    channel.close();
    document.removeEventListener("keydown", onKey);
  });
  window.addEventListener("pagehide", () => send({ type: "bye" }));
  show(root);
  send({ type: "hello" });
}
