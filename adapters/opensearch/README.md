# OpenSearch adapter

This module is the first production adapter for
`github.com/faustbrian/go-search`. It translates typed requests to the
OpenSearch API without making engine-specific ranking behavior part of the core
contract.

The adapter is stable at v1.0.0 and supports Go 1.26.6 on platforms where the
Go HTTP client and the selected credential provider are supported. Its exact
backend contract is OpenSearch 2.19.6 or 3.8.0 over HTTPS/JSON REST with
`opensearch-go/v4` v4.7.3.

## Install

```sh
go get github.com/faustbrian/go-search/adapters/opensearch@v1.0.0
```

Import it as `github.com/faustbrian/go-search/adapters/opensearch`. The core
`github.com/faustbrian/go-search` module is an owned dependency; the adapter
does not make OpenSearch mandatory for core consumers.

## Five-minute start

The checked [`ExampleClient_Search`](example_test.go) constructs a client with
an explicit transport, resolver, authorizer, limits, timeout, and cursor codec,
then performs a typed search. From a repository checkout, run it with:

```sh
go test -C adapters/opensearch -run '^ExampleClient_Search$'
```

Replace the example transport with a peer-verified deployment transport and
application-owned policy before production use. The adapter never discovers
credentials, endpoints, authorization, or tenancy from global configuration.
Proxying defaults off; selecting `ProxyEnvironment` explicitly opts an
adapter-managed transport into the process environment's proxy policy.

## Production configuration outline

```go
client, err := opensearch.New(opensearch.Config{
    Endpoints:            []string{"https://search.example.internal:9200"},
    RequestTimeout:       3 * time.Second,
    MaximumResponseBytes: 8 << 20,
    Signer:               signer,
    Search: &opensearch.SearchConfig{
        Limits:      search.DefaultLimits(),
        CursorCodec: cursorCodec,
        Resolver:    resolver,
		Authorizer: searchAuthorizer,
		WriteGuard: writeGuard,
		MaximumOpenPointInTimes: 64,
        LocaleAnalyzers: map[string]string{
            "fi": "finnish",
            "en": "english",
        },
    },
	Lifecycle: &opensearch.LifecycleConfig{
		Authorizer:         lifecycleAuthorizer,
		Verifier:           lifecycleVerifier,
		CutoverGuard:       applicationWriteFence,
		MutationGuard:      durableLifecycleMutationCoordinator,
		CleanupGuard:       durableCleanupEligibilityGuard,
		ReindexCursorCodec: reindexCursorCodec,
	},
})
if err != nil {
    return err
}
defer func() { _ = client.Close() }()
```

The search authorizer approves the complete logical query, disclosure scope,
and bounded pagination cost intent before the resolver maps
tenant/logical-index access to a safe physical index plus mapping fingerprint.
Opaque cursor bytes are not disclosed to policy. Lifecycle calls use a separate
authorizer, semantic verifier, application-owned write fence, durable
cross-instance mutation coordinator, and encrypted task-cursor codec.
Irreversible deletion additionally requires a durable cleanup-eligibility guard
spanning final generation checks and backend delete while the same mutation
coordinator excludes competing create and alias operations.
`WriteGuard` separately approves every fully cloned write or bulk unit before
physical index resolution. Omitting it intentionally constructs a read-only
client: `Write` and `Bulk` fail with `ErrWriteDisabled` before network I/O.
The guard must consult application-owned durable current documents or
tombstones: OpenSearch delete-version tombstones expire after `index.gc_deletes`
and cannot by themselves prevent an older replay from resurrecting a deleted
projection. A target resolver returns the request alias as `IndexTarget.Name`
and the exact backing generation expected in response metadata as
`IndexTarget.PhysicalName`; update that physical generation atomically inside
the write-fenced alias cutover before writers resume.
Authentication options are mutually exclusive, credentials are resolved for
each request, the adapter does not retry a configured transport invocation,
response bodies are bounded, and borrowed transports remain caller-owned.

`New(Config)` validates and copies mutable configuration before returning; it
does not perform backend I/O or start package-owned background workers. The
client is safe for concurrent use. After an operation observes the closed state
at an admission check, it cannot dispatch a backend request. Some entry points
perform validation or invoke caller-owned authorizers, resolvers, or guards
before that late check, even when the operation starts after `Close`. Operations
that already passed admission may still invoke credentials, signers, clocks, or
observers, dispatch, and finish after `Close` returns. Coordinate application
shutdown before calling `Close`; it does not wait for these operations. Closing
also closes tracked point-in-time state and releases only adapter-owned
transport resources. A borrowed `http.RoundTripper`, credential provider,
signer, resolver, authorizers, guards, and observers remain caller-owned.

Caller-initiated backend work observes the caller context and the configured
`RequestTimeout`. For each adapter-created HTTP request, the adapter invokes the
configured `http.RoundTripper` once and adds no retry around that invocation; a
caller-supplied transport may implement its own internal behavior. One public
operation may create multiple bounded HTTP requests. When `Search` still owns a
PIT, its final deletion deliberately uses `context.WithoutCancel` so caller
cancellation cannot leak backend state. That cleanup receives a fresh
`RequestTimeout`, is awaited before `Search` returns, and exposes any cleanup
failure. When another error already exists the cleanup error is joined with it;
after an otherwise successful short or empty page, cleanup failure discards the
result and is returned as the operation error.

## Semantics

- Cursor searches use point-in-time plus `search_after`; PIT cleanup failures
  are observable, and continuation keep-alive is capped by the signed cursor's
  remaining absolute lifetime. Continuations are single-consumer per client;
  the per-client process-local PIT budget is configured explicitly.
- Writes use external version semantics and preserve partial or unknown bulk
  outcomes.
- Typed queries, filters, sorts, projection, aggregations, highlights,
  suggestions, geo values, and approved locale analyzers are encoded directly.
- Trusted callers may use a bounded `RawExtensionQuery` explicitly bound to
  `opensearch`; unrestricted caller-supplied DSL remains outside the contract.
- Unsupported or unsafe combinations fail before network execution.
- Index create, resumable reindex, verification, fenced alias cutover, rollback
  seam, and cleanup are separately authorized. `CutoverAlias` retains the
  application write fence across final verification and alias mutation.

Use this adapter when the core search contract must target its exact tested
OpenSearch matrix. Do not use it for Elasticsearch, arbitrary OpenSearch
versions, unrestricted user DSL, application data ownership, automatic retry,
or deployment-owned snapshots and backup orchestration.

## Failure, security, and operations

Use `errors.Is` against stable sentinels on every error path. Backend execution,
cancellation, overload, and ambiguous mutation failures may additionally wrap
`*opensearch.Failure`; use `errors.As` to read its operation, category,
retryability, and known-versus-unknown outcome when present. Configuration,
local validation, disabled-feature, and some fail-closed policy errors are
sentinel-only and do not carry `*Failure`. A retryable classification does not
authorize an automatic retry; applications own the total retry budget and must
reconcile unknown writes. Backend bodies, credentials, endpoints, documents,
queries, tenant labels, PIT identifiers, and cursor contents are redacted from
public errors and telemetry.

HTTPS with peer verification is the default. `AllowInsecureHTTP` permits only
explicit loopback HTTP endpoints and is rejected whenever basic credentials or
a signer is configured; it is a local unauthenticated development/test opt-in,
not a deployment mode. Proxying and discovery require explicit bounded policy,
and search, write, and lifecycle authority are separate fail-closed application
seams. Start with the [security guide](docs/security.md), then use
the [deployment and shutdown checklist](docs/operations.md),
[observability and capacity guidance](docs/observability.md), the
[FAQ](docs/faq.md), and [troubleshooting runbook](docs/troubleshooting.md).
Compatibility, upgrades, and backup ownership are documented in
[compatibility](docs/compatibility.md) and [upgrades](docs/upgrades.md).

See the [documentation index](docs/README.md), including deployment, AWS,
security, pagination, migration/rebuild, observability, upgrades, backups, and
compatibility. Observable REST and adapter policy choices are recorded in the
[specification decision register](docs/specification-decisions.md).

For shared package families, selection guidance, ownership, and lifecycle
vocabulary, see the versioned [v1.5.3 Golib ecosystem
index](https://github.com/faustbrian/go-library-tools/blob/v1.5.3/docs/ecosystem/README.md)
and its [Integration and data movement family](https://github.com/faustbrian/go-library-tools/blob/v1.5.3/docs/ecosystem/design-language.md#package-families-and-selection).

## Integration tests

Real-backend tests require an explicitly supplied disposable OpenSearch URL and
never use a running production service. Unit and checked-example tests require
no backend. See [conformance](docs/conformance.md),
[real-cluster testing](docs/real-cluster-testing.md), and
[repository verification](../../CONTRIBUTING.md#verification).

For API details, use [pkg.go.dev](https://pkg.go.dev/github.com/faustbrian/go-search/adapters/opensearch)
and the [public API inventory](docs/api-inventory.md). For help, use the
repository [support policy](../../SUPPORT.md); report vulnerabilities privately
through [SECURITY.md](../../SECURITY.md). Release and migration impact is in the
[adapter changelog](CHANGELOG.md) and repository
[deprecation policy](../../DEPRECATION.md).

## License

MIT. See [LICENSE](LICENSE).

See the [repository documentation](../../docs/README.md) for adoption,
operations, and consistency guidance.
