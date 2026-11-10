# ADR-0185: The user manual is kept current by checks, not by memory alone

- **Status:** Proposed
- **Implementation:** Not started
- **Date:** 2026-11-10
- **Deciders:** Project initiators (request of 2026-11-10: a decision so that the user manual always stays current and is kept up to date; the means below are proposed and await the requester's confirmation)
- **Open question:** whether the change check should block merging from the start or only warn first, as the DCO check did (ADR-0019); whether the manual should name the release it describes; whether screenshots belong in the manual once it is kept current
- **Question checked:** 2026-11

## Context and problem statement

ADR-0183 put a user manual in every UI language into the binary and keeps its languages in step: every manual must have the English one's outline of headings, and every UI language must have a manual. What those checks cannot see is whether the manual still describes the product. `AGENTS.md` asks every change to what people see or can do to update both manuals in the same change, but nothing holds a change to that rule.

The day the manual was merged showed what happens without a check. Three changes landed within hours: renaming pictures (ADR-0180), renaming files (ADR-0182) and presenting pictures (ADR-0176). The manual covered them in one sentence each until a separate change expanded them, and the three PRs passed CI with or without the manual. The manual goes stale in two ways:

- **Missing:** a new feature, view or control is not described at all.
- **Wrong:** a described control was renamed, moved or removed, and the manual names what no longer exists.

How does the project make a stale manual visible before a change is merged, without asking for manual edits where none are due?

## Decision drivers

- **Visible at review, not later:** the manual is wrong from the moment a change merges without it; a reminder at release time is too late for agents and contributors who have moved on.
- **No busywork:** a fix in server code, a test or a refactor changes nothing people see; it must not need a manual edit.
- **Checks over memory:** agents write most changes, and they follow what CI enforces more reliably than prose (as with the catalogue, ADR-0017, and the invariants).
- **Precedent:** the DCO check already reads the pull request description and began as a warning (ADR-0018, ADR-0019).
- **One binary, no new dependency** (ADR-0024): the checks are Go tests and a small command, like `cmd/dcocheck`.

## Considered options

1. **A rule in `AGENTS.md` only** — the state since ADR-0183.
2. **A checkbox in a pull request template** ("Manual updated / not needed").
3. **Two checks in CI:**
   - a test that every UI label the manual names exists in the catalogue of its language;
   - a change check that a pull request touching what people see changes the manuals, or says in its description why it need not.
4. **Generate the manual from the code and the catalogue.**
5. **A manual review before every release**, by the maintainer.

## Decision outcome

Chosen option: **3**, with the rule of option 1 kept as the explanation of the checks (proposed with this record).

### Check A: what the manual names exists (test, in the tree)

- A Go test in `internal/web` reads every manual and collects the UI labels it names in bold, as the manual writes them (`**Rename**`, `**Präsentationsfenster**`).
- Each label must be a value of the catalogue of the manual's language, compared without ellipsis and trailing punctuation; a value with a placeholder such as `Present {n} pictures` matches with the placeholder as any word.
- Bold text that is no UI label is listed in `internal/web/manual_terms.txt`, one entry per line, each with a reason. Examples are key names (**L**, **Enter**), field contents and product names.
- When a label is renamed or removed in the catalogue, the test fails until the manual follows. This catches the **wrong** manual on every push, in every pull request and locally.

### Check B: a change to what people see touches the manual (CI, per pull request)

- A command `cmd/manualcheck <base> <head>` compares the pull request with its base, as `cmd/dcocheck` does.
- A pull request **changes what people see** when it:
  - adds or changes values in `locales/en.js`;
  - adds a route or a view in `assets/app.js`;
  - adds a module under `internal/web/static/assets`.
  Changes to tests, server code without UI strings, documentation and the manuals themselves are not.
- Such a pull request passes when it changes `manual/en.md` and every other manual, or when its description holds a line of its own, `Manual: not needed — <reason>`. Examples of a reason: a fixed typo in a label, an error message no one looks up.
- This catches the **missing** manual at review, where the author still knows what changed.
- The job is named `Manual (user-visible changes)`. It starts in **advisory** mode, reporting without failing, as the DCO check did. Making it required is the maintainer's decision; it then goes into the ruleset by pull request (ADR-0003).

### What stays as it is

- `AGENTS.md` keeps the rule and names both checks, so an agent knows why a check fails and how to answer it.
- ADR-0183's tests stay: the same outline in every language, a manual for every UI language, no ß in German.
- The test guide asks testers to report what the manual describes wrongly; such a report fails no check but is a finding.

### Consequences

- **Positive:** a renamed label cannot leave the manual behind; a new feature reaches review with the question whether the manual covers it, answered in the open.
- **Positive:** agents get a failing check with a message saying what to do, in place of a rule they may skip.
- **Negative / trade-offs accepted:** the change check judges by files and catalogue values, not by meaning. A new behaviour without a new string passes unnoticed, and a string change that needs no manual needs the line in the description.
- **Negative / trade-offs accepted:** the bold-label convention becomes binding: a label named in plain text is not checked. Writers keep labels in bold, as the manual already does.
- **Negative / trade-offs accepted:** neither check can tell whether a paragraph is still true or the German says what the English says; that stays with the authors, the review and the test guide.
- **Follow-ups / risks to watch:** help links from views to chapters (ADR-0183) would give Check B a finer signal; a list of chapters per module could narrow which chapter a change must touch.

## Pros and cons of the options

### Option 1: a rule in AGENTS.md only

Strongest form: no tooling, no false alarms; authors know best whether people see a change.

- Bad: the day the manual was merged, three user-visible changes passed without manual updates; nothing reminds an author who forgets.

### Option 2: a pull request template checkbox

Strongest form: the question is asked at the right moment, at no cost, for humans and agents alike.

- Bad: a checkbox is ticked without looking; the repository has no template so far, and a template is a layout, not a check.

### Option 4: generate the manual

Strongest form: a generated manual is never stale; labels, routes and keys come from the code itself.

- Bad: a manual explains why and in which order. Generated text lists controls and cannot explain a workflow or a limitation; the result would be a reference, not a manual.

### Option 5: a review before every release

Strongest form: one person reads the whole manual against the product at a known moment and catches what no check can.

- Good: it finds paragraphs that are no longer true.
- Bad: releases follow each other within hours (v0.13.3 to v0.13.8 on two days); a full read each time would not happen, and a fault found then belongs to a change long merged. It stays welcome as an occasional pass, not as the guarantee.

## Links

- amends [ADR-0183](0183-a-user-manual-in-every-ui-language-behind-the-account-menu.md) (the user manual)
- relates to ADR-0003 (repository settings and required checks), ADR-0017 (the message catalogue), ADR-0018 and ADR-0019 (the DCO check and its advisory mode)
