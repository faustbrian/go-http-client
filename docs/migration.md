# Migration Guide

## Root v2 import identity

For root `v2.0.0`, select
`github.com/faustbrian/go-http-client/v2@v2.0.0` and change application imports
to `github.com/faustbrian/go-http-client/v2`. There are no independent nested
modules or version-specific source directories.

Root v1 and v2 exported types have distinct Go identities. Migrate each
composed request/client boundary together. A published wrapper that exports
`RequestSpec` or `LayerOptions`, including Localized v4, cannot substitute v2
types in a compatible patch; it needs an explicit compatible route or a
separately authorized major migration. Maintained HTTP and Service integration
fixtures adopt the actual public v2 module only after publication. Existing v1
consumers do not acquire v2 credential bounds by retaining their old imports.

## Finite credential admission

Built-in authentication now rejects oversized ordinary-valid credentials and
returned OAuth token fields instead of scanning, encoding or caching them
without a credential policy. This intentionally narrows previously accepted
inputs and requires a new major release from main, with the corresponding Go
major module/import suffix; do not treat it as a patch release.

Existing constructor function signatures are unchanged. Existing keyed options
literals keep working with finite defaults; update unkeyed literals for the new
`CredentialPolicy` field. Applications that need different supported budgets
must use the additive `WithPolicy` constructors or the option field, and handle
secret-safe admission errors. Zero means finite defaults, not unlimited. Review
both aggregate raw sizes and encoded expansion before selecting limits.

Use a consistent policy on a token source/cache and its editor if they should
accept the same credentials. A stricter editor can reject a token admitted by a
more generous cache, without changing the request. Opaque Extra metadata keeps
its previous shallow sharing semantics; it is not a deep independent copy or a
bounded metadata representation. Its trusted source/application owner must
bound acquisition and metadata, as described in the authentication cookbook.

## Adopting the module

1. Keep vendor DTOs and endpoint methods in the existing vendor package.
2. Replace ad hoc transports with one shared `httpclient.Client` and preserve a
   close path owned by that wrapper.
3. Move credentials into attempt-scoped authentication middleware.
   Credential origins now require HTTPS by default. Use `AllowInsecure` only
   for local test servers and migrate production HTTP endpoints before adding
   authentication.
4. Define endpoint retry and idempotency policy explicitly; remove nested retry
   loops from generated clients or outer application wrappers.
5. Classify status before bounded success decoding and make response ownership
   visible on every exit.
6. Add scope dimensions before enabling cache, cookies, OAuth refresh,
   coalescing, limiters, or breakers in multi-tenant code.
7. Prove the integration with strict scripted fixtures and `httptest.Server`.

## Upgrading releases

Read every changelog entry between versions, then review the compatibility
surfaces listed in `compatibility.md`. Re-run contract fixtures, redirect and
credential tests, retry/idempotency tests, race tests, and downstream compile
tests. Do not copy profile defaults into application code; inspect resolved
values and provenance instead.

Persisted fixture schemas never migrate implicitly. Register an explicit
`FixtureMigrator` for each understood source version, sanitize its output, and
rewrite it using the current recorder format. Expiry remains enforced after
migration.
