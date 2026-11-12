// Unit tests of the import of notes's form (noteimport.js, ADR-0209): the
// name an import takes from its archive and the query of its request.
// They run in Node.js without a browser and without dependencies:
//
//   node --test "internal/web/jstest/*.test.mjs"
import { test } from "node:test";
import assert from "node:assert/strict";
import { importName, importQuery } from "../static/assets/noteimport.js";

test("an import is named after its archive, without the export's own ID", () => {
  assert.equal(importName("My Vault.zip"), "My Vault");
  assert.equal(importName("Export-6f1c2a4e-3b7d-4e0a-9c1b-2d3e4f5a6b7c.zip"), "Export", "the next export updates the same import");
  assert.equal(importName("Wiki 0123456789abcdef0123456789abcdef.ZIP"), "Wiki");
  assert.equal(importName(".zip"), "notes");
  assert.equal(importName(undefined), "notes");
  assert.equal(importName("x".repeat(100) + ".zip").length, 80);
});

test("the query names the import, its types and a dry run", () => {
  assert.equal(importQuery({ name: " Vault ", type: "page", rowType: "page", dryRun: true, lang: "de" }).toString(), "name=Vault&type=page&dryRun=true&lang=de");
  assert.equal(importQuery({ name: "V", type: "page", rowType: "task" }).toString(), "name=V&type=page&rowType=task");
  assert.equal(importQuery({}).toString(), "name=");
});
