# ADR-0244: The commands that call a server trust an organisation's own certificate authority on request

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17: implement the bundle of certificate authorities for private ones, the open question ADR-0242 left). That it covers the commands that call a server and not the server's own outgoing calls, and its form, were proposed with the implementation; accepted by the requester on 2026-11-17, the open question below staying open.
- **Open question:** whether the server's own outgoing calls should trust such a bundle too, and which of them: process engines, single sign-on, mail, backups, connectors
- **Question checked:** 2026-11

## Context and problem statement

Since ADR-0242 the binary terminates TLS itself with the certificates an operator gives it.
Inside an organisation these often come from its own certificate authority, which no operating system trusts unless someone installs it there.

Two commands of the binary call a server over HTTP: `mcp`, the adapter a local agent runs to use a server with an API token (ADR-0012), and `healthcheck`, the liveness probe.
Both verify the server's certificate against the authorities of the host they run on, and nothing else.
Against a server with a certificate of an organisation's own authority, every call of `mcp` fails with an error that names the certificate, far from its cause, and a probe by the server's name fails likewise.
The probe on the server's own host is not affected: given the server's certificate files, it trusts exactly those (ADR-0242).

ADR-0242 left this open.
The reference project met the same: atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md) added `--tls-ca`, a bundle added to the host's authorities, first for one server calling another, then for its worker and at last for its `mcp` command, after assuming that a person's machine would hold the organisation's authority anyway.
It reaches no further than atlas calling atlas, and there is no way to skip verification anywhere.

How do these commands reach a server whose certificate an organisation's own authority signed, without a way around verification?

## Decision drivers

- **Nothing existing changes.** A command started as before verifies as before.
- **No way around verification.** No switch that skips it: it would be reached for when a certificate is wrong, which is when it must not exist.
- **Added, never replacing.** Naming one's own authority must not lose the publicly trusted ones.
- **A wrong setting is seen at once**, where its cause is, not in every later call.
- **The smallest reach.** What trusts the bundle is named, not everything the binary calls.
- **One answer in both projects** where the reasons carry over.

## Considered options

1. **A bundle for the commands that call a server:** `-tls-ca` on `mcp` and `healthcheck`.
2. **A bundle for everything the binary calls**, the server's outgoing calls included.
3. **Leave it to the host's store of authorities.**
4. **A switch that skips verification.**

## Decision outcome

Chosen option: **1**.

- **Setting:** `mcp -tls-ca FILE` and `healthcheck -tls-ca FILE`, a PEM file of one or more certificates of authorities; where the flag is not given, `OT_TLS_CA` names the file.
- **Added to the host's authorities.** The command verifies against what the host trusts and the file's authorities besides.
  Names, dates and chains are checked as always.
- **A file that cannot be used stops the command** before it calls anything (exit 2): a file that is missing, cannot be read or holds no certificate.
  For `mcp` the agent then sees a process that did not start, not tool calls that all fail.
- **The probe:** given the server's certificate files (`-tls-cert`, `OT_TLS_CERT`), `healthcheck` keeps trusting exactly those certificates, whatever `-tls-ca` says: they say more than an authority does, and the container image sets them for the server anyway.
  The bundle serves a probe by the server's name, from another host.
- **The server does not read the setting.** `serve` trusts the host's authorities for everything it calls, as before; `OT_TLS_CA` in a server's environment changes nothing but the probe run there.
- **Code:** `certs.Roots` builds the authorities; `trusting` in `cmd/orbithek` gives both commands their transport.
  No dependency is added.

### What this does not cover

- The server's own outgoing calls, for which see the open question.
- Client certificates: `-tls-ca` names whom a command trusts, not who may call a server.

### Consequences

- **Positive:** a local agent reaches a server inside an organisation with one flag or one variable; a probe from another host verifies the server by name; nothing changes for anyone who sets neither.
- **Negative / trade-offs accepted:** whoever holds an authority in the file can present any name to these commands, as with every authority a host trusts; the file has to reach each machine an agent runs on; the variable's name stands beside `OT_TLS_CERT` and `OT_TLS_KEY` yet is not read by the server, which the documentation has to say.
- **Follow-ups / risks to watch:** the open question; requests for a switch that skips verification, which this record refuses.

## Pros and cons of the options

### Option 1: a bundle for the commands that call a server (chosen)

Strongest form: it answers exactly the question ADR-0242 left, in the form the reference project arrived at, and what trusts the bundle can be named in one sentence.

- Good: small, checked at the start, the same flag in both projects.
- Bad: a server that calls services of the same organisation is not helped.

### Option 2: a bundle for everything the binary calls

Strongest form: an organisation with its own authority uses it everywhere, for the identity provider, the mail server, the backup store and the process engine, and the container image holds no store an operator can add to; one setting would serve them all.

- Good: one setting for an installation inside an organisation.
- Bad: it widens whom the server trusts for every outgoing call at once, including those that reach public services on behalf of people (web links, calendars, photo sources), where an added authority can present any name; which calls should take it is a decision of its own, per kind of call, and not one this request made.

### Option 3: leave it to the host

Strongest form: an operating system's store is the one place authorities belong, administrators fill it for every program at once, and a second place to configure trust is a second place to get it wrong.

- Good: nothing to build; one store per machine.
- Bad: atlas assumed this for its `mcp` command and then added the flag; an agent's machine is often not managed by whoever runs the server, and adding to the store needs rights the person may lack.

### Option 4: skip verification

Strongest form: it works at once, for a test or a first try, with nothing to distribute.

- Good: no file to pass around.
- Bad: the API token then goes to whoever answers; the switch outlives the test; both projects refuse it for the same reason.

## Open questions

- **The server's outgoing calls.** `serve` calls process engines, identity providers, mail servers, backup destinations and connectors' sources, each of which may sit behind an organisation's own authority.
  Whether a bundle should apply there, and to which of these, is undecided; the nearest to this record is the call to a process engine, which is one product of the same operator calling another.

## Links

- amends ADR-0242 (its open question on private certificate authorities settled for the commands)
- relates to ADR-0012 (the `mcp` command) and ADR-0027 (the API token it carries)
- atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md), whose `--tls-ca` this follows
