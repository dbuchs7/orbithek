# ADR-0246: A display name each person chooses

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-19
- **Deciders:** Project initiators (request of 2026-11-19 that the greeting on the start page shows a display name, which a person defines on their account; decided on 2026-11-19 that other people keep seeing the username, which under single sign-on is often the mail address); proposed for the maintainer's acceptance
- **Open question:** whether an account created through single sign-on takes the provider's `name` claim as its first display name
- **Question checked:** 2026-11

## Context and problem statement

The start page greets the reader by the time of day and their username: "Good morning, sbu".
An account knows no other name.
The username identifies the account at sign-in; it is 1 to 64 visible characters without spaces (ADR-0027), chosen by an administrator, by the person requesting the account (ADR-0061), or taken from the provider's `preferred_username` or mail address on a first sign-in through single sign-on (ADR-0030).
It is often a short form, a staff number or a mail address, not the name a person wants to be addressed by.
Where does a name a person wants to be called by live, who sets it, and where is it shown?

## Decision drivers

- A person decides how they are addressed, without an administrator.
- The username keeps identifying the account; signing in, API tokens, MCP clients and searches for a person must not change.
- Account data lives in the accounts journal (ADR-0027); users are system objects, exempt from D1.
- A name shown in the interface must not be able to break a line, hide characters or turn text around.
- One concern per change: the greeting is what was asked for.

## Considered options

1. **A display name in the accounts journal**, set by the person on their account page, used by the greeting, the username standing in when it is empty.
2. **The provider's `name` claim** on sign-in through single sign-on.
3. **A profile item** of a ready-made type that holds the name and more.
4. **A changeable username**: let the person rename the account itself.

## Decision outcome

Chosen option: **"A display name in the accounts journal"**, because it serves every account, local or external, leaves the identity of the account untouched, and follows how the language (ADR-0017), the profile picture (ADR-0050) and the room of daily notes (ADR-0205) are already kept.

- **Journal:** `DisplayNameSet` (`displayName`) in the accounts journal; an empty name clears it.
  `Accounts.SetDisplayName` trims the name and refuses one of more than 64 characters, one that is not valid UTF-8, and one holding a control or formatting character (Unicode Cc and Cf); the zero-width non-joiner and joiner stay allowed, since some scripts spell names with them.
  Setting the same name again records nothing.
- **API:** `PUT /api/me/display-name` with `{"displayName": "…"}`; 400 for a refused name.
  `GET /api/me` names it as `user.displayName`, left out when none is set.
  The name is given to the person themselves only; no other route names it.
  A read token cannot set it, as with every write.
- **UI:** the account page has a card *Display name*, whose placeholder and help text name the username that stands in when the field is empty.
  The start page greets with the display name, else the username.
- **Other people see the username**, not the display name: beside comments, in an item's history, in fields that name a person, in the list of accounts and in the account menu the username stays.
  Under single sign-on the username is often the mail address (ADR-0030), and that is what others keep recognising a person by.
  A display name chosen freely by its owner is therefore never shown to anyone else, so nobody can be misled by a name imitating another person's.

### Consequences

- **Positive:** a person is greeted by the name they chose; nothing that identifies or finds an account changes.
- **Negative / trade-offs accepted:** the greeting names a person by their display name, the rest of the interface by their username.
  The browser's length limit on the field counts UTF-16 units, so a name of characters outside the Basic Multilingual Plane may be cut shorter than the server allows; the server's check of 64 characters is the binding one.
- **Follow-ups / risks to watch:** should others ever see the display name, two people could choose the same one, or one imitating another person's; that needs a record of its own, the username shown beside the name, and an audit of changes (ADR-0029), which a name seen only by its owner does not need.

## Pros and cons of the options

### A display name in the accounts journal

Strongest form: a field of the account, set by its owner, beside the username that keeps its role; one event, one route, one card.

- Good: works for local and external accounts alike; no administrator involved.
- Good: the username, which tokens, MCP clients, searches and audits refer to, stays.
- Bad: one more name per person, which the interface has to choose between.

### The provider's `name` claim

Strongest form: the organisation's directory already holds the person's full name, maintained by the people responsible for it; taking it on every sign-in keeps it current without anyone typing it, and nobody can choose a misleading name.

- Good: no input needed; names follow the directory.
- Bad: local accounts have no provider and would keep being greeted by their username.
- Bad: the person cannot choose how they are addressed (a shorter form, a name not yet in the directory).
- Neutral: taking the claim as the first value of option 1 combines both; left open.

### A profile item

Strongest form: a profile page per person, an item like every other (ADR-0001), could hold the name, a description, skills and contact details, searchable and versioned.

- Good: history and search come for free.
- Bad: a profile item would live in some room, with that room's roles and restrictions deciding who reads a person's name; the greeting would depend on an item one can trash.
- Bad: much larger than what was asked; users are system objects, exempt from D1 for exactly this reason.

### A changeable username

Strongest form: one name per person is simpler than two; letting the owner rename the account removes the need for a second name.

- Good: no second name to choose between.
- Bad: a username is limited to visible characters without spaces, so "Anna Müller" is not one.
- Bad: renaming changes how one signs in, and what audit entries, people fields and other people know the person by.

## Links

- relates to ADR-0017 (UI language per user), ADR-0027 (accounts, sessions and tokens), ADR-0030 (single sign-on), ADR-0050 (profile pictures in the accounts journal), ADR-0061 (account requests), ADR-0205 (the room of daily notes in the accounts journal)
