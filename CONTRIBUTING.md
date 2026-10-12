# Contributing

## Before Editing

1. Read [`AGENTS.md`](AGENTS.md) and the affected module's docs.
2. Run standalone Go tests in the affected module.
3. Identify owned dependencies and reverse dependants in `modules.json`.
4. Preserve unrelated work and generated/corpus provenance.

## Changes

Keep commits focused and conventional. Update every affected changelog with
the behavior and migration impact. Public API changes require compatibility
evidence and documentation. Specification behavior requires a decision record,
fixture coverage, and interoperability evidence.

New direct dependencies and dependency updates must follow the
[dependency governance policy](AGENTS.md#dependencies-and-supply-chain). Package-local
update bots are forbidden; the root policy owns every module and action update.

Specification-backed changes must follow the
[specification governance contract](AGENTS.md#design), update
the affected stable decision entries, and complete the Specification Decisions
section of the pull request template. An unresolved interpretation or stale
source pin is release-blocking; peer behavior cannot silently select policy.
OpenSearch adapter changes must also update its
[decision register](adapters/opensearch/docs/specification-decisions.md).

Full scheduled and release mutation gates must finish with zero surviving viable
mutants. Local mutation is optional, not a universal contribution prerequisite.

Do not add package-local workflows, permanent replacements, machine-specific
paths, bypass flags, broad mutation exclusions, or aggregate quality metrics
that hide a failing package.

## Verification

Standalone tests need no sibling checkout or ecosystem goal ledger. Run each
intentional module independently; untagged adapter tests use deterministic
transports, not a live engine:

```bash
GOWORK=off go test ./...
(cd adapters/opensearch && GOWORK=off go test ./...)
```

These tests include fuzz seeds and saved corpus regressions. With the
checksum-pinned public verifier selected by `.golib.yaml`, use:

```bash
make inventory
make check
make ci
```

`check` and `ci` select bounded local checks. The adapter manifest still requires
OpenSearch and the `integration` test tag: the service-aware Make route may
require Docker and is not equivalent to the untagged commands above. Ordinary
CI consumes the public verifier binary. The adapter's bounded race selection
checks PIT duplicate ownership/concurrent use, idempotent release after close
and reentrant clock callbacks without broad stress campaigns.

Short active exploration selects one target; optional local mutation selects
one package:

```bash
GOMAXPROCS=2 GOWORK=off go test -run '^$' -fuzz '^FuzzCursorDecode$' -fuzztime=2s -parallel=1 -timeout=2m .
golib mutation --module . --package .
```

`make check-full` retains full assurance for both modules: all eight native
fuzz campaigns, root and adapter contracts, repeated stress/leak/fault/soak,
clean consumers, benchmarks, conformance and rolling-upgrade interoperability.
The adapter Makefile also provides bounded `check` and explicit `check-full`.
Backend changes retain the exact-version [real-cluster contracts](adapters/opensearch/docs/real-cluster-testing.md),
including security, mixed-version peers and benchmark sampling. Tagged live
tests require a disposable `OPENSEARCH_URL` and matching
`OPENSEARCH_EXPECTED_VERSION`; never use a shared or production service.
Public API changes retain root/adapter compatibility and direct-consumer
checks; release rehearsal is `golib release dry-run --all`. A local pass is not
full assurance, hosted-CI or release qualification. Report unavailable, skipped
and failed boundaries honestly.

## Adding A Module

Follow [repository structure policy](AGENTS.md#repository-structure). New modules
require an explicit purpose, ownership boundary, dependency review, package
catalog entry, full quality gates, documentation, changelog, license, security
policy, compatibility plan, and release dry-run.

## Source Documentation

Install the checksum-pinned public verifier declared in `.golib.yaml`.
`make source-docs-check` validates objective Go comment, marker, link and
generated-producer structure; `make check`, `make check-full` and `make ci`
require it before their existing checks. Shared CI runs the same strict
contract and includes its result in Required. This is structural enforcement,
not a substitute for source-truth review or behavioral verification.
