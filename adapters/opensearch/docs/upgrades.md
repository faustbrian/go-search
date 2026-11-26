# Upgrades and backup boundaries

The release matrix proves exact OpenSearch versions `2.19.8` and `3.10.1` with
official Go client `v4.8.0`. Treat any other patch, plugin, analyzer, or managed
service profile as unverified until conformance is rerun.

Before an OpenSearch upgrade:

1. read the server breaking-change and rolling-upgrade guidance for every hop;
2. snapshot according to the deployment provider's supported repository and
   restore procedure, and test restoration independently;
3. verify plugin/analyzer availability and mapping compatibility;
4. run conformance, workload benchmarks, failover, partial-shard/bulk, PIT-loss,
   TLS/auth rotation, migration, rebuild, and reconciliation tests in a
   disposable environment;
5. drain or bound long-lived PIT cursors and pause alias cutovers;
6. roll data nodes and cluster-manager nodes in the provider-required order,
   verifying health and adapter operations after each replacement;
7. resume projections, reconcile from the durable checkpoint, and monitor the
   dashboards before completing the rollout.

An OpenSearch snapshot is not the source-of-truth backup and this adapter does
not create, retain, encrypt, or restore snapshots. The application database and
durable outbox need their own backup/restore contract. A search snapshot can
reduce recovery time, but a full rebuild from authoritative data must remain
tested. Alias rollback handles a bad index generation; it does not downgrade a
server binary or replace a tested restore plan.

The supported-version matrix separately exercises the deployment operation: it
mounts a fresh bounded filesystem repository into each disposable OpenSearch
container, registers that repository through the official client, snapshots one
isolated physical index, deletes and restores it, and verifies document sources
and external versions through this adapter. This proves the local filesystem
fixture only. Managed repository credentials, encryption, retention, provider
restore procedures, and application source-of-truth recovery remain owned and
must be tested by the deployment.

The compatibility review was refreshed on 2026-08-10 against the official
OpenSearch 2.19.6 and 3.8.0 release notes, the `opensearch-go` v4.7.3 release,
and the current rolling-upgrade guide. OpenSearch 2.19.6 includes bulk-hang and
circuit-breaker fixes. OpenSearch 3.8.0 includes timeout status, malformed
mapping, deserialization-depth, path-boundary, and dependency security fixes.
The adapter does not adopt new 3.8-only APIs, so its HTTP surface remains in the
2.19/3.8 common subset.

The server matrix moved to OpenSearch 2.19.8 and 3.10.1 on 2026-11-26. The
2.19.7 and 2.19.8 patches harden the server and fix a node halt triggered by a
zstd-encoded request body on 2.19.4 through 2.19.6
([GHSA-68xw-fqpj-h922](https://github.com/opensearch-project/OpenSearch/security/advisories/GHSA-68xw-fqpj-h922)).
OpenSearch 3.10.0 fixes bulk request corruption on shard indexing backpressure
([#20727](https://github.com/opensearch-project/OpenSearch/pull/20727)): a
shard rejection now fails only that shard's items with a per-item HTTP 429
instead of failing the whole bulk request. The adapter already reports such an
item as `OutcomeRejected` with `Retryable` set, and the other items keep their
own outcomes. 3.10.0 also removes the experimental Views feature, which the
adapter never called. The REST specification files the adapter depends on are
unchanged between the old and new pins, so its HTTP surface stays in the
2.19/3.10 common subset.

The current client adoption moves to v4.8.0 without adding a server version
or selecting the official retry/router or bulk-utility paths. The separately
reviewed AWS signer now passes request cancellation to credential retrieval.

Primary references:

- <https://github.com/opensearch-project/OpenSearch/releases/tag/2.19.8>
- <https://github.com/opensearch-project/OpenSearch/releases/tag/3.10.1>
- <https://github.com/opensearch-project/OpenSearch/releases/tag/3.10.0>
- <https://github.com/opensearch-project/OpenSearch/releases/tag/2.19.6>
- <https://github.com/opensearch-project/OpenSearch/releases/tag/3.8.0>
- <https://github.com/opensearch-project/opensearch-go/releases/tag/v4.7.3>
- <https://github.com/opensearch-project/opensearch-go/releases/tag/v4.8.0>
- <https://docs.opensearch.org/latest/migrate-or-upgrade/rolling-upgrade/>
