# ADR-DRAFT: Calls to process engines trust an organisation's own certificate authority on request

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-18
- **Deciders:** Project initiators (request of 2026-11-18: implement the bundle of certificate authorities for the server's outgoing calls to atlas, the open question ADR-0244 left). That one setting covers every process engine, and its name, are proposed with the implementation, to be confirmed by the requester.
- **Open question:** whether the server's other outgoing calls should trust such a bundle too: single sign-on, mail, backups, connectors
- **Question checked:** 2026-11

## Context and problem statement

The server calls process engines: it reads their processes, forms and decisions, follows their user tasks and completes them for the people connected to them (ADR-0129, ADR-0151, ADR-0152).
The engine in use is atlas, which since its ADR-0191 can terminate TLS itself with certificates its operator supplies.
Inside an organisation those often come from its own certificate authority.

The client these calls are made with verifies an engine's certificate against the authorities of the host the server runs on, and nothing else.
An engine with a certificate of an organisation's own authority is therefore refused: every read fails with a certificate error, and blocks that show its processes say that the engine is not available.
The container image holds the host's authorities as files of the image, which an operator cannot add to without building another image.

ADR-0244 gave the commands that call a server a bundle of authorities to trust and left the server's own outgoing calls open, naming the call to a process engine as the nearest: one product of the same operator calling another.
atlas settled the mirror image the same way: its `--tls-ca` reaches a peer atlas and no third party ([ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md)).

How does the server reach an engine whose certificate an organisation's own authority signed, without widening whom it trusts elsewhere?

## Decision drivers

- **Nothing existing changes.** A server started as before verifies engines as before.
- **No way around verification** (ADR-0244).
- **Added, never replacing.** An engine with a publicly issued certificate keeps working beside one with the organisation's own.
- **The smallest reach.** The authority counts for calls to engines, not for what the server fetches on behalf of people or from public services.
- **A wrong setting stops the server**, as other settings of the environment do.
- **The address rule stays apart.** Which addresses an engine may have is decided by `OT_ENGINE_ALLOW_PRIVATE` alone.

## Considered options

1. **One bundle for all process engines:** `OT_ENGINE_TLS_CA`.
2. **A bundle per engine:** `OT_ENGINE_<ID>_TLS_CA`.
3. **`OT_TLS_CA` read by the server too**, for its calls to engines.
4. **A bundle for every outgoing call of the server.**
5. **Leave it to the host's store of authorities.**

## Decision outcome

Chosen option: **1**.

- **Setting:** `OT_ENGINE_TLS_CA`, the path of a PEM file of one or more certificates of authorities.
  Unset, engines are verified against the host's authorities, as before.
- **Added to the host's authorities**, for every call the server makes to a process engine: the reads with an engine's credential, the reads and starts through an engine's public routes, and the exchange of codes and tokens when people connect their accounts (ADR-0152).
  Names, dates and chains are checked as always.
- **No other call reads it.** Single sign-on, mail, backups, connectors, calendars, web links and photo sources verify against the host's authorities.
- **A file that cannot be used stops the server at its start** (exit 1): missing, unreadable, or without a certificate.
  The start logs that engines are verified against the file's authorities too.
- **Addresses.** An engine with such a certificate is usually in the server's own network, which `OT_ENGINE_ALLOW_PRIVATE=on` allows as before; the bundle neither needs nor replaces that setting.
- **Code:** `engine.NewClientTrusting` takes the authorities `certs.Roots` builds (ADR-0244); `serve` gives the one client to the reader of engines and to the broker of people's connections.
  No dependency is added.

### Consequences

- **Positive:** a server and an engine of the same organisation talk over TLS with the organisation's own certificates, in the container image too; nothing changes for installations that do not set it; the trust added stays with the calls it was added for.
- **Negative / trade-offs accepted:** whoever holds an authority in the file can present any engine's name to the server and so receive that engine's credential, as with every authority a host trusts; one bundle counts for all engines, so an authority meant for one also vouches for the others; the setting's name sits among the variables of engines (`OT_ENGINE_<ID>_…`), which an engine called `TLS` would share a prefix with.
- **Follow-ups / risks to watch:** the open question; a request for a bundle per engine, should engines of different organisations be read by one server.

## Pros and cons of the options

### Option 1: one bundle for all engines (chosen)

Strongest form: the engines a server reads are set up by its administrators and belong, as a rule, to the organisation that runs it, whose one authority signs them all; a file may hold several authorities where it does not.

- Good: one setting beside `OT_ENGINE_ALLOW_PRIVATE`, which is also one for all engines; nothing to repeat per engine.
- Bad: no way to say that an authority vouches for one engine only.

### Option 2: a bundle per engine

Strongest form: trust is narrowest when each engine names its own authority, as each already names its own credential; an authority then cannot vouch for an engine it was not named for.

- Good: the least trust per call.
- Bad: one client per engine instead of one for all, in the reader and in the broker; a variable per engine for what is, inside one organisation, the same file every time; nothing asks for it yet.

### Option 3: `OT_TLS_CA` read by the server

Strongest form: one variable for "the organisation's authority" is the least to learn, and atlas uses one name for its flag in every place.

- Good: no new name.
- Bad: ADR-0244 decided that the server does not read that variable, so that setting it for the probe in a server's environment changes nothing else; one name would then mean two reaches, and a later decision on other outgoing calls would have to take it apart again.

### Option 4: every outgoing call

Strongest form: an organisation with its own authority uses it for its identity provider, mail server and backup store too; one setting would serve an installation inside it.

- Good: one setting for everything inside an organisation.
- Bad: the server also fetches web pages, calendars and photos on behalf of people, where an added authority can present any name; which calls should take it is a decision per kind of call, left open here as in ADR-0244.

### Option 5: leave it to the host

Strongest form: the host's store is where authorities belong, and an operator who builds an image can add to it.

- Good: nothing to build.
- Bad: the released image cannot be added to without building another, for exactly the installations this is for.

## Open questions

- **The other outgoing calls.** An identity provider, a mail server, a backup destination or a connector's source may sit behind an organisation's own authority as well.
  Whether each should take a bundle, the same or its own, is undecided.

## Links

- amends ADR-0244 (its open question settled for process engines)
- relates to ADR-0129 (reading process engines), ADR-0151 (linked tasks), ADR-0152 (people's connections to an engine)
- relates to ADR-0127 (the rule for addresses, which this leaves as it is)
- atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md), whose `--tls-ca` reaches a peer atlas only
