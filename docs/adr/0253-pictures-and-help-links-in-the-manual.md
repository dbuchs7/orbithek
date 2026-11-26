# ADR-0253: Pictures of the UI in the manual, and help links that open its chapters

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-26
- **Deciders:** the maintainer asked for the manual to show current screenshots and icons, so that readers recognise what they read about, and for the UI to jump to a topic of the manual where needed (2026-11-26)
- **Open question:** which further chapters should show a picture; how a stale picture gets noticed is answered by ADR-0254
- **Question checked:** 2026-11

## Context and problem statement

The manual (ADR-0183) is text only: a picture written in its Markdown becomes a link, as pictures from elsewhere do in a body (ADR-0133).
Readers who are not used to the product meet names of buttons and cards they cannot place, and they reach the manual only through the account menu, never from the place they need help with.
ADR-0183 left both open as follow-ups: screenshots, "which would need pictures served with the UI and a way to keep them current", and help links from single views to their chapter.

How does the manual show pictures of the UI without a second rendering path, how are they made and kept, and how does a view name the chapter about it in every language?

## Decision drivers

- One binary and offline (ADR-0009, ADR-0024): pictures ship with the UI.
- No markup from strings (ADR-0014): the manual stays blocks shown by the read-only editor.
- The manual stays plain Markdown, readable on GitHub and in any editor.
- A chapter is named the same way in every language, though its heading is not.
- Pictures are many bytes in a binary; they must earn their place and not linger unused.
- Making a picture again must be one command, not a craft.

## Considered options

1. **Pictures and anchors as the manual's own syntax, converted by `assets/manual.js`; pictures taken by a script against a running server and committed; help links naming anchors.**
2. Pictures as data URLs inside the Markdown.
3. Pictures taken in CI on every build, not committed.
4. Help links naming the heading's text.

## Decision outcome

Chosen option: **1**.

### Pictures

- A picture is a line of its own: `![caption](img/<lang>/<name>.png "60 left")`.
  The path is relative to the manual's folder, so GitHub shows it beside the Markdown file; the title gives the width in percent of the column and optionally `left` or `right`, as a picture block of a body has (ADR-0056).
- `manualParts` takes these lines out before the Markdown conversion and `manualBlocks` puts picture blocks in their place, with the path as their `ref`; the reading view gives the editor a source of files that serves `ref` from `/assets/manual/`.
  Bodies are unchanged: there a picture stays a link.
- Screenshots are per language (`img/en/`, `img/de/`), since they show the UI's words; the symbols of the start page's modules are language-neutral (`img/icons/`).
- `e2e/manual-shots.cjs` seeds a fresh server with example content (a room with pages and tasks, a process engine, the shop's types and a catalogue, a second person) and takes every picture in every language; `e2e/shrink-pictures.py` reduces them to a palette of 256 colours, about a third of their size.
  Both are run by hand when a view the manual shows has changed; the pictures are committed.
- The manual's tests require every manual to show the English one's pictures (with its own language in the path) and every picture shown to exist (`internal/web/jstest/manual.test.mjs`); `TestManualPicturesAreShown` requires every embedded picture to be shown, so none ships unused.

### Help links

- A heading may end with an anchor, `### The shop of an engine {#shop}`; it is not shown.
  Every manual carries the English one's anchors in the same order (tested), so an anchor names the same chapter in every language.
- `#/manual?section=<anchor>` opens the manual in the UI's language at that chapter and marks it; switching the language keeps the chapter.
- `assets/help.js` draws a help link: a small «?» beside a heading, titled "Open the manual at this topic".
  The views and cards that have a chapter carry one: the shop and its cards, the process engines in the administration and among one's connected engines, the task list, kanbans, roadmap, calendar, album, whiteboards, starmap, comments, notes, search, queries, AI access, the account page and the administration.

### Consequences

- **Positive:** readers see the view a chapter speaks of, and get from a view to its chapter in one click.
- **Positive:** the manual stays one Markdown file per language, still readable on GitHub with its pictures.
- **Negative / trade-offs accepted:** the pictures add about 650 KB to the binary.
- **Negative / trade-offs accepted:** a picture shows the UI of the day it was taken; ADR-0254 makes the manual check warn when a change touches what a picture shows.
- **Follow-ups / risks to watch:** pictures for further chapters.

## Pros and cons of the options

### Option 2: data URLs in the Markdown

Strongest form: the manual and its pictures are one file per language; nothing can go missing.

- Good: no folder of pictures to keep in step.
- Bad: the Markdown becomes unreadable and unreviewable; a changed picture is a changed line of thousands of characters.

### Option 3: pictures taken in CI

Strongest form: the pictures are always of the code being built, so they never go stale.

- Good: no stale picture.
- Bad: a release depends on a browser run to have its manual; a picture changes with every font or timing difference of the runner, so no one reviews what readers will see.

### Option 4: help links by heading text

Strongest form: no new syntax; the heading is the anchor.

- Good: nothing to write but the heading.
- Bad: headings differ by language and change with rewording, so a link breaks on the next edit of a title, silently.

## Links

- amended by ADR-0254 (pictures kept current by a check)
- relates to ADR-0183 (the user manual), ADR-0185 (the manual kept current by checks), ADR-0133 (Markdown into blocks), ADR-0056 (picture widths), ADR-0014 (no markup from strings)
