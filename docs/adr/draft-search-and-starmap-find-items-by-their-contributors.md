# ADR-DRAFT: Search and starmap find items by everyone who made or changed them

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-27
- **Deciders:** Project initiators (request of 2026-11-27 that the search finds items by their contributors too, settling the second half of ADR-0263's open question)
- **Open question:** whether the display name (ADR-0246) is searched too, which needs other people to see it and so a record of its own
- **Question checked:** 2026-11

## Context and problem statement

ADR-0263 lets the search, its suggestions and the starmap find an item by its creator's username.
It left open whether the people who changed an item, not only its creator, find it too.
Someone who rewrote most of a page, or who answers for a task they did not open, is not found by their name.

The query language already knows those people: `contributor` names the creator and the actor of every event that changed the item itself, as `updated` counts changes (ADR-0261).
The store keeps the events of each item in an index (`touching`), so they are read without walking the whole log.

## Decision drivers

- One notion of who made or changed an item, so the search, its suggestions, the starmap and the query language agree (ADR-0261).
- The decisions of ADR-0263 stand: derived from the log, never stored; usernames only, never mail addresses or display names; nothing names an item the caller may not read, nor lists the accounts (D10).
- The search stays a scan of the live state (ADR-0033), and its cost is measured.

## Considered options

1. **Contributors everywhere ADR-0263 names the creator**: the search matches the usernames of everyone who made or changed an item, the suggestions count and open by `contributor`, the starmap's nodes carry the others who changed an item.
2. **Contributors in the search only**, the suggestions and the starmap kept to the creator.
3. **A choice in the suggestions** between "made" and "made or changed", two entries per person.

## Decision outcome

Chosen option: **1**, because the same word should find the same items in the search box, in the suggestions and on the map, and the query language already draws the line there.

- **Who counts:** the creator and the actor of every event in `changesItem` on the item itself: its title, fields, state, text, labels, place, files and comments.
  Relations, access, holds and watches are not changes of the item, so they make nobody a contributor (ADR-0261).
  `Store.eachContributor` names each once, the creator first, then in the order of their first change.
- **Search:** `Session.Search` adds the usernames of the item's contributors that hold a term of the search as one region, scored like a property, below the title, as ADR-0263 does for the creator.
  Each user's username is looked up and compared once per search; a username that holds no term adds nothing to compare.
- **Cost:** `BenchmarkSearchByContributors` — 10,000 pages of about 1 KB, every hundredth changed by a second person whose username holds the word — takes about 26 ms per search on one core of a CI-class runner, against about 23 ms for `BenchmarkSearch`, which finds the same number of pages by a word in their text; it allocates less, about 0.3 MB against 1.2 MB.
- **People:** `GET /api/people` counts, per username, the readable live items that person made or changed (`Session.Contributors`), and names only contributors to items the caller reads.
  The suggestions say "made or changed *n* items", and choosing a person opens `contributor = "<username>" ORDER BY updated DESC`.
- **Starmap:** `StarNode.Contributors` holds the others who changed an item, in the order of their first change; the API gives them as `contributors`, by username.
  The map's search finds an item by them, and an item's panel says who made it and who changed it.

### Consequences

- **Positive:** a person finds what they worked on, not only what they opened, with the same word everywhere; the query `creator = "…"` still narrows to what someone made.
- **Negative / trade-offs accepted:** a common username matches more items: everyone who once fixed a typo is found by it; such hits rank below hits in the title.
- **Negative / trade-offs accepted:** each search walks the changes of every candidate item; at 10,000 pages that cost about 3 ms more.
- **Negative / trade-offs accepted:** an automated change (an import, a connector, an engine's synchronisation) makes its account a contributor, as `contributor` already counts it.
- **Follow-ups / risks to watch:** when the search moves to an index (ADR-0033's follow-up), the contributors' usernames belong in it, updated in `apply` with every change event.

## Pros and cons of the options

### Option 1: contributors everywhere

Strongest form: one definition, the query language's, for every place that finds by a person.

- Good: the suggestion, the search and the map agree on what a name finds.
- Bad: broader hits for a name than the creator alone.

### Option 2: contributors in the search only

Strongest form: the suggestions and the map keep the narrow, unambiguous answer "who made it", while the free search is generous.

- Bad: typing a name and choosing the suggested person would show fewer items than pressing Enter, which reads as a bug.

### Option 3: a choice in the suggestions

Strongest form: the reader decides between "made" and "made or changed" without writing a query.

- Bad: two entries per person crowd a list of at most four people; the query page already offers the narrower `creator` in one edit.

## Links

- amends [ADR-0263](0263-search-and-starmap-find-items-by-their-creator.md) (what a name finds), settling half of its open question
- uses `contributor` and `changesItem` of [ADR-0261](0261-a-query-field-for-who-made-or-changed-an-item.md)
- measured as [ADR-0033](0033-full-text-search-by-scanning-the-live-state.md) asks; respects [ADR-0246](0246-a-display-name-each-person-chooses.md)
