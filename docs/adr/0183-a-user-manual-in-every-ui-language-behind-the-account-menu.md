# ADR-0183: A user manual in every UI language, behind the account menu

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10 for a user manual in English and German, linked as "Manual" under the profile picture in the header)
- **Open question:** whether the manual should be written as items of a room of its own, so administrators can adapt it to their installation; whether chapters should open at the place of the UI they describe (a help link per view)
- **Question checked:** 2026-11

## Context and problem statement

Orbithek has grown to many modules — rooms and pages, tasks, kanbans, calendar, album, whiteboards, starmaps, governance, process engines — and the UI explains each only where it is used. People new to it have no single place that tells them what the product does and where to find it. The request is a user manual in English and German, reachable from the account menu in the header.

Where does the manual live, how does it reach the reader, and how is it kept in step across languages?

## Decision drivers

- One binary (ADR-0009, ADR-0024): the manual must ship with the server and work offline, with nothing fetched from elsewhere.
- No markup from strings (ADR-0014, ADR-0031): the manual must not be shown by turning text into HTML.
- English is the source; German follows in Swiss spelling (ADR-0017).
- The manual is long prose with headings and tables, unlike the short strings of the message catalogue.
- Translations drift unless a test notices.

## Considered options

1. **One Markdown file per UI language in `assets/manual`, embedded with the UI, shown read-only at `#/manual` as blocks of the item model through the Markdown conversion of ADR-0133, led by a table of contents; linked as *Manual* from the account menu.**
2. Static HTML pages next to `draw.html`, served by a route of their own.
3. The manual as keys of the message catalogue.
4. The manual as items of a room that every installation is seeded with.

## Decision outcome

Chosen option: **1**.

- **Source:** `internal/web/static/assets/manual/en.md` and `de.md`, plain Markdown readable on GitHub and in any editor. The first line is the title.
- **Delivery:** the files are embedded with the UI and served under `/assets/`, which is public like every other asset; the manual describes the product and holds nothing of an installation.
- **Reading view:** `#/manual` fetches the file of the UI's language, or of the one `?lang=` names, converts it with `markdownToBlocks` (ADR-0133), puts a table of contents after the title and shows it with the read-only editor every body uses (`assets/manual.js`, `manualView` in `assets/app.js`). Links to other sites keep their targets only as in a body (`isSafeLink`). Links above the text switch the language.
- **Entry point:** the account menu lists *Manual* (*Handbuch*) after *AI access*.
- **Keeping it in step:** `internal/web/jstest/manual.test.mjs` requires every manual to have the English one's outline — the same headings at the same levels — to convert without losing links or pictures, and the German one to write ss, never ß; `TestManualPerLanguage` requires a manual for every UI language. A change that alters what people see updates both manuals.

### Consequences

- **Positive:** the manual ships, versioned, with the binary it describes; it reads like any page, in light and dark looks and in the corporate design; a translation that drops or adds a chapter fails the tests.
- **Negative / trade-offs accepted:** the manual is the product's, not the installation's — an operator cannot add local rules to it; the outline test cannot see whether a paragraph says the same in both languages; the Markdown conversion's limits (headings to level 3, plain-text table cells) bound the manual's layout.
- **Follow-ups / risks to watch:** help links from single views to their chapter; screenshots, which would need pictures served with the UI and a way to keep them current.

## Pros and cons of the options

### Option 1 — Markdown per language, shown as blocks (chosen)

Strongest form: one plain source per language, the reading view and conversion the product already has, and a test that holds the languages together.

- Good: no new rendering path; no new route; offline; reviewable as text.
- Bad: bounded by what the block model shows; the manual cannot be edited in the running product.

### Option 2 — static HTML pages

Strongest form: full control of layout, printable as is, and readable without signing in or running the UI's script.

- Good: richer layout; works without JavaScript.
- Bad: a second look to keep in step with the UI's themes and corporate design; hand-written HTML in two languages drifts; a route of its own for little gain.

### Option 3 — the message catalogue

Strongest form: the catalogue is already where every language of the UI lives, and its tests already compare languages.

- Good: one mechanism for every text.
- Bad: chapters of prose split into hundreds of keys are hard to write and review; the catalogue would carry the manual into every page load.

### Option 4 — items of a seeded room

Strongest form: the manual would be content like any other — searchable, commentable, adaptable per installation.

- Good: operators adapt it; it is found by the search.
- Bad: seeding content into every data directory touches the event log (ADR-0025) and leaves it out of date with every release; a seeded room needs roles, and an edited copy no longer follows the product.

## Links

- relates to ADR-0009 and ADR-0024 (one binary), ADR-0014 and ADR-0031 (no markup from strings), ADR-0017 (languages), ADR-0133 (Markdown into blocks)
