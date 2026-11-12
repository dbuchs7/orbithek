// Recording sound into an item's files
// (ADR-draft-recording-sound-into-an-items-files): the browser records from
// the microphone and the recording is uploaded as a file of the item, which
// its file list then plays. Nothing is transcribed, and nothing is kept in
// the browser: a recording not attached is discarded.
import { h } from "./dom.js";
import { t } from "./i18n.js";
import { RECORDING_BITS, RECORDING_SECONDS, canRecord, clockText, failureKey, recordingName, recordingType } from "./recorderparts.js";

// recorderControl is the button that records sound, or null where this
// browser cannot record. attach(file) uploads the recording; it is called
// once, after the person stops and attaches it. status shows what failed.
export function recorderControl(attach, status) {
  if (!canRecord(globalThis)) return null;
  const box = h("div", { class: "recorder", id: "recorder" });
  const start = h("button", { type: "button", class: "secondary", id: "record-sound", text: t("att.record") });
  box.append(start);
  start.addEventListener("click", () => record(box, start, attach, status));
  return box;
}

async function record(box, start, attach, status) {
  status.replaceChildren();
  start.disabled = true;
  let stream;
  try {
    stream = await navigator.mediaDevices.getUserMedia({ audio: true });
  } catch (err) {
    start.disabled = false;
    status.replaceChildren(h("p", { class: "error", role: "alert", text: t(failureKey(err)) }));
    return;
  }
  const type = recordingType((x) => MediaRecorder.isTypeSupported(x));
  let recorder;
  try {
    recorder = new MediaRecorder(stream, { ...(type ? { mimeType: type } : {}), audioBitsPerSecond: RECORDING_BITS });
  } catch (err) {
    stream.getTracks().forEach((tr) => tr.stop());
    start.disabled = false;
    status.replaceChildren(h("p", { class: "error", role: "alert", text: t("att.recordFailed") }));
    return;
  }
  const started = new Date();
  const chunks = [];
  let keep = false;
  let timer = 0;
  const clock = h("span", { class: "recorder-clock", role: "timer", "aria-live": "off", text: clockText(0) });
  const stop = h("button", { type: "button", class: "primary", id: "record-stop", text: t("att.recordStop") });
  const discard = h("button", { type: "button", class: "link danger", id: "record-discard", text: t("att.recordDiscard") });
  const panel = h("div", { class: "recorder-live", role: "group", "aria-label": t("att.recording") },
    h("span", { class: "recorder-dot", "aria-hidden": "true" }), " ", h("span", { text: t("att.recording") }), " ", clock, " ", stop, " ", discard);
  start.hidden = true;
  box.append(panel);

  const end = (attachIt) => {
    keep = attachIt;
    clearInterval(timer);
    stop.disabled = discard.disabled = true;
    if (recorder.state !== "inactive") recorder.stop();
    else finish();
  };
  const finish = async () => {
    stream.getTracks().forEach((tr) => tr.stop());
    panel.remove();
    start.hidden = false;
    start.disabled = false;
    if (!keep || !chunks.length) return;
    const made = recorder.mimeType || type || "audio/webm";
    const file = new File(chunks, recordingName(t("att.recordingName"), started, made), { type: made.split(";")[0] });
    status.replaceChildren(h("p", { class: "muted", role: "status", text: t("att.recordSaving") }));
    try {
      await attach(file);
    } catch (err) {
      status.replaceChildren(h("p", { class: "error", role: "alert", text: err.message }));
    }
  };
  recorder.addEventListener("dataavailable", (e) => { if (e.data && e.data.size) chunks.push(e.data); });
  recorder.addEventListener("stop", finish);
  recorder.addEventListener("error", () => {
    status.replaceChildren(h("p", { class: "error", role: "alert", text: t("att.recordFailed") }));
    end(false);
  });
  stop.addEventListener("click", () => end(true));
  discard.addEventListener("click", () => end(false));
  // The recording stops by itself at its longest and is attached; leaving
  // the view, or the view being drawn again, discards it and frees the
  // microphone.
  timer = setInterval(() => {
    if (!box.isConnected) return end(false);
    const ms = Date.now() - started.getTime();
    clock.textContent = clockText(ms);
    if (ms >= RECORDING_SECONDS * 1000) end(true);
  }, 250);
  recorder.start(1000);
}
