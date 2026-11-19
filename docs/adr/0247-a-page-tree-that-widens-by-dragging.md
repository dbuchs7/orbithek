# ADR-0247: A page tree that widens by dragging

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-19
- **Deciders:** Project initiators. A person asked for the page tree on the left to be widened with the mouse, since long page titles are cut off at its fixed width. Accepted 2026-11-19 by the requester.

## Context and problem statement

The page tree (ADR-0041) stands beside a space's views at a width the stylesheet sets: between 220 and 320 px, 16 % of the window in between.
A room whose pages carry long titles, such as a number followed by a name, shows only the first words of each; the rest is cut off with an ellipsis.
ADR-0100 lets the tree fold away for more room for the content, but offers nothing for the opposite need: more room for the tree.
It considered resizing by dragging and rejected it as more code, fiddly on touch screens, and a width that is hard to get back to.

How does the tree become wider, or narrower, when a person wants it, without losing what ADR-0100 settled?

## Decision drivers

- Long titles readable in the tree, without opening each page.
- The default unchanged for everyone who does nothing.
- A way back to the default that cannot be missed.
- Usable by mouse, touch and keyboard, and announced to screen readers.
- No new dependency, no build step (ADR-0014, ADR-0031).

## Considered options

1. **A handle at the tree's right edge that widens or narrows it by dragging and by keys; a double-click resets it; the width is kept per browser.**
2. **Fixed widths to choose from**, such as narrow, normal and wide, behind a button beside the fold button.
3. **Titles that wrap** onto several lines instead of being cut off.
4. **Keep ADR-0100 as it is:** fold the tree, or read the full title in its tooltip.

## Decision outcome

Chosen option: **1**.

- **The handle:** an 8 px strip in the gap to the right of the tree column, with the `col-resize` cursor; a 2 px line in the accent colour shows on hover, on keyboard focus and while dragging.
- **Dragging** uses pointer events with pointer capture, so a mouse, a pen and a finger work alike (`touch-action: none` on the handle only).
  While dragging, the whole page keeps the resize cursor and no text is selected.
- **Keys:** the handle is a focusable `separator` with `aria-valuenow`, `aria-valuemin` and `aria-valuemax`.
  The left and right arrows move the edge by 16 px, with Shift by 64 px; Home makes the tree as narrow as it goes, End as wide.
- **Bounds:** at least 180 px, at most half the window.
  The stylesheet clamps the width to the same bounds, so a width kept on a wide screen still fits a narrower window.
- **Back to the default:** a double-click on the handle forgets the width; the tree takes the default of ADR-0041 and ADR-0100 again.
- **Kept per browser** in `localStorage` (`ot.tree.width`), for every page, like the folded columns, and set before the first view is drawn.
- **What ADR-0100 settled stays:** a folded tree hides the handle with its content; below 800 px, where the tree stands above the content, there is no handle; reading mode hides it with the tree.
- **Code:** `assets/treewidth.js` holds the arithmetic, tested in Node (`jstest/treewidth.test.mjs`), and the handle; `withTree` in `assets/app.js` adds it to the tree column; the width is the custom property `--tree-w`.

### Consequences

- **Positive:**
  - Long titles can be read in the tree at a width the person chooses.
  - Nothing changes for those who never touch the handle.
- **Negative / trade-offs accepted:**
  - A wide tree leaves less room for the content and the side column; the bound of half the window keeps the content readable.
  - The handle is a target of 8 px; on a touch screen it is hit less easily than a button.
  - The width is per browser, not per person: another browser starts at the default.
- **Follow-ups / risks to watch:**
  - The item's side column could widen the same way if it is asked for.

## Pros and cons of the options

### Option 1: dragging the edge

Strongest form: any width, chosen while seeing the titles it shows.

- Good: the width fits the titles of the room at hand, not a guess made beforehand.
- Good: ADR-0100's objections are met: the code is one small module; pointer events cover touch; a double-click brings the default back.
- Bad: a narrow target on touch screens.

### Option 2: fixed widths

Strongest form: one click, always the same widths, easy to get back to.

- Good: a large target and no fine motor control needed.
- Bad: the right width depends on the titles of the room; three steps are either too few or a menu of their own.
- Bad: one more button beside the fold button in a strip of 32 px.

### Option 3: wrapping titles

Strongest form: every title readable at once, without any control.

- Good: nothing to learn.
- Bad: the tree grows taller and a deep tree loses its shape; a room of long titles becomes a column of paragraphs.
- Bad: it changes the tree for everyone, also for those who prefer it compact.

### Option 4: keep ADR-0100

Strongest form: no new control, no new code, and the tooltip already holds the full title.

- Good: nothing to maintain.
- Bad: reading titles one tooltip at a time does not scale to a list of similar titles that differ only at the end, the case that prompted this record.

## Links

- amends [ADR-0100](0100-a-tidy-top-bar-and-columns-that-fold-away.md) (option 3, resizing by dragging, rejected there) and [ADR-0041](0041-page-tree-and-sibling-order.md) (the page tree beside the content)
