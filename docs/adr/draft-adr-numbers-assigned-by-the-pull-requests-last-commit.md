# ADR-DRAFT: ADR numbers are assigned by the pull request's last commit

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: avoid the merge conflicts that parallel pull requests keep having. This answers the open question of ADR-0007 with its option 3; who runs the numbering in practice is a default proposed with the implementation.
- **Open question:** whether the numbering should later move into a workflow that pushes to `main` (option 2 of ADR-0007), should the ruleset of ADR-0003 ever admit one bypass actor
- **Question checked:** 2026-11

## Context and problem statement

ADR-0007 decided that a record in flight carries no number (`docs/adr/draft-<slug>.md`, headed `# ADR-DRAFT: Title`, cited as `ADR-draft-<slug>`), but left open who assigns the number: a workflow on `main` after the merge, or the pull request itself as its last commit. Until that was settled, numbers were still taken on the branch.

On 2026-11-12 three of four open pull requests added a record, two of them under the same number as the record `main` had just received. Each was renumbered once by hand, and one twice, once more after the other had been merged: file, heading, index row and every citation in code, manual and `AGENTS.md`. Two sessions did the same renumbering on the same pull request at the same time.

## Decision drivers

- Parallel branches must not collide on numbers or on the index.
- The ruleset of ADR-0003 has no bypass actors and admits no direct push to `main`.
- `main` must always be consistent: the tests in `docs/adr` hold on every commit.
- The step that remains must be mechanical, so that a person or an agent can do it in seconds.

## Considered options

1. **The pull request numbers its drafts as its last commit** (option 3 of ADR-0007)
2. **A workflow numbers drafts on `main` after the merge** (option 2 of ADR-0007, as atlas does)
3. **Keep taking the next free number on the branch** and let `cmd/adrnumbers` warn early

## Decision outcome

Chosen option: **1, the pull request numbers its drafts as its last commit**, because it is the only option that both removes the clash and leaves the ruleset of ADR-0003 as it is.

- A record is written as `docs/adr/draft-<slug>.md` with the heading `# ADR-DRAFT: Title`, no index row, and cited as `ADR-draft-<slug>` in code and documents. `go test ./docs/adr` checks the front matter of drafts like that of every record, that no draft has an index row, and that every `ADR-draft-<slug>` citation names a draft that exists.
- When the pull request is otherwise ready and up to date with `main`, whoever merges it — the maintainer, or the agent of the pull request at their request — runs `go run ./docs/adr/cmd/adrnum` and commits the result. The tool gives every draft the next free number in slug order, renames the file, rewrites its heading, appends its index row and rewrites every citation in the tree (`docs/adr/number.go`).
- Should `main` receive another record before the merge, the required "up to date" check (ADR-0003) stops the merge; the numbering commit is then dropped or reverted, `main` merged, and the tool run again.
- A draft that reaches `main` unnumbered is legal: the tests accept it, and the next pull request that runs the tool numbers it.

### Consequences

- **Positive:** two branches that each add a record touch different files; the index and the citations change only in the last commit, mechanically.
- **Negative / trade-offs accepted:** one manual step before the merge; a draft's number is not known while it is discussed, so a conversation cites its slug.
- **Follow-ups / risks to watch:** `cmd/adrnumbers` keeps warning about clashes of numbered records, which now only happen when two pull requests are numbered at the same time.

## Pros and cons of the options

### 1. Numbered by the pull request's last commit

Strongest form: no new rights for anyone; the numbering commit is reviewed like every other; the up-to-date rule already serialises it.

- Good: compatible with ADR-0003; the clash disappears from the work on a branch.
- Bad: a step that can be forgotten; then a draft reaches `main` and waits for the next run, which is harmless but untidy.

### 2. Numbered by a workflow on main

Strongest form: nobody ever thinks about numbers; it runs on every push to `main`, where "the next free number" has exactly one answer.

- Good: no manual step at all.
- Bad: a bot must push to `main`, which needs a bypass actor in the ruleset and weakens the guarantee of ADR-0003 that every change on `main` went through a reviewed pull request.

### 3. Numbers taken on the branch

Strongest form: the number is known from the first commit and can be cited at once; no tool.

- Good: simple.
- Bad: with several agents working in parallel, a clash is the normal case; each one costs a renumbering across files and, as on 2026-11-12, a second one when the first pull request is merged.

## Links

- answers the open question of [ADR-0007](0007-adr-numbers-assigned-at-merge.md)
- relates to [ADR-0003](0003-repository-governance-settings.md) (ruleset), atlas [ADR-0170](https://github.com/pblumer/atlas/blob/main/docs/adr/0170-adr-numbers-assigned-at-merge.md) (the tool this one is adapted from)
