package opensearch_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/faustbrian/go-search"
	adapter "github.com/faustbrian/go-search/adapters/opensearch"
)

type fuzzResponseTransport struct{ body string }

func (transport fuzzResponseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(transport.body))}, nil
}

// FuzzInfoResponseBoundary supplies at most 8192 synthetic body bytes to the
// 4096-byte Info decoder bound. It checks panic resistance with no network service;
// known malformed/sanitization outcomes remain deterministic-test assertions.
func FuzzInfoResponseBoundary(f *testing.F) {
	f.Add(`{"name":"node","cluster_name":"cluster","cluster_uuid":"uuid","version":{"number":"3.2.0"}}`)
	f.Add(`{}`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			t.Skip()
		}
		client, err := adapter.New(adapter.Config{Endpoints: []string{"http://127.0.0.1:9200"}, AllowInsecureHTTP: true, Transport: fuzzResponseTransport{body: body}, TransportOwnership: adapter.TransportBorrowed, RequestTimeout: time.Second, MaximumResponseBytes: 4096})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = client.Info(t.Context())
		_ = client.Close()
	})
}

// FuzzSearchResponseBoundary supplies bounded synthetic response bytes to an
// authorized offset search with a 4096-byte response cap. It checks panic resistance
// at translation/decode, not real-server ranking or cursor/PIT interoperability.
func FuzzSearchResponseBoundary(f *testing.F) {
	f.Add(`{"took":1,"timed_out":false,"_shards":{"total":1,"successful":1,"skipped":0,"failed":0},"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}}`)
	f.Add(`{"hits":{"total":{"value":18446744073709551616,"relation":"eq"},"hits":[{"_source":{"secret":"credential"}}]}}`)
	f.Add(`{"took":0,"timed_out":false,"_shards":{"total":1,"successful":1,"failed":0},"hits":{"total":{"value":0,"relation":"eq"},"hits":[]}} trailing`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			t.Skip()
		}
		client := fuzzSearchClient(t, body, nil)
		_, _ = client.Search(t.Context(), search.Request{
			Tenant: "tenant", Index: "documents", Query: search.MatchAllQuery{},
			Sort: []search.Sort{{Field: search.DocumentIDSortField, Direction: search.Ascending}},
			Page: search.OffsetPage{Size: 10},
		})
	})
}

// FuzzBulkResponseBoundary probes bounded synthetic positional bulk responses.
// It requires enabled write admission and validates request attribution whenever
// Bulk accepts a response; it does not prove backend receipt or retry safety.
func FuzzBulkResponseBoundary(f *testing.F) {
	f.Add(`{"took":1,"errors":false,"items":[{"index":{"_index":"documents-v1","_id":"id","_version":1,"status":201,"result":"created"}}]}`)
	f.Add(`{"took":-1,"errors":false,"items":[]}`)
	f.Add(`{"took":1,"errors":true,"items":[{"index":{"_id":"other","_version":1,"status":200,"error":{"type":"credential-value"}}}]}`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			t.Skip()
		}
		client := fuzzSearchClient(t, body, nil)
		limits := search.DefaultLimits()
		document, err := search.NewDocument("tenant", "documents", "id", 1, json.RawMessage(`{"name":"value"}`), limits)
		if err != nil {
			t.Fatal(err)
		}
		request := search.BulkRequest{Operations: []search.WriteOperation{search.IndexDocument(document)}, Refresh: search.RefreshWaitFor}
		result, err := client.Bulk(t.Context(), request)
		if errors.Is(err, adapter.ErrWriteDisabled) {
			t.Fatal("bulk response fuzzing stopped before response decoding")
		}
		if err == nil {
			if validationErr := result.ValidateRequest(request); validationErr != nil {
				t.Fatalf("accepted bulk response lost request attribution: %v", validationErr)
			}
		}
	})
}

// FuzzLifecycleResponseBoundary constructs bounded synthetic lifecycle responses.
// A synchronous fixture guard admits the callback so seeds and mutations reach
// response decoding. This synthetic guard does not model durable exclusion;
// deterministic cases establish successful and malformed response outcomes.
func FuzzLifecycleResponseBoundary(f *testing.F) {
	f.Add(`{"acknowledged":true,"shards_acknowledged":true}`)
	f.Add(`{"acknowledged":true,"shards_acknowledged":false}`)
	f.Add(`{"acknowledged":true,"shards_acknowledged":true}{}`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			t.Skip()
		}
		client := fuzzSearchClient(t, body, &adapter.LifecycleConfig{Authorizer: adapter.LifecycleAuthorizerFunc(func(context.Context, string, []string) error { return nil }), MutationGuard: allowLifecycleMutationGuard()})
		limits := search.DefaultLimits()
		definition, err := search.NewIndexDefinition("documents-v1", json.RawMessage(`{}`), json.RawMessage(`{"properties":{"name":{"type":"keyword"}}}`), limits)
		if err != nil {
			t.Fatal(err)
		}
		err = client.CreateIndex(t.Context(), "tenant", definition)
		if errors.Is(err, adapter.ErrLifecycleMutationGuardRequired) {
			t.Fatal("lifecycle response fuzzing stopped before mutation admission and response decoding")
		}
	})
}

// fuzzSearchClient owns a deterministic borrowed synthetic transport and cleanup.
// Search/write authorization permits reaching those decoders; lifecycle policies
// are retained as supplied rather than silently adding a durable mutation guard.
func fuzzSearchClient(t *testing.T, body string, lifecycle *adapter.LifecycleConfig) *adapter.Client {
	t.Helper()
	codec, err := search.NewCursorCodec([]byte("0123456789abcdef0123456789abcdef"), time.Now, 4096)
	if err != nil {
		t.Fatal(err)
	}
	client, err := adapter.New(adapter.Config{
		Endpoints: []string{"http://127.0.0.1:9200"}, AllowInsecureHTTP: true,
		Transport: fuzzResponseTransport{body: body}, TransportOwnership: adapter.TransportBorrowed,
		RequestTimeout: time.Second, MaximumResponseBytes: 4096,
		Search: &adapter.SearchConfig{
			Limits: search.DefaultLimits(), CursorCodec: codec, Authorizer: allowSearchAuthorization(), WriteGuard: allowWriteAuthorization(),
			Resolver: adapter.IndexResolverFunc(func(context.Context, string, string, adapter.IndexAccess) (adapter.IndexTarget, error) {
				return adapter.IndexTarget{Name: "documents-v1", PhysicalName: "documents-v1", Fingerprint: "mapping-v1"}, nil
			}),
		},
		Lifecycle: lifecycle,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}
