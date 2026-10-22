# Compatibility Policy

The root v3 release line uses `github.com/faustbrian/go-http-client/v3`.
Select it with `go get github.com/faustbrian/go-http-client/v3@v3.0.0`.
Earlier majors remain available at their original module and import paths.
Retry scheduling types and terminal errors from
`github.com/faustbrian/go-retry/v3` are part of this module's public contract,
so a new go-retry major requires a new major here. Root releases
use `v<version>` tags from main. The module requires Go 1.27.0, as declared in
`go.mod`; a future increase to that minimum is a compatibility change announced
in the changelog.

Before `v1`, minor releases MAY contain reviewed breaking changes, but every
break MUST be documented with migration guidance. Patch releases MUST remain
backward compatible. At and after `v1`, incompatible exported API or documented
behavior changes require a new major version.

Compatibility includes exported Go APIs, error classification, serialization,
protocol behavior, persistence schemas, environment variables, command output,
resource ownership, ordering, retry/idempotency semantics, and documented
defaults. A compile-compatible change can still be behaviorally breaking.

Specification-backed modules MUST NOT diverge from their declared standards.
Ambiguities require [documented decisions](docs/specification-decisions.md)
and stable tests. API replacement follows [`DEPRECATION.md`](DEPRECATION.md).

The module has no public subpackages or separately released adapters. Its sole
public package is `httpclient`; the current API areas and ownership boundaries
are listed in the [API reference](docs/api-reference.md).
