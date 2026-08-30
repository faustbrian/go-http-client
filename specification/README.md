# Specification conformance matrix

`manifest.tsv` pins every normative source used by the HTTP client decision
register. RFC sources use immutable RFC Editor text. W3C Trace Context uses a
dated Recommendation snapshot instead of the mutable latest-version URL.
SHA-256 digests make source drift explicit.

The canonical
[`docs/specification-decisions.md`](../docs/specification-decisions.md)
register records every material interpretation, consequence, and condition for
reconsideration behind this conformance matrix.

The module claims conformance only for behavior exposed by its public policy
surface and linked executable evidence. It does not implement an HTTP wire
stack; framing and protocol-version transport are delegated to Go's `net/http`.

Run the focused map and evidence check with:

```console
make conformance
```

For an update, download the exact manifest URL, verify provenance, calculate
`shasum -a 256`, review errata and successor specifications, update affected
decisions and tests, and then update the manifest. A digest change alone MUST
NOT silently change behavior.

## Decision matrix

| Decision | Source | Executable evidence | Differential status |
| --- | --- | --- | --- |
| HTTPCLIENT-DEC-001 | RFC 3986 5 | `TestNewRequestSpecResolvesSameOriginReference`, `TestNewRequestSpecRejectsUnsafeBaseOrReference`, `TestRequestURLValidationContracts` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-002 | RFC 9110 4 through 15 | `TestRequestSpecBuildsLayeredIndependentTrailers`, `TestRequestSpecTrailersReachStandardHTTPServer`, `TestRequestSpecRejectsUnsafeOrBodylessTrailers` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-003 | RFC 9110 4 through 15 | `TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin`, `TestSessionRedirectPolicyControlsCrossOriginCookies`, `TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-004 | RFC 7617 2 | `TestCredentialEditorsApplyBasicBearerAndAPIKeys`, `TestAuthenticationMiddlewareReappliesOnlyWithinTrustedOrigin`, `TestAuthenticationRejectsCleartextCredentialTransport` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-005 | RFC 6749 2.3, 3.2, and 4.4 | `TestClientCredentialsTokenSourceCoordinatesRefreshAndBypassesMiddleware`, `TestClientCredentialsTokenSourceSupportsExplicitParameterAuthentication`, `TestCachedTokenSourceCoordinatesCancelableWaiters` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-006 | RFC 9110 4 through 15 | `TestRetryReplaysSafeRequestsWithDeterministicBackoff`, `TestRetryRequiresReplayableBodyAndExplicitUnsafeOptIn`; `scripts/check-retry-peer.sh` | maintained peer comparison |
| HTTPCLIENT-DEC-007 | RFC 9110 4 through 15 | `TestRetryUsesBoundedRetryAfterAndClosesDiscardedResponses`, `TestRetryDelayExactBoundaries`, `TestRateLimitObservationExactBoundaries`, `TestRateLimitHeaderObservationMatrix` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-008 | RFC 9111 3 through 5 | `TestSharedCacheDoesNotReuseAuthorizationWithoutExplicitPermission`, `TestCacheMiddlewareHonorsOnlyIfCachedMaxStaleAndStaleIfError`, `TestCacheVaryMatchingNormalizesEquivalentHeaderLineLayout`, `TestCacheValidationDoesNotReplaceRepresentationDependentHeaders` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-009 | RFC 9110 4 through 15 | `TestDefaultTransportDisablesImplicitCompression`, `TestCompressionMiddlewareDecodesGzipWithExplicitMetadata`, `TestCompressionMiddlewareEnforcesAbsoluteAndRatioBounds`, `TestCompressionWorkerStopsWhenAttemptMiddlewareShortCircuits` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-010 | RFC 9110 4 through 15 | `TestWithRangeClonesRequestAndAppliesStrongValidator`, `TestValidateRangeResponseContinuesRestartsOrCompletes`, `TestRangePolicyRejectsUnsafeRequestsAndMismatchedResponses`, `TestRangePolicyParserAndValidatorBoundaries` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-011 | RFC 8288 2 and 3 | `TestLinkPaginatorParsesAndResolvesRFCLinks`, `TestCursorPaginatorPreservesOpaqueCursorExactly` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-012 | RFC 6265 5 and 8 | `TestSessionCookieJarsAreOptInAndIsolatedPerClient`, `TestSessionRedirectPolicyControlsCrossOriginCookies` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-013 | W3C-REC-20211123 3 | `TestW3CTraceContextIsValidatedAndInjectedOnTrustedAttempts`, `TestW3CTraceContextPrimitiveBoundaries` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-014 | RFC 8259 2 through 8 | `TestDecodeJSONResponseStreamsBoundedStrictDocumentAndCloses`, `TestDecodeJSONResponseRejectsMediaTypeLimitAndTrailingData`, `TestDecodeJSONResponseEmptyAndMalformedSemantics` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-015 | RFC 9110 4 through 15 | `TestIdempotencyKeyIsStableAcrossRetriesAndDistinctAcrossOperations`, `TestIdempotencyRedirectPolicyPreservesOnlyMatchingOperationIdentity`, `TestCallerContextIdempotencyKeyHasExplicitProvenance` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-016 | RFC 8288 2 and 3 | `TestCursorPaginatorPreservesOpaqueCursorExactly`, `TestLinkPaginatorParsesAndResolvesRFCLinks` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-017 | RFC 9110 4 through 15 | `TestPipelineExecutesNestedLifecycleInDeterministicOrder`, `TestPipelineRunsAttemptScopeForEveryAttempt`, `TestPipelineResponseReplacementClosesSupersededBody`, `TestPipelineContainsMiddlewareAndTransportPanics` | not assessed; standard-library delegation is not labeled as a protocol peer |
| HTTPCLIENT-DEC-018 | RFC 9110 4 through 15 | `TestRequestSpecBuildsIndependentReplayableByteBodies`, `TestRequestSpecStreamingBodyIsExplicitlyOneShot`, `TestClassifyResponseLeavesAcceptedBodyCallerOwned`, `TestPipelineResponseReplacementClosesSupersededBody`, `TestPipelineClosesResponsesReturnedWithErrors` | not assessed; standard-library delegation is not labeled as a protocol peer |
