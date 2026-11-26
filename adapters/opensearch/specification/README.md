# OpenSearch specification conformance

[Documentation map](../../../docs/index.md) · [Repository README](../../../README.md)
[Next: Documentation map](../../../docs/index.md)

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

## Server matrix adoption: 2026-11-26

Review owner: OpenSearch adapter maintainers. The supported server matrix
moves from 2.19.6 and 3.8.0 to the current patch releases, OpenSearch
[2.19.8](https://github.com/opensearch-project/OpenSearch/releases/tag/2.19.8)
and [3.10.1](https://github.com/opensearch-project/OpenSearch/releases/tag/3.10.1);
`opensearch-go/v4` stays at 4.8.0. This supersedes the "matrix stays" verdict
of the release-feed review below.

| Pin | Previous | Current |
| --- | --- | --- |
| 2.19 source commit | `97d3c13bf22a4a72ac11dc503fe44c97662b9161` | `b11b1e2cedf8e72b37d629956a0a740871f20a50` |
| 2.19 source archive SHA-256 | `290a4eaf01a2518673fef28f348d44dc67b1b681693f0e7456a8f9c27c5c5acb` | `3d7d7a076c25da37034f0679b6b0e3927bb3d60ee897c1219798f635cb227db4` |
| 3.x source commit | `e5a3c5691be87af6c12dbe3e158c59c04ee72973` | `8f41a7a09efd4fc25dfb52f8fa392a1bb930c260` |
| 3.x source archive SHA-256 | `f44b9342b958a3082f971207f46eb88c25d3004268555e1f2472b6a7346305d8` | `43489f9905ed878ad5e45f5c856fad835e75bd46187370ec04379020dd830340` |
| 2.19 image index digest | `sha256:8690b204fe914c60ca76d451ac73bc0481e034d32d3779944c8caca56a2b003f` | `sha256:0171f36eee4c50f5251e858458216612c4b86bbf4cba4eb28e3b0708944266cf` |
| 3.x image index digest | `sha256:bcc1797519726ceb6d651d4a3e60b7c30da91793914a8dfe75fd441d4f641509` | `sha256:02358b2be62b268a404a54581184d027ac0dfa8f875be15fe4c4fa3f8884f464` |

Each commit is the one its release tag names. Each archive digest was stable
across two independent downloads, and each image digest is the multi-arch index
the registry serves for the release tag. The monitored `cluster.health`,
`search`, `bulk` and `reindex` REST specification files are byte-identical at
the old and new commits, so their content digests are unchanged and only their
URLs, versions and specification names moved. Across the whole REST
specification, 2.19.8 changes only `nodes.hot_threads`; 3.10.1 adds a `limit`
parameter to the `_cat` APIs, adds the cluster describe-settings and task
deletion APIs, and updates documentation links, including the one for
`delete_pit`. The adapter calls none of the changed or added APIs, and the
`delete_pit` request shape is unchanged.

The 2.19.7 and 2.19.8 server hardening was dispositioned in the release-feed
review below; 2.19.7 also fixes
[GHSA-68xw-fqpj-h922](https://github.com/opensearch-project/OpenSearch/security/advisories/GHSA-68xw-fqpj-h922),
a node halt on a zstd-encoded request body that affected the previous 2.19.6
pin. 3.10.0 fixes bulk request corruption on shard indexing backpressure
([#20727](https://github.com/opensearch-project/OpenSearch/pull/20727)): a
shard rejection now fails only that shard's items with a per-item HTTP 429
`rejected_execution_exception`. Decision 012 already classifies a per-item 429
as a retryable rejection while preserving every other item's outcome, and the
bulk partial-outcome unit test pins that classification, so no adapter code
changes. 3.10.0 also removes the experimental Views feature, which the adapter
never called. 3.10.1 fixes scripted source reads on compound segments and
updates its JDK and Reactor dependencies.

Decision 001 names the new matrix. Every decision whose source authority is a
server specification takes the new specification name and version, and each
new canonical digest is appended to the decision history; decisions 002 to 005
and 017 bind only the client authority and keep their digests. Decision 018
requires the real-server conformance run for a source pin change; the version,
security and rolling-upgrade matrices ran on the new image digests for this
adoption.

## Client adoption: 2026-10-06

The current matrix adopts `opensearch-go/v4` `4.8.0` at immutable source
`161f33e0d923e7414b3e390c2b40af8f4a311310`, retaining server `2.19.6`/`3.8.0`.
Current source locks and decisions reflect that client. Prior dated feed
reviews below remain historical dispositions, not the current client pin.
The adapter still owns transport, retries, admission, and response bounds;
the official AWS signer now propagates request cancellation to credentials.

## Release-feed review: 2026-11-26

Review owner: OpenSearch adapter maintainers. All seven monitoring entries
were retrieved with the checker's request profile. The five immutable source
authorities match their existing pins. The two release feeds moved; each new
digest was stable across two independent requests:

| Authority | Previous SHA-256 | Current SHA-256 |
| --- | --- | --- |
| `opensearch-releases` | `918a2a81dad88c4a8748bc6f134827aeac600d196642b02670c96d6f575e64f8` | `7be78aac0942aea57191ea88abba53d89704dedaf83f98d4185bbd878bb755b9` |
| `opensearch-go-releases` | `5338117cdde28ee79e978fff6f56cf2b38cff6479fc7f09cfd94642c6848362d` | `e6d626a68448234f09e60a1a0fb8b6ccf64eb8bb1ff452ae7de89a617cbc3a31` |

The server feed added
[2.19.7](https://github.com/opensearch-project/OpenSearch/releases/tag/2.19.7),
[2.19.8](https://github.com/opensearch-project/OpenSearch/releases/tag/2.19.8),
[3.10.0](https://github.com/opensearch-project/OpenSearch/releases/tag/3.10.0)
and [3.10.1](https://github.com/opensearch-project/OpenSearch/releases/tag/3.10.1);
3.9.0 was dispositioned on 2026-10-02. The 2.19 patches are server-side
hardening of the supported line: a `query_string` nesting limit, the
completion-suggester `max_determinized_states` cap, Mustache partial
resolution disabled in search templates, bounded JSON deserialization
recursion, stricter `scroll_id` validation, filesystem-repository `base_path`
validation, bounded `min_hash` parameters, a ptrace syscall filter and
dependency updates. The adapter's typed encoder emits none of the affected
constructs: full text becomes `multi_match`, suggestions emit only prefix
completion options, pagination uses PIT with `search_after` rather than
scroll, and the adapter issues no search-template or snapshot-repository
requests. Analyzer and filter definitions, including any `min_hash` filter,
remain application-owned index-template content under decision 014, and
`RawExtensionQuery` remains an authorizer-gated, caller-owned query payload.
3.10.0 removes the experimental Views feature and macOS x64 archives, neither
used here, and returns per-item errors instead of failing a whole bulk
request on shard indexing backpressure. That 3.10 behavior is outside the
supported matrix; adopting it requires the real-server conformance run that
decision 018 sets for any source pin change.

The client feed added
[v4.8.1](https://github.com/opensearch-project/opensearch-go/releases/tag/v4.8.1),
[v4.8.2](https://github.com/opensearch-project/opensearch-go/releases/tag/v4.8.2)
and v5.0.2 through v5.2.1. v4.8.1 changes no client code; it raises the
repository toolchain and bumps AWS SDK patch versions, which this module
resolves through its own `go.mod`. v4.8.2 changes only the official
`opensearchtransport` router's node selection. The adapter supplies its own
`poolTransport` and uses only the root client's `Stream` call and the
`signer` package, so
neither patch reaches an adapter path. v5 remains an unadopted major version
whose partial-failure defaults differ from v4; upstream now describes v4 as
maintenance-only.

No REST path, parameter, JSON shape, retry ownership, cursor, versioning or
lifecycle surface changes. The supported matrix stays exactly OpenSearch
2.19.6 and 3.8.0 with `opensearch-go/v4` 4.8.0; all twenty-three decisions,
source pins and conformance bindings remain unchanged.
Moving the matrix to a newer patch release is a separate supported-matrix and
conformance change. Only the two release-feed digests above were updated;
URLs and the 30-day interval are unchanged.

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
