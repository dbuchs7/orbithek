// A live whiteboard (ADR-0169, ADR-0170, ADR-0171): the drawing page in a
// live session. The page follows the session's server-sent events and
// sends operations; it shows the others' cursors and what they hold, and
// adds the tools of a workshop: notes, voting, a timer, comments on
// elements and a presentation. When the server asks, the keeper's page
// draws the scene as it was at that event and sends the picture.
//
// The page runs under the drawing page's own policy (ADR-0053); it talks
// to the API of its own origin, with the session's cookie.
import { setLanguage, t } from "./i18n.js";
import { get, post, put, ApiError } from "./api.js";
import { KEY, PADDING, regionsOf, rootsOf, withRegions } from "./drawprocesses.js";
import * as P from "./whiteboard-parts.js";
import {
  React, Excalidraw, exportToBlob, MIME_TYPES, convertToExcalidrawElements, getCommonBounds, CaptureUpdateAction,
} from "/vendor/excalidraw.js";

const h = React.createElement;
const NEVER = CaptureUpdateAction.NEVER;

// startWhiteboard joins the whiteboard of item and file and shows it in
// root. palette makes the palette chooser of the drawing page, bind hands
// it Excalidraw's API, onLibrary its library.
export function startWhiteboard({ root, dark, lang, item, file, palette, bind, onLibrary }) {
  setLanguage(lang);
  const base = `/api/items/${encodeURIComponent(item)}/whiteboards/${encodeURIComponent(file)}`;
  const S = {
    api: null, client: "", editable: false, keeper: false, drawing: "", offset: 0, closed: "",
    truth: new Map(), files: new Set(), heldBy: new Map(), mine: new Set(),
    participants: [], pointers: new Map(), timer: null, vote: null, presentation: null, saved: null,
    comments: [], names: new Map(), panel: "", following: true, colour: "yellow", message: "",
    commentOn: null, lastChange: 0, pendingResult: false, initial: null, appState: null, rang: 0,
  };
  document.body.dataset.live = "on";

  // --- sending -------------------------------------------------------------
  let queue = [];
  let sending = false;
  const flushSoon = P.throttle(() => flush(), 60);
  const enqueue = (...ops) => {
    for (const op of ops) {
      if (op.op === "pointer") queue = queue.filter((o) => o.op !== "pointer");
      queue.push(op);
    }
    flushSoon();
  };
  async function flush() {
    if (sending || !queue.length || !S.client) return;
    sending = true;
    const ops = queue;
    queue = [];
    try {
      const res = await post(base + "/live", { client: S.client, ops });
      refused(res?.refused || []);
    } catch (err) {
      say(err.message);
    } finally {
      sending = false;
      if (queue.length) flushSoon();
    }
  }
  // act sends workshop operations at once and says why they failed.
  async function act(...ops) {
    try {
      const res = await post(base + "/live", { client: S.client, ops });
      refused(res?.refused || []);
      return true;
    } catch (err) {
      say(err.message);
      return false;
    }
  }
  function refused(list) {
    const back = [];
    let deselect = false;
    for (const r of list) {
      if (r.current) {
        S.truth.set(r.current.id, r.current);
        back.push(r.current);
      }
      if (r.by) {
        S.mine.delete(r.id);
        deselect = true;
        say(t("wb.heldBy", { name: r.by }));
      }
    }
    if (back.length) apply(back, true);
    if (deselect && S.api) {
      const sel = { ...(S.api.getAppState().selectedElementIds || {}) };
      for (const r of list) if (r.by) delete sel[r.id];
      S.api.updateScene({ appState: { selectedElementIds: sel }, captureUpdate: NEVER });
    }
  }

  // --- the scene -------------------------------------------------------------
  const sceneOf = () => S.api.getSceneElementsIncludingDeleted();
  // Excalidraw changes its elements in place. What the server holds (truth)
  // and what is queued to be sent are therefore copies, never the objects
  // the scene shows, or a change would already look known when it is made.
  const copy = (el) => structuredClone(el);

  // apply puts elements from the server into the scene, shown locked where
  // someone else holds them; force puts them even when not newer.
  function apply(els, force = false) {
    if (!S.api) return;
    const map = new Map(sceneOf().map((e) => [e.id, e]));
    for (const el of els) {
      if (!force && !P.newer(el, S.truth.get(el.id)) && map.has(el.id)) continue;
      S.truth.set(el.id, el);
      map.set(el.id, copy(P.shown(el, S.heldBy, S.client)));
    }
    S.api.updateScene({ elements: P.byIndex([...map.values()]), captureUpdate: NEVER });
    // Excalidraw loads the pictures of image elements only when files are
    // added, not when elements arrive: a picture whose file came first
    // would stay blank. Adding no file loads what is missing.
    if (els.some((el) => el.type === "image")) S.api.addFiles([]);
    redraw(P.pictures(els));
    measure(els);
    paint();
  }

  // relock shows elements again as their holders hold them.
  function relock(ids) {
    if (!S.api) return;
    const map = new Map(sceneOf().map((e) => [e.id, e]));
    const shownAnew = [];
    for (const id of ids) {
      const el = S.truth.get(id);
      if (!el) continue;
      map.set(id, copy(P.shown(el, S.heldBy, S.client)));
      shownAnew.push(el);
    }
    S.api.updateScene({ elements: P.byIndex([...map.values()]), captureUpdate: NEVER });
    redraw(P.pictures(shownAnew));
  }

  // redraw shows the image elements of files anew once their pictures are
  // decoded. Excalidraw draws a picture it has loaded only for the element
  // objects it began loading for; an element shown anew meanwhile, as a
  // lock or a newer version arrives, keeps the blank placeholder it was
  // first drawn with. What one holds is left alone, so as not to disturb
  // one's own work on it.
  const decoding = new Set();
  function redraw(fileIds) {
    const files = S.api?.getFiles() || {};
    for (const id of fileIds) {
      if (decoding.has(id) || !files[id]?.dataURL) continue;
      decoding.add(id);
      const img = new Image();
      img.src = files[id].dataURL;
      img.decode().catch(() => {}).then(() => {
        decoding.delete(id);
        // Excalidraw began loading the same picture first; a moment more
        // lets it finish.
        setTimeout(() => {
          const scene = sceneOf();
          const again = scene.filter((el) => el.type === "image" && el.fileId === id && !S.mine.has(el.id));
          if (!again.length) return;
          const map = new Map(scene.map((e) => [e.id, e]));
          again.forEach((el) => map.set(el.id, copy(el)));
          S.api.updateScene({ elements: P.byIndex([...map.values()]), captureUpdate: NEVER });
        }, 50);
      });
    }
  }

  function onChange(elements, appState, files) {
    const moved = !S.appState || S.appState.scrollX !== appState.scrollX || S.appState.scrollY !== appState.scrollY ||
      S.appState.zoom?.value !== appState.zoom?.value || S.appState.width !== appState.width;
    S.appState = appState;
    document.body.dataset.elements = String(elements.filter((e) => !e.isDeleted).length);
    if (moved) {
      paint();
      if (S.presentation?.active && S.presentation.client === S.client && !S.presentation.frame) sendView();
    }
    if (!S.api || !S.client || !S.editable) return;
    const ops = [];
    for (const [id, f] of Object.entries(files || {})) {
      if (!S.files.has(id)) {
        S.files.add(id);
        ops.push({ op: "file", file: { id: f.id, mimeType: f.mimeType, dataURL: f.dataURL, created: f.created } });
      }
    }
    const all = sceneOf();
    for (const el of P.changed(all, S.truth)) {
      // An image goes out once its file has, or the others would get a
      // picture they cannot show.
      if (el.type === "image" && el.fileId && !el.isDeleted && !S.files.has(el.fileId)) continue;
      const out = copy(P.sendable(el, S.truth, S.heldBy, S.client));
      S.truth.set(el.id, out);
      ops.push({ op: "put", element: out });
    }
    if (ops.some((o) => o.op === "put")) {
      S.lastChange = Date.now();
      paint();
    }
    const selected = Object.keys(appState.selectedElementIds || {});
    if (appState.editingTextElement) selected.push(appState.editingTextElement.id);
    const wanted = P.lockSet(selected, all);
    const { lock, release } = P.difference(S.mine, wanted);
    if (lock.length) {
      lock.forEach((id) => S.mine.add(id));
      ops.push({ op: "lock", ids: lock });
    }
    if (release.length) {
      release.forEach((id) => S.mine.delete(id));
      ops.push({ op: "release", ids: release });
    }
    if (ops.length) enqueue(...ops);
  }

  // The cursor, with its tool and button: the laser pointer's trail is
  // drawn for the others while its button is down.
  const sendPointer = P.throttle((x, y, tool, button) => enqueue({ op: "pointer", x, y, tool, button }), 100);
  const sendView = P.throttle(() => S.appState && enqueue({ op: "view", ...P.viewOf(S.appState) }), 250);

  // collaborators are the others as Excalidraw draws them: a cursor and
  // what they hold, in their colour.
  function collaborators() {
    const map = new Map();
    for (const p of S.participants) {
      if (p.client === S.client || p.agent) continue;
      const colour = P.colourOf(p.colour);
      const held = {};
      for (const [id, c] of S.heldBy) if (c === p.client) held[id] = true;
      const at = S.pointers.get(p.client);
      map.set(p.client, {
        id: p.client, socketId: p.client, username: p.name, color: { background: colour, stroke: colour },
        selectedElementIds: held, button: at?.button === "down" ? "down" : "up",
        ...(at ? { pointer: { x: at.x, y: at.y, tool: at.tool === "laser" ? "laser" : "pointer" } } : {}),
      });
    }
    S.api?.updateScene({ collaborators: map, captureUpdate: NEVER });
  }

  // measure has the keeper measure text an agent wrote by estimate
  // (ADR-0169), with Excalidraw's own measures.
  function measure(els) {
    if (!S.keeper || !S.editable) return;
    const fixed = [];
    for (const el of els) {
      if (el.type !== "text" || el.isDeleted || !el.customData?.[KEY]?.measure || S.heldBy.get(el.id)) continue;
      try {
        const container = el.containerId ? S.truth.get(el.containerId) : null;
        const skeleton = container
          ? { type: container.type, x: container.x, y: container.y, width: container.width, height: container.height, label: { text: el.originalText, fontSize: el.fontSize } }
          : { type: "text", x: el.x, y: el.y, text: el.originalText, fontSize: el.fontSize };
        const made = convertToExcalidrawElements([skeleton]).find((e) => e.type === "text");
        if (!made) continue;
        const { [KEY]: _, ...custom } = el.customData;
        fixed.push({
          ...copy(el), text: made.text, width: made.width, height: made.height, x: made.x, y: made.y, lineHeight: made.lineHeight,
          customData: Object.keys(custom).length ? custom : undefined, version: el.version + 1, versionNonce: Math.floor(Math.random() * 2 ** 31),
        });
      } catch {
        // The estimate stays.
      }
    }
    if (fixed.length) {
      const map = new Map(sceneOf().map((e) => [e.id, e]));
      fixed.forEach((el) => map.set(el.id, el));
      S.api.updateScene({ elements: P.byIndex([...map.values()]), captureUpdate: NEVER });
    }
  }

  // --- events from the server ---------------------------------------------
  const es = new EventSource(base + "/live");
  const on = (kind, fn) => es.addEventListener(kind, (e) => {
    try {
      fn(JSON.parse(e.data));
    } catch (err) {
      say(err.message);
    }
  });
  es.onerror = () => {
    if (es.readyState === EventSource.CLOSED) say(S.closed || t("wb.gone"));
    else say(t("wb.reconnecting"));
  };

  on("snapshot", (d) => {
    S.client = d.client;
    S.editable = !!d.editable;
    S.keeper = !!d.keeper;
    S.drawing = d.drawing;
    S.offset = (d.now || Date.now()) - Date.now();
    S.truth = new Map((d.elements || []).map((e) => [e.id, e]));
    S.heldBy = new Map((d.locks || []).map((l) => [l.id, l.client]));
    S.mine = new Set();
    S.files = new Set(Object.keys(d.files || {}));
    S.participants = d.participants || [];
    S.timer = d.timer || null;
    S.vote = d.vote || null;
    S.presentation = d.presentation || null;
    S.saved = d.saved || null;
    S.message = "";
    document.body.dataset.client = S.client;
    document.body.dataset.keeper = String(S.keeper);
    const elements = P.byIndex([...S.truth.values()]).map((e) => copy(P.shown(e, S.heldBy, S.client)));
    const files = Object.values(d.files || {});
    if (!S.api) {
      S.initial = { elements, files: Object.fromEntries(files.map((f) => [f.id, f])), appState: { viewBackgroundColor: d.background || "#ffffff" } };
    } else {
      S.api.updateScene({ elements, captureUpdate: NEVER });
      if (files.length) S.api.addFiles(files);
      redraw(P.pictures(elements));
      collaborators();
    }
    paint();
    loadComments();
    measure([...S.truth.values()]);
  });
  on("elements", (d) => apply(d.elements || []));
  on("files", (d) => {
    const files = Object.values(d.files || {});
    files.forEach((f) => S.files.add(f.id));
    if (files.length) S.api?.addFiles(files);
    redraw(files.map((f) => f.id));
  });
  on("locks", (d) => {
    const ids = [];
    for (const l of d.locks || []) {
      if (l.client) S.heldBy.set(l.id, l.client);
      else S.heldBy.delete(l.id);
      if (l.client !== S.client) S.mine.delete(l.id);
      ids.push(l.id);
    }
    relock(ids);
    collaborators();
  });
  on("presence", (d) => {
    S.participants = d.participants || [];
    const was = S.keeper;
    S.keeper = !!d.keeper;
    document.body.dataset.keeper = String(S.keeper);
    if (S.keeper && !was) measure([...S.truth.values()]);
    const present = new Set(S.participants.map((p) => p.client));
    for (const id of S.pointers.keys()) if (!present.has(id)) S.pointers.delete(id);
    const me = S.participants.find((p) => p.client === S.client);
    if (me && me.editable !== S.editable) S.editable = !!me.editable;
    collaborators();
    paint();
  });
  on("pointers", (d) => {
    for (const p of d.pointers || []) if (p.client !== S.client) S.pointers.set(p.client, p);
    collaborators();
  });
  on("timer", (d) => {
    S.timer = d.timer;
    if (d.now) S.offset = d.now - Date.now();
    paint();
  });
  on("vote", (d) => {
    S.vote = d.vote;
    if (S.pendingResult && S.vote?.closed) {
      S.pendingResult = false;
      postResult();
    }
    paint();
  });
  on("presentation", (d) => {
    S.presentation = d.presentation;
    if (!S.presentation?.active) S.following = true;
    follow();
    paint();
  });
  on("checkpoint", (d) => checkpoint(d.token));
  on("saved", (d) => {
    S.saved = d.saved;
    document.body.dataset.saved = String(d.saved?.at || "");
    paint();
  });
  on("comments", () => loadComments());
  on("closed", (d) => {
    S.closed = d.reason || t("wb.gone");
    es.close();
    S.editable = false;
    say(S.closed);
  });

  // --- checkpoints -----------------------------------------------------------

  // checkpoint draws the scene as the server held it at the event, and
  // sends the picture; the server writes its own scene into it.
  async function checkpoint(token) {
    const elements = P.byIndex([...S.truth.values()].filter((e) => !e.isDeleted));
    if (!elements.length || !S.api) return;
    try {
      let size = null;
      const blob = await exportToBlob({
        elements, files: S.api.getFiles(), mimeType: MIME_TYPES.png, exportPadding: PADDING,
        getDimensions: (width, height) => (size = { width, height, scale: 1 }),
        appState: { ...S.api.getAppState(), exportEmbedScene: false, exportBackground: true, exportWithDarkMode: false },
      });
      const regions = size ? regionsOf(elements, getCommonBounds(rootsOf(elements)), size) : [];
      const png = regions.length ? new Blob([withRegions(new Uint8Array(await blob.arrayBuffer()), regions)], { type: MIME_TYPES.png }) : blob;
      const q = new URLSearchParams({ client: S.client, token });
      const res = await fetch(`${base}/checkpoints?${q}`, {
        method: "POST", credentials: "same-origin", headers: { "Content-Type": "application/octet-stream" }, body: png,
      });
      if (!res.ok && res.status !== 409) throw new ApiError(res.status, (await res.json().catch(() => ({}))).error || res.statusText);
    } catch (err) {
      say(t("wb.saveFailed", { error: err.message }));
    }
  }

  // --- comments ----------------------------------------------------------------
  async function loadComments() {
    try {
      const res = await get(`/api/items/${encodeURIComponent(item)}/comments?limit=500`);
      S.comments = res.comments || [];
      const unknown = [...new Set(S.comments.map((c) => c.author).filter((a) => a && !S.names.has(a)))];
      if (unknown.length) {
        const n = await get("/api/users/names?" + unknown.slice(0, 100).map((id) => "id=" + encodeURIComponent(id)).join("&"));
        unknown.forEach((id) => S.names.set(id, n.names?.[id] ?? id));
      }
      paint();
    } catch {
      // Comments are shown when they can be read.
    }
  }
  async function addComment(text, element) {
    const byId = new Map([...S.truth.values()].map((e) => [e.id, e]));
    await post(`/api/items/${encodeURIComponent(item)}/comments`, {
      text, anchor: { drawing: S.drawing, element: element.id, quote: P.textOf(element, byId).slice(0, 500) },
    });
    enqueue({ op: "comments" });
    S.commentOn = null;
    await loadComments();
  }
  async function reply(text, root) {
    await post(`/api/items/${encodeURIComponent(item)}/comments`, { text, reply: root.id });
    enqueue({ op: "comments" });
    await loadComments();
  }
  async function resolve(c) {
    await put(`/api/items/${encodeURIComponent(item)}/comments/${c.id}/resolved`);
    enqueue({ op: "comments" });
    await loadComments();
  }

  // --- the workshop's tools ----------------------------------------------------

  function addNote() {
    const s = S.api.getAppState();
    const { x, y } = P.viewOf(s);
    const shift = (S.truth.size % 5) * 16;
    const [note] = convertToExcalidrawElements([{
      type: "rectangle", x: x - 100 + shift, y: y - 100 + shift, width: 200, height: 200,
      backgroundColor: P.NOTE_COLOURS[S.colour], strokeColor: "#1e1e1e", strokeWidth: 1, roughness: 0, fillStyle: "solid",
      roundness: { type: 3 }, customData: { [KEY]: { kind: "note" } },
    }]);
    S.api.updateScene({ elements: [...sceneOf(), note], appState: { selectedElementIds: { [note.id]: true } }, captureUpdate: CaptureUpdateAction.IMMEDIATELY });
    // The canvas takes the keyboard and opens the note's text, as Enter on
    // a selected shape does.
    const canvas = document.querySelector(".excalidraw");
    canvas?.focus();
    setTimeout(() => canvas?.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true })), 50);
    say(t("wb.noteHint"));
  }

  async function postResult() {
    const elements = [...S.truth.values()];
    const lines = P.ranking(S.vote?.results, elements).map((r, i) => t("wb.voteLine", { n: i + 1, text: r.text || t("wb.noText"), votes: r.votes }));
    const text = [t("wb.voteResult", { title: S.vote?.title || t("wb.voteUntitled"), voters: S.vote?.voters || 0 }), ...lines].join("\n");
    try {
      await post(`/api/items/${encodeURIComponent(item)}/comments`, { text });
      enqueue({ op: "comments" });
      await loadComments();
    } catch (err) {
      say(err.message);
    }
  }

  // follow fits the view to what the presenter shows.
  function follow() {
    const p = S.presentation;
    if (!S.api || !p?.active || p.client === S.client || !S.following) return;
    if (p.frame) {
      const f = S.truth.get(p.frame);
      if (f) S.api.scrollToContent(P.inFrame(f, [...S.truth.values()]), { fitToViewport: true, viewportZoomFactor: 0.9, animate: true });
    } else if (p.zoom) {
      const s = S.api.getAppState();
      S.api.updateScene({ appState: { ...P.scrollFor(p, s), zoom: { value: p.zoom } }, captureUpdate: NEVER });
    }
  }

  function selectedOne() {
    const ids = Object.keys(S.api?.getAppState().selectedElementIds || {});
    return ids.length === 1 ? S.truth.get(ids[0]) : null;
  }

  // The timer is drawn every half second while it runs; at its end the
  // page says so, once, with a short tone.
  setInterval(() => {
    const left = P.timeLeft(S.timer, Date.now(), S.offset);
    if (left === null) return;
    if (left === 0 && !S.timer.paused && S.rang !== S.timer.ends) {
      S.rang = S.timer.ends;
      say(t("wb.timeUp"));
      try {
        const ctx = new AudioContext();
        const osc = ctx.createOscillator();
        osc.connect(ctx.destination);
        osc.start();
        osc.stop(ctx.currentTime + 0.4);
      } catch {
        // No sound where the browser allows none.
      }
    }
    paint();
  }, 500);

  let sayTimer = null;
  function say(text) {
    S.message = text;
    clearTimeout(sayTimer);
    if (text && !S.closed) sayTimer = setTimeout(() => { S.message = ""; paint(); }, 6000);
    paint();
  }

  // --- the page --------------------------------------------------------------

  const colourNames = () => ({
    yellow: t("wb.colourYellow"), green: t("wb.colourGreen"), blue: t("wb.colourBlue"),
    pink: t("wb.colourPink"), orange: t("wb.colourOrange"), violet: t("wb.colourViolet"),
  });
  const formValue = (e, name) => new FormData(e.target).get(name);
  const submit = (fn) => (e) => {
    e.preventDefault();
    fn(e);
  };

  function people() {
    return h("div", { id: "wb-people", className: "wb-people", "aria-label": t("wb.people") },
      ...S.participants.map((p) => h("span", {
        key: p.client, className: "wb-face" + (p.agent ? " agent" : ""), title: p.agent ? t("wb.agent", { name: p.name }) : p.name,
        style: { background: P.colourOf(p.colour) },
      }, (p.name || "?").slice(0, 2).toUpperCase())));
  }

  function clockChip() {
    const left = P.timeLeft(S.timer, Date.now(), S.offset);
    if (left === null) return null;
    return h("span", { className: "wb-clock" + (left === 0 ? " up" : "") + (S.timer.paused ? " paused" : ""), role: "timer", "aria-label": t("wb.timer") },
      P.clock(left));
  }

  function savedState() {
    let text = t("wb.unsaved");
    if (S.saved && (S.saved.at - S.offset) >= S.lastChange) {
      const at = new Date(S.saved.at - S.offset).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
      text = S.saved.stale ? t("wb.savedStale", { at }) : t("wb.saved", { at, name: S.saved.by || "—" });
    } else if (!S.lastChange) {
      text = "";
    }
    return h("span", { id: "wb-state", className: "wb-state" }, text);
  }

  function bar() {
    const p = S.presentation;
    const following = p?.active && p.client !== S.client && S.following;
    const presenting = p?.active && p.client === S.client;
    const fs = P.frames([...S.truth.values()]);
    const at = p?.frame ? fs.findIndex((f) => f.id === p.frame) + 1 : 0;
    const open = (panel) => () => { S.panel = S.panel === panel ? "" : panel; paint(); };
    const threads = P.threadsByElement(S.comments, S.drawing);
    let count = 0;
    threads.forEach((list) => { count += list.length; });
    return h("div", { id: "wb-bar", className: "wb-bar" },
      S.editable && !following ? h("span", { className: "wb-group" },
        h("button", { id: "wb-note", type: "button", onClick: addNote }, t("wb.note")),
        h("select", {
          id: "wb-colour", "aria-label": t("wb.noteColour"), value: S.colour, onChange: (e) => { S.colour = e.target.value; paint(); },
        }, ...Object.keys(P.NOTE_COLOURS).map((c) => h("option", { key: c, value: c }, colourNames()[c])))) : null,
      h("button", { id: "wb-vote", type: "button", className: S.vote?.open ? "on" : "", onClick: open("vote") }, t("wb.vote")),
      h("button", { id: "wb-timer", type: "button", className: S.timer?.running ? "on" : "", onClick: open("timer") }, t("wb.timer")),
      h("button", { id: "wb-comments", type: "button", className: S.panel === "comments" ? "on" : "", onClick: open("comments") }, t("wb.comments", { n: count })),
      presenting
        ? h("span", { className: "wb-group wb-presenting" },
          fs.length ? h("button", { type: "button", "aria-label": t("wb.previous"), onClick: () => act({ op: "frame", action: "previous" }) }, "‹") : null,
          fs.length ? h("span", { id: "wb-slide" }, t("wb.slide", { n: at, of: fs.length })) : h("span", {}, t("wb.followMe")),
          fs.length ? h("button", { type: "button", "aria-label": t("wb.next"), onClick: () => act({ op: "frame", action: "next" }) }, "›") : null,
          h("button", { id: "wb-stop", type: "button", onClick: () => act({ op: "present", action: "stop" }) }, t("wb.stopPresenting")))
        : h("button", {
          id: "wb-present", type: "button", disabled: !!p?.active,
          onClick: () => {
            const sel = selectedOne();
            act({ op: "present", action: "start", target: sel?.type === "frame" ? sel.id : "", ...P.viewOf(S.api.getAppState()) });
          },
        }, t("wb.present")),
      S.editable && !following ? h("button", { id: "wb-save", type: "button", onClick: () => act({ op: "save" }) }, t("wb.save")) : null,
      savedState());
  }

  function votePanel() {
    const v = S.vote || {};
    const body = [];
    if (v.open) {
      body.push(h("p", { key: "t" }, h("strong", {}, v.title || t("wb.voteUntitled"))),
        h("p", { key: "l", id: "wb-votes-left" }, t("wb.votesLeft", { n: v.left ?? 0, of: v.per })),
        h("p", { key: "v", className: "muted" }, t("wb.voters", { n: v.voters || 0 })),
        h("p", { key: "h", className: "muted" }, t("wb.voteHint")));
      if (S.editable) {
        body.push(h("p", { key: "b" },
          h("button", { id: "wb-vote-end", type: "button", onClick: async () => { S.pendingResult = true; if (!(await act({ op: "vote", action: "end" }))) S.pendingResult = false; } }, t("wb.voteEnd")),
          " ", h("button", { type: "button", className: "link", onClick: () => act({ op: "vote", action: "cancel" }) }, t("wb.voteCancel"))));
      }
    } else if (v.closed) {
      const rows = P.ranking(v.results, [...S.truth.values()]);
      body.push(h("p", { key: "t" }, h("strong", {}, v.title || t("wb.voteUntitled"))),
        h("ol", { key: "r", id: "wb-results" }, ...rows.slice(0, 10).map((r) => h("li", { key: r.id }, `${r.text || t("wb.noText")} – ${t("wb.votesN", { n: r.votes })}`))),
        S.editable ? h("p", { key: "c" }, h("button", { type: "button", onClick: () => act({ op: "vote", action: "clear" }) }, t("wb.voteClear"))) : null);
    }
    if (!v.open && S.editable) {
      body.push(h("form", { key: "f", id: "wb-vote-form", onSubmit: submit((e) => act({ op: "vote", action: "start", title: formValue(e, "title"), votes: Number(formValue(e, "votes")) || 3 })) },
        h("label", {}, t("wb.voteTitle"), h("input", { name: "title", maxLength: 200 })),
        h("label", {}, t("wb.votesPer"), h("input", { name: "votes", type: "number", min: 1, max: 20, defaultValue: 3 })),
        h("button", { type: "submit" }, t("wb.voteStart"))));
    }
    if (!body.length) body.push(h("p", { key: "n", className: "muted" }, t("wb.noVote")));
    return panel("vote", t("wb.vote"), body);
  }

  function timerPanel() {
    const running = S.timer?.running;
    const body = [];
    if (running) {
      body.push(h("p", { key: "c", className: "wb-big" }, clockChip()));
      if (S.editable) {
        body.push(h("p", { key: "b" },
          S.timer.paused
            ? h("button", { type: "button", onClick: () => act({ op: "timer", action: "resume" }) }, t("wb.resume"))
            : h("button", { type: "button", onClick: () => act({ op: "timer", action: "pause" }) }, t("wb.pause")),
          " ", h("button", { type: "button", onClick: () => act({ op: "timer", action: "add", seconds: 60 }) }, t("wb.addMinute")),
          " ", h("button", { id: "wb-timer-stop", type: "button", onClick: () => act({ op: "timer", action: "stop" }) }, t("wb.stop"))));
      }
    } else if (S.editable) {
      body.push(h("form", { key: "f", id: "wb-timer-form", onSubmit: submit((e) => act({ op: "timer", action: "start", seconds: Math.round(Number(formValue(e, "minutes")) * 60) || 300 })) },
        h("label", {}, t("wb.minutes"), h("input", { name: "minutes", type: "number", min: 0.5, max: 120, step: 0.5, defaultValue: 5 })),
        h("button", { type: "submit" }, t("wb.start"))));
    } else {
      body.push(h("p", { key: "n", className: "muted" }, t("wb.noTimer")));
    }
    return panel("timer", t("wb.timer"), body);
  }

  function commentsPanel() {
    const byId = new Map([...S.truth.values()].map((e) => [e.id, e]));
    const threads = P.threadsByElement(S.comments, S.drawing);
    const replies = (id) => S.comments.filter((c) => c.reply === id && !c.deleted);
    const name = (id) => S.names.get(id) || id;
    const sel = selectedOne();
    const body = [];
    if (S.editable) {
      body.push(sel
        ? h("form", { key: "new", id: "wb-comment-form", onSubmit: submit(async (e) => { const text = formValue(e, "text"); if (text) { await addComment(text, sel).catch((err) => say(err.message)); e.target.reset(); } }) },
          h("blockquote", {}, P.textOf(sel, byId) || t("wb.element", { kind: sel.type })),
          h("textarea", { name: "text", rows: 3, required: true, "aria-label": t("wb.commentLabel") }),
          h("button", { type: "submit" }, t("wb.comment")))
        : h("p", { key: "hint", className: "muted" }, t("wb.commentHint")));
    }
    const list = [];
    threads.forEach((roots, element) => {
      for (const c of roots) {
        const el = byId.get(element);
        list.push(h("li", { key: c.id, className: "wb-thread" + (S.commentOn === element ? " focus" : ""), id: "wb-thread-" + c.id },
          h("button", { type: "button", className: "link", onClick: () => { if (el) S.api.scrollToContent([el], { animate: true }); } },
            c.anchor.quote || (el ? P.textOf(el, byId) : "") || t("wb.elementGone")),
          h("p", {}, h("strong", {}, name(c.author)), " ", c.text),
          ...replies(c.id).map((r) => h("p", { key: r.id, className: "wb-reply" }, h("strong", {}, name(r.author)), " ", r.text)),
          S.editable ? h("form", { onSubmit: submit(async (e) => { const text = formValue(e, "text"); if (text) { await reply(text, c).catch((err) => say(err.message)); e.target.reset(); } }) },
            h("input", { name: "text", "aria-label": t("wb.replyLabel"), placeholder: t("wb.reply") }),
            " ", h("button", { type: "button", className: "link", onClick: () => resolve(c).catch((err) => say(err.message)) }, t("wb.resolve"))) : null));
      }
    });
    body.push(list.length ? h("ul", { key: "l", id: "wb-threads", className: "wb-threads" }, ...list) : h("p", { key: "none", className: "muted" }, t("wb.noComments")));
    return panel("comments", t("wb.commentsTitle"), body);
  }

  function panel(id, title, body) {
    return h("aside", { id: "wb-panel-" + id, className: "wb-panel", "aria-label": title },
      h("header", {}, h("strong", {}, title), h("button", { type: "button", className: "link", "aria-label": t("common.close"), onClick: () => { S.panel = ""; paint(); } }, "×")),
      ...body);
  }

  // badges show votes and comments on the elements, where they are on the
  // screen.
  function badges() {
    if (!S.appState) return null;
    const v = S.vote || {};
    const threads = P.threadsByElement(S.comments, S.drawing);
    const out = [];
    for (const el of S.truth.values()) {
      if (el.isDeleted) continue;
      const pos = P.onScreen(el.x + el.width, el.y, S.appState);
      if (pos.left < 8 || pos.top < 8 || pos.left > S.appState.width - 8 || pos.top > S.appState.height - 8) continue;
      const parts = [];
      if (v.open && P.votable(el)) {
        const n = v.mine?.[el.id] || 0;
        parts.push(h("span", { key: "v", className: "wb-vote-badge" },
          n ? h("button", { type: "button", "aria-label": t("wb.retract"), onClick: () => act({ op: "vote", action: "retract", target: el.id }) }, "−") : null,
          h("b", {}, String(n)),
          h("button", { type: "button", className: "wb-cast", "aria-label": t("wb.cast"), disabled: !v.left, onClick: () => act({ op: "vote", action: "cast", target: el.id }) }, "+")));
      } else if (v.closed && v.results?.[el.id]) {
        parts.push(h("span", { key: "r", className: "wb-vote-badge result" }, h("b", {}, String(v.results[el.id]))));
      }
      const list = threads.get(el.id);
      if (list?.length) {
        parts.push(h("button", {
          key: "c", type: "button", className: "wb-comment-badge", title: t("wb.comments", { n: list.length }),
          onClick: () => { S.panel = "comments"; S.commentOn = el.id; paint(); },
        }, String(list.length)));
      }
      if (parts.length) out.push(h("div", { key: el.id, className: "wb-badge", "data-element": el.id, style: { left: pos.left + "px", top: pos.top + "px" } }, ...parts));
    }
    return h("div", { className: "wb-badges" }, ...out);
  }

  function banner() {
    const p = S.presentation;
    if (!p?.active || p.client === S.client) return null;
    return h("div", { id: "wb-banner", className: "wb-banner" }, t("wb.presenting", { name: p.name }), " ",
      h("button", { type: "button", className: "link", onClick: () => { S.following = !S.following; follow(); paint(); } },
        S.following ? t("wb.unfollow") : t("wb.follow")));
  }

  const UI = { canvasActions: { loadScene: false, saveToActiveFile: false, export: false, saveAsImage: false } };
  const topRight = () => h(React.Fragment, null, clockChip(), people(), palette ? palette("palette") : null);
  const excalidrawAPI = (a) => {
    S.api = a;
    bind?.(a);
    collaborators();
  };

  function paint() {
    if (!S.initial) return;
    const following = S.presentation?.active && S.presentation.client !== S.client && S.following;
    root.render(h(React.Fragment, null,
      h(Excalidraw, {
        excalidrawAPI, langCode: lang === "de" ? "de-DE" : "en", theme: dark ? "dark" : "light", initialData: S.initial,
        isCollaborating: true, viewModeEnabled: !S.editable || following, onChange, onLibraryChange: onLibrary,
        onPointerUpdate: ({ pointer, button }) => pointer && sendPointer(pointer.x, pointer.y, P.pointerTool(pointer.tool), button === "down" ? "down" : "up"),
        renderTopRightUI: topRight, UIOptions: UI,
      }),
      badges(), banner(),
      S.panel === "vote" ? votePanel() : null,
      S.panel === "timer" ? timerPanel() : null,
      S.panel === "comments" ? commentsPanel() : null,
      bar(),
      S.message ? h("div", { id: "wb-message", className: "wb-message", role: "status" }, S.message) : null));
    document.body.dataset.state = "open";
  }

  window.addEventListener("pagehide", () => es.close());
  return { close: () => es.close() };
}
