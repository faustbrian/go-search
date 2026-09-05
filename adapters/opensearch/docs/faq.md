# OpenSearch adapter FAQ

## Does the adapter retry requests?

No. Each adapter operation makes one bounded attempt. Applications decide
whether a classified, idempotent outcome may be retried within their total work
budget. Reconcile an unknown mutation outcome before changing its version or
issuing a non-idempotent follow-up.

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
adapter-owned backend state; its failure is joined into the returned error.

## Does Close terminate caller-owned resources?

No. `Close` rejects new work, closes tracked point-in-time ownership, and
releases idle connections only when transport ownership was transferred to the
adapter. Borrowed transports and application callbacks remain caller-owned.

For incident symptoms and recovery actions, continue with the
[troubleshooting runbook](troubleshooting.md).
