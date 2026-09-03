# Changelog

## Unreleased

### Changed

- Adopt the `go-library-tools` v1.4.0 schema-v2 cohesion contract and immutable
  reusable CI workflow without changing adapter API or runtime behavior.
- Keep package-owned verification executable from the nested module boundary
  used by the shared repository gate.
- Record the reviewed `opensearch-go` v5.0.0-rc6 release-feed drift as
  monitoring-only while retaining the exact v4.7.3 client, behavior, and
  support boundary for OPENSEARCH-DEC-001 through OPENSEARCH-DEC-005 and
  OPENSEARCH-DEC-017.

### Documentation

- Point the adapter README directly to the immutable v1.4.0 ecosystem index
  and integration-and-data-movement family guidance.
- Publish the adapter's family, selection, ownership, lifecycle, supported
  OpenSearch matrix, and delivery metadata with versioned ecosystem
  navigation.

- Remove completed implementation plans and link to package-owned guidance.
- Add machine-enforced OpenSearch decision, conformance, authority-monitoring,
  and immutable-history records for the exact 2.19.6/3.8.0 server and 4.7.3
  client matrix.

#### Specification Decisions

- OPENSEARCH-DEC-001 sha256:87f4d17cc03f47cd36dd12f9d1b669a989ca3c8b1b8317cd94afd16f6a44e805
- OPENSEARCH-DEC-002 sha256:85da03df0c652a99065b3326d69cce5727ff1f3b24fcbc3d059ec84d8da6e755
- OPENSEARCH-DEC-003 sha256:79ea9e0cbe25b39763f1ad5030501198dd7e44cf26271e5bfe6ba2cbc6b247aa
- OPENSEARCH-DEC-004 sha256:afcd21d67efe4b6e5ea1d5927f3c8898a68cd11f5b898f2fb26f72c4725a7d6d
- OPENSEARCH-DEC-005 sha256:50196dbe63f6a15d3b7bdbeda8c4392f3165f54b2eefbe1437e38029e25b9cd7
- OPENSEARCH-DEC-006 sha256:a9162197d99813e9786ae58cf81eba9369241fbddc232c959f715989c9d3a9f4
- OPENSEARCH-DEC-007 sha256:77464d5588bd1beb1528a4b8d6983a5547cf8815ffe2e6c608a279ab4a1a2557
- OPENSEARCH-DEC-008 sha256:1dae9fad76468f3be23c70c1a2e489dce35a915088a98edd7f4ddf88f4494a73
- OPENSEARCH-DEC-009 sha256:a58ecb4e117ef864bc0f3ba1b9446443bce134837ccd88ef111895beb30ce1ee
- OPENSEARCH-DEC-010 sha256:b8c2a6bdcf7718f419439d515478154fcd76f292f9a31f41f01d30a76b72d632
- OPENSEARCH-DEC-011 sha256:82aa7d64cde8d8be29d1a7a245f27211303f6b7e430f17c85ccae783569ae587
- OPENSEARCH-DEC-012 sha256:08800e9c38e5c2682f1b769230d804f36ce19ad2c84b42318271c2c781448ae6
- OPENSEARCH-DEC-013 sha256:e77cee92c8bec37c290b221b9a681ed8a556680db2000f8cec7cd374802090e4
- OPENSEARCH-DEC-014 sha256:d97653a6b35e6f93de09165691f5b442ab558a2f6512dcff7aabec2286a6aa41
- OPENSEARCH-DEC-015 sha256:1308af46c001bc450110896397d67173fcaf41550451ef61d07380a95d893012
- OPENSEARCH-DEC-016 sha256:50ed31523798bcf6d919ed5d927512a0864fbbcb6ae8d205221d27177657d85b
- OPENSEARCH-DEC-017 sha256:cd87c2a79e9240db5ddd198101b999e9220f53d5dd011bc058273452f48cbe62
- OPENSEARCH-DEC-018 sha256:d05eb192141312fe9ad997b735bf1f9ff399adf228639f83321752294a81d783
- OPENSEARCH-DEC-019 sha256:f1821323db1d18013b81fe07a43e7ec0a7dc47a19f483616ae164e685c609e08
- OPENSEARCH-DEC-020 sha256:b25a129fb854ad213ab639ce526ff0668ac564f0909937eb77bd9000e97d7b33
- OPENSEARCH-DEC-021 sha256:9fa388a41c1e37016b295fc346ddf643fbb5a142898837feb89bed075b19ad56
- OPENSEARCH-DEC-022 sha256:3e73af1d9099103a15c167b603afb50c58ccaac628877fa2633bc32e78f7761a
- OPENSEARCH-DEC-023 sha256:b04b2b84f27da9007fe6abb6fe625d835073c651e53d1c5caa0a6deea9b3c1fe

[Decision register](docs/specification-decisions.md)

## 1.0.0 - 2026-08-25

### Fixed

- Require bounded-load readiness to converge within a strict window instead
  of failing on a transient shard-initialization sample.

### Documentation

- Link the adapter README to package-owned documentation.

### Changed

- Publish the module from its standalone `github.com/faustbrian/go-search/adapters/opensearch` identity while preserving its documented API and behavior.
- Complete the destructive-cleanup decision record with explicit security,
  compatibility, and wire consequences for the required durable lifecycle
  mutation guard.

- Reject unsupported locale mappings and foreign raw-query extensions before
  creating a point-in-time search context.
- Add an auditable OpenSearch REST decision register, pinned source provenance,
  and fail-closed source integrity verification.
- Refresh real-cluster compatibility from OpenSearch 2.19.3/3.6.0 to the
  current 2.19.6/3.8.0 releases while retaining official client v4.7.3.
- Cap concurrency channels, queued callers, locale analyzer maps, and discovery
  trust rules before allocation or configuration cloning.
- Hold each in-flight admission permit until its response body reaches EOF or
  is closed, so slow or abandoned bodies remain inside the configured bound.
- Exercise a deployment-owned filesystem snapshot restore for each supported
  OpenSearch version while preserving external document versions, alongside
  full source-of-truth rebuild and reconciliation evidence.
- Require complete pre-resolution search authorization, including bounded
  pagination cost intent; AES-256-GCM-encrypted, expiring,
  tenant/source/target-bound reindex cursors; live target-definition
  fingerprint attestation; and an application-owned write fence across final
  verification and migration cutover.

- Require one application-owned durable lifecycle mutation coordinator across
  index creation, alias changes, cutover, and cleanup so final deletion
  eligibility cannot race another instance reactivating the generation.
- Run ten independent equivalent-semantics benchmark samples for each supported
  real OpenSearch version in the release matrix instead of a single sample.
- Make `CursorCodec` the sole cursor clock owner; `SearchConfig.Clock` remains
  source-compatible but is deprecated and ignored.
- Renew encrypted reindex cursors after every successful incomplete poll so a
  long-running backend task remains safely resumable without unbounded tokens.
- Disable write capabilities unless an application-owned durable `WriteGuard`
  authorizes immutable bounded operations against current documents or
  tombstones before target resolution, preventing stale replay after backend
  delete-version garbage collection.
- Split resolved request aliases from exact physical response generations and
  require resolver generation updates inside write-fenced alias cutover.
- Apply the 1,024-file-descriptor stress ceiling to disposable single-node
  clusters while using OpenSearch's required 65,536 minimum for multi-node
  rolling-upgrade proof.
- Reject `ActionUpdate` with `search.ErrUnsupported` before index resolution or
  transport because OpenSearch external-version writes cannot preserve shared
  update-existing semantics. Callers that intentionally accept create-or-replace
  semantics should use `ActionIndex` or `ActionUpsert`.
- Classify bulk HTTP 404 outcomes as `OutcomeNotFound` only for deletes;
  non-delete items now report `OutcomeFailed` when a safe backend failure is
  present and `OutcomeUnknown` otherwise.

### Fixed

- Copy validated explicit proxy URLs during client construction so later
  caller mutation cannot redirect requests or inject proxy credentials.
- Bound every returned cluster identity field to 1,024 bytes and reject
  control characters before the values can escape the adapter.
- Reject lifecycle verification of one generation against itself and cleanup
  bindings that identify a physical generation as the logical alias before
  authorization, guard callbacks, or backend requests.
- Reject invalid UTF-8 in bounded backend response bodies before JSON decoding
  can normalize poisoned identifiers or diagnostic text.
- Reject invalid UTF-8 locale configuration and lifecycle tenant or migration
  identities, and bound migration IDs before policy callbacks, resolver work,
  or backend requests.
- Isolate external-version ranges between benchmark samples and reject any
  release transcript with failed or missing samples before comparison.
- Replace the real security matrix's `all_access` runtime fixture with
  tenant-A-only read/write credentials and a separate bounded operator role;
  prove tenant-B and administrative denials on OpenSearch 2.19.6 and 3.8.0.
- Bind every accepted hit, bulk item, and successful single-write response to
  the resolver's exact physical generation while returning only the logical
  index to callers; reject response sources outside requested projections.
- Bound initial and rotated PIT identifiers, cap continuation keep-alive by the
  signed deadline's remaining lifetime, and classify a missing resource as PIT
  expiry only for cursor searches.
- Measure repeated real-network cleanup against explicit retained goroutine,
  file-descriptor, and heap budgets in the leak gate.
- Preserve the underlying known or unknown alias-mutation outcome when
  classifying cutover-guard contract failures, and wait for every started
  callback so an in-flight request cannot mutate after the caller returns;
  concurrent repeated callbacks can no longer displace or block the primary
  callback result.
- Reject exhausted cursor traversal before another search is dispatched and
  enforce cumulative item and response-byte limits on final short pages.
- Reject successful write and bulk items whose status/result pair does not
  match the requested index, upsert, or delete action.
- Classify transport loss and malformed success acknowledgements for PIT
  creation and deletion as unknown outcomes, and never repeat a failed PIT
  cleanup inside one search call.
- Start cursor expiry when the PIT is created, cap search response reads at the
  stricter result limit, and require OpenSearch's search, bulk, reindex, count,
  and health response metadata instead of accepting absent zero values.
- Redact lifecycle-verifier failures behind stable typed classifications while
  preserving cancellation and deadline semantics.
- Omit OpenSearch's unsupported `require_alias` parameter from single-document
  deletes while retaining external versions, so version-ordered deletes work
  through resolved write aliases on OpenSearch 2.19.6.

### Added

- Initial OpenSearch v4 client adapter for typed search, point-in-time cursor
  pagination, externally versioned writes and bulk operations, trusted node
  discovery, bounded transport decoding, and authorized index lifecycle flows.
- Explicit `opensearch`-bound raw query objects for trusted callers, with core
  size and JSON-object validation before transport execution.
- Fixed-deadline PIT cursors that cannot extend the configured total traversal
  duration on subsequent pages.
- Pre-network analyzer validation and final encoded bulk-byte enforcement.
- Authorized templates, process-local backpressure/circuit telemetry, health
  and capacity reports, migration/rebuild/reconciliation flows, AWS signing,
  and exact OpenSearch 2.19.6/3.8.0 compatibility evidence.
- Search authorization receives the complete query and result-disclosure scope;
  raw extensions are unavailable unless an authorizer is configured.
