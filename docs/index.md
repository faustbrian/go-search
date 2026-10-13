# search documentation map

[Repository README](../README.md) · [Current Golib portal](https://github.com/faustbrian/go-library-tools/blob/main/docs/ecosystem/README.md) · [Published ecosystem snapshot](https://github.com/faustbrian/go-library-tools/blob/v1.8.4/docs/ecosystem/README.md)

This map links package-owned adoption and operational guidance. Start with
the repository README for purpose, installation, and ownership boundaries.
Published compatibility sets are frozen multi-package baselines; they do not
certify every current main revision or a deployed application.

## Module boundaries

Import only the modules needed by your application. A deprecated facade,
preview adapter, or internal harness is not interchangeable with a stable
root API; follow its own README and migration guidance before composing it.

| Module | Status | Owner detail |
| --- | --- | --- |
| `github.com/faustbrian/go-search` | stable | [README](../README.md) |
| `github.com/faustbrian/go-search/adapters/opensearch` | stable | [README](../adapters/opensearch/README.md) |

## Choose and adopt

- [Documentation](README.md).
- [Track and Location adoption](adoption.md).
- [API and ownership](api.md).
- [Bulk ingestion and rebuilds](bulk-and-rebuild.md).
- [Consistency and source of truth](consistency.md).
- [FAQ](faq.md).
- [Pagination](pagination.md).
- [PostgreSQL or search](postgresql-vs-search.md).
- [Schema migrations](schema-migrations.md).

## Operate and evaluate boundaries

- [Operations and capacity](operations.md).
- [Security](security.md).
- [Search threat model](threat-model.md).

## Owned adapters, examples, and harnesses

- [OpenSearch adapter](../adapters/opensearch/README.md).
- [OpenSearch adapter documentation](../adapters/opensearch/docs/README.md).

## Maintainer, specification, and evidence references

- [OpenSearch specification conformance](../adapters/opensearch/specification/README.md).
- [Source documentation audit](source-documentation-audit.md).

## Related next steps

[Contributor setup and checks](../CONTRIBUTING.md), [Compatibility policy](../COMPATIBILITY.md), [Release history](../CHANGELOG.md), [Private vulnerability reporting](../SECURITY.md), [Support](../SUPPORT.md).
