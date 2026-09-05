# Pagination and cursor ownership

Offset pagination is bounded and targets the resolved read alias directly. Use
it only for shallow, non-snapshot result windows.

Cursor pagination uses an OpenSearch point in time (PIT) plus `search_after`.
The final sort must be stable and end in `_id`. The signed cursor binds tenant,
logical index, normalized request fingerprint, physical mapping fingerprint,
PIT ID, sort values, page/item/byte totals, and expiry. A cursor from another
tenant, query, index generation, or signing key is rejected before search.
After PIT creation is acknowledged, the cursor codec fixes an absolute deadline
bounded by `Limits.MaxCursorDuration`. The codec clock is authoritative; the
deprecated adapter `SearchConfig.Clock` is ignored. Time spent querying and
consuming pages uses that budget. Every continuation sends only the whole-
millisecond lifetime remaining until the signed deadline, so backend keep-alive
cannot slide beyond cursor validity.

The first cursor request creates a PIT. A full page transfers PIT ownership to
the returned cursor. A short/empty page closes it. Any failure while the
adapter still owns the PIT triggers cleanup; final cleanup failures are returned
instead of hidden. A lost or malformed cleanup acknowledgement is an unknown
outcome and is not retried automatically. Abandoned cursors live until their
configured OpenSearch keep-alive, so keep that duration short.
PIT cleanup deliberately detaches from caller cancellation with
`context.WithoutCancel`, retains context values, and receives a fresh
adapter-level `RequestTimeout`. `Search` waits for that bounded deletion before
returning. A cleanup failure joins an existing search error. After an otherwise
successful short or empty page, however, cleanup failure discards the result and
is returned as the operation error. The local PIT lease remains retained after
deletion failure because the backend deletion outcome is unknown; it continues
to count against `MaximumOpenPointInTimes` until its signed expiry is reaped by
a later reserve, acquire, or snapshot operation, or until `Close` clears the
tracker. The backend PIT may remain until its keep-alive expires. This exception
applies only to adapter-owned final cleanup. Credential-provider and signer
callbacks needed for that delete receive the detached, freshly bounded cleanup
context; search authorization and resolution callbacks use the original caller
context.
`MaximumOpenPointInTimes` bounds leases owned or adopted by one client process;
`PointInTimeSnapshot` exposes only that process-local aggregate to operators.
It excludes cursors owned by other instances and must not be exposed as
tenant-facing metrics. The in-memory fake declares PIT and cursor semantics
unsupported and has no matching admission tracker.

One cursor continuation is single-consumer per client. Concurrent replay on the
same client fails explicitly with `ErrPointInTimeInUse`; applications routing a
cursor across instances must supply aggregate distributed admission and
single-consumer coordination because local trackers cannot observe each other.

PIT expiry is an expected classified failure. Restart traversal from the first
page if policy permits; never splice a new PIT into an old cursor. Rotate cursor
signing keys with an explicit overlap or invalidation plan, and never log cursor
contents.

Lifecycle reindex continuation uses a separate opaque cursor. It encrypts the
backend task ID, tenant, physical source, physical target, and expiry. Each
successful incomplete poll returns a newly encrypted cursor with a renewed
bounded lease; callers must durably replace the previous token.
Tampering, cross-tenant or cross-generation reuse, expiry, unsafe task paths,
and configured byte-limit violations fail before task polling. It does not
share the search PIT cursor format or signing lifecycle.
