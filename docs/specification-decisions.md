# HTTP client specification decisions

This register records observable choices made by `http-client` where HTTP and
related standards permit multiple behaviors or where the package deliberately
adds a stricter application policy. Normative sources control over examples,
peer defaults, and historical behavior. Immutable source versions and digests
are recorded in the [conformance matrix](../specification/manifest.tsv).

Statuses are `resolved`, `unresolved`, or `superseded`. Resolved decisions are
part of the compatibility contract. A changed interpretation requires HTTP,
security, resource, compatibility, executable-evidence, and changelog review.

## HTTPCLIENT-DEC-001: Base URL and reference resolution

- **Status, owner, and classification:** `resolved`; `http-client` maintainers;
  normative resolution with defensive same-origin policy.
- **Source and issue:** RFC 3986 [reference resolution](https://www.rfc-editor.org/rfc/rfc3986.html#section-5)
  and RFC 9110 [URI references](https://www.rfc-editor.org/rfc/rfc9110.html#section-4.1)
  and [origins](https://www.rfc-editor.org/rfc/rfc9110.html#section-4.3.1).
  Generic resolution permits authority changes, userinfo, fragments, and path
  traversal that an integration base URL must not silently trust.
- **Interpretations and peer behavior:** Accept every legal reference, reject
  all relatives, compare hosts only, or resolve normally then enforce canonical
  origin equality. `net/url.ResolveReference` performs generic RFC resolution
  and deliberately supplies no integration trust boundary.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Resolve by RFC 3986, then require
  HTTP(S), valid authority, no userinfo, and the same canonical scheme, host,
  and effective port. Fragments are not request-target data. This preserves
  standard path/query resolution while preventing origin escape and SSRF;
  multi-origin integrations need explicit clients or egress policy.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestNewRequestSpecResolvesSameOriginReference`,
  `TestNewRequestSpecRejectsUnsafeBaseOrReference`,
  `TestRequestURLValidationContracts`, and `FuzzRequestSpecURL` cover
  `NewRequestSpec` and `RequestSpec.Build`. Same-origin enforcement is
  package-owned, not an RFC 3986 requirement. Reconsider only for a versioned
  multi-origin API with an explicit trust and credential model.

Decision data: `{"id":"HTTPCLIENT-DEC-001","title":"Base URL and reference resolution","status":"resolved","owner":"http-client maintainers","classification":"interoperability policy","decision_scope":"defensive","specification":"RFC 3986 URI Generic Syntax","version":"RFC-3986","source_authority":"rfc3986","section":"5","requirement_strength":"not specified","issue":"Base URL and reference resolution exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Resolve generic references and then require a valid fragment-free HTTP or HTTPS URL at the same canonical origin.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestNewRequestSpecResolvesSameOriginReference","TestNewRequestSpecRejectsUnsafeBaseOrReference","TestRequestURLValidationContracts"],"fixture_evidence":[],"fuzz_evidence":["FuzzRequestSpecURL"],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["NewRequestSpec","RequestSpec.Build"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc3986.txt

## HTTPCLIENT-DEC-002: Header fields, trailers, and deterministic layering

- **Status, owner, and classification:** `resolved`; maintainers; normative
  field syntax plus deterministic application policy.
- **Source and issue:** RFC 9110 [fields](https://www.rfc-editor.org/rfc/rfc9110.html#section-5)
  and [trailers](https://www.rfc-editor.org/rfc/rfc9110.html#section-6.5).
  HTTP permits repeated field lines and field-specific combination while Go
  maps do not preserve order and request layers need explicit precedence.
- **Interpretations and peer behavior:** Replace all values, append all values,
  preserve incidental map order, or distinguish replace/append operations and
  reject prohibited trailers. `net/http.Header` preserves value slices but
  does not define package layering.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Validate names and values; use
  immutable explicit layer precedence; distinguish append from replace; sort
  package-owned canonical output while retaining repeated-value order; require
  a body for trailers and reject routing, framing, and credential fields. This
  prevents splitting and signing/cache differentials while retaining standard
  repeated fields within finite request bounds.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestRequestSpecBuildsLayeredIndependentTrailers`,
  `TestRequestSpecTrailersReachStandardHTTPServer`,
  `TestRequestSpecRejectsUnsafeOrBodylessTrailers`, and `FuzzHeaderValidation`
  cover request header/trailer APIs. Metadata precedence is package policy.
  Reconsider for a supported signature profile requiring another canonical form.

Decision data: `{"id":"HTTPCLIENT-DEC-002","title":"Header fields, trailers, and deterministic layering","status":"resolved","owner":"http-client maintainers","classification":"omission","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Header fields, trailers, and deterministic layering exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Validate fields, apply explicit immutable layer precedence, preserve repeated-value order, and reject prohibited trailers.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestRequestSpecBuildsLayeredIndependentTrailers","TestRequestSpecTrailersReachStandardHTTPServer","TestRequestSpecRejectsUnsafeOrBodylessTrailers"],"fixture_evidence":[],"fuzz_evidence":["FuzzHeaderValidation"],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["RequestSpec.WithHeader","RequestSpec.WithTrailer","RequestSpec.Build"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-003: Redirect methods, replay, and trust boundaries

- **Status, owner, and classification:** `resolved`; maintainers;
  transport-specific interoperability and defensive policy.
- **Source and issue:** RFC 9110 [redirection](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.4)
  and Go's `net/http.Client` redirect contract. Statuses differ on method
  preservation, bodies may not be replayable, and redirects can cross origin
  or downgrade transport.
- **Interpretations and peer behavior:** Never follow; delegate everything to
  `net/http`; preserve every method; or retain Go method mechanics while
  enforcing replay, egress, and credential trust independently. Clients differ
  on POST rewriting and sensitive-header forwarding.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Preserve documented Go 301/302/303
  and 307/308 mechanics, but enforce finite redirect count, replayability,
  egress, and session policy. Credentials, idempotency, cookies, trace state,
  and sensitive headers are reapplied or stripped per attempt and canonical
  origin; HTTPS downgrade is untrusted. Cross-origin leakage and unbounded
  replay are prevented; nonstandard POST or credential behavior is explicit.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin`,
  `TestSessionRedirectPolicyControlsCrossOriginCookies`,
  `TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity`, and
  `FuzzRedirectCredentialBoundary` cover `Client` and attempt middleware.
  Reconsider when Go changes redirect mechanics or a versioned delegation
  profile is introduced.

Decision data: `{"id":"HTTPCLIENT-DEC-003","title":"Redirect methods, replay, and trust boundaries","status":"resolved","owner":"http-client maintainers","classification":"interoperability policy","decision_scope":"transport-specific","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Redirect methods, replay, and trust boundaries exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Use Go redirect mechanics with finite replay, egress, session, identity, and per-attempt trust enforcement.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin","TestSessionRedirectPolicyControlsCrossOriginCookies","TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity"],"fixture_evidence":[],"fuzz_evidence":["FuzzRedirectCredentialBoundary"],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["Client.Do","SessionOptions","AuthenticationOptions"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-004: Basic, Bearer, and API-key request authentication

- **Status, owner, and classification:** `resolved`; maintainers; normative
  Basic/Bearer encoding plus defensive transport policy; API key is application
  policy.
- **Source and issue:** RFC 7617 [Basic](https://www.rfc-editor.org/rfc/rfc7617.html#section-2),
  RFC 6750 [Bearer](https://www.rfc-editor.org/rfc/rfc6750.html#section-2.1),
  and RFC 9110 [credentials](https://www.rfc-editor.org/rfc/rfc9110.html#section-11.4).
  These do not define reusable-client origin scope, and API keys have no
  universal HTTP scheme.
- **Interpretations and peer behavior:** Set credentials once, forward through
  redirects, permit cleartext, infer one API-key shape, or apply one explicit
  editor to each trusted HTTPS attempt. Generic clients commonly expose raw
  setters without origin enforcement; vendor API-key formats conflict.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Use standard Basic and Bearer forms;
  make API-key placement caller-named and explicitly nonstandard; require HTTPS
  except narrow local-test policy; reject conflicting state; apply only to
  trusted canonical origins; and redact bounded immutable credentials. This
  blocks origin/cleartext leakage while preserving standard wire forms.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestCredentialEditorsApplyBasicBearerAndAPIKeys`,
  `TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin`,
  `TestAuthenticationRejectsCleartextCredentialTransport`, and
  `FuzzAuthenticationInputs` cover credential editors and authentication
  middleware. Reconsider for a standardized supported vendor scheme.

Decision data: `{"id":"HTTPCLIENT-DEC-004","title":"Basic, Bearer, and API-key request authentication","status":"resolved","owner":"http-client maintainers","classification":"omission","decision_scope":"defensive","specification":"RFC 7617 Basic HTTP Authentication","version":"RFC-7617","source_authority":"rfc7617","section":"2","requirement_strength":"not specified","issue":"Basic, Bearer, and API-key request authentication exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Use standard Basic and Bearer wire forms and explicit API-key placement only on trusted HTTPS attempts.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestCredentialEditorsApplyBasicBearerAndAPIKeys","TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin","TestAuthenticationRejectsCleartextCredentialTransport"],"fixture_evidence":[],"fuzz_evidence":["FuzzAuthenticationInputs"],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["NewBasicAuth","NewBearerAuth","NewAPIKeyAuth","NewAuthenticationMiddleware"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc7617.txt

## HTTPCLIENT-DEC-005: OAuth 2.0 client credentials and token reuse

- **Status, owner, and classification:** `resolved`; maintainers; normative
  grant behavior plus concurrency and lifecycle policy.
- **Source and issue:** RFC 6749 [client authentication](https://www.rfc-editor.org/rfc/rfc6749.html#section-2.3),
  [token endpoint](https://www.rfc-editor.org/rfc/rfc6749.html#section-3.2), and
  [client credentials](https://www.rfc-editor.org/rfc/rfc6749.html#section-4.4).
  Request-body credentials are permitted only for clients unable to use Basic;
  caching, refresh coordination, cancellation, and recursion are unspecified.
- **Interpretations and peer behavior:** Basic only, parameter credentials by
  default, one refresh per waiter, recursive integration middleware, or
  explicit authentication with one bounded refresh. `x/oauth2` composes token
  sources but does not own this client's lifecycle and resource scopes.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Default to Basic; expose parameter
  authentication explicitly and never combine it with Basic; use hardened
  transport while bypassing integration cookies/retries/middleware; coordinate
  one refresh; copy tokens; validate with the injected clock; and honor caller
  and client cancellation. This prevents duplicate credentials, recursive
  retries, stampedes, and waiter leaks while preserving legacy opt-in.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestOAuth2AuthIntegratesReusableTokenSource`,
  `TestClientCredentialsTokenSourceCoordinatesRefreshAndBypassesMiddleware`,
  `TestClientCredentialsTokenSourceSupportsExplicitParameterAuthentication`,
  and `TestCachedTokenSourceCoordinatesCancelableWaiters` cover OAuth editors
  and token sources. Reconsider when OAuth BCP or `x/oauth2` contracts change.

Decision data: `{"id":"HTTPCLIENT-DEC-005","title":"OAuth 2.0 client credentials and token reuse","status":"resolved","owner":"http-client maintainers","classification":"optional behavior","decision_scope":"defensive","specification":"RFC 6749 OAuth 2.0 Authorization Framework","version":"RFC-6749","source_authority":"rfc6749","section":"2.3, 3.2, and 4.4","requirement_strength":"not specified","issue":"OAuth 2.0 client credentials and token reuse exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Default to Basic client authentication, expose parameter authentication explicitly, and coordinate one bounded cancellable token refresh.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestClientCredentialsTokenSourceCoordinatesRefreshAndBypassesMiddleware","TestClientCredentialsTokenSourceSupportsExplicitParameterAuthentication","TestCachedTokenSourceCoordinatesCancelableWaiters"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["ClientCredentialsOptions","NewClientCredentialsTokenSource","NewOAuth2Auth"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc6749.txt

## HTTPCLIENT-DEC-006: Retry eligibility and ambiguous delivery

- **Status, owner, and classification:** `resolved`; maintainers; defensive
  policy derived from RFC 9110 [safe](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.1)
  and [idempotent](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2)
  method semantics.
- **Source and issue:** RFC 8470 [425 Too Early](https://www.rfc-editor.org/rfc/rfc8470.html#section-5.2)
  and RFC 6585 [429 Too Many Requests](https://www.rfc-editor.org/rfc/rfc6585.html#section-4)
  supplement RFC 9110 status semantics. A transport failure does not reveal
  whether a server received an operation. Idempotent semantics reduce
  duplicate-effect risk but do not make a body replayable; a key alone does
  not prove server deduplication.
- **Interpretations and peer behavior:** Retry every transient failure, GET
  only, all idempotent methods, or require method eligibility, replayable
  content, finite policy, and stronger proof for unsafe operations. SDK status
  lists differ and often overgeneralize key presence.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Retry is opt-in. Defaults permit
  replayable GET, HEAD, OPTIONS, TRACE, PUT, and DELETE for bounded transient
  transport failures and statuses 408, 425, 429, 500, 502, 503, and 504.
  Unsafe methods also require endpoint opt-in, applied idempotency policy, and
  replayability. Attempts, elapsed time, delay, and drains are bounded and
  cancellation-aware, preventing hidden side effects and amplification.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestRetryReplaysSafeRequestsWithDeterministicBackoff`,
  `TestRetryRequiresReplayableBodyAndExplicitUnsafeOptIn`,
  `TestRetryHonorsCancellationAndReportsSecretSafeExhaustion`, and
  `FuzzRetryPolicy` cover retry options and errors. HTTP defines semantics, not
  one retry algorithm. Reconsider for a stronger standardized endpoint profile.

Decision data: `{"id":"HTTPCLIENT-DEC-006","title":"Retry eligibility and ambiguous delivery","status":"resolved","owner":"http-client maintainers","classification":"interoperability policy","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Retry eligibility and ambiguous delivery exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"github.com/hashicorp/go-retryablehttp v0.7.8 retries a replayable POST after HTTP 503 by default, while this package requires explicit unsafe-operation and idempotency policy.","selected_behavior":"Retry replayable safe or idempotent methods under finite policy; unsafe methods also require explicit policy and package-applied idempotency.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestRetryReplaysSafeRequestsWithDeterministicBackoff","TestRetryRequiresReplayableBodyAndExplicitUnsafeOptIn"],"fixture_evidence":[],"fuzz_evidence":["FuzzRetryPolicy"],"interoperability_evidence":[],"differential_evidence":["specification/differential.tsv","specification/retry-peer.sum","scripts/check-retry-peer.sh"],"public_apis":["RetryOptions","NewRetryMiddleware","RetryExhaustedError"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-007: Retry-After and vendor rate-limit fields

- **Status, owner, and classification:** `resolved`; maintainers; normative
  field parsing plus bounded delay and vendor-extension policy.
- **Source and issue:** RFC 9110 [`Retry-After`](https://www.rfc-editor.org/rfc/rfc9110.html#section-10.2.3)
  allows HTTP date or delay seconds but does not define malformed fallback,
  clock skew, caps, or proprietary remaining/reset precedence.
- **Interpretations and peer behavior:** Ignore hints, trust without bounds,
  fail malformed input, or parse standard values first and clamp them while
  keeping vendor fields opt-in. Clients disagree on past dates and precedence;
  no universal vendor header contract exists.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Parse delay seconds or HTTP date;
  treat past dates as zero; fall back on malformed/overflowing values; give
  standard `Retry-After` precedence; require explicit vendor field config; and
  clamp every wait to policy and deadline. This prevents overflow and unbounded
  server-directed sleeps while preserving cancellation.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestRetryUsesBoundedRetryAfterAndClosesDiscardedResponses`,
  `TestRetryDelayExactBoundaries`, `TestRateLimitObservationExactBoundaries`,
  and `TestRateLimitHeaderObservationMatrix` cover retry/rate observation.
  Reconsider when a stable adopted rate-limit field standard replaces profiles.

Decision data: `{"id":"HTTPCLIENT-DEC-007","title":"Retry-After and vendor rate-limit fields","status":"resolved","owner":"http-client maintainers","classification":"omission","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Retry-After and vendor rate-limit fields exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Parse Retry-After delay seconds or HTTP dates, handle malformed and past values deterministically, and clamp all waits.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestRetryUsesBoundedRetryAfterAndClosesDiscardedResponses","TestRetryDelayExactBoundaries","TestRateLimitObservationExactBoundaries","TestRateLimitHeaderObservationMatrix"],"fixture_evidence":[],"fuzz_evidence":["FuzzRetryPolicy"],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["RetryOptions","RateLimitOptions"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-008: HTTP cache storage, variants, and stale responses

- **Status, owner, and classification:** `resolved`; maintainers; RFC 9111
  behavior with stricter credential and resource policy.
- **Source and issue:** RFC 9111 [storage](https://www.rfc-editor.org/rfc/rfc9111.html#section-3),
  [freshness](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.2),
  [validation](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.3),
  [`Vary`](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.1), and
  [invalidation](https://www.rfc-editor.org/rfc/rfc9111.html#section-4.4).
  A library must still choose mode, identity, limits, and refresh ownership.
- **Interpretations and peer behavior:** Cache GET by URL, disable authenticated
  caching, emulate a shared cache only, or expose explicit mode with secure
  variant identity. Client caches often handle Authorization and `Vary`
  inconsistently; browser behavior is unsafe for multi-tenant services.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Caching is opt-in. Store only complete
  bounded responses admitted by method, status, and directives. Authorization
  reuse is explicit and credential-isolated; sensitive `Vary` values become
  digests; revalidation merges permitted metadata only; stale directives,
  only-if-cached, unsafe invalidation, and refresh scheduling remain explicit.
  This prevents cross-tenant disclosure, partial poisoning, unbounded storage,
  and hidden goroutines while preserving RFC directives and validators.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestSharedCacheDoesNotReuseAuthorizationWithoutExplicitPermission`,
  `TestCacheMiddlewareHonorsOnlyIfCachedMaxStaleAndStaleIfError`,
  `TestCacheVaryMatchingNormalizesEquivalentHeaderLineLayout`, and
  `TestCacheValidationDoesNotReplaceRepresentationDependentHeaders` cover
  cache middleware/store APIs. Reconsider when relied-on RFC semantics change.

Decision data: `{"id":"HTTPCLIENT-DEC-008","title":"HTTP cache storage, variants, and stale responses","status":"resolved","owner":"http-client maintainers","classification":"optional behavior","decision_scope":"defensive","specification":"RFC 9111 HTTP Caching","version":"RFC-9111","source_authority":"rfc9111","section":"3 through 5","requirement_strength":"not specified","issue":"HTTP cache storage, variants, and stale responses exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Cache only complete bounded admitted responses with credential isolation, secure variants, constrained revalidation, and explicit stale policy.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestSharedCacheDoesNotReuseAuthorizationWithoutExplicitPermission","TestCacheMiddlewareHonorsOnlyIfCachedMaxStaleAndStaleIfError","TestCacheVaryMatchingNormalizesEquivalentHeaderLineLayout","TestCacheValidationDoesNotReplaceRepresentationDependentHeaders"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["CacheOptions","NewCacheMiddleware","MemoryCache"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9111.txt

## HTTPCLIENT-DEC-009: Content coding and decompression ownership

- **Status, owner, and classification:** `resolved`; maintainers; normative
  coding semantics plus defensive resource policy.
- **Source and issue:** RFC 9110 [`Content-Encoding`](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.4)
  and [`Accept-Encoding`](https://www.rfc-editor.org/rfc/rfc9110.html#section-12.5.3).
  Go can transparently request/decode gzip, hiding wire metadata and size;
  coding lists also require explicit nesting and ownership policy.
- **Interpretations and peer behavior:** Keep transparent transport behavior,
  decode every known list, expose compressed bytes only, or disable implicit
  behavior and require bounded middleware. `net/http.Transport` automatically
  negotiates gzip under its defaults; clients report decoded metadata differently.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Owned transports disable implicit
  compression. Explicit middleware negotiates supported coding, updates
  metadata deterministically, and bounds output and compression ratio. Request
  compression preserves replay safety. Unsupported, malformed, nested beyond
  policy, truncated, or over-budget content fails closed and closes streams.
  This prevents bombs, hidden buffering, worker leaks, and unsafe replay.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestDefaultTransportDisablesImplicitCompression`,
  `TestCompressionMiddlewareDecodesGzipWithExplicitMetadata`,
  `TestCompressionMiddlewareEnforcesAbsoluteAndRatioBounds`, and
  `TestCompressionWorkerStopsWhenAttemptMiddlewareShortCircuits` cover
  compression APIs. Reconsider additional coding only with equivalent proofs.

Decision data: `{"id":"HTTPCLIENT-DEC-009","title":"Content coding and decompression ownership","status":"resolved","owner":"http-client maintainers","classification":"optional behavior","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Content coding and decompression ownership exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Disable implicit transport compression and use explicit middleware with deterministic metadata and bounded decoded size and ratio.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestDefaultTransportDisablesImplicitCompression","TestCompressionMiddlewareDecodesGzipWithExplicitMetadata","TestCompressionMiddlewareEnforcesAbsoluteAndRatioBounds","TestCompressionWorkerStopsWhenAttemptMiddlewareShortCircuits"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["CompressionOptions","NewCompressionMiddleware"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-010: Byte ranges and resumable downloads

- **Status, owner, and classification:** `resolved`; maintainers; normative
  range validation with explicit resume policy.
- **Source and issue:** RFC 9110 [ranges](https://www.rfc-editor.org/rfc/rfc9110.html#section-14),
  [`If-Range`](https://www.rfc-editor.org/rfc/rfc9110.html#section-13.1.5),
  [206](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.3.7), and
  [416](https://www.rfc-editor.org/rfc/rfc9110.html#section-15.5.17). A resume
  can receive 206, 200, or 416 with absent, weak, changed, or inconsistent data.
- **Interpretations and peer behavior:** Append every 206, fail any 200,
  restart on 200, accept weak validators, or validate exact range and strong
  identity before continuation. Download helpers differ on restart UX.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Apply only to eligible safe requests;
  use exact offsets and strong validators when supplied; continue matching 206;
  restart on policy-allowed 200; accept consistent completion 416; reject
  mismatched unit, offset, total, validator, or length before replacement.
  This prevents mixed files, gaps, overlap, overflow, and partial destination
  corruption while preserving standards-conforming servers.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestWithRangeClonesRequestAndAppliesStrongValidator`,
  `TestValidateRangeResponseContinuesRestartsOrCompletes`,
  `TestRangePolicyRejectsUnsafeRequestsAndMismatchedResponses`, and
  `TestRangePolicyParserAndValidatorBoundaries` cover range/transfer APIs.
  Reconsider for multipart ranges or a versioned weak-validator profile.

Decision data: `{"id":"HTTPCLIENT-DEC-010","title":"Byte ranges and resumable downloads","status":"resolved","owner":"http-client maintainers","classification":"ambiguity","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Byte ranges and resumable downloads exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Continue only exact matching ranges, restart only when allowed, accept consistent completion, and reject mismatched representation metadata.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestWithRangeClonesRequestAndAppliesStrongValidator","TestValidateRangeResponseContinuesRestartsOrCompletes","TestRangePolicyRejectsUnsafeRequestsAndMismatchedResponses","TestRangePolicyParserAndValidatorBoundaries"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["WithRange","ValidateRangeResponse","ResumeDownloadToFile"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-011: Link-header pagination

- **Status, owner, and classification:** `resolved`; maintainers; normative
  parsing plus ambiguity and trust policy.
- **Source and issue:** RFC 8288 [HTTP serialization](https://www.rfc-editor.org/rfc/rfc8288.html#section-3)
  and [relation types](https://www.rfc-editor.org/rfc/rfc8288.html#section-2.1).
  Links allow quoted commas, relative targets, repeated parameters, and more
  than one `next`; first-match selection can change traversal.
- **Interpretations and peer behavior:** Split commas, choose first/last next,
  reject duplicates, or parse RFC syntax and reject competing targets. Many
  clients use simplified comma splitting that fails quoted values.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Parse without splitting quoted
  commas, recognize relation tokens, resolve relative targets against the
  response URL, require a syntactically valid absolute HTTP(S) destination,
  reject malformed or competing next targets, and keep continuation opaque.
  The caller-provided fetcher owns origin allowlisting and egress enforcement;
  the generic paginator does not silently impose same-origin semantics. Bounds
  on fields, links, parameters, and continuation prevent parser and resource
  abuse without misrepresenting URL syntax validation as a trust decision.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestLinkPaginatorParsesAndResolvesRFCLinks`,
  `TestParseNextLinkHandlesQuotedCommasAndRejectsAmbiguity`, and
  `TestLinkParserBoundaryMatrix` cover Link pagination. RFC 8288 does not pick
  among multiple next links. Reconsider for a safe named vendor precedence.

Decision data: `{"id":"HTTPCLIENT-DEC-011","title":"Link-header pagination","status":"resolved","owner":"http-client maintainers","classification":"ambiguity","decision_scope":"defensive","specification":"RFC 8288 Web Linking","version":"RFC-8288","source_authority":"rfc8288","section":"2 and 3","requirement_strength":"not specified","issue":"Link-header pagination exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Parse quoted Link syntax, resolve validated HTTP or HTTPS targets, reject competing next targets, and preserve continuation opaquely.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestLinkPaginatorParsesAndResolvesRFCLinks","TestCursorPaginatorPreservesOpaqueCursorExactly"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["NewLinkPaginator","NewCursorPaginator"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc8288.txt

## HTTPCLIENT-DEC-012: Cookie jars and redirect scope

- **Status, owner, and classification:** `resolved`; maintainers; standards
  delegation plus stricter service-client isolation policy.
- **Source and issue:** RFC 6265 [user-agent requirements](https://www.rfc-editor.org/rfc/rfc6265.html#section-5)
  and [security](https://www.rfc-editor.org/rfc/rfc6265.html#section-8).
  Cookie domain/path scope can exceed an integration origin; ambient jars also
  create hidden mutable state and lifecycle ambiguity.
- **Interpretations and peer behavior:** Global jar, RFC matching only, no
  cookies, or per-client opt-in with independent redirect/persistence policy.
  Browser clients enable cookies automatically; Go has no jar unless supplied.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Disable cookies by default. Opt-in
  sessions own or borrow an isolated public-suffix-aware jar. Same-origin
  redirect scope is default; broader scope is explicit. Persistence is bounded,
  versioned, caller-controlled, and tied to client lifecycle. This prevents
  tenant crossover, public-suffix cookies, unbounded persistence, and secret
  leakage while retaining standard jar matching.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestSessionCookieJarsAreOptInAndIsolatedPerClient`,
  `TestSessionRedirectPolicyControlsCrossOriginCookies`,
  `TestSessionPersistenceLoadSaveAndJarOwnership`, and
  `TestSessionRejectsInvalidConfiguration` cover session APIs. Reconsider when
  adopted RFC 6265bis semantics change or a versioned SSO profile is added.

Decision data: `{"id":"HTTPCLIENT-DEC-012","title":"Cookie jars and redirect scope","status":"resolved","owner":"http-client maintainers","classification":"optional behavior","decision_scope":"defensive","specification":"RFC 6265 HTTP State Management Mechanism","version":"RFC-6265","source_authority":"rfc6265","section":"5 and 8","requirement_strength":"not specified","issue":"Cookie jars and redirect scope exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Disable cookies by default and give each enabled session an isolated jar with explicit redirect and persistence policy.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestSessionCookieJarsAreOptInAndIsolatedPerClient","TestSessionRedirectPolicyControlsCrossOriginCookies"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["SessionOptions","New"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc6265.txt

## HTTPCLIENT-DEC-013: W3C Trace Context propagation

- **Status, owner, and classification:** `resolved`; maintainers; normative
  syntax with stricter trust and cardinality policy.
- **Source and issue:** W3C Trace Context Level 1 Recommendation, 23 November
  2021, [`traceparent`](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/#traceparent-header)
  and [`tracestate`](https://www.w3.org/TR/2021/REC-trace-context-1-20211123/#tracestate-header).
  The standard defines syntax but application boundaries may require stripping;
  baggage is separate and can carry secrets or high-cardinality values.
- **Interpretations and peer behavior:** Forward unchanged, regenerate every
  attempt, same-origin only, or validate v00 and apply trust/baggage policy.
  OpenTelemetry propagators handle syntax but leave trust boundaries to apps.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Validate v00 IDs, flags, and bounded
  tracestate; preserve logical-operation trace relationship with explicit
  physical attempts; strip state on untrusted redirects; and forward only
  allowlisted baggage. This prevents metadata leakage, malformed propagation,
  and uncontrolled cardinality while interoperating with valid v00 peers.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestW3CTraceContextIsValidatedAndInjectedOnTrustedAttempts`,
  `TestW3CTraceContextPrimitiveBoundaries`, and telemetry trust tests cover
  observability APIs. Reconsider only when a newer W3C level is explicitly adopted.

Decision data: `{"id":"HTTPCLIENT-DEC-013","title":"W3C Trace Context propagation","status":"resolved","owner":"http-client maintainers","classification":"interoperability policy","decision_scope":"defensive","specification":"W3C Trace Context Level 1 Recommendation 2021-11-23","version":"W3C-REC-20211123","source_authority":"w3c-trace-context-1","section":"3","requirement_strength":"not specified","issue":"W3C Trace Context propagation exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Validate trace context, inject it only on trusted attempts, and apply bounded baggage filtering.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestW3CTraceContextIsValidatedAndInjectedOnTrustedAttempts","TestW3CTraceContextPrimitiveBoundaries"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["TelemetryOptions","TelemetryPropagator","W3CTraceContext"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.w3.org/TR/2021/REC-trace-context-1-20211123/

## HTTPCLIENT-DEC-014: Response media types, empty bodies, and JSON documents

- **Status, owner, and classification:** `resolved`; maintainers; normative
  message semantics plus strict decoding policy.
- **Source and issue:** RFC 9110 [`Content-Type`](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.3),
  [`Content-Length`](https://www.rfc-editor.org/rfc/rfc9110.html#section-8.6),
  status semantics, and RFC 8259 [JSON grammar](https://www.rfc-editor.org/rfc/rfc8259.html#section-2).
  Statuses differ on content; missing type is not JSON; lengths can be wrong;
  and one decoder call does not reject trailing documents.
- **Interpretations and peer behavior:** Sniff JSON, ignore media type, accept
  trailing values, trust length, or validate status/type/actual bounded bytes
  and exactly one document. SDKs differ on missing types and empty success.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Classify status independently;
  handle HEAD, 1xx, 204, 205, and 304 through explicit empty policy; require an
  accepted media type; validate meaningful declared lengths; decode one bounded
  complete document; reject trailing non-whitespace by default; and always
  close the body. This prevents confusion, oversized work, hidden documents,
  and leaks, with bounded redacted errors.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestDecodeJSONResponseStreamsBoundedStrictDocumentAndCloses`,
  `TestDecodeJSONResponseRejectsMediaTypeLimitAndTrailingData`,
  `TestDecodeJSONResponseEmptyAndMalformedSemantics`, and
  `TestResponseDecodersRejectDeclaredLengthMismatch` cover response decoders.
  Reconsider multi-document JSON only through a separate versioned API.

Decision data: `{"id":"HTTPCLIENT-DEC-014","title":"Response media types, empty bodies, and JSON documents","status":"resolved","owner":"http-client maintainers","classification":"ambiguity","decision_scope":"defensive","specification":"RFC 8259 JSON","version":"RFC-8259","source_authority":"rfc8259","section":"2 through 8","requirement_strength":"not specified","issue":"Response media types, empty bodies, and JSON documents exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Require an allowed JSON media type, one bounded document, explicit empty-body policy, no trailing document, and deterministic closure.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestDecodeJSONResponseStreamsBoundedStrictDocumentAndCloses","TestDecodeJSONResponseRejectsMediaTypeLimitAndTrailingData","TestDecodeJSONResponseEmptyAndMalformedSemantics"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["DecodeJSONResponse","JSONResponseOptions"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc8259.txt

## HTTPCLIENT-DEC-015: Application idempotency keys

- **Status, owner, and classification:** `resolved`; maintainers;
  package-defined application protocol and retry-safety policy.
- **Source and issue:** RFC 9110 [idempotent methods](https://www.rfc-editor.org/rfc/rfc9110.html#section-9.2.2).
  Base HTTP does not make unsafe requests idempotent merely because they carry
  a key; vendors differ on names, scope, retention, and response replay.
- **Interpretations and peer behavior:** Global key, infer retry safety from
  presence, regenerate per attempt, or declare endpoint contract and retain one
  key per logical operation. SDKs differ and vendor behavior is not universal.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Idempotency is explicit per endpoint
  with configurable header. Generated keys use 128 bits of cryptographic
  entropy. One validated redacted key remains stable across trusted eligible
  attempts but does not enable unsafe retry without endpoint opt-in and
  replayability. This avoids duplicate identities without overstating servers.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestIdempotencyKeyIsStableAcrossRetriesAndDistinctAcrossOperations`,
  `TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity`,
  `TestDefaultIdempotencyGeneratorUsesSecureIndependentKeysAndCustomHeader`,
  and `TestIdempotencyRejectsInvalidPolicyAndCallerKeys` cover idempotency APIs.
  Reconsider if a final adopted standard is deliberately supported.

Decision data: `{"id":"HTTPCLIENT-DEC-015","title":"Application idempotency keys","status":"resolved","owner":"http-client maintainers","classification":"omission","decision_scope":"application-policy","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Application idempotency keys exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Bind one provenance-tagged bounded key to an explicit logical operation and never treat caller header presence alone as retry proof.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestIdempotencyKeyIsStableAcrossRetriesAndDistinctAcrossOperations","TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity","TestCallerContextIdempotencyKeyHasExplicitProvenance"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["IdempotencyOptions","NewIdempotencyMiddleware","WithIdempotencyKey"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-016: Pagination continuation and finite traversal

- **Status, owner, and classification:** `resolved`; maintainers; application
  policy and resource-safety profile. RFC 8288 applies only to Link pagination;
  pages, offsets, cursors, totals, and custom continuations have no universal
  HTTP standard.
- **Source and issue:** RFC 8288 [Web Linking](https://www.rfc-editor.org/rfc/rfc8288.html#section-3)
  governs Link pagination only. APIs otherwise differ on empty-token
  termination, opaque cursor handling, totals, repeated continuations, and
  unstable data. An unbounded generic iterator can loop forever.
- **Interpretations and peer behavior:** Normalize cursors, stop on empty page,
  trust totals, follow repeats, or preserve each strategy while applying common
  bounds and cycles. Vendor clients commonly have implicit unbounded loops.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Keep strategies distinct; preserve
  cursor bytes opaquely; remain lazy, sequential by default, cancellable, and
  bounded by page/item/byte/duration/empty-page/continuation limits; reject
  repeated continuation state; and copy explicit resume state. This prevents
  loops, overflow, memory growth, token disclosure, and unsafe concurrency.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestCursorPaginatorPreservesOpaqueCursorExactly`,
  `TestPaginationStrategiesRejectInvalidConfigurationAndOverflow`,
  `TestPaginationValidationAndTransitionBoundaries`, and Link tests cover all
  paginator APIs. Reconsider for an adopted standard or separate bounded
  independent-page concurrency API.

Decision data: `{"id":"HTTPCLIENT-DEC-016","title":"Pagination continuation and finite traversal","status":"resolved","owner":"http-client maintainers","classification":"omission","decision_scope":"application-policy","specification":"RFC 8288 Web Linking","version":"RFC-8288","source_authority":"rfc8288","section":"2 and 3","requirement_strength":"not specified","issue":"Pagination continuation and finite traversal exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Preserve opaque continuation, reject cycles, and enforce finite page, item, byte, elapsed-time, and empty-page policy.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestCursorPaginatorPreservesOpaqueCursorExactly","TestLinkPaginatorParsesAndResolvesRFCLinks"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["NewCursorPaginator","NewLinkPaginator","PaginationBounds"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc8288.txt

## HTTPCLIENT-DEC-017: Operation and attempt middleware lifecycle

- **Status, owner, and classification:** `resolved`; maintainers;
  package-defined execution and ownership policy layered on Go's
  `http.RoundTripper` contract.
- **Source and issue:** RFC 9110 [message abstraction](https://www.rfc-editor.org/rfc/rfc9110.html#section-6)
  defines HTTP messages, not application middleware ordering. Retries,
  redirects, cache revalidation, and authentication can create multiple
  physical exchanges for one logical call, so an undifferentiated middleware
  chain can duplicate side effects or miss later attempts.
- **Interpretations and peer behavior:** Run every middleware once, run every
  middleware for every exchange, use registration order, or distinguish
  logical-operation and physical-attempt scopes with deterministic ordering.
  Framework clients expose incompatible middleware/interceptor lifecycles.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  A logical operation runs operation
  middleware once; every physical transport exchange runs attempt middleware.
  Stage, scope, priority, registration layer, and stable name determine order,
  not map or registration accident. Short circuits skip only downstream work;
  after/completion stages unwind deterministically; superseded or invalid
  responses are closed; panic and cancellation remain typed lifecycle events.
  This prevents repeated operation side effects and ensures credentials,
  tracing, limits, and policy apply to every actual attempt.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestPipelineResolvesLayerOverridesAndDeterministicOrder`,
  `TestPipelineExecutesNestedLifecycleInDeterministicOrder`,
  `TestPipelineRunsAttemptScopeForEveryAttempt`, and
  `TestClientRunsOperationOnceAndAttemptMiddlewareForRedirects` cover
  middleware constructors, information, resolution, and `Client.Do`. Reconsider
  only through a compatibility-reviewed pipeline version.

Decision data: `{"id":"HTTPCLIENT-DEC-017","title":"Operation and attempt middleware lifecycle","status":"resolved","owner":"http-client maintainers","classification":"implementation-defined behavior","decision_scope":"application-policy","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Operation and attempt middleware lifecycle exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Run operation scope once and attempt scope for each retry or redirect under deterministic layering, panic containment, and response ownership.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestPipelineExecutesNestedLifecycleInDeterministicOrder","TestPipelineRunsAttemptScopeForEveryAttempt","TestPipelineResponseReplacementClosesSupersededBody","TestPipelineContainsMiddlewareAndTransportPanics"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["Middleware","MiddlewareOptions","Pipeline","Client.Do"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## HTTPCLIENT-DEC-018: Request body replay and response body ownership

- **Status, owner, and classification:** `resolved`; maintainers; Go transport
  contract plus explicit package resource-ownership policy.
- **Source and issue:** RFC 9110 [content](https://www.rfc-editor.org/rfc/rfc9110.html#section-6.4)
  defines representation semantics, while Go `net/http.Request.GetBody` and
  `http.Response.Body` define runtime mechanics. Automatic buffering would
  change streaming and memory behavior; unclear closure rules leak connections
  or double-close caller resources.
- **Interpretations and peer behavior:** Buffer every request, infer replay from
  seekability, treat all streams as one-shot, close every response internally,
  or make replay and ownership explicit at each API boundary. Client wrappers
  differ on whether decoders, classifiers, retries, and callers own closure.
- **Selected behavior, security and resource consequences, compatibility and wire consequences:**
  Immutable byte/form/factory bodies
  explicitly support independent replay; streaming bodies are explicitly
  one-shot. The package never upgrades one-shot content through hidden
  buffering. A successful raw `Do` response is caller-owned; package decoders,
  classifiers, retries, replacements, caches, and transfer helpers close every
  response they consume or discard. Constructor and middleware failures close
  newly owned bodies, and close errors remain observable where contractually
  useful. This keeps memory finite, enables connection reuse, and prevents both
  leaks and accidental replay.
- **Evidence, public surface, upstream, and reconsideration:**
  `TestRequestSpecBuildsIndependentReplayableByteBodies`,
  `TestRequestSpecStreamingBodyIsExplicitlyOneShot`,
  `TestClassifyResponseLeavesAcceptedBodyCallerOwned`,
  `TestPipelineResponseReplacementClosesSupersededBody`, and
  `TestPipelineClosesResponsesReturnedWithErrors` cover request body types,
  `RequestSpec`, `Client.Do`, middleware, classifiers, and decoders. Reconsider
  only for a distinct bounded-spooling API with explicit storage ownership.

Decision data: `{"id":"HTTPCLIENT-DEC-018","title":"Request body replay and response body ownership","status":"resolved","owner":"http-client maintainers","classification":"implementation-defined behavior","decision_scope":"defensive","specification":"RFC 9110 HTTP Semantics","version":"RFC-9110","source_authority":"rfc9110","section":"4 through 15","requirement_strength":"not specified","issue":"Request body replay and response body ownership exposes observable policy that the cited specification does not completely select for a reusable integration client.","interpretations":["Delegate every choice to the underlying Go implementation","Apply explicit package policy at the owned boundary"],"peer_behavior":"Maintained implementations expose materially different policy defaults for this boundary.","selected_behavior":"Replay only explicit replayable bodies, keep streams one-shot, leave accepted bodies caller-owned, and close internally discarded bodies.","rationale":"The selected behavior makes compatibility, trust, lifecycle, and failure semantics explicit.","security_consequences":"Untrusted protocol input is handled only within the documented validation and trust policy.","resource_consequences":"Processing, retained state, waits, and stream ownership remain within documented finite bounds.","compatibility_consequences":"Changing this selection requires specification-decision and compatibility review.","wire_consequences":"Wire-visible behavior follows the selected policy and cited executable evidence.","executable_evidence":["TestRequestSpecBuildsIndependentReplayableByteBodies","TestRequestSpecStreamingBodyIsExplicitlyOneShot","TestClassifyResponseLeavesAcceptedBodyCallerOwned","TestPipelineResponseReplacementClosesSupersededBody","TestPipelineClosesResponsesReturnedWithErrors"],"fixture_evidence":[],"fuzz_evidence":[],"interoperability_evidence":[],"differential_evidence":[],"public_apis":["RequestBody","RequestSpec.WithBody","Client.Do","ClassifyResponse"],"documentation":["docs/specification-decisions.md"],"upstream_status":"No unresolved upstream erratum is known for this selected package policy.","reconsider_when":"A pinned source, monitored erratum, maintained peer, or supported Go contract changes this boundary."}`
Authoritative URL: https://www.rfc-editor.org/rfc/rfc9110.txt

## Unresolved decisions

No known material interpretation for the currently supported HTTP/1.1 and
HTTP/2 policy surfaces is unresolved at this revision. HTTP/3 is outside v1
scope and is not silently resolved; adoption requires decisions for fallback,
0-RTT replay, ownership, telemetry, and interoperability. New standards,
errata, or peer disagreements remain unresolved until they receive a stable
identifier, source analysis, executable evidence, and maintainer disposition.
