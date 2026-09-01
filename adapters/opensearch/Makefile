GO ?= go
GOLIB ?= golib
SOAK_DURATION ?= 5s
STRESS_COUNT ?= 10
INTEGRATION_WRITE_BENCH_TIME ?= 1x
INTEGRATION_READ_BENCH_TIME ?= 20x
INTEGRATION_BENCH_COUNT ?= 10

.PHONY: benchmark-integration check clean-consumer conformance docs fault image-pins integration integration-matrix interoperability leak operations rolling-upgrade safety security-matrix soak soak-integration specification-sources stress

check:
	$(GOLIB) check --module adapters/opensearch

benchmark-integration:
	test -n "$${OPENSEARCH_URL:-}" -a -n "$${OPENSEARCH_EXPECTED_VERSION:-}"
	$(GO) test -tags=integration -run='^$$' -bench='^BenchmarkSharedSearchSemantics$$/.*/^(indexing|bulk_indexing)$$' -benchmem -benchtime=$(INTEGRATION_WRITE_BENCH_TIME) -count=$(INTEGRATION_BENCH_COUNT) .
	$(GO) test -tags=integration -run='^$$' -bench='^BenchmarkSharedSearchSemantics$$/.*/^(query|pagination|cursor_pagination)$$' -benchmem -benchtime=$(INTEGRATION_READ_BENCH_TIME) -count=$(INTEGRATION_BENCH_COUNT) .

stress:
	$(GO) test -race -run='^TestEvidenceStress' -count=$(STRESS_COUNT) .

leak:
	$(GO) test -run='^TestEvidenceLeak' -count=$(STRESS_COUNT) .

fault:
	$(GO) test -run='^TestEvidenceFault' -count=$(STRESS_COUNT) .

soak:
	SEARCH_SOAK_DURATION=$(SOAK_DURATION) $(GO) test -run='^TestEvidenceSoak' -count=1 .

soak-integration:
	test -n "$${OPENSEARCH_URL:-}" -a -n "$${OPENSEARCH_EXPECTED_VERSION:-}"
	OPENSEARCH_SOAK_DURATION=$(SOAK_DURATION) $(GO) test -tags=integration -run='^TestRealOpenSearchBoundedLoad$$' -count=1 .

image-pins:
	./scripts/check-image-pins.sh

specification-sources:
	./scripts/check-specification-sources.sh

integration:
	test -n "$${OPENSEARCH_URL:-}" -a -n "$${OPENSEARCH_EXPECTED_VERSION:-}"
	$(GO) test -tags=integration -run TestRealOpenSearchConformance .

integration-matrix: image-pins
	./scripts/test-version-matrix.sh

rolling-upgrade: image-pins
	./scripts/test-rolling-upgrade.sh

security-matrix: image-pins
	./scripts/test-security-matrix.sh

conformance: specification-sources integration-matrix

interoperability: rolling-upgrade

docs:
	test -s README.md
	for file in docs/*.md; do test -s "$$file"; done
	$(GO) test ./...

operations:
	$(GO) test -run='^TestOperationalArtifactsAndIncidentDrills$$' -count=1 .

clean-consumer:
	./scripts/check-clean-consumer.sh

safety:
	! rg -n --glob '*.go' --glob '!*_test.go' '(^|[[:space:]])"unsafe"|//go:linkname|import "C"' .
