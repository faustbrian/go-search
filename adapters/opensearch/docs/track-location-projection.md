# Track and Location projection composition

This composition projects tracking records with their latest known location
into OpenSearch. The application datastore remains authoritative; the index is
rebuildable derived state and returned identifiers must be re-authorized
against current source data before disclosure.

The checked
[`ExampleClient_trackLocationProjection`](../track_location_projection_example_test.go)
creates a `search.ProjectionEvent`, applies it through
`search.ProjectionConsumer` and the public OpenSearch adapter, then reads it
through a typed `search.Request`. Its bounded HTTP transport is a test seam,
not an OpenSearch emulator; the adapter's pinned real-cluster conformance
matrix owns backend compatibility.

Run the recipe from a repository checkout:

```sh
go test -C adapters/opensearch -run \
  '^(TestTrackLocationProjectionRecipe|ExampleClient_trackLocationProjection)$'
```

## Modules and dependency direction

- Required: `github.com/faustbrian/go-search` for the projection, write, query,
  result, and error contracts; and
  `github.com/faustbrian/go-search/adapters/opensearch` for the OpenSearch
  implementation.
- Optional: an application-owned transaction/outbox, queue, retry policy,
  telemetry pipeline, and source-data authorization boundary. Those components
  call the search contracts; neither search module imports or owns them.
- The application constructs the adapter and passes it to
  `search.NewProjectionConsumer`. The OpenSearch adapter owns REST translation,
  bounded responses, transport admission, and its own client lifecycle. It
  does not own application records, transactions, checkpoints, credentials,
  tenant policy, mappings, or the OpenSearch deployment.

The supported released pair is core `v1.0.0` with adapter
`adapters/opensearch/v1.0.0`, Go 1.26.6, `opensearch-go/v4` v4.7.3, and
OpenSearch 2.19.6 or 3.8.0. The checked recipe validates the public composition;
the [compatibility guide](compatibility.md) defines the separately maintained
real-backend matrix.

## Construction and execution order

1. The application loads endpoints and credentials from its secret/config
   system and creates a cursor-signing key. The search modules do not read the
   environment or discover secrets.
2. Construct `opensearch.Config` with bounded limits and timeout, explicit
   transport ownership, a tenant-aware `IndexResolver`, fail-closed
   `SearchAuthorizer` and `WriteGuard`, and a `search.CursorCodec`.
   Resolve reads and writes to distinct least-privilege aliases while retaining
   the exact backing generation in `IndexTarget.PhysicalName` for response
   attribution. Non-delete writes require an alias and fail closed otherwise.
3. Call `opensearch.New`. Construction validates and copies configuration but
   performs no network I/O and starts no goroutine.
4. Commit the authoritative Track or Location mutation and its
   `ProjectionEvent` to the same application-owned transaction/outbox.
5. After commit, dispatch the event at least once. A consumer calls
   `ProjectionConsumer.Handle`; the adapter authorizes the write, resolves the
   tenant's write alias, and applies an externally versioned full-document
   upsert.
6. Search only after the application's projection-lag policy permits it. The
   adapter authorizes the complete query before resolving the read alias.
   Re-authorize returned IDs against source data before returning results.

This composition has no middleware stack. Policy order is fixed as local
validation, application authorization/guard, tenant target resolution, then
one bounded adapter transport invocation.

## Idempotency, ordering, and visibility

`ProjectionEvent.IdempotencyKey` is the application outbox/delivery identity;
the application stores its processing checkpoint durably. Search-side ordering
uses the monotonically increasing source revision as the external document
version. A duplicate or older delivery returns
`search.OutcomeVersionConflict`, so it cannot overwrite a newer projection.
It is a known terminal outcome for that delivery, not permission to retry
forever. Never derive versions from worker time or delivery order.

`search.RefreshWaitFor` makes the recipe's successful write visible before the
subsequent query. Production paths may choose `RefreshNone` for throughput, but
then the application must expose projection lag rather than promise immediate
read-after-write visibility.

## Failure and recovery

- Validation and policy denial happen before transport I/O. Fix the event or
  authorization state; do not retry unchanged permanent input.
- `OutcomeVersionConflict` means the indexed version is equal or newer.
  Acknowledge the delivery after confirming this expected ordering result.
- `OutcomeRejected` may be retryable only inside the caller's bounded total
  retry budget.
- `OutcomeUnknown` or an `*opensearch.Failure` with `OutcomeKnown == false`
  requires source/outbox reconciliation before retry because the write may
  have committed.
- A successful projection does not acknowledge the source transaction. The
  dispatcher acknowledges only after recording its application-owned delivery
  checkpoint; a failed acknowledgement may redeliver safely through external
  versioning.
- Mapping or schema changes use the separately fenced migration/rebuild
  lifecycle. Never repair source data from the derived index.

## Configuration, secrets, and observability

The application owns endpoints, TLS roots, credentials or signer, tenant alias
mapping, authorization decisions, write tombstones/current-version checks,
cursor key rotation, timeouts, retry budgets, and the OpenSearch deployment.
Credentials are supplied through a rotation-aware provider and must not appear
in event payloads, idempotency keys, errors, logs, metrics, or traces. Tenant
and tracking identifiers are high-cardinality values and require explicit
application disclosure policy.

The adapter emits bounded observations through its configured telemetry hooks;
the application creates, flushes, and shuts down the telemetry backend. Record
projection lag and delivery outcome at the dispatcher boundary, and correlate
source transaction, outbox delivery, write, and query using application-owned
safe correlation identifiers.

## Close and shutdown

Stop accepting projection work, drain or cancel the application-owned
dispatcher, wait for its in-flight `Handle` calls, and then call
`opensearch.Client.Close`. `Close` is idempotent, closes tracked point-in-time
state, and releases only adapter-owned idle connections. It does not wait for
already admitted operations and does not close borrowed transports,
credentials, resolvers, guards, authorizers, or telemetry resources. Close
those caller-owned resources afterward in their dependency order.
