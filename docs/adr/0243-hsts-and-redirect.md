# ADR-0243: A transport policy on request and a redirect from plain HTTP

- **Status:** Proposed
- **Implementation:** Partial
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17: implement HSTS and the redirect from port 80, the open question ADR-0242 left). That both are settings, off unless asked for, and their forms are defaults proposed with the implementation, to be confirmed by the requester.
- **Open question:** whether the redirect listener should answer the challenges of a certificate authority (`/.well-known/acme-challenge/`) for a program that renews certificates beside the server; whether an installation behind a reverse proxy wants the policy's value set by the proxy alone
- **Question checked:** 2026-11

## Context and problem statement

Since ADR-0242 the binary terminates TLS itself.
Two things a reverse proxy usually does beside that were left open there, and ADR-0037 named the first as a follow-up.

A reader who types a host name without `https://` reaches port 80, where nothing answers: the browser reports that the site cannot be reached.
And a browser that once reached the site over HTTPS still sends a later request for `http://…` in the clear, where whoever sits on the path can answer in the server's place.
`Strict-Transport-Security` (HSTS, RFC 6797) closes the second: a browser that received it over HTTPS turns every later request to that host into HTTPS by itself, for the time the header names.

Both reach beyond this server.
The policy binds the host name, not the port: a browser that holds it also turns `http://host:5000` into `https://host:5000`, which fails where another service on the same host answers plain HTTP only.
It lasts in each browser until its time runs out, whatever the server does afterwards.
And port 80 is one port of the host, which another program may hold.

atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md) left both out: a redirect needs a listener on port 80 and an opinion about the public name, and the policy is "a footgun to enable by default on a hostname" the server does not own.

How does the binary send readers of plain HTTP to HTTPS and keep them there, without binding hosts and ports it was not given?

## Decision drivers

- **Nothing existing changes.** A server started as before sends no new header and opens no new port.
- **Nothing beyond the server by default.** What binds the whole host name is switched on by whoever answers for that name.
- **Every setting has a way back.** A policy sent can be withdrawn; a redirect leaves nothing behind in browsers.
- **Several names (ADR-0242).** A reader sent to HTTPS keeps the name they asked for.
- **No request in the clear is made worse.** A redirect must not teach clients to keep sending credentials over plain HTTP.

## Considered options

1. **Both as settings, off by default:** `OT_HSTS_MAX_AGE` and `-redirect-addr`.
2. **Both on whenever the binary terminates TLS.**
3. **The redirect alone**, the policy left to a reverse proxy.
4. **Neither**, as atlas.

## Decision outcome

Chosen option: **1**.

### The transport policy

- **Setting:** `OT_HSTS_MAX_AGE`, a number of seconds, e.g. `31536000` for a year. Unset, the header is not sent.
  A value that is no whole number from zero up stops the server at its start.
- **The header:** `Strict-Transport-Security: max-age=<seconds>`, with every answer to a request that arrived over HTTPS: over the server's own TLS (ADR-0242), or through a trusted proxy that says so (ADR-0037).
  Over plain HTTP it is left out, where browsers ignore it.
  A browser also ignores it on a connection whose certificate it did not accept, so trying a certificate that is not trusted binds nobody.
- **Never `includeSubDomains`, never `preload`:** both bind hosts this server is not.
- **The way back:** `OT_HSTS_MAX_AGE=0` tells each browser to forget the policy at its next visit over HTTPS; only then should TLS be taken away.
- **A setting without effect is said:** with neither certificates nor trusted proxies no request counts as secure, and the start logs a warning.

### The redirect

- **Setting:** `serve -redirect-addr ADDR`, or `OT_REDIRECT_ADDR`, e.g. `:80`: a second listener beside `-addr`.
  It needs `-tls-cert` and `-tls-key`; without them it is a usage error (exit 2), since nothing would answer the HTTPS it names.
  An address that cannot be listened on stops the server at its start.
- **GET and HEAD** are answered with `302 Found` to `https://<host><path>?<query>`: the host the request names, without its port, so every name keeps its own.
  A host that is not a plain name or address is refused.
- **The port of HTTPS** is 443 unless `OT_PUBLIC_URL` is an `https` address that names another, which the redirect then names.
  The port of `-addr` does not tell: a container publishes it under another.
- **Temporary, not permanent.** Browsers keep a permanent redirect without an end; it would outlast a server that stops terminating TLS, with no setting to withdraw it.
  What makes HTTPS stick is the policy above, which has an end and a way back.
- **Every other method is refused** with `400`: its body and credentials have crossed the network in the clear already, and a client that is redirected would send them so again each time, unnoticed.
- **Nothing else is answered there:** no content, no API, no liveness probe.
- **One life:** the redirect listener and the server start and stop together; either failing ends the process.

### Consequences

- **Positive:** a reader who types the bare name arrives; with the policy set, later requests never leave the browser in the clear; nothing changes for installations that set neither; several names each stay themselves.
- **Negative / trade-offs accepted:** the policy binds every port of the host name in each browser, which breaks plain-HTTP services on the same name, and it lasts up to the time set even if the server changes; the operator must know this, which is why it is a setting and documented beside it. The first request of a new browser is still in the clear; only a preload list would close that, which is left out. The redirect trusts the host a request names for the address it sends that same client to. A program that renews certificates through port 80 cannot use that port while the redirect holds it.
- **Follow-ups / risks to watch:** the open questions; whether operators set a long policy before trying a short one, which the documentation advises against.

## Pros and cons of the options

### Option 1: both as settings, off by default (chosen)

Strongest form: the two things that reach beyond the server happen only when whoever answers for the host asks for them, and each can be taken back.

- Good: no installation is bound by an upgrade; the trade-off is read before it is taken.
- Bad: an installation that would be safer with both gets neither until someone sets them.

### Option 2: both on with TLS

Strongest form: whoever gave the server certificates wants HTTPS, and safe defaults should not need reading; most guides recommend both.

- Good: the safest outcome without configuration, for a host that serves only this.
- Bad: a host name shared with plain-HTTP services on other ports, common on a home or office server, loses them in every browser that visited, for as long as the policy says; port 80 may be held by another program, and the server would fail to start after an upgrade.

### Option 3: the redirect alone

Strongest form: the redirect is harmless and helps at once; the policy is the risky half and can stay with a proxy.

- Good: nothing that outlasts the server.
- Bad: a redirect without the policy can be answered by anyone on the path, every time; the half that protects is the one left out.

### Option 4: neither

Strongest form: it is what atlas decided, both are a reverse proxy's craft, and a server that does less has less to explain.

- Good: no new surface.
- Bad: the one binary reaches HTTPS but not the readers who type its name, and ADR-0037's follow-up stays open.

## Open questions

- **Certificate renewal over port 80.** A program that proves control of a name to a certificate authority over HTTP needs `/.well-known/acme-challenge/` on port 80.
  The redirect listener does not serve it; whether it should hand such requests to a directory or a program is undecided, as obtaining certificates is outside ADR-0242.
- **Behind a reverse proxy.** The policy is sent for requests a trusted proxy marks as HTTPS.
  A proxy that sets its own policy then sends two headers or replaces this one; whether the setting should apply only to the server's own TLS is open.

## Links

- amends ADR-0242 (its open question on the transport policy and the redirect settled)
- relates to ADR-0037 (what counts as a secure request; its follow-up on `Strict-Transport-Security`)
- relates to ADR-0027 and ADR-0068 (the credentials a request in the clear would carry)
- atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md), which leaves both out
