#!/bin/sh
set -eu

root=$(git rev-parse --show-toplevel)
consumer=$(mktemp -d "${TMPDIR:-/tmp}/search-opensearch-consumer.XXXXXX")
trap 'find "$consumer" -depth -delete' EXIT HUP INT TERM
cd "$consumer"
go mod init example.com/search-opensearch-consumer >/dev/null
go mod edit -replace github.com/faustbrian/go-search="$root/pkg/search"
go mod edit -replace github.com/faustbrian/go-search/adapters/opensearch="$root/adapters/opensearch"
go get github.com/faustbrian/go-search/adapters/opensearch@v0.0.0 >/dev/null
printf '%s\n' \
	'package consumer' \
	'import adapter "github.com/faustbrian/go-search/adapters/opensearch"' \
	'var _ = adapter.SupportedOpenSearchVersions' >consumer.go
GOWORK=off go test ./...
