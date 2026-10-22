# Security Policy

## Supported versions

The latest stable v1, v2 and v3 releases and `main` are supported; v3 is
supported once published. The latest minor release of the current major
version receives security fixes. Older release lines may receive fixes at
maintainer discretion.

The v2 and v3 release lines include finite credential-admission safeguards.
They do not apply to consumers that still import published v1. Publishing a
new major does not withdraw earlier support promises above.

## Reporting a vulnerability

Do not open a public issue for a suspected vulnerability. Use GitHub private
vulnerability reporting for this repository. Include affected versions,
impact, reproduction steps, and any suggested mitigation. Avoid sending live
credentials, customer data, or production traffic captures.

The maintainer will acknowledge a report within seven days, coordinate
validation and remediation privately, and credit the reporter unless they
prefer anonymity. Disclosure timing is coordinated after a fix is available.

## Security contract

The module follows `GO-SAFETY-1`: production code contains no `unsafe`, cgo,
or `go:linkname`. Default clients use finite timeouts. Credentials are
same-origin by default, error and telemetry surfaces exclude payload secrets,
and persisted HTTP fixtures require sanitization. Run `make check` for module
integrity, vulnerability scanning, and the source audit.
