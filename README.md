# search

[![CI](https://github.com/faustbrian/go-search/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/faustbrian/go-search/actions/workflows/ci.yml)
[![CodeQL](https://img.shields.io/badge/CodeQL-required-blue)](https://github.com/faustbrian/go-search/actions/workflows/ci.yml)
[![Coverage](https://img.shields.io/badge/coverage-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Mutation](https://img.shields.io/badge/mutation-100%25_required-blue)](CONTRIBUTING.md#verification)
[![Documentation](https://img.shields.io/badge/docs-checked_in_CI-blue)](docs/)
[![Go Reference](https://pkg.go.dev/badge/github.com/faustbrian/go-search.svg)](https://pkg.go.dev/github.com/faustbrian/go-search)
[![Release](https://img.shields.io/github/v/release/faustbrian/go-search?sort=semver)](https://github.com/faustbrian/go-search/releases)
[![Go](https://img.shields.io/badge/go-1.26.6-00ADD8?logo=go)](https://go.dev/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`search` provides backend-neutral contracts for bounded document indexing,
typed querying, cursor pagination, schema migration, and reconciliation. Search
indexes are rebuildable derived state; an application datastore remains the
source of truth.

The core module has no search-engine dependency. The first production adapter
is [`adapters/opensearch`](adapters/opensearch/README.md). Backend capabilities
are explicit: unsupported typed features fail validation instead of silently
degrading to a query string.

`search` is a stable v1 module. It supports Go 1.26.6 on every platform
supported by that toolchain and has no required service or runtime dependency.

## Install

```sh
go get github.com/faustbrian/go-search@v1.0.0
```

Import the core contract as `github.com/faustbrian/go-search`. Test code may
also import `github.com/faustbrian/go-search/searchtest`. Applications that
need OpenSearch install its [independently versioned adapter](adapters/opensearch/README.md).

## Quick start

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    "github.com/faustbrian/go-search"
    "github.com/faustbrian/go-search/searchtest"
)

func main() {
    ctx := context.Background()
    limits := search.DefaultLimits()
    engine, err := searchtest.NewFake(limits)
    if err != nil {
        log.Fatal(err)
    }
    document, err := search.NewDocument(
        "tenant-a", "locations", "hel", 1,
        json.RawMessage(`{"country":"FI"}`), limits,
    )
    if err != nil {
        log.Fatal(err)
    }
    if _, err = engine.Write(
        ctx, search.IndexDocument(document), search.RefreshWaitFor,
    ); err != nil {
        log.Fatal(err)
    }
    result, err := engine.Search(ctx, search.Request{
        Tenant: "tenant-a",
        Index:  "locations",
        Query:  search.TermQuery{Field: "country", Value: search.StringValue("FI")},
        Sort:   []search.Sort{{Field: search.DocumentIDSortField, Direction: search.Ascending}},
        Page:   search.OffsetPage{Size: 10},
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Hits()[0].ID)
}
```

Every document has a stable ID and positive external version. Every bulk item
retains its own applied, rejected, conflict, or unknown outcome. Returned
documents, fields, diagnostics, aggregations, and suggestions are owned copies.

## Adoption

- Use PostgreSQL full-text search when the relational source of truth, simple
  ranking, and transactional consistency are more important than independent
  search scaling or engine analyzers.
- Use this module when Track or Location needs typed full-text, geo, facets,
  highlighting, suggestions, cursor consistency, or independently rebuilt
  indexes.
- Keep raw backend extensions behind trusted application policy. Never accept
  unrestricted raw DSL from an untrusted caller.

Do not use this module as the source of truth, a deployment manager, a hidden
retry layer, or a portable ranking promise. The application owns durable data,
authorization, retry budgets, ranking acceptance, and backend selection.

## Contract

- **Construction and configuration:** value constructors validate before
  adapter I/O. `DefaultLimits` is the documented safe non-zero profile; all
  `Limits` fields are required when callers provide another profile. Input and
  returned ownership rules are detailed in the [API guide](docs/api.md).
- **Context and lifecycle:** every blocking interface accepts the caller's
  `context.Context`. Core values own no resources or background goroutines.
  `Migrator.Run` and `Reconciler.Run` are caller-bounded operations; adapter
  resources retain their own explicit cleanup contract.
- **Errors and outcomes:** use `errors.Is` for exported semantic sentinels.
  Bulk operations retain per-item applied, rejected, conflict, failed, or
  unknown outcomes. Unknown external outcomes require reconciliation before a
  changed-version retry. See [API and ownership](docs/api.md) and
  [consistency](docs/consistency.md).
- **Concurrency:** immutable values and returned data are owned copies unless
  the API guide explicitly identifies a borrowed input. `searchtest.Fake` is
  safe for concurrent use. Application callbacks, durable stores, and backend
  clients retain their documented synchronization ownership.
- **Security:** identifiers and hostile JSON are bounded, raw extensions are
  explicit, and sensitive payloads must not cross error or observation
  boundaries. See the [security guide](docs/security.md).

## Packages and integrations

- [`search`](https://pkg.go.dev/github.com/faustbrian/go-search) owns the
  backend-neutral contracts.
- [`searchtest`](https://pkg.go.dev/github.com/faustbrian/go-search/searchtest)
  provides a bounded deterministic fake and reusable conformance helpers; it
  does not emulate backend ranking or analyzers.
- [`adapters/opensearch`](adapters/opensearch/README.md) is the optional
  OpenSearch implementation and the only production adapter currently
  published by this repository.

See the [documentation index](docs/README.md), [API guide](docs/api.md),
[operations guide](docs/operations.md), and [FAQ](docs/faq.md).

Compatibility and migration policy are documented in [COMPATIBILITY.md](COMPATIBILITY.md)
and [DEPRECATION.md](DEPRECATION.md). Capacity and performance are workload
specific; use the budgets in the [operations guide](docs/operations.md) and do
not treat fake results as backend benchmarks. For problems, consult the
[FAQ](docs/faq.md), then use [support](SUPPORT.md). Report vulnerabilities
privately through [SECURITY.md](SECURITY.md). See [examples](docs/adoption.md),
[testing guidance](CONTRIBUTING.md#verification), and the
[changelog](CHANGELOG.md).

## Development

```sh
make check
```

The `golib` executor creates task-owned disposable Go caches for its checks.
The module gate enforces exact statement coverage and supports race, fuzz,
mutation, benchmark, API, and security checks.

For shared package families, selection guidance, ownership, and lifecycle
vocabulary, see the versioned [v1.5.3 Golib ecosystem
index](https://github.com/faustbrian/go-library-tools/blob/v1.5.3/docs/ecosystem/README.md)
and its [Integration and data movement family](https://github.com/faustbrian/go-library-tools/blob/v1.5.3/docs/ecosystem/design-language.md#package-families-and-selection).

## License

MIT. See [LICENSE](LICENSE).
