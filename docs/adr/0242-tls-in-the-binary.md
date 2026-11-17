# ADR-0242: TLS in the binary, with certificates from files chosen by the name asked for

- **Status:** Accepted
- **Implementation:** Landed
- **Date:** 2026-11-17
- **Deciders:** Project initiators (request of 2026-11-17: the listener answers requests for several host names, supports Server Name Indication and takes `-tls-cert` and `-tls-key`; the requester asked to see whether atlas has the same subject and how it settled it). On 2026-11-17 the requester decided the first open question: TLS 1.3 alone, as in atlas. Accepted by the requester on 2026-11-17, with the defaults proposed with the implementation; the open questions below stay open.
- **Open question:** how a server that answers several names names itself, as `OT_PUBLIC_URL` and the sign-in redirect hold one address; whether the commands that call a server (`mcp`) need a bundle of certificate authorities for a private one
- **Question checked:** 2026-11

## Context and problem statement

The server answers plain HTTP and nothing else.
`serve` takes `-addr` and `-data-dir`, listens with `net.Listen` and hands the listener to `http.Server.Serve`;
there is no certificate flag and no TLS configuration on the way in.
ADR-0037 describes the server "commonly run behind a reverse proxy that terminates TLS", and its settings make that arrangement safe.
No record decides that the binary must not terminate TLS itself: the absence is the state the server has always been in.

Three things follow from it.
An installation without a reverse proxy has no HTTPS at all, though the product is one binary (ADR-0024) and sessions, API tokens and OAuth tokens (ADR-0027, ADR-0068) must not cross a network in the clear.
A server reached by several host names, such as one name inside a network and one outside, needs a certificate for each, and which one to present is known only from the name the client asks for in its TLS handshake, the Server Name Indication (SNI).
And the HTTP side already answers every name: no route checks the `Host` header, so nothing has to be allowed there.

The reference project has the same subject and settled it.
atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md) gives its binary an optional TLS 1.3 listener with `--tls-cert` and `--tls-key`, one pair of files, read again when they change.
Its `GetCertificate` ignores the name asked for: one certificate answers every name, so several names need one certificate that lists them all.
It also binds a second listener in the clear on the loopback address, because its own child processes call the server over HTTP and no certificate names `127.0.0.1`.

How does the binary terminate TLS itself, for several host names, without taking on more of the life of a certificate than it must?

## Decision drivers

- **Nothing existing changes.** A server started without the new settings behaves exactly as before, behind a proxy or not.
- **One binary (ADR-0024).** No second process is needed for a small installation; no dependency, no CGO.
- **Half a configuration never answers in the clear.** A certificate without its key stops the server.
- **A renewal needs no restart.** Certificates last weeks to months; a restart ends live sessions (ADR-0010).
- **No setting that weakens.** No cipher list, no version switch, and no way to skip verification anywhere.
- **Several names, several certificates.** Names of different domains are often certified apart, by different authorities.
- **Follow atlas where the reasons carry over**, and say where they do not.

## Considered options

1. **Stay with plain HTTP**, and record that TLS is the reverse proxy's.
2. **One certificate pair**, as atlas: several names through one certificate that lists them.
3. **Several pairs, chosen by the name asked for** (SNI), the first pair answering every other name.
4. **A directory of certificates**, every pair found in it served.
5. **Certificates obtained by the binary** (ACME).

## Decision outcome

Chosen option: **3**, because it is option 2 for whoever has one certificate and adds only the choice among several.

- **Settings.** `serve -tls-cert FILE -tls-key FILE`, PEM files: the certificate chain with the server's certificate first, and its private key.
  Both flags are given once per certificate, the key at the same place as its certificate, or once with the files separated by commas.
  Where no flag names a file, `OT_TLS_CERT` and `OT_TLS_KEY` do, separated by commas, which is how the container image is configured; flags replace the environment as a whole.
  Unequal numbers of certificates and keys are a usage error (exit 2).
  Without any, the server answers in the clear as before.
- **Start.** Every pair is read at the start, after root is given up (ADR-0038) and before the data directory is opened: a missing file, a key that does not belong to its certificate or a file the server's account cannot read stops the server (exit 1).
  The start logs each certificate's names and the end of its validity.
- **The name chooses the certificate.** For each handshake the server presents the first pair, in the order given, whose certificate is made for the name the client asks for and whose key type the client can verify;
  wildcard names count as certificates define them.
  A name no certificate is made for, and a client that names none (one that asks by address), get the first pair's certificate, as `crypto/tls` does by default; the client then decides whether to trust it.
  Two pairs for one name with keys of different kinds (RSA and ECDSA) are therefore chosen by what the client supports.
- **The name chooses nothing else.** Every name reaches the same server and the same content; the `Host` header is not checked, as before.
  This is not virtual hosting.
- **Renewal.** Before choosing, the server looks up the modification times of every pair's files and reads a changed pair again.
  A change that cannot be read (a renewal caught half written, a file gone) keeps the certificate loaded before and warns once, not on every handshake; the next change is tried again.
- **Protocol.** TLS 1.3 alone, as in atlas.
  TLS 1.3 fixes its ciphers in the protocol, so there is no list to set, nothing an operator can weaken and nothing to review; a client that cannot speak it is refused, not served an older version.
  There is no setting for the least version; should an installation need TLS 1.2, that is a change to this record, added as a documented choice rather than by loosening the default.
- **HTTP/2.** With TLS the server speaks HTTP/2 with clients that offer it, as `net/http` does unasked.
  A test holds that a stream is flushed as it is written and takes a write deadline over HTTP/2, which the live session's event stream relies on (ADR-0010).
- **Secure requests.** A request that arrived over the server's own TLS counts as secure, so cookies are `Secure`, as behind a trusted proxy that says so (ADR-0037).
  `OT_TRUSTED_PROXIES` keeps its meaning; TLS in the binary and a proxy in front can be combined.
- **Liveness probe.** `healthcheck` asks the server on the loopback address, which no certificate names.
  Given the certificate files (`-tls-cert`, or `OT_TLS_CERT` as the server has it), it asks `https://127.0.0.1:8080/healthz` by default and trusts exactly the server certificates in those files, at whatever address; it checks neither names nor dates, since an expired certificate is no reason to restart a server.
  Without the files an `https` probe verifies as any client does.
  The image's health check therefore works unchanged when TLS is set through the environment.
- **Code.** `internal/certs` holds the pairs, the choice, the renewal and the probe's trust; `server.Serve` takes the TLS configuration, or none.
  No dependency is added: `crypto/tls` is the standard library.

### What this does not cover

- Obtaining certificates (ACME), a generated certificate, client certificates, the certificate stores of operating systems and keys protected by a passphrase.
- A redirect from port 80 and `Strict-Transport-Security`, which ADR-0037 names as a follow-up.
- A second listener in the clear, as atlas binds on the loopback address: this server has no process of its own that calls it over HTTP; the one caller on the same host, the probe, is served by the pinned certificates instead.
- Trust in a private certificate authority for the commands that call a server; they verify against the host's roots, as before.

### Consequences

- **Positive:** an installation without a reverse proxy has HTTPS from the one binary; several names each get the certificate made for them; a renewal takes effect at the next handshake; nothing changes for servers started as before.
- **Negative / trade-offs accepted:** every certificate problem of an operator is now also this project's (chains, formats, expiry); the files must be readable by the account the server runs as, which for a key is a deliberate permission; each handshake costs two file lookups per pair; a client that asks for an unknown name gets a certificate for another name rather than a refusal; a request in the clear to the TLS port is answered with an error, not redirected; a client without TLS 1.3 is not served, which may concern calendar programs subscribing to feeds (ADR-0168) and scripts on systems whose TLS library lacks it, an assumption that was not measured.
- **Follow-ups / risks to watch:** the open questions below; handshake errors of port scanners and TCP health checks are logged as warnings by `net/http`; HTTP/2 is new for every handler and is tested on a stream, not on every route.

## Pros and cons of the options

### Option 1: stay with plain HTTP

Strongest form: a reverse proxy does certificates better than a wiki ever will, operators run one anyway, and ADR-0037 already makes that arrangement safe; no code is the least to maintain.

- Good: no new support surface.
- Bad: the one binary needs a second process for the least an installation on a network needs; the absence was never a decision.

### Option 2: one certificate pair

Strongest form: it is what atlas ships, the smallest configuration, and one certificate listing several names covers most installations; two servers of one operator would be configured alike.

- Good: two flags with one value each; nothing to choose per handshake.
- Bad: names certified by different authorities, or renewed apart, cannot be served; SNI, which the request names, is not used.

### Option 3: several pairs chosen by name (chosen)

Strongest form: one pair behaves exactly as option 2, and the choice is the one `crypto/tls` already makes for a list of certificates, kept while the files may change.

- Good: covers one and several certificates with the same two flags; no new concept for the operator.
- Bad: pairing by order is easy to get wrong with many files, caught at the start only when a key does not belong to its certificate.

### Option 4: a directory of certificates

Strongest form: tools that renew certificates write directories, a new name needs no restart, and nothing is paired by hand.

- Good: no order to keep; names come and go with files.
- Bad: needs a convention for file names and for the certificate that answers unknown names; a stray file changes what the server presents; more to explain than two flags.

### Option 5: certificates obtained by the binary

Strongest form: for a server reachable from the internet, renewal stops being anyone's task.

- Good: the complete answer for that kind of installation.
- Bad: an account key, a cache, a reachable port 80 or a DNS credential and a state machine, and a dependency or a protocol implementation; installations inside a network, the ones without a proxy, can least use it.

## Open questions

- **One address for several names.** The server names itself by `OT_PUBLIC_URL` in OAuth (ADR-0068), in links of messages and in the sign-in redirect registered at an identity provider (ADR-0030).
  With that set, an assistant connecting through a second name gets metadata for the first, which RFC 9728 does not allow, and single sign-on returns to the first.
  Several names are therefore served, but one of them remains the address for these functions until a record decides otherwise.
- **Private certificate authorities.** `orbithek mcp -url https://…` verifies against the host's roots.
  atlas added `--tls-ca` for this after its first release of TLS; the same may be wanted here.
- **HSTS and port 80.** Settled by ADR-draft-hsts-and-redirect: both are settings, off unless asked for.

## Links

- relates to ADR-0024 (one binary), ADR-0037 (trusted reverse proxies, secure requests), ADR-0038 (the account the server runs as)
- relates to ADR-0010 (the live session's event stream), ADR-0027 and ADR-0068 (what must not cross a network in the clear), ADR-0168 (calendar feeds)
- atlas [ADR-0191](https://github.com/pblumer/atlas/blob/main/docs/adr/0191-built-in-tls-listener.md), the reference for the listener, the renewal and what is left out
