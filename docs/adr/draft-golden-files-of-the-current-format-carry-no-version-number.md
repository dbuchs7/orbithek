# ADR-DRAFT: Golden files of the current format carry no version number

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-12
- **Deciders:** Project initiators. Request of 2026-11-12: avoid the merge conflicts that parallel pull requests keep having.
- **Open question:** whether raising `CodecVersion` should again freeze the files of the format it replaces as `events.vNN.*` for `TestOlderVersionsStillReplay`, as was done up to version 53 and not for versions 54 to 58
- **Question checked:** 2026-11

## Context and problem statement

`TestPersistedFormatIsStable` (ADR-0025) compares what `item.EncodeBatch` writes for logs covering every event kind with golden files. Until now those files carried the codec version twice: in their names (`events.v59.album.golden.jsonl`) and at the start of every event (`{"v":59,"seq":…`). Raising `CodecVersion` therefore renamed all 95 files and changed 1,152 lines.

Two pull requests that each change the format clash in all of these files, although each changes the shape of only a few. A pull request that changes no format but the content of one golden file, such as a ready-made type, clashes with every pull request that raises the version. On 2026-11-12 both happened.

## Decision drivers

- The golden files must keep pinning everything the encoder writes, including that every event carries `CodecVersion`.
- Raising the version must not touch files whose shape did not change.
- Files of earlier versions, which `TestOlderVersionsStillReplay` decodes, stay as they are.

## Considered options

1. **The current files carry no version**: names `events.current.*`, and `"v":"current"` in place of the number
2. **Keep the version in names and events** and resolve the clashes by regenerating
3. **Keep the version in the events, drop it from the names**

## Decision outcome

Chosen option: **1, the current files carry no version**.

- The files of the current format are named `testdata/events.current.<log>.golden.jsonl`. Every event in them starts `{"v":"current","seq":` instead of the number.
- Before comparing, the test checks that every event was written with `CodecVersion` and only then replaces the number, so the version is still pinned. `assertRebuilds` puts `CodecVersion` back before it decodes a file.
- Raising `CodecVersion` changes no golden file; a change of the format changes exactly the files whose logs show it. `-update` stays the last step of such a change, never the fix for a failing test.

### Consequences

- **Positive:** a version bump is a one-line change plus the decoder; two format changes clash only where both change the same log; content changes no longer clash with bumps.
- **Negative / trade-offs accepted:** a file no longer says by its name which version it pins; that is `CodecVersion` at the commit, as Git shows.
- **Follow-ups / risks to watch:** the open question above; an open pull request that still edits `events.v59.*` must take the edit over to `events.current.*` once this is merged.

## Pros and cons of the options

### 1. No version in the current files

Strongest form: the file pins the shape, the test pins the version, and each changes only when its own subject changes.

- Good: no mass renames; a bump touches no golden file.
- Bad: a placeholder that must be put back before decoding.

### 2. Version in names and events

Strongest form: each file says exactly which version it pins, without reading code; a clash is resolved by `-update`, which is mechanical.

- Good: no placeholder.
- Bad: every bump rewrites 95 files, so every pair of format changes clashes everywhere, and the "mechanical" resolution has to be redone after each merge.

### 3. Version in the events only

Strongest form: no renames, and the files stay valid logs that can be decoded as they are.

- Good: smaller change than option 1.
- Bad: a bump still changes 1,152 lines, so the clashes remain, only without renames.

## Links

- relates to [ADR-0025](0025-event-log-file-and-in-memory-state.md) (persisted format), [ADR-0005](0005-test-driven-development.md)
