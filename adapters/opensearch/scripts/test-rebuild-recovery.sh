#!/bin/sh
set -eu

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=opensearch-images.env
. "$script_dir/opensearch-images.env"
# shellcheck source=docker-test-ownership.sh
. "$script_dir/docker-test-ownership.sh"
adapter_dir=$(CDPATH='' cd -- "$script_dir/.." && pwd)
root=$(CDPATH='' cd -- "$adapter_dir/../.." && pwd)
cd "$adapter_dir"

# One bounded, optional hosted campaign; never inherit a workspace or warm cache.
task_root=$(mktemp -d "${TMPDIR:-/tmp}/golib-search-rebuild.XXXXXX")
opensearch_run_id="rebuild-${task_root##*/}"
container=''
image=''
remove_image=0
cleanup() {
	status=$?
	trap - EXIT HUP INT TERM
	if [ -n "$container" ]; then
		opensearch_remove_owned_if_present container "$container" || status=1
	fi
	if [ "$remove_image" -eq 1 ] && [ -n "$image" ]; then
		docker image rm "$image" >/dev/null || status=1
	fi
	chmod -R u+w "$task_root" || status=1
	find "$task_root" -depth -delete || status=1
	exit "$status"
}
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' HUP TERM
mkdir -p "$task_root/build" "$task_root/modules" "$task_root/tmp"
export GOCACHE="$task_root/build" GOMODCACHE="$task_root/modules" GOTMPDIR="$task_root/tmp"
export TMPDIR="$task_root/tmp" GOENV=off
export GOWORK=off GOTOOLCHAIN=local GOFLAGS='-mod=readonly -p=2' GOMAXPROCS=2
export GOPROXY=https://proxy.golang.org GOSUMDB=sum.golang.org GOPRIVATE='' GONOPROXY='' GONOSUMDB=''
test "$(go env GOVERSION)" = "go$(cat "$root/.go-version")"
go version
go list -m github.com/faustbrian/go-search github.com/opensearch-project/opensearch-go/v4
go mod verify
"$script_dir/check-image-pins.sh"

for release in old new; do
	case "$release" in
	old) version=$opensearch_old_version; digest=$opensearch_old_digest ;;
	new) version=$opensearch_new_version; digest=$opensearch_new_digest ;;
	esac
	image="$opensearch_image_repository@$digest"
	remove_image=0
	if ! docker image inspect "$image" >/dev/null 2>&1; then
		remove_image=1
	fi
	container="golib-opensearch-$opensearch_run_id-$release"
	docker run -d --name "$container" --label "$opensearch_owner_label_key=$opensearch_run_id" \
		-p 127.0.0.1::9200 --cpus=1 --memory=1g --pids-limit=512 --ulimit nofile=1024:1024 \
		-e discovery.type=single-node -e DISABLE_SECURITY_PLUGIN=true \
		-e OPENSEARCH_JAVA_OPTS='-Xms384m -Xmx384m' "$image" >/dev/null
	port=$(docker port "$container" 9200/tcp | sed -n 's/.*://p')
	ready=0
	for _ in $(seq 1 120); do
		if curl --connect-timeout 2 --max-time 3 --fail --silent "http://127.0.0.1:$port/" >/dev/null; then
			ready=1
			break
		fi
		sleep 1
	done
	if [ "$ready" -ne 1 ]; then
		printf 'disposable OpenSearch %s did not become ready\n' "$version" >&2
		exit 1
	fi
	printf 'process-loss rebuild: OpenSearch %s@%s\n' "$version" "$digest"
	OPENSEARCH_URL="http://127.0.0.1:$port" OPENSEARCH_EXPECTED_VERSION="$version" \
		go test -tags=integration -run '^TestRealOpenSearchConformanceRebuildReconciliationRollbackAndConcurrentApplications$' \
		-count=1 -timeout=5m -v .
	opensearch_assert_container_limits "$container"
	opensearch_remove_owned container "$container"
	container=''
	if [ "$remove_image" -eq 1 ]; then
		docker image rm "$image" >/dev/null
	fi
	image=''
	remove_image=0
done
