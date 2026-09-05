# OpenSearch adapter FAQ

## Does the adapter retry requests?

For each adapter-created HTTP request, the adapter invokes the configured
`http.RoundTripper` once and adds no retry around that invocation. A
caller-supplied transport may implement internal retries or other behavior that
the adapter cannot count. One public operation may create multiple separately
bounded requests, such as PIT create, search, and delete or lifecycle polling.
Applications decide whether a classified, idempotent outcome may be retried
within their total work budget. Reconcile an unknown mutation outcome before
changing its version or issuing a non-idempotent follow-up.

## Does every error contain a structured Failure?

No. Use `errors.Is` for stable sentinels on every path. Use `errors.As` to
inspect `*opensearch.Failure` when backend execution, cancellation, overload,
or an ambiguous mutation produces structured operation and outcome details.
Configuration, local validation, disabled-feature, and some fail-closed policy
errors are returned as sentinels without a `*Failure`.

## Can a deployment use plain HTTP?

No supported deployment should. `AllowInsecureHTTP` accepts only an explicit
loopback endpoint and cannot be combined with basic credentials or request
signing. It exists for unauthenticated disposable local tests. Production and
shared environments use peer-verified HTTPS.

## Why can Search return after its caller is cancelled?

If `Search` still owns an OpenSearch point in time, it performs one final
deletion with caller cancellation detached. The deletion receives a fresh
`RequestTimeout`, is awaited before return, and cannot retry automatically.
This bounded cleanup prevents caller cancellation from silently leaking
adapter-owned backend state. If `Search` already has an error, cleanup failure
is joined with it. If a short or empty page otherwise succeeded, cleanup failure
discards that result and becomes the returned operation error.

## Does Close terminate caller-owned resources?

No. Once an operation observes the closed state at an admission check, it cannot
dispatch a backend request. Some new operations can still validate or invoke
caller-owned authorizers, resolvers, or guards before reaching that late check.
Operations already past admission may invoke credentials, signers, clocks, or
observers, dispatch, and finish after `Close` returns; `Close` does not wait for
them. Applications must coordinate shutdown. `Close` also closes tracked
point-in-time ownership and releases idle connections only when transport
ownership was transferred to the adapter. Borrowed transports and application
callbacks remain caller-owned.

For incident symptoms and recovery actions, continue with the
[troubleshooting runbook](troubleshooting.md).
