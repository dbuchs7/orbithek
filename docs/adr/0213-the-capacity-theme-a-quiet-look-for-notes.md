# ADR-0213: The Capacity theme, a quiet look for notes in light and dark

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: a ready-made theme, light and dark, for people who keep their notes in the wiki and are used to a quiet, paper-like look; the requester named it *Capacity*. Colours, sizes and shapes are defaults proposed with the implementation.
- **Open question:** whether the theme should also be offered as a personal look on the account page, as the cockpit is (ADR-0156), rather than only for the whole installation.
- **Question checked:** 2026-11

## Context and problem statement

The ready-made themes (ADR-0085) restyle the product in one step: colours and shapes, light and dark where they define a dark scheme. They lean towards strong colours (Ocean, Sunset, Berry), an authority's look (Swiss, ADR-0120) or play (Comic, Sketch). People who use the wiki mainly for their own notes, daily notes and reading ask for a calmer look: a white page, little chrome, large titles and an airy text.

Which look does the product offer them, and how does it ship?

## Decision drivers

- One click under the corporate design, as every theme (ADR-0085); logo, colours and font set by administrators still apply over it.
- Light and dark, following the system or the bulb in the header (ADR-0070, ADR-0095).
- Nothing fetched from elsewhere and no font files added: the product's own typeface stack is used (ADR-0052).
- Contrast of text and links at least that of the product's own look.

## Considered options

1. **A ninth theme stylesheet in `internal/site/themes`, with a light and a dark scheme.**
2. **A personal look on the account page, beside the cockpit (ADR-0156).**
3. **A change of the product's own look.**

## Decision outcome

Chosen option: **1**, because it follows the mechanism every other theme uses and reaches every person of an installation at once.

- **Light:** white page and surfaces, a soft grey (`#f7f7f8`) for the side panel and the page tree, hairline borders (`#ececef`), near-black text (`#1d1d20`), grey secondary text (`#74747c`) and one blue-violet accent (`#4f5fe6`).
- **Dark:** a near-black page (`#18191c`), surfaces a step lighter, light grey text (`#e8e8ec`) and a lighter accent (`#8d9bff`) with dark ink on it.
- **Shapes:** corners of 10 and 7 pixels; the search field, the *New* button, labels, chips and badges as pills; secondary buttons a quiet grey fill, the main one in the accent; flat bars with a hairline instead of a shadow; cards without hard frames that lift on hover.
- **Type:** page titles at 28 pixels, bold and close-set; the text of a page at 15 pixels with a line height of 1.7; quotes set off by a grey rule.
- **Name and choice:** the theme's ID is `capacity`, offered last in the design card with a preview of white, the soft grey and the accent.

### Consequences

- **Positive:** a calm look is one click away, in light and dark, without a font or an asset added.
- **Negative / trade-offs accepted:**
  - As every theme, it restyles components by their class names; a change of the markup can leave it partly applied until it is adjusted (ADR-0085).
  - The contrast was checked by eye against the product's look, not measured by the check of ADR-0052, which knows only the design's own colours.
  - A theme changes colours and shapes, not the layout: the page tree and the side panel stay where the product puts them.
- **Follow-ups / risks to watch:** the theme is checked by screenshots and by the smoke test's step that applies it, not by a visual test.

## Pros and cons of the options

### Option 2: a personal look

Strongest form: each person chooses the look they work best with, as with the cockpit, without an administrator.

- Good: no installation-wide change for a preference of some.
- Bad: a personal look is a second mechanism with its own stylesheet and switch; the cockpit is dark only and stands apart for a reason (ADR-0156). Offering every theme personally is a larger decision, left as the open question.

### Option 3: change the product's own look

Strongest form: if the quiet look serves more people, the product should simply look like this.

- Bad: the product's look is shared with the atlas console on purpose (ADR-0031, ADR-0065); changing it for one group changes it for everyone.

## Links

- extends ADR-0085 (ready-made themes)
- relates to ADR-0052 (corporate design), ADR-0070 (dark scheme), ADR-0095 (light and dark in the header), ADR-0156 (cockpit look)
