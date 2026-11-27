# ADR-0265: Search compares a letter with a diacritic as the letter without it

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-27
- **Deciders:** Project initiators. Request of 2026-11-27, from a review of release v0.21.6: the search misses words written with umlauts and accents, which ADR-0033 lists as not built.
- **Open question:** whether the German way of writing an umlaut out (`ae` for `ä`, `ue` for `ü`) should find it too, so that `Zuerich` finds `Zürich`; and whether letters of other scripts with marks of their own (Greek, Cyrillic) should be compared without them.
- **Question checked:** 2026-11

## Context and problem statement

ADR-0033 decides that a search folds its terms and the text it scans: lower case, and `ß` as `ss`.
It names what is missing: "no diacritic folding beyond `ß` (`ä` does not match `a`)", and expects that folding to need Unicode normalisation and so a new dependency.

In German, French and Italian text this is felt daily.
`zurich` does not find *Zürich*, `cafe` not *Café*, and a name is found only when it is typed with the accents its page has.
The same comparison serves the filter of every paged list (ADR-0098) and the names and texts of the query language (ADR-0136).

The browser already compares otherwise: the filter that narrows a list shown on a page takes the accents off (`assets/filter.js`), so the same word is found in a short list and missed in a long one that the server filters.

## Decision drivers

- A word is found however its accents are typed, in both directions.
- The server and the browser compare alike.
- One binary, few dependencies (ADR-0009, ADR-0024).
- The scan's cost stays as ADR-0033 measured it: one pass, one reused buffer.

## Considered options

1. Leave the comparison as it is
2. Compare a letter with a diacritic as its base letter, by a table of the Latin letters
3. Unicode normalisation (NFD) and dropping every combining mark, with `golang.org/x/text`
4. Write the German umlauts out (`ä` as `ae`)

## Decision outcome

Chosen option: **2, a table of the Latin letters**, because it finds what people type in the languages the product is used in, costs no dependency and no second pass, and makes the server compare as the browser does.

- **What is folded.** After lowering, the letters from `à` (U+00E0) to `ž` (U+017E) are compared as their base letters: `ä` as `a`, `é` as `e`, `ç` as `c`, `ñ` as `n`, `ł` as `l`, `š` as `s`. The ligatures `æ` and `œ` are compared as `ae` and `oe`. `ß` stays `ss`. A combining mark (U+0300 to U+036F) is dropped, so a letter typed with a separate accent is the same letter.
- **Both ways.** The query and the text are folded alike: `zurich` finds *Zürich*, and `zürich` finds *Zurich*.
- **Where.** `appendFold` in `internal/item/search.go` is the one comparison: the search, the filter of paged lists (`HasWords`) and the comparisons of the query language that use it follow together.
- **Other scripts** are lowered and otherwise compared as they are.
- **Snippets.** A folded letter is still one letter, so the place of a hit in the text is found as before; only the ligatures and `ß` shift it by a letter, which the snippet already allows for.

### Consequences

- **Positive:** names and words are found however their accents are typed; the filter of a list finds the same on the server as in the browser; no dependency; the benchmark of ADR-0033 is unchanged within its noise (about 22 ms for 10,000 items).
- **Negative / trade-offs accepted:** words that differ only by a diacritic are no longer told apart: `Bar` finds *Bär*, `schon` finds *schön*. A search prefers finding too much to missing a page. An equality of the query language on a text (`title = "Bär"`) matches *Bar* too.
- **Negative / trade-offs accepted:** the table covers the Latin letters up to U+017E; letters beyond it (Vietnamese, for one) are compared with their marks.
- **Follow-ups / risks to watch:** the open question on umlauts written out; an index, when ADR-0033's follow-up comes, must fold in the same way.

## Pros and cons of the options

### 1. Leave the comparison as it is

Strongest form: the comparison is exact and predictable; `Bar` and `Bär` are different words, and nobody is surprised by a hit they did not type.

- Good: no change; precise.
- Bad: the daily misses stay, and the server keeps comparing differently from the browser.

### 2. A table of the Latin letters

Strongest form: a fixed, visible table of some hundred letters does in one pass what people expect, with nothing to add to the binary.

- Good: no dependency; one pass; the cost unchanged; matches the browser's filter for these letters.
- Bad: not all of Unicode; the table is the project's to keep.

### 3. Unicode normalisation with `golang.org/x/text`

Strongest form: it is the standard's answer and covers every script and every composed letter, with nothing to maintain; ADR-0033 expected it.

- Good: complete and standard.
- Bad: a new direct dependency for a comparison (its tables add to the binary); normalising allocates or needs a transformer per scan, against the one reused buffer ADR-0033 measured; letters such as `ø`, `ł` and `đ` have no decomposition and would need a table all the same.

### 4. Write the German umlauts out

Strongest form: it is how German writes an umlaut without the key for it, and keeps `Bar` and `Bär` apart.

- Good: precise for German.
- Bad: `zurich` still misses *Zürich*, the commonest case; it is wrong for other languages (`naïve`, `Citroën`); it differs from the browser's filter. It may come as an addition, which the open question leaves.

## Links

- extends [ADR-0033](0033-full-text-search-by-scanning-the-live-state.md) (the folding it left open); relates to ADR-0098 (filters of lists), ADR-0136 (query language), ADR-0009 and ADR-0024 (one binary, few dependencies)
