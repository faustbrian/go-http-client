# Security Model

[Documentation index](README.md) · [Repository entry](../README.md)

The baseline is explicit trust and selected finite accounting limits. Standard
transport timeouts and header sizes are bounded; helper byte/count limits do not
measure every allocation or interrupt arbitrary blocking I/O. Cancellation is
cooperative for borrowed providers, callbacks and stream implementations.

Credentials are attempt-local, HTTPS-only, and same-origin by default. HTTP is
available only through the explicit local-test `AllowInsecure` option.
Cross-origin redirects strip authorization, proxy authorization, cookies,
trace context, baggage, and configured sensitive headers. Query credentials
are explicit and discouraged. Package error strings suppress selected sensitive
data, but exported fields,
raw status headers, unwrapped causes, custom telemetry and redactor output remain
application-classified. Persisted body digests can reveal guessable inputs.

Egress filtering is opt-in: use `EgressPolicy` for untrusted or dynamic
destinations. An unconfigured client performs caller-directed HTTP without
address-class restrictions. The configured policy validates
scheme, authority, port, origin, address class, CIDRs, redirects, proxies, and
every resolved DNS answer at connection time. Metadata, private, loopback,
link-local, multicast, Unix-socket, userinfo, and wildcard behavior require
explicit treatment. Use `TLSPolicy` for custom roots, server identity, client
identity, or additive SPKI pinning without disabling certificate validation.

File-transfer destinations and resume partial paths are explicit trusted
application capabilities, never paths selected by response headers. Callers
must own the containing directories and prevent untrusted path selection or
concurrent symlink replacement. Same-directory staging and atomic publication
do not provide filesystem confinement against an attacker-controlled directory.

Do not log request or response bodies. Redact bounded error excerpts before
vendor mapping. Scope caches, cookies, OAuth tokens, coalescing, limiters,
breakers, transports, and metrics by the identity dimensions required by the
application. Never use tenant, credential, cursor, raw path, query, or vendor
message values as metric labels.

`make check` verifies module checksums, runs `govulncheck`, and enforces
`GO-SAFETY-1`: no production `unsafe`, cgo, or `go:linkname`. Report suspected
vulnerabilities privately according to the repository `SECURITY.md`.

The maintained threat model, policy matrix, findings, evidence, and release
findings are in [security assurance](security-assurance.md).

Choose the next guide in the [documentation index](README.md)
or return to the [repository entry](../README.md) for installation and ownership.
