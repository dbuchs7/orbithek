# ADR-DRAFT: A space carries a short description

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-18
- **Deciders:** Project initiators (request of 2026-11-18 that rooms can be given a short description text)
- **Open question:** whether the description should take formats or links; whether search and the starmap of all rooms should use it; whether a space should have a key of its own so that what describes it can be sealed like content
- **Question checked:** 2026-11

## Context and problem statement

A space has a key, a display name (ADR-0086) and an icon (ADR-0062).
With many spaces, name and key alone often do not tell what a space is for, who works in it or what belongs there.
People have to open a space, or ask, to find out.

Where does a short text that says what a space is for live, who writes it, and how is it kept?

## Decision drivers

- A space is a system object, not content (D1): a description of it is no item and has no body, history of its own, relations or comments.
- Space administrators decide on their space, as for its name (ADR-0086); nobody else.
- Who cannot see a space must not learn of it through its description (D10).
- The change is part of the log, so it survives restarts, backups and replays (ADR-0025).
- Content never reaches disk in plaintext (ADR-0026). Content is sealed with a key per item; a space has no key.

## Considered options

1. **An event of the item store, `SpaceDescribed`, with the text in plain, like the name; text with sensitive data refused.**
2. **The same event, the text sealed with a new key per space.**
3. **An asset or setting of the site journal, named by the space's key, as the icon is (ADR-0062).**
4. **An item of a ready-made type in the space, shown as its description.**

## Decision outcome

Chosen option: **1**.

- **Event:** a new event kind, `SpaceDescribed`, carries the text in the field `spaceDescription` (codec version 70); an empty text removes the description.
  Versions 1 to 69 are still read; their spaces have no description.
  It is a space event like `SpaceRenamed`: the change feed reports it to those who watch the space.
- **Who:** a space's administrators, and system administrators, who administer every space.
  Others with a role in the space are refused; to those without a role, the space does not exist.
- **Rules:** plain text, trimmed, line breaks kept (`\r\n` becomes `\n`), other control characters refused, at most 300 characters (`maxSpaceDescription`, a shape of the space like the length of its name, not a budget).
  Text in which the check of ADR-0040 finds sensitive data (passwords, keys, account numbers) is refused, since it would be stored unencrypted and shown to every member.
  The same text again records nothing; a space in the trash cannot be described.
- **Interface:** `PUT /api/spaces/{space}/description` with `{"description": …}`; `GET /api/spaces` names each space's `description` when it has one, so the MCP tool `list_spaces` gives it to agents too.
  Setting it is not offered as an MCP tool, as renaming is not.
- **Restore:** `import -from backup` (ADR-0124) gives a space it creates the description it had in the backup, with its name; one the rules refuse now is left out with a note in the report.
- **UI:** the space's settings page has a *Description* card under *Display name*; the description shows below the name on the start page's tiles (at most three lines) and in its list, and below the heading of the space's page.

### Consequences

- **Positive:** what a space is for can be read where the space is chosen, without opening it.
- **Positive:** one event kind, handled as the name is: no new store, journal or key.
- **Negative / trade-offs accepted:** the description is stored unencrypted, like the name, and stays in the log after a purge of the space, as the name does.
  The refusal of sensitive data narrows what can leak but does not make the text content; the settings page says so.
- **Negative / trade-offs accepted:** the codec version rises. A data directory written by this version is not read by earlier releases, as with every earlier rise.
- **Follow-ups / risks to watch:** if people want longer texts, formats or links, a space's home page (an item) is the place, not a longer description.

## Pros and cons of the options

### Option 2: sealed with a key per space

Strongest form: free text typed by people is exactly what ADR-0026 wants sealed; a key per space would also let a purge destroy what described the space.

- Good: no plaintext free text on disk.
- Bad: a new family of keys in `keys.wal`, with rotation, backup and purge rules, the codec's sealing (bound to an item ID) widened to spaces, and the name would then be the only plain text left, which invites sealing it too. Far more than a few lines of presentation call for, while the refusal of sensitive data covers the case that matters most.

### Option 3: the site journal

Strongest form: the icon already lives there (ADR-0062); no codec change.

- Bad: the site journal holds presentation chosen by administrators, not text written per space; the description would not be part of the change feed, of `import -from backup` of a space (ADR-0124) or of the space's own history, and the site journal is no more encrypted than the log.

### Option 4: an item in the space

Strongest form: an item is content, sealed, versioned, searchable and commentable (D1); a space's home page could carry this.

- Bad: a short line shown on a tile would depend on finding one item among many, readable or not by the reader (D9), and would need a type in every installation. It also mixes what describes the container with what it contains.

## Links

- relates to ADR-0086 (renaming a space), ADR-0062 (space icons), ADR-0026 (content is sealed), ADR-0040 (sensitive data), ADR-0025 (the event log and its codec)
