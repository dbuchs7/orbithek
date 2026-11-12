// The plain parts of recording sound into an item's files
// (ADR-draft-recording-sound-into-an-items-files): which format to record,
// what the file is called, how long it ran. They run without a browser and
// are tested by internal/web/jstest/recorder.test.mjs; recorder.js uses
// them with the microphone.

// AUDIO_TYPES are the types of sound the server tells by a file's bytes
// and sends to be played (item.AudioTypes).
export const AUDIO_TYPES = ["audio/webm", "audio/ogg", "audio/mp4", "audio/mpeg", "audio/wave"];

// isAudio reports whether a file of an item plays as sound.
export const isAudio = (a) => AUDIO_TYPES.includes(a?.type);

// RECORDING_TYPES are the formats asked of the browser's recorder, best
// first: Opus in WebM (Chromium-based browsers, Firefox) or Ogg (Firefox),
// AAC in MP4 (Safari).
export const RECORDING_TYPES = ["audio/webm;codecs=opus", "audio/ogg;codecs=opus", "audio/webm", "audio/mp4"];

// RECORDING_BITS is the bit rate asked of the recorder: speech stays clear,
// and the longest recording stays well within the default size of a file.
export const RECORDING_BITS = 64000;

// RECORDING_SECONDS is the longest recording; it stops by itself then. At
// RECORDING_BITS that is about 14 MB.
export const RECORDING_SECONDS = 30 * 60;

// recordingType is the first of RECORDING_TYPES the browser records, ""
// when it records none of them and is left to choose.
export function recordingType(supported) {
  for (const type of RECORDING_TYPES) {
    try {
      if (supported(type)) return type;
    } catch {
      // a browser that cannot answer for a type does not record it
    }
  }
  return "";
}

// extensionOf is the file extension of what a recorder made, from the type
// it reports.
export function extensionOf(type) {
  const base = String(type || "").split(";")[0].trim().toLowerCase();
  return { "audio/webm": "webm", "video/webm": "webm", "audio/ogg": "ogg", "audio/mp4": "m4a", "video/mp4": "m4a", "audio/mpeg": "mp3" }[base] || "webm";
}

const two = (n) => String(n).padStart(2, "0");

// recordingName names a recording by when it started, in the browser's
// time: "Recording 2026-11-12 08-14.webm". label is the word for a
// recording in the UI's language; the time has no colon, which some
// systems do not allow in file names.
export function recordingName(label, started, type) {
  const d = started instanceof Date ? started : new Date(started);
  const stamp = `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}-${two(d.getMinutes())}`;
  return `${String(label || "Recording").trim()} ${stamp}.${extensionOf(type)}`;
}

// clockText writes how long a recording has run: "0:07", "12:30",
// "1:02:03".
export function clockText(ms) {
  const s = Math.max(0, Math.floor((Number(ms) || 0) / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  return h ? `${h}:${two(m)}:${two(s % 60)}` : `${m}:${two(s % 60)}`;
}

// canRecord reports whether a browser can record sound here: a secure
// context (HTTPS or localhost), the microphone's interface and a recorder.
export function canRecord(env) {
  return Boolean(env && env.isSecureContext && env.navigator?.mediaDevices?.getUserMedia && typeof env.MediaRecorder === "function");
}

// failureKey names the message for a microphone that could not be opened,
// by the error the browser gave.
export function failureKey(err) {
  switch (err?.name) {
    case "NotAllowedError":
    case "SecurityError":
      return "att.recordDenied";
    case "NotFoundError":
    case "OverconstrainedError":
      return "att.recordNoMicrophone";
    case "NotReadableError":
      return "att.recordBusy";
    default:
      return "att.recordFailed";
  }
}
