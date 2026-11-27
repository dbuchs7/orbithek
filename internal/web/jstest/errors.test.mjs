// How a failed request is worded for the reader (ADR-0017): the kind of
// failure in their language, by its status, then the server's detail.
// Run with: node --test internal/web/jstest/
import { test } from "node:test";
import assert from "node:assert/strict";

import { ApiError, errorDetail, setNamer, wordError } from "../static/assets/api.js";
import { setLanguage } from "../static/assets/i18n.js";

// setLanguage also tells the page its language; there is no page here.
globalThis.document ??= { documentElement: {} };

const id = "1eb98b0a772efa26bfc6549221fba5c7";

test("a failure is worded by its status, with the server's detail after it", () => {
  setLanguage("en");
  assert.equal(wordError(403, "forbidden: " + id + " may not see the roles of PLAT"), "You may not do that: " + id + " may not see the roles of PLAT");
  assert.equal(wordError(404, 'not found: key "NOPE-99"'), 'Not found: key "NOPE-99"');
  assert.equal(wordError(400, 'validation failed: space "PLAT" exists'), 'That was not accepted: space "PLAT" exists');
  assert.equal(wordError(409, "PLAT-1 is referenced by PLAT-2"), "That does not fit what is there: PLAT-1 is referenced by PLAT-2");
  assert.equal(wordError(500, "the change was not made"), "The server could not do that: the change was not made");
  assert.equal(wordError(404, ""), "Not found.");
  assert.equal(wordError(418, "a teapot"), "a teapot"); // a status without a wording keeps the server's
});

test("a refused sign-in is not an ended session", () => {
  setLanguage("en");
  assert.equal(wordError(401, "wrong username or password", "/api/login"), "Wrong username or password.");
  assert.equal(wordError(401, "authentication required", "/api/items"), "You are not signed in, or your session has ended.");
  assert.equal(wordError(403, "this account is suspended; ask an administrator", "/api/login"), "You may not do that: this account is suspended; ask an administrator");
});

test("the lead is in the reader's language", () => {
  setLanguage("de");
  assert.equal(wordError(401, "wrong username or password", "/api/login"), "Benutzername oder Passwort stimmen nicht.");
  assert.equal(wordError(403, "forbidden: may not edit PLAT-1"), "Das ist Ihnen nicht erlaubt: may not edit PLAT-1");
  setLanguage("en");
});

test("people are named, not their IDs", () => {
  setNamer((who) => (who === id ? "vera" : null));
  assert.equal(errorDetail("forbidden: " + id + " may not create in"), "vera may not create in");
  assert.equal(errorDetail("ffffffffffffffffffffffffffffffff is unknown"), "ffffffffffffffffffffffffffffffff is unknown");
  setNamer(() => null);
});

test("an error carries the worded message and the server's own words", () => {
  setLanguage("en");
  const err = new ApiError(404, 'not found: key "NOPE-99"', undefined, undefined, null, "/api/items/NOPE-99");
  assert.equal(err.message, 'Not found: key "NOPE-99"');
  assert.equal(err.detail, 'not found: key "NOPE-99"');
  // A failure the UI words itself keeps the server's words as its message.
  assert.equal(new ApiError(400, "at character 8: expected a value after =", 8).message, "at character 8: expected a value after =");
  assert.equal(new ApiError(400, "bad credentials", undefined, "credentials").message, "bad credentials");
});
