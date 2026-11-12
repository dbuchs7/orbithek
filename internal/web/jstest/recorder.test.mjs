// Unit tests of the plain parts of recording sound into an item's files
// (recorderparts.js, ADR-draft-recording-sound-into-an-items-files). They
// run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import {
  AUDIO_TYPES, RECORDING_BITS, RECORDING_SECONDS, canRecord, clockText, extensionOf, failureKey, isAudio, recordingName, recordingType,
} from "../static/assets/recorderparts.js";

test("the format asked of the recorder is the first it records", () => {
  assert.equal(recordingType((t) => t.startsWith("audio/webm")), "audio/webm;codecs=opus", "Chromium-based browsers");
  assert.equal(recordingType((t) => t.startsWith("audio/ogg")), "audio/ogg;codecs=opus", "Firefox where it prefers Ogg");
  assert.equal(recordingType((t) => t === "audio/mp4"), "audio/mp4", "Safari");
  assert.equal(recordingType(() => false), "", "none of them: the browser chooses");
  assert.equal(recordingType(() => { throw new Error("no"); }), "", "a browser that cannot answer");
});

test("a recording's extension follows the type the recorder reports", () => {
  assert.equal(extensionOf("audio/webm;codecs=opus"), "webm");
  assert.equal(extensionOf("audio/ogg; codecs=opus"), "ogg");
  assert.equal(extensionOf("audio/mp4"), "m4a");
  assert.equal(extensionOf("video/mp4"), "m4a", "Safari may report its sound as video/mp4");
  assert.equal(extensionOf(""), "webm");
});

test("a recording is named by when it started, without a colon", () => {
  const started = new Date(2026, 10, 12, 8, 4, 59);
  assert.equal(recordingName("Recording", started, "audio/webm;codecs=opus"), "Recording 2026-11-12 08-04.webm");
  assert.equal(recordingName("Aufnahme", started, "audio/mp4"), "Aufnahme 2026-11-12 08-04.m4a");
  assert.equal(recordingName("", started, "audio/ogg"), "Recording 2026-11-12 08-04.ogg");
  assert.ok(!recordingName("Recording", started, "").includes(":"));
});

test("how long a recording ran", () => {
  assert.equal(clockText(0), "0:00");
  assert.equal(clockText(7999), "0:07");
  assert.equal(clockText(750000), "12:30");
  assert.equal(clockText(3723000), "1:02:03");
  assert.equal(clockText(-5), "0:00");
  assert.equal(clockText(RECORDING_SECONDS * 1000), "30:00");
});

test("the longest recording stays within the default size of a file", () => {
  assert.ok(RECORDING_BITS / 8 * RECORDING_SECONDS < 25 * 1024 * 1024);
});

test("which files play as sound", () => {
  for (const type of AUDIO_TYPES) assert.ok(isAudio({ type }), type);
  assert.ok(!isAudio({ type: "video/webm" }), "a video plays as a video");
  assert.ok(!isAudio({ type: "application/octet-stream" }));
  assert.ok(!isAudio(null));
});

test("recording needs a secure context, the microphone and a recorder", () => {
  const getUserMedia = () => {};
  const ok = { isSecureContext: true, navigator: { mediaDevices: { getUserMedia } }, MediaRecorder: function MediaRecorder() {} };
  assert.ok(canRecord(ok));
  assert.ok(!canRecord({ ...ok, isSecureContext: false }), "plain HTTP");
  assert.ok(!canRecord({ ...ok, navigator: {} }), "no microphone interface");
  assert.ok(!canRecord({ ...ok, MediaRecorder: undefined }), "no recorder");
  assert.ok(!canRecord(undefined));
});

test("a microphone that could not be opened says why", () => {
  assert.equal(failureKey({ name: "NotAllowedError" }), "att.recordDenied");
  assert.equal(failureKey({ name: "NotFoundError" }), "att.recordNoMicrophone");
  assert.equal(failureKey({ name: "NotReadableError" }), "att.recordBusy");
  assert.equal(failureKey(new Error("x")), "att.recordFailed");
});
