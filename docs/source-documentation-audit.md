# Source documentation audit

## Scope and initial contract plan

Tier A comments and directly affected maintained prose only from `caddd5839a23926cdc651ec74e12f89f07566d2c`.
This repository worker owns the local batch; coordinator owns remote delivery/CI.
Reproduce inventory with `git ls-files "*.go"`, `git ls-files "*go.mod"`,
`golib source-docs check --json` and
`rg "^func (Test|Fuzz|Benchmark|Example)" --glob "*_test.go"`.

All 93 tracked Go files, one /v2 module, 38 implementation files,
55 assurance files and four source directories are in scope.
Initial structural gaps: 557, primarily public fields/interface methods.
Useful existing root/header/type contracts require semantic review, not filler.
No production build/platform variant or generated Go is present. One deprecated
observability method and existing security/analyzer annotations must be retained.

Trace URL/header/body construction, authentication/redirect origin boundaries,
DNS/proxy/TLS egress controls, retry/idempotency and policy composition, response
ownership/partial decoding/draining, cancellation/Close, concurrency scopes/pools,
cache/session/token limits, transfers/resume publication windows, fixture privacy
and pagination state before adding guarantees. Direct consumers are the root
client/middleware/helpers and three loopback-only example packages. No production
requests, runtime/dependency/fixture/authority changes or new campaigns are scoped.

Smallest native checks: Go1.27.2 formatting and all-file AST/token/literal/directive
identity, root render plus all applicable package docs, configured docs recipe,
affected examples only, offline source/local-document/specification structure,
local links, pinned changed-content/full-current/staged secrets and complete Tier A
diff review. API mode is apidiff (`api/baseline-v2.txt`), not full go-doc prose;
frozen API/specification decisions/history/input pins remain unchanged unless an
actually contradictory current consequence needs a separately native append.
Go runs wait for the coordinator's serialized slot with three owned caches.

## Complete tracked coverage

| Go file | Initial gaps | Review boundary |
| --- | --- | --- |
| `a_client_test.go` | 0 | TestNewClientProvidesFiniteSafeDefaults; TestRetryStopsAtMaximumAttempts; additional cases/helpers |
| `auth.go` | 14 | public contract and private invariant source |
| `auth_basic_test.go` | 0 | TestBasicAuthRejectsASCIIControls; TestBasicAuthPreservesValidCredentialWireValues |
| `auth_test.go` | 0 | TestCredentialEditorsApplyBasicBearerAndAPIKeys; TestCredentialEditorsRejectInvalidConfigurationWithoutRenderingSecrets; additional cases/helpers |
| `benchmark_test.go` | 0 | BenchmarkMiddlewarePipeline; BenchmarkRetryFirstAttempt |
| `body.go` | 5 | public contract and private invariant source |
| `cache.go` | 40 | public contract and private invariant source |
| `cache_mutation_test.go` | 0 | TestCacheExactConfigurationAndMemoryBoundaries; TestCacheExactPolicyAndProtocolBoundaries; additional cases/helpers |
| `cache_test.go` | 0 | TestCacheMiddlewareStoresFreshVariantsAndReturnsIndependentBodies; TestSharedCacheDoesNotReuseAuthorizationWithoutExplicitPermission; additional cases/helpers |
| `circuit_breaker.go` | 8 | public contract and private invariant source |
| `circuit_breaker_integration_test.go` | 0 | TestGoCircuitBreakerRecordsOneLogicalCompletionAcrossRetries; TestGoCircuitBreakerHalfOpenProbeOwnsBoundedRetries; additional cases/helpers |
| `circuit_breaker_test.go` | 0 | TestCircuitBreakerWrapsOneLogicalOperationAcrossRetries; TestRateLimitRejectionBypassesCircuitAdmission; additional cases/helpers |
| `client.go` | 3 | public contract and private invariant source |
| `client_test.go` | 0 | TestNewClientRejectsNegativeTimeout; TestNewClientAppliesAndValidatesConnectTimeout; additional cases/helpers |
| `compression.go` | 14 | public contract and private invariant source |
| `compression_test.go` | 0 | TestDefaultTransportDisablesImplicitCompression; TestCompressionMiddlewareDecodesGzipWithExplicitMetadata; additional cases/helpers |
| `credential_aggregate_contract_test.go` | 0 | TestClientCredentialsAggregateInputAdmission |
| `credential_policy.go` | 3 | public contract and private invariant source |
| `credential_policy_test.go` | 0 | TestCredentialAdmissionBasicBoundary; TestCredentialAdmissionStaticEncodedBoundaries; additional cases/helpers |
| `egress.go` | 22 | public contract and private invariant source |
| `egress_test.go` | 0 | TestDefaultEgressPolicyAllowsPublicHTTPSAndDeniesUnsafeClasses; TestEgressPolicyAppliesOriginHostPortAndCIDRAllowlists; additional cases/helpers |
| `egress_transport.go` | 0 | public contract and private invariant source |
| `egress_transport_test.go` | 0 | TestEgressDialerValidatesEveryDNSAnswerBeforeConnecting; TestEgressDialerConnectsOnlyToValidatedResolvedAddress; additional cases/helpers |
| `examples/ethereumjsonrpc/example_test.go` | 0 | Example |
| `examples/githubrest/example_test.go` | 0 | Example |
| `examples/vendorcomposition/example_test.go` | 0 | Example_vendorComposition |
| `examples/vendorcomposition/receipt_test.go` | 0 | TestVendorCompositionRetriesThenReusesCache; TestVendorCompositionPreservesUnknownPOSTOutcomeWithoutRetry; additional cases/helpers |
| `fixture.go` | 25 | public contract and private invariant source |
| `fixture_persistence.go` | 5 | public contract and private invariant source |
| `fixture_persistence_test.go` | 0 | TestRecorderWritesDeterministicVersionedFixtureAndLoaderChecksExpiry; TestFixtureLoaderMigratesExplicitSchemasAndRejectsHostileInput; additional cases/helpers |
| `fixture_record.go` | 11 | public contract and private invariant source |
| `fixture_record_test.go` | 0 | TestRecorderSanitizesBeforeFixtureStorageAndPreservesLiveExchange; TestRecorderStoresOnlyStableTransportFailureCategory; additional cases/helpers |
| `fixture_replay_test.go` | 0 | TestReplayTransportMatchesCanonicalRequestsAndClonesResponses; TestReplayTransportRejectsUnmatchedAndUnusedInteractionsSafely; additional cases/helpers |
| `form_test.go` | 0 | TestFormBodySnapshotsCanonicalRepeatedValuesAndReplays; TestFormBodyRepresentsEmptyFormExplicitly |
| `fuzz_test.go` | 0 | FuzzRequestSpecURL; FuzzHeaderValidation; additional cases/helpers |
| `hardening_benchmark_test.go` | 0 | BenchmarkRequestPolicyPaths; BenchmarkRequestConstructionAndSerialization; additional cases/helpers |
| `idempotency.go` | 13 | public contract and private invariant source |
| `idempotency_test.go` | 0 | TestIdempotencyKeyIsStableAcrossRetriesAndDistinctAcrossOperations; TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity; additional cases/helpers |
| `identity.go` | 6 | public contract and private invariant source |
| `identity_test.go` | 0 | TestClientAssignsStableDistinctLogicalOperationIdentity; TestCallerCanSupplyValidatedOperationIdentity; additional cases/helpers |
| `lifecycle_leak_test.go` | 0 | TestLifecycleLeakCloseCancelsPendingOperation |
| `main_test.go` | 0 | TestMain |
| `multipart.go` | 9 | public contract and private invariant source |
| `multipart_hardening_test.go` | 0 | TestMultipartBodyRejectsEveryBoundedMetadataClass; TestMultipartBodyDefaultsLimitAndSnapshotsHeaders; additional cases/helpers |
| `multipart_test.go` | 0 | TestMultipartBodyIsDeterministicReplayableAndLengthExact; TestMultipartBodyPreservesOneShotStreamingAndOwnership; additional cases/helpers |
| `oauth2.go` | 14 | public contract and private invariant source |
| `oauth2_cache.go` | 5 | public contract and private invariant source |
| `oauth2_test.go` | 0 | TestOAuth2AuthIntegratesReusableTokenSource; TestContextOAuth2AuthUsesRequestContext; additional cases/helpers |
| `oauth2_token_contract_test.go` | 0 | TestContextOAuth2TokenValidatorRejectsNil |
| `observability.go` | 26 | public contract and private invariant source |
| `observability_slog.go` | 1 | public contract and private invariant source |
| `observability_test.go` | 0 | TestTelemetryModelsOneOperationAndNumberedRetryAttempts; TestTelemetryObserverAndPropagatorCanBeAdoptedIndependently; additional cases/helpers |
| `pagination.go` | 29 | public contract and private invariant source |
| `pagination_strategies.go` | 33 | public contract and private invariant source |
| `pagination_strategies_test.go` | 0 | TestPageNumberPaginatorAdvancesTypedPages; TestOffsetPaginatorUsesStableLimitSteps; additional cases/helpers |
| `pagination_test.go` | 0 | TestPaginatorIsLazyBoundedAndPreservesTypedItems; TestPaginatorResumePreservesBufferedItemsAndCycleHistory; additional cases/helpers |
| `pipeline.go` | 18 | public contract and private invariant source |
| `pipeline_test.go` | 0 | TestPipelineResolvesLayerOverridesAndDeterministicOrder; TestPipelineRejectsInvalidMiddlewareDefinitions; additional cases/helpers |
| `pool.go` | 30 | public contract and private invariant source |
| `pool_test.go` | 0 | TestPoolRunsBoundedWorkersAndReturnsStableInputOrder; TestPoolCollectAllPreservesPerRequestFailures; additional cases/helpers |
| `profile.go` | 40 | public contract and private invariant source |
| `profile_test.go` | 0 | TestBuiltInPolicyProfilesAreFiniteAndVersioned; TestPolicyOverridePrecedenceAndProvenance; additional cases/helpers |
| `range.go` | 14 | public contract and private invariant source |
| `range_test.go` | 0 | TestWithRangeClonesRequestAndAppliesStrongValidator; TestValidateRangeResponseContinuesRestartsOrCompletes; additional cases/helpers |
| `rate_limit.go` | 28 | public contract and private invariant source |
| `rate_limit_test.go` | 0 | TestFirstPartyRateLimitAlgorithmsReserveDeterministically; TestRateLimitMiddlewareDelaysFutureAdmissionFromResponse; additional cases/helpers |
| `request.go` | 6 | public contract and private invariant source |
| `request_test.go` | 0 | TestNewRequestSpecResolvesSameOriginReference; TestNewRequestSpecRejectsUnsafeBaseOrReference; additional cases/helpers |
| `response.go` | 12 | public contract and private invariant source |
| `response_codec_test.go` | 0 | TestDecodeResponseUsesBoundedCallerCodecAndCloses; TestDecodeResponseRejectsTrailingLimitAndCodecFailure; additional cases/helpers |
| `response_drain.go` | 3 | public contract and private invariant source |
| `response_drain_test.go` | 0 | TestDrainResponseCompletesWithinBoundAndCloses; TestDrainResponseReturnsTypedLimitAndCloses; additional cases/helpers |
| `response_length_test.go` | 0 | TestResponseDecodersRejectDeclaredLengthMismatch; TestResponseDecodersDistinguishExplicitZeroUnknownAndExactLength; additional cases/helpers |
| `response_test.go` | 0 | TestDecodeJSONResponseStreamsBoundedStrictDocumentAndCloses; TestDecodeJSONResponseRejectsMediaTypeLimitAndTrailingData; additional cases/helpers |
| `resume.go` | 8 | public contract and private invariant source |
| `resume_test.go` | 0 | TestResumeDownloadToFileAppendsValidatedRangeAndPublishesAtomically; TestResumeDownloadToFileRestartsWhenServerReturnsFullResponse; additional cases/helpers |
| `retry.go` | 26 | public contract and private invariant source |
| `retry_test.go` | 0 | TestRetryReplaysSafeRequestsWithDeterministicBackoff; TestRetryUsesExplicitOwnedDelaySequence; additional cases/helpers |
| `scope.go` | 19 | public contract and private invariant source |
| `scope_test.go` | 0 | TestPolicyScopeKeysAreDeterministicResourceSpecificAndOpaque; TestPolicyScopeSupportsCustomDimensionsAndSnapshotsInput; additional cases/helpers |
| `session.go` | 13 | public contract and private invariant source |
| `session_test.go` | 0 | TestSessionCookieJarsAreOptInAndIsolatedPerClient; TestSessionRedirectPolicyControlsCrossOriginCookies; additional cases/helpers |
| `status.go` | 18 | public contract and private invariant source |
| `status_test.go` | 0 | TestClassifyResponseLeavesAcceptedBodyCallerOwned; TestClassifyResponseReturnsRedactedMappedStatusAndCloses; additional cases/helpers |
| `tls_policy.go` | 6 | public contract and private invariant source |
| `tls_policy_test.go` | 0 | TestTLSPolicyAppliesRootsServerNameAndPins; TestTLSPolicyRejectsPinMismatch; additional cases/helpers |
| `trace_context.go` | 2 | public contract and private invariant source |
| `trailer_test.go` | 0 | TestRequestSpecBuildsLayeredIndependentTrailers; TestRequestSpecTrailersReachStandardHTTPServer; additional cases/helpers |
| `transfer.go` | 24 | public contract and private invariant source |
| `transfer_file.go` | 4 | public contract and private invariant source |
| `transfer_file_test.go` | 0 | TestCopyResponseToFileAtomicallyReplacesValidatedDestination; TestCopyResponseToFileLeavesDestinationUntouchedOnValidationFailure; additional cases/helpers |
| `transfer_test.go` | 0 | TestCopyResponseStreamsWithDigestProgressAndClosure; TestCopyResponseProvidesNonEmptyTransferBuffer; additional cases/helpers |
| `transport_integration_test.go` | 0 | TestTransportIntegrationNegotiatesHTTPVersions; TestTransportIntegrationReusesHTTP1Connection; additional cases/helpers |

## Investigated constraints and remaining questions

The structural inventory identifies missing prose, not semantic completeness.
Source/test/specification tracing will distinguish supported behavior and retained
limits from deployment/provider assumptions and unresolved rationale. No source
comments are written as substitutes for runtime repairs or conformance evidence.

## Investigated contracts and evidence

- `RequestSpec` copies metadata and byte bodies, but borrows query encoders and
  body providers. All absolute/network-path references are rejected; scheme/raw
  Host comparison is not effective-port normalization. Header/query sizes have
  no total quota. Existing URL tests and the fragment-bearing fuzz seed do not
  assert fragment rejection. DEC-001's selected fragment-free policy is retained;
  current consequences now expose the implementation gap rather than redefining it.
- Profiles apply each inner standard-client Do timeout (not an outer admission/retry
  deadline) and construction-time owned-transport
  connections only. Other values are explicit recipes, including shutdown and
  body limits. Pipeline callbacks are borrowed; after-stages execute in resolved
  forward order, before successful caller body consumption. Panic containment
  retains raw payloads in typed fields, not safe-to-export values.
- Standard transport bounds do not prove a custom transport's bounds. Direct
  `HTTPClient` use and client-credentials acquisition bypass `Client.Do` URL
  validation/pipeline/body registration. Egress validates every DNS answer before
  numeric dialing; proxy checks constrain the proxy connection, not the proxy's
  destination resolution. Custom roots replace platform roots; SPKI pins may
  match any presented certificate after normal PKI validation. Secret erasure is
  not promised. Existing egress/TLS tests use controlled resolvers/loopback servers.
- Authentication pairs share operation context state; use consistent origin and
  sensitive-header policies. Query/body secrets are not stripped by header policy.
  Basic ASCII-control rejection remains intact, including 2026-10-11 history.
  Credential quotas exclude opaque OAuth Extra and source acquisition work.
  Token structs are copied but Extra is shallow; refresh leaders are synchronous,
  followers cancelable, and clock callbacks under locks must not reenter.
- `Close` signals cancellation and cleans registered bodies/owned idle transport
  state; it neither joins active calls/providers nor imposes ShutdownTimeout.
  Multipart/gzip Close joins its worker and requires source Close to unblock Read.
  Retry elapsed checks are between exchanges/delays; byte drains have no standalone
  time deadline against a hostile reader. Built-in limiter reservations are not
  refunded after cancellation. Float rate/very-large capacity accounting is not
  exact integer arithmetic. Provider callbacks own cooperation and concurrency.
- Response decoders close admitted bodies but can return partial typed values on
  errors; limits bound input, not codec allocation. Status excerpts alone pass a
  redactor: cloned headers/request IDs/vendor fields and unwrapped causes remain
  raw. Transport error URLs still retain paths. Public ad-hoc telemetry fields
  are not validated; only method projection/ID exclusion is automatic. Baggage
  names are admitted, not total value size or sensitivity. Correlation headers
  are still set on cross-origin attempts.
- Transfer I/O is cooperatively cancelable; partial writes and post-rename sync
  errors cannot promise an untouched destination. Temporary Chmod failure leaves
  the response body open. Resume append rollback can fail, 200 restart truncates
  without rollback, and 416 proves length equality rather than validator identity.
  Trusted paths do not imply symlink/hardlink confinement or cross-call locking.
- Cache memory limits count body lengths, not all retained bytes. Backend keys/
  variant digests are not encryption; coalescing entries have no global distinct-key
  admission bound and caller schedulers own eventual task execution/termination.
  Paginator callbacks execute under its mutex, item/continuation values are shallow,
  and elapsed checks cannot interrupt Fetch. Pool input is shallow-copied before
  count admission; reported byte/memory totals are checked after executor output
  exists. Unknown lazy sources hit the count bound without probing EOF.
- Replay snapshots collections and returns independent response bodies; matching
  sorts query/header values and intentionally loses redacted query identity.
  Captures precede ordered admission; mismatches retain the interaction and
  scripted failures consume it. Recorder interaction admission is post-exchange.
  Body SHA-256 supports matching, not confidentiality/authentication. Persisted
  loading owns file-byte limits; trusted migrators/redactors own callback work.
- Compression ratio accounting starts after 1 KiB decoded and includes compressed
  prefetch. NaN currently passes construction and disables ratio comparison, while
  the absolute decoded byte cap remains. DEC-009's selected finite-ratio policy
  stays unchanged; only truthful current consequences and append-only history change.

All 55 assurance sources were inspected for entrypoints, input partitions,
assertions, fixture/helpers and cleanup ownership. Root TestMain disables global
HTTP/DNS, fixes proxy environment and owns goleak; existing examples use local
TLS servers. Seven saved-seed fuzz functions and eleven benchmark functions are
inventory evidence, not newly executed conformance/performance proof. Existing
mutation-specific tests and platform-neutral fixture files remain unchanged.
There are no production platform/build variants, generated Go inputs or generator
operations. No source producer was executed for structural admission. The one
`Deprecated:` guidance and security/analyzer annotations are preserved; a blank
paragraph makes the existing guidance render canonically.

## Missing documentation versus remaining source questions

The initial 557 structural gaps were missing field/interface/enum/method contracts,
not evidence of 557 runtime defects. Adjacent comments now distinguish copied
containers from borrowed/aliased values, body-accounting limits from heap/time
bounds, and exchange completion from body/lifecycle completion. No general
security certification or fixture/protocol campaign is claimed.

Maintainer follow-ups remain outside this prose batch: enforce the selected
fragment-free URL and finite compression-ratio admission policies; close the
response on temporary Chmod failure; decide separate auth-scope state if composing
conflicting policies; consider total metadata/distinct-key admission and bounded
custom provider adapters where hostile workloads require them. Extreme cache
MaxInt64 capture arithmetic and minimum float-rate duration conversion have
source-visible unchecked boundaries; this audit does not claim runtime repair or
newly executed hostile numerical proof. Resume publication/rollback and proxy
trust limits require application policy, not a silently invented guarantee.

Frozen apidiff baseline, module manifests/dependencies, runtime fixtures, protocol
source/version pins and dated specification sections are preserved. Only current
DEC-001/009 consequences change; their selected behavior, IDs and authority remain,
with original digest histories retained as prefixes after native typed generation.

## Selected local results

- Go 1.27.2 formatted all 93 files and parsed identical executable ASTs,
  non-implicit tokens and literals against the recorded base. Only five optional
  inserted semicolons differ: fixture.go +1, response.go +2, transfer.go +2, from
  expanding public single-line carriers. All six annotations/deprecation lines
  retain their exact text. 1,295 public contracts and 1,259 comment groups were
  inspected/rendered; three test-only example directories have no production
  package to render with go-doc.
- Native `go doc -all .` renders the canonical package and public contracts.
  Configured `golib docs check --module .` runs that same native root-doc recipe
  and nonempty-document check: 157 links, 155 successful, zero errors, two excluded.
  `golib source-docs check --json` includes the staged audit and reports no findings.
  API mode remains apidiff; its frozen baseline is not rewritten for prose.
- A disposable stdlib helper copied the native typed decision/digest algorithm,
  first matched all 18 original register records to their existing histories,
  then appended only the two authorized consequence digests. All original
  history prefixes, selected behaviors, stable IDs and source pins are intact.
  Native `golib specification check` accepts one module and all 18 decisions.
- Disposable offline observations confirmed Build retains `#section` metadata
  and compression construction accepts NaN. No transport/exchange was invoked.
  These establish existing gaps, not repairs or passing protocol conformance.
- No example executable changed and the actual docs recipe contains no Example
  test operation. Runtime test suites, active fuzz, benchmarks, race/leak,
  mutation, conformance and production requests were not run for this Tier A pass.
- The unsupported initial `go tool gofmt` invocation made no source changes;
  the pinned toolchain's own bin/gofmt completed formatting and a clean final
  format check. Exact three owned Go caches, helper/input copies and renders
  were removed, including failure-session storage; absence was verified.

Complete ordinary Tier A diff inspection found no executable/API/dependency/
fixture change and no unsupported new guarantees. Existing runtime limitations
remain explicit follow-ups rather than disguised passing behavior.

Pinned Gitleaks v8.30.1 full changed-current-file snapshots, added-content snapshots
and staged changed-line scanning all returned zero findings. Exact task-owned
snapshot/report/log resources were removed immediately. No allowlist or scanner
policy was changed. The final batch remains locally committed only until the
coordinator completes remote-main delivery and exact-source CI.

## Objective Enforcement

The checksum-pinned public Tools v2.4.0 verifier now supplies
`make source-docs-check` as a prerequisite of the existing local and CI Make
routes. Ordinary shared CI selects strict source-documentation checking and
requires its result. Native prerequisites, operation budgets, security and
release controls are retained. Hosted exact-head evidence is recorded by the
coordination ledger; configuration alone does not establish green delivery.
