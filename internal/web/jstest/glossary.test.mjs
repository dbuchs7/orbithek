// One action has one name wherever it appears (AGENTS.md): the catalogues
// are held to the few names that had drifted apart.
import { test } from "node:test";
import assert from "node:assert/strict";

import en from "../static/assets/locales/en.js";
import de from "../static/assets/locales/de.js";

// Every string of a catalogue, the plural forms of a key each on their own.
function strings(catalogue) {
  const out = [];
  for (const [key, value] of Object.entries(catalogue)) {
    if (typeof value === "string") out.push([key, value]);
    else for (const [form, text] of Object.entries(value)) out.push([`${key}.${form}`, text]);
  }
  return out;
}

test("signing in is never called logging in", () => {
  const found = strings(en).filter(([, text]) => /\blog(ged|ging)? (in|out)\b/i.test(text)).map(([key]) => key);
  assert.deepEqual(found, []);
  const german = strings(de).filter(([, text]) => /\b(ein|aus)(ge)?logg|\bLogin\b/i.test(text)).map(([key]) => key);
  assert.deepEqual(german, []);
});

test("there is one Cancel", () => {
  for (const [name, catalogue] of [["en", en], ["de", de]]) {
    const word = catalogue["common.cancel"];
    const twins = strings(catalogue).filter(([key, text]) => text === word && key !== "common.cancel").map(([key]) => key);
    assert.deepEqual(twins, [], `${name}: use common.cancel`);
  }
});

test("what can be restored is moved to the trash, not deleted", () => {
  for (const key of ["item.trash", "admin.trashSpace", "album.folderDelete", "album.collectionDelete"]) {
    assert.match(en[key], /trash/i, key);
    assert.doesNotMatch(en[key], /delete/i, key);
    assert.match(de[key], /Papierkorb/, key);
    assert.doesNotMatch(de[key], /löschen/i, key);
  }
});

test("taking an item on is Assign to me", () => {
  assert.equal(en["tasks.assignMe"], "Assign to me");
  assert.equal(en["tasks.claim"], undefined);
});
