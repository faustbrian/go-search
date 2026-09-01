GO ?= go
STRESS_COUNT ?= 10
SOAK_DURATION ?= 5s

.PHONY: opensearch-conformance opensearch-contract opensearch-docs opensearch-interoperability root-contract root-docs

root-contract:
	./scripts/check-coverage.sh
	$(GO) test -race -run '^TestEvidenceStress' -count=$(STRESS_COUNT) ./searchtest
	$(GO) test -run '^TestEvidenceFault' -count=$(STRESS_COUNT) ./searchtest
	./scripts/check-clean-consumer.sh

root-docs:
	test -s README.md
	for file in docs/*.md; do test -s "$$file"; done
	$(GO) test ./...

opensearch-contract:
	$(MAKE) -C adapters/opensearch stress leak fault soak image-pins specification-sources operations clean-consumer safety GO=$(GO) STRESS_COUNT=$(STRESS_COUNT) SOAK_DURATION=$(SOAK_DURATION)

opensearch-conformance:
	$(MAKE) -C adapters/opensearch conformance security-matrix GO=$(GO)

opensearch-docs:
	$(MAKE) -C adapters/opensearch docs GO=$(GO)

opensearch-interoperability:
	$(MAKE) -C adapters/opensearch interoperability GO=$(GO)
