# OpenSearch specification conformance

The [decision register](../docs/specification-decisions.md), [source pins](manifest.tsv), authority monitoring, machine conformance, and decision history govern the exact OpenSearch adapter matrix.

Decision-history and changelog digests come from the validator's canonical
`encoding/json` representation of the decision schema. Empty optional fields
tagged `omitempty`, such as `differential_evidence`, are absent from that digest
input. A hash of the raw source JSON object therefore differs and is not the
decision digest.

| Decision | Status | Executable boundary |
| --- | --- | --- |
| OPENSEARCH-DEC-001 | resolved | TestSupportedOpenSearchVersionsIncludesCurrentRelease |
| OPENSEARCH-DEC-002 | resolved | TestNewRejectsUnsafeOrAmbiguousTransportConfiguration |
| OPENSEARCH-DEC-003 | resolved | TestClientSelectsConfiguredNodesWithoutImplicitRetries |
| OPENSEARCH-DEC-004 | resolved | TestPoolRotationAndEndpointOrder |
| OPENSEARCH-DEC-005 | resolved | TestInfoRejectsMalformedAndOversizedResponsesWithoutLeakingBodies |
| OPENSEARCH-DEC-006 | resolved | TestSearchEncodingWireContract |
| OPENSEARCH-DEC-007 | resolved | TestSearchTranslatesTypedCapabilitiesWithoutCrossIndexLeakage |
| OPENSEARCH-DEC-008 | resolved | TestSearchEncodingWireContract |
| OPENSEARCH-DEC-009 | resolved | TestSearchUsesPITSearchAfterAndSignedQueryBoundCursor |
| OPENSEARCH-DEC-010 | resolved | TestSearchRejectsMoreHitsThanRequested |
| OPENSEARCH-DEC-011 | resolved | TestWriteUsesExternalVersioningForSupportedDocumentActions |
| OPENSEARCH-DEC-012 | resolved | TestBulkEncodesExternalVersionsAndPreservesPartialOutcomes |
| OPENSEARCH-DEC-013 | resolved | TestLifecycleImplementsCreateResumableReindexVerifyCutoverAndCleanup |
| OPENSEARCH-DEC-014 | resolved | TestIndexTemplatesUseAuthorizedComposableTemplateAPI |
| OPENSEARCH-DEC-015 | resolved | TestHealthAndCapacityPreserveOperationalSignals |
| OPENSEARCH-DEC-016 | resolved | TestFailureDiagnosticAndClassificationContract |
| OPENSEARCH-DEC-017 | resolved | TestAdmissionRejectsExcessWorkWithoutReachingTransport |
| OPENSEARCH-DEC-018 | resolved | TestRealOpenSearchConformance |
| OPENSEARCH-DEC-019 | resolved | TestSearchFailsClosedBeforeResolutionWithoutAuthorization |
| OPENSEARCH-DEC-020 | resolved | TestReindexCursorIsEncryptedBoundAndExpiring |
| OPENSEARCH-DEC-021 | resolved | TestVerifyIndexRequiresSemanticVerifierAfterCountPreflight |
| OPENSEARCH-DEC-022 | resolved | TestRealOpenSearchSnapshotRestore |
| OPENSEARCH-DEC-023 | resolved | TestAliasMutationCannotBypassActiveCleanupExclusion |

The version matrix is provider interoperability and version-differential evidence. It does not imply equivalent ranking, mappings, analyzers, plugins, managed-service extensions, or unlisted patch behavior. Release and errata feed drift blocks the online check pending review and never changes behavior automatically.

## Release-feed review: 2026-10-02

Review owner: OpenSearch adapter maintainers. This source-based review of the
current server and Go-client release feeds does not widen the supported server
`2.19.6`/`3.8.0` and client `4.7.3` matrix, change normative source pins, or
upgrade dependencies.

[OpenSearch 3.9.0](https://github.com/opensearch-project/OpenSearch/releases/tag/3.9.0)
adds APIs and changes behavior, including task-result deletion, HTTP 400 for
clause-limit failures, and a completion-suggester resource fix. The primary
[completion fix](https://github.com/opensearch-project/OpenSearch/pull/22924)
identifies unbounded `max_determinized_states` in regex/fuzzy completion options
as CVE-2026-63136. Our `PrefixSuggestion` encoder emits only prefix, field, size
and duplicate suppression; it does not expose those regex/fuzzy options.
`RawExtensionQuery` is inserted into the query slot, not the suggestion slot.
This is a narrow source-based exposure disposition, not an affected-version,
severity, or whole-backend safety claim. Operators must review their deployed
backend and any access outside this adapter. Supporting 3.9.0 requires a
separate supported-matrix and conformance change.

[Client v4.8.0](https://github.com/opensearch-project/opensearch-go/releases/tag/v4.8.0)
backports transport retry, request-mutation, signing, body-lifecycle and bulk
utility fixes, and removes the v5 preview package. The new
[v5.0.0 client](https://github.com/opensearch-project/opensearch-go/releases/tag/v5.0.0)
changes public APIs and default partial-failure reporting. This adapter
constructs an official client with its own `poolTransport`, calls `Stream`,
implements admission/signing/cleanup itself, and decodes bulk and search
outcomes itself; it does not select the official retry/router/background
transport, bulk utility or preview API. Those changed paths therefore do not
justify a blind dependency upgrade. The signer remains a separately reviewed
dependency boundary, and a client upgrade needs focused compatibility/privacy
review before adoption.

Only the two changed release-feed hashes in `monitoring.json` were refreshed.
All five immutable monitored source authorities matched their existing pins.
Maintainers must reconsider this disposition on another feed change, backend
deployment change, new advisory, or adoption of an upstream transport/API path.
