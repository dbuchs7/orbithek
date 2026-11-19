# ADR-0248: The sign-in provider names a first display name

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-19
- **Deciders:** Project initiators (request of 2026-11-19 to take the single sign-on `name` claim as the first display name, the open question of ADR-0246). Accepted by the requester on 2026-11-19; the open question below stays open.
- **Open question:** whether a later change of the name at the provider should reach a display name that came from it and was never changed on the account page
- **Question checked:** 2026-11

## Context and problem statement

A person chooses a display name on their account page, by which the start page greets them; without one, the username stands in (ADR-0246).
Under single sign-on the username is the provider's `preferred_username` or, failing that, the mail address (ADR-0030), so most people signing in that way are greeted by their mail address until they type a name.
The provider usually knows the person's full name and asserts it in the standard `name` claim of the ID token.
Should that name become the display name, and when must it not?

## Decision drivers

- People signing in through single sign-on should be greeted by their name without typing it.
- The person decides how they are addressed (ADR-0246): a name chosen, changed or cleared on the account page is never overruled.
- A name from outside passes the same checks as one typed on the account page.
- A provider's name that cannot be taken must never stop a sign-in.
- The accounts journal stays the one record of a display name; replaying it gives the same state.

## Considered options

1. **Take the name once**, at any sign-in through single sign-on, as long as the account never had a display name set.
2. **Take the name only when the account is created** by a first sign-in.
3. **Take the name at every sign-in**, so the display name follows the provider.
4. **Show the provider's name as a suggestion** on the account page, taken only when the person saves it.

## Decision outcome

Chosen option: **"Take the name once"**, because it reaches new accounts and the accounts that already exist without a display name, and still never overrules what a person did on their account page.

- **Claim:** `sso.Identity.Name` is the ID token's standard claim `name`; no other claim is combined into one (`given_name`, `family_name` stay unread).
- **When:** every sign-in through single sign-on calls `Accounts.SetFirstDisplayName` before the session starts, for a new account and an existing one alike.
- **Only the first:** the accounts journal remembers whether an account ever had a display name set, cleared included (any `DisplayNameSet`). Only an account that never had one takes the provider's name. Once taken, a later sign-in with another name changes nothing; a name cleared on the account page does not come back.
- **Checks:** the name is trimmed and checked as on the account page (at most 64 characters, valid UTF-8, no control or formatting characters; ADR-0246). An empty name is not taken. A refused name is not taken either, the account stays free to take a valid one at a later sign-in, and the sign-in goes on. A failure to save it is logged and does not stop the sign-in.
- **Record:** the same `DisplayNameSet` event as a name saved on the account page; nothing in the journal tells the two apart.
- **Visibility:** unchanged from ADR-0246 — the display name is given to its owner only; others keep seeing the username.

### Consequences

- **Positive:** people signing in through single sign-on are greeted by their name from their first sign-in, and existing accounts without a display name take it at their next sign-in.
- **Negative / trade-offs accepted:** a name changed at the provider later does not follow (the open question). A provider's name over 64 characters is not taken at all rather than cut, since cutting could end mid-name.
- **Follow-ups / risks to watch:** a provider that asserts a misleading name gives it to the person only, as no one else sees a display name; should others see it one day (ADR-0246), names from a provider need the same care as typed ones.

## Pros and cons of the options

### Take the name once

Strongest form: one rule covers new and existing accounts, and the first choice anyone makes on the account page ends the provider's say.

- Good: no input needed, also for accounts made before this change.
- Good: never overrules the person.
- Bad: one more fact per account in the journal's state (whether a name was ever set).

### Take the name only when the account is created

Strongest form: the narrowest change; it touches only just-in-time provisioning (ADR-0013), and the existing accounts are exactly as their owners left them.

- Good: nothing changes for anyone who already has an account.
- Bad: every account that already exists keeps being greeted by its mail address until its owner types a name, which leaves out the people the request is about.

### Take the name at every sign-in

Strongest form: the organisation's directory is maintained by the people responsible for it; following it keeps every display name current, also after a marriage or a correction, without anyone touching the account page.

- Good: names stay current.
- Bad: overrules what the person chose on the account page at their next sign-in, against ADR-0246's driver that the person decides how they are addressed.
- Neutral: following the provider only while the person never changed the name would combine both; left open.

### Show the provider's name as a suggestion

Strongest form: nothing is set without the person's action, and the person sees where the name came from.

- Good: no name the person did not confirm.
- Bad: the greeting keeps showing the mail address until the person visits the account page and saves, which most never do.
- Bad: the suggestion has to be kept somewhere until then, a second name per account.

## Links

- settles the open question of ADR-0246 (a display name each person chooses)
- relates to ADR-0013 (just-in-time provisioning), ADR-0030 (OpenID Connect single sign-on)
