# Search threat model

Model version: 1. Reviewed source baseline:
`a93ac8817a9a172ffe552f3a4c1694bb74bd7e9b` (remote main, 2026-10-02).
Owner: go-search maintainers. This document records a source audit, not a
scanner result, runtime certification, or declaration that the ecosystem
security goal is complete.

## Scope and assets

The root `github.com/faustbrian/go-search` module owns typed documents, query
validation, signed cursors, projection events, reconciliation, migration
coordination contracts, and the `searchtest` fake. The only maintained backend
adapter is the separately releasable nested
`github.com/faustbrian/go-search/adapters/opensearch` module. No other backend
implementation is implied by the core interfaces.

Assets include tenant documents, field permissions, query and source contents,
external versions and durable tombstones, migration state, aliases and physical
generations, PIT/task identifiers, cursor keys, credentials, and cluster
identity. Search state is rebuildable; the application datastore, outbox,
authorization policy, and deployment are authoritative.

Attackers may control tenant-facing requests, sources, identifiers, query
trees, raw extensions, pagination tokens, or backend responses. Compromised
backends, DNS/proxies, dependencies, CI actions, and maintainers can cross
different boundaries. Application-provided callbacks, transports, clocks,
signers, stores, and guards are trusted implementations, not sandboxed plugins.

## Boundaries and existing controls

| Boundary | Controls and source | Evidence already present in source |
| --- | --- | --- |
| Input to core validation | [Documents](../document.go) bound bytes, depth and nodes, reject malformed UTF-8 and duplicate keys, and copy sources. [Queries](../query.go) preflight bytes, collection sizes, depth, clauses, capabilities and traversal before encoding. [Limits](../limits.go) reject unbounded configurations and traversal multiplication overflow. | [Document tests](../document_test.go), [query hardening tests](../query_hardening_internal_test.go) |
| Schema and path construction | [Definitions](../schema.go) validate physical names and canonical bounded settings/mappings. Adapter [search targets](../adapters/opensearch/search.go) and [lifecycle resources](../adapters/opensearch/lifecycle.go) enforce their narrower supported grammar; logical names are resolved rather than interpolated into physical paths. | [Schema tests](../schema_test.go), [template tests](../adapters/opensearch/template_test.go) |
| Search and field authority | [Search](../adapters/opensearch/search.go) snapshots and authorizes the full query/projection/highlight/aggregation/suggestion and pagination intent before resolution or IO. Empty projection is explicitly full-source access. Response attribution and projected fields are validated. | [Authorization tests](../adapters/opensearch/search_authorization_test.go), [response semantics tests](../adapters/opensearch/search_semantics_test.go) |
| Writes, replay and bulk | [Write guard](../adapters/opensearch/write_authorization.go) receives immutable single-tenant intent before resolution. [Bulk](../adapters/opensearch/bulk.go) bounds items/bytes, attributes every item and reports unknown outcomes without retry. External versioning does not replace durable tombstones after backend garbage collection. | [Write authorization tests](../adapters/opensearch/write_authorization_test.go), [bulk tests](../adapters/opensearch/bulk_test.go) |
| Endpoints and credentials | [Configuration](../adapters/opensearch/config.go) requires explicit authorities, rejects URL userinfo, defaults to HTTPS and no proxy, restricts plaintext opt-in to unauthenticated loopback, rotates credentials per request and disables implicit retries. Explicit [discovery](../adapters/opensearch/discovery.go) validates the full replacement topology against configured DNS/CIDR trust. | [Configuration tests](../adapters/opensearch/config_test.go), [discovery tests](../adapters/opensearch/discovery_test.go) |
| Response decoding and privacy | [Bounded reader](../adapters/opensearch/info.go) caps decoded bytes, checks UTF-8 and reads one byte beyond the limit to detect overflow. Request paths close bodies on failure/success. [Failures](../adapters/opensearch/failure.go) and adapter callback failures expose classifications, not raw causes. [Telemetry](../adapters/opensearch/telemetry.go) carries fixed operational fields and contains observer panic without reporting its value. | [Failure tests](../adapters/opensearch/failure_test.go), [telemetry tests](../adapters/opensearch/telemetry_test.go) |
| Cursors and resources | Root [HMAC-SHA256 cursors](../cursor.go) bind tenant/index/query/generation, canonical token encoding, expiry and traversal totals. Adapter [reindex cursors](../adapters/opensearch/lifecycle_cursor.go) use standard AES-256-GCM for task confidentiality. [PIT ownership](../adapters/opensearch/pit_tracker.go) bounds local admission, single consumption and expiry. [Resilience](../adapters/opensearch/resilience.go) bounds in-flight/queued work with explicit queue waits. | [Cursor tests](../cursor_test.go), [reindex cursor tests](../adapters/opensearch/lifecycle_cursor_test.go), [resilience tests](../adapters/opensearch/resilience_test.go) |
| Reconciliation and lifecycle | [Reconciliation](../reconcile.go) bounds combined retained input, validates ordered records, requires guarded authoritative deletion versions and never overwrites same-version divergence. [Migration](../lifecycle.go) persists dispatch intent and binds resumable state to a plan. Adapter lifecycle requires authorization, durable mutation exclusion, semantic verification, write fences and cleanup eligibility; direct deletion fails closed. | [Reconciliation tests](../reconcile_test.go), [migration tests](../lifecycle_test.go), [cleanup guard tests](../adapters/opensearch/lifecycle_cleanup_guard_test.go) |

These are references to existing tests, not claims that they were executed by
this audit. The direct owned consumer is the OpenSearch module; its executable
[Track/Location composition](../adapters/opensearch/docs/track-location-projection.md)
is an adoption example, not proof of a deployed application's policy.
`searchtest` deliberately lacks backend authorization, PIT and ranking semantics
and must not certify production security or OpenSearch behavior.

## Residual risk dispositions

The following are accepted ownership boundaries, conditional on their stated
mitigations. They are not acceptance of a demonstrated vulnerability. Owners
must reconsider a disposition when its condition no longer holds.

| ID and risk | Owner and rationale | Mitigation | Review condition |
| --- | --- | --- | --- |
| SEARCH-01: application policy, callbacks and core error disclosure | Application security owner; backend-neutral interfaces cannot implement tenant policy or redact arbitrary application errors. Core stores, readers, indexers, outboxes, authorizers and observers may return caller-defined errors. | Authorize every field and logical resource; keep trusted callbacks context-aware, concurrency-safe and bounded. Sanitize core orchestration errors before tenant responses or logs. Adapter callback failures are redacted separately. | New callback implementation, tenant-facing exposure, or changes to cancellation/error contracts. |
| SEARCH-02: custom transport and DNS/proxy authority | Deployment/network owner; configured seed authorities are trusted operator input, not a universal SSRF filter. Discovery DNS suffix checks do not pin resolved IPs. Borrowed/custom transports own TLS, proxying, dialing, redirect-like behavior and context compliance; an owned supplied HTTP transport can retain caller TLS settings when Config.TLS is nil. | Use verified TLS, controlled DNS/egress, narrow authorities/CIDRs and explicit proxy policy. Never use tenant-controlled endpoints or a transport that forwards secrets elsewhere. Review supplied transports independently; prefer explicit verified Config.TLS for owned HTTP transports. | Endpoint, DNS, proxy, transport, certificate or managed-service topology change. |
| SEARCH-03: cursor confidentiality and cross-instance admission | Application owner; root signed cursors provide integrity, not encryption, and local PIT admission is not a fleet quota or replay store. | Do not place secrets in root cursor state; treat cursors as sensitive and do not log them. Rotate keys with a compatibility window; enforce principal/fleet budgets and single-consumer continuation across instances. Reindex task cursors are encrypted but still sensitive. | New cursor fields, key policy, public token exposure or multi-instance pagination changes. |
| SEARCH-04: abandoned or ambiguous backend work | Deployment and application lifecycle owners; cancellation cannot undo a dispatched mutation. Close releases local/transport ownership, not every server PIT or task; abandoned PITs rely on backend expiry. Cleanup intentionally uses a fresh bounded request after caller cancellation. | Use finite keepalive/timeouts, backend resource quotas and operation reconciliation. Surface unknown outcomes; never blindly retry destructive or externally versioned work. Monitor cleanup failures and task/PIT pressure. | Backend expiry/resource configuration, retries, shutdown policy or lifecycle changes. |
| SEARCH-05: durable replay and lifecycle exclusion | Application data/lifecycle owner; backend tombstone retention is finite, and process-local guards cannot exclude other applications. Low-level SwapAlias is not verified cutover. | Durable current-version/tombstone WriteGuard; shared cross-instance mutation guard; semantic verifier, write fence and cleanup eligibility. Treat bootstrap/recovery primitives as explicitly authorized operations. | Outbox retention, tombstone retention, shared guard/store, cutover or mixed-version deployment changes. |
| SEARCH-06: finite but deployment-inappropriate budgets | Deployment capacity owner; positive caller-selected core limits and per-request response caps are not guarantees that aggregate resource use is affordable. Expensive backend queries, mapping cardinality and fleet traffic need service policy. | Set schema-specific byte/depth/node and query budgets, bounded deadlines on caller callbacks, admission limits, principal quotas and backend circuit/resource limits. Avoid raw extensions sourced from unrestricted caller JSON. | Limit/schema/query-feature changes, traffic growth or new service budgets. |
| SEARCH-07: operational and presentation disclosure | Application/operator owner; Info, Discover, Health, Capacity, snapshots and telemetry are operator-wide rather than tenant-authorized APIs. Highlights and sources are data, not safe HTML. | Separate operator and tenant clients/endpoints; restrict cluster identity access, escape display output, avoid raw payload/error logging and sanitize artifacts/examples. | New metrics labels, diagnostics fields, UI rendering or operator endpoint exposure. |
| SEARCH-08: supply-chain and maintainer compromise | Repository maintainers; source validation does not prove dependencies or CI tools uncompromised. The owned CI delegates to immutable go-library-tools source. | Review dependency/tool pin updates, protect release credentials, retain exact-source CI and release evidence, and use the private reporting route. | Dependency/action/tool changes, advisory, maintainer access or release boundary. |

## Non-applicable and unverified boundaries

The core does not implicitly open networks, files, databases, processes or
environment credentials. It has no archive extraction, SQL execution,
filesystem path resolution, plugin registry or hidden background worker.
Application implementations of those interfaces remain outside that statement.
The adapter performs explicit network IO; AWS credential discovery belongs to
the explicitly supplied signer/provider. Request signing is not endpoint or
tenant authorization. Standard-library crypto is used; there is no custom
cryptographic primitive or credential-comparison authenticator in this package.

This bounded audit did not identify a confirmed new production defect.
Four focused lifecycle-guard tests, adapter vet and a public RawMessage and
nominal-interface consumer passed on local Go 1.27.1. A single existing guard
test also passed with the race detector before its synchronization repair;
no broader test, fuzz/race/load campaign, scanner, live OpenSearch,
credential-provider, Docker or release/clean-consumer check was executed by
this audit. Existing test
names and specification decisions are not substitutes for fresh gate results.
The inspected baseline's [CI run 36847328225](https://github.com/faustbrian/go-search/actions/runs/36847328225)
failed: online specification authority review, root API compatibility, and the
adapter race run's asynchronous lifecycle mutation guard test were not green.
The guard test released its callback before the adapter could reliably observe
guard return; it now waits for post-return context cancellation without changing
the production guard or its unknown-outcome assertion. Pinned API snapshots of
the same current root source establish that its old baseline exactly matches
Go 1.27 with JSONv2 disabled; only the standard-library JSON alias transition
changes the default snapshot. The root baseline was refreshed after public
consumer proof. The [release-feed disposition](../adapters/opensearch/specification/README.md#release-feed-review-2026-10-02)
records why monitoring pins changed without expanding the supported matrix.
Required hosted CI against the delivered repair remains a separate gate.
CodeQL passed in the inspected run, but does not replace the failed contracts
or establish all security scanners passed.
Dependency vulnerability/license scans, history-addition secret scanning,
workflow analysis and required exact-source hosted CI remain separately
assessed evidence; no scanner success is asserted here. Live service checks
require environment-specific authority. The repository security goal remains
open until its applicable hostile-boundary tests and security gates are proven.

This documentation, test-synchronization and generated API-baseline batch does
not change either module's public source API, production behavior, dependencies
or supported backend matrix. Its release verdict
is **no module release required**; delivery is a repository maintenance commit.
Future behavioral findings require a focused regression and proportional
verification, and confirmed vulnerabilities require affected-version guidance
and coordinated security release rather than unrelated fleet releases.

## Reporting and review

Use the [private security reporting process](../SECURITY.md); adapter reporters
should include the adapter and OpenSearch versions. Do not put tenant payloads,
credentials, keys, cursors, PIT/task IDs or reporter data into public artifacts.
Review this model when a listed condition changes or before a relevant security
release, and increment its model version for material boundary changes.
