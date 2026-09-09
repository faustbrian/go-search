package opensearch_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/faustbrian/go-search"
	"github.com/faustbrian/go-search/adapters/opensearch"
)

func TestTrackLocationProjectionRecipe(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	if err := runTrackLocationProjection(t.Context(), &output); err != nil {
		t.Fatal(err)
	}
	const want = "applied tracking-123 42\nversion_conflict tracking-123 42\ntracking-123 Helsinki\n"
	if output.String() != want {
		t.Fatalf("output = %q, want %q", output.String(), want)
	}
}

func ExampleClient_trackLocationProjection() {
	var output bytes.Buffer
	if err := runTrackLocationProjection(context.Background(), &output); err != nil {
		panic(err)
	}
	fmt.Print(output.String())
	// Output:
	// applied tracking-123 42
	// version_conflict tracking-123 42
	// tracking-123 Helsinki
}

func runTrackLocationProjection(ctx context.Context, output io.Writer) error {
	const (
		tenant        = "tenant-a"
		logicalIndex  = "tracking-projections"
		readAlias     = "tenant-a-tracking-projections-read"
		writeAlias    = "tenant-a-tracking-projections-write"
		physicalIndex = "tenant-a-tracking-projections-v1"
	)

	limits := search.DefaultLimits()
	cursorCodec, err := search.NewCursorCodec(
		[]byte("0123456789abcdef0123456789abcdef"), time.Now, 4096,
	)
	if err != nil {
		return err
	}
	backend := &projectionTransport{}
	client, err := opensearch.New(opensearch.Config{
		Endpoints:            []string{"https://search.example.internal:9200"},
		Transport:            backend,
		TransportOwnership:   opensearch.TransportOwned,
		RequestTimeout:       time.Second,
		MaximumResponseBytes: 16 << 10,
		Search: &opensearch.SearchConfig{
			Limits:      limits,
			CursorCodec: cursorCodec,
			Resolver: opensearch.IndexResolverFunc(func(_ context.Context, gotTenant, gotIndex string, access opensearch.IndexAccess) (opensearch.IndexTarget, error) {
				if gotTenant != tenant || gotIndex != logicalIndex {
					return opensearch.IndexTarget{}, errors.New("projection target denied")
				}
				var alias string
				switch access {
				case opensearch.IndexRead:
					alias = readAlias
				case opensearch.IndexWrite:
					alias = writeAlias
				default:
					return opensearch.IndexTarget{}, errors.New("projection access denied")
				}
				return opensearch.IndexTarget{
					Name: alias, PhysicalName: physicalIndex,
					Fingerprint: "tracking-projection-v1",
				}, nil
			}),
			Authorizer: opensearch.SearchAuthorizerFunc(func(_ context.Context, authorization opensearch.SearchAuthorization) error {
				if authorization.Tenant != tenant || authorization.Index != logicalIndex {
					return errors.New("projection query denied")
				}
				return nil
			}),
			WriteGuard: opensearch.WriteGuardFunc(func(_ context.Context, authorization opensearch.WriteAuthorization) error {
				operations := authorization.Operations()
				if len(operations) != 1 || operations[0].Tenant != tenant || operations[0].Index != logicalIndex {
					return errors.New("projection write denied")
				}
				return nil
			}),
		},
	})
	if err != nil {
		return err
	}

	closed := false
	defer func() {
		if !closed {
			_ = client.Close()
		}
	}()

	event, err := search.NewProjectionEvent(
		tenant,
		logicalIndex,
		"tracking-123",
		42,
		search.ProjectionUpsert,
		json.RawMessage(`{"tracking_id":"tracking-123","status":"in_transit","location":{"code":"HEL","name":"Helsinki"}}`),
		"tracking-123:42",
		limits,
	)
	if err != nil {
		return err
	}
	consumer, err := search.NewProjectionConsumer(client)
	if err != nil {
		return err
	}
	outcome, err := consumer.Handle(ctx, event, search.RefreshWaitFor)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(output, "%s %s %d\n", outcome.State, outcome.ID, outcome.Version)

	duplicate, duplicateErr := consumer.Handle(ctx, event, search.RefreshWaitFor)
	if !errors.Is(duplicateErr, opensearch.ErrVersionConflict) || duplicate.State != search.OutcomeVersionConflict {
		return fmt.Errorf("duplicate projection outcome: %s: %w", duplicate.State, duplicateErr)
	}
	_, _ = fmt.Fprintf(output, "%s %s %d\n", duplicate.State, duplicate.ID, duplicate.Version)

	result, err := client.Search(ctx, search.Request{
		Tenant: tenant,
		Index:  logicalIndex,
		Query: search.TermQuery{
			Field: "location.code", Value: search.StringValue("HEL"),
		},
		Sort: []search.Sort{{
			Field: search.DocumentIDSortField, Direction: search.Ascending,
		}},
		Page: search.OffsetPage{Size: 10},
	})
	if err != nil {
		return err
	}
	hits := result.Hits()
	if len(hits) != 1 {
		return fmt.Errorf("projection hits: got %d, want 1", len(hits))
	}
	var projection struct {
		Location struct {
			Name string `json:"name"`
		} `json:"location"`
	}
	if err := json.Unmarshal(hits[0].Source, &projection); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(output, "%s %s\n", hits[0].ID, projection.Location.Name)

	if err := client.Close(); err != nil {
		return err
	}
	closed = true
	if !backend.closedIdleConnections() {
		return errors.New("owned transport was not closed")
	}
	if _, err := client.Capabilities(ctx); !errors.Is(err, opensearch.ErrClosed) {
		return fmt.Errorf("post-close capability error = %v, want ErrClosed", err)
	}
	return nil
}

// projectionTransport is a bounded HTTP seam for the public adapter contract.
// It intentionally implements only the two requests exercised by this recipe;
// real backend semantics remain covered by the adapter's conformance matrix.
type projectionTransport struct {
	mu      sync.Mutex
	version uint64
	source  json.RawMessage
	closed  bool
}

func (transport *projectionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.mu.Lock()
	defer transport.mu.Unlock()

	switch {
	case request.Method == http.MethodPut && strings.HasPrefix(request.URL.Path, "/tenant-a-tracking-projections-write/_doc/"):
		return transport.write(request)
	case request.Method == http.MethodPost && request.URL.Path == "/tenant-a-tracking-projections-read/_search":
		return transport.search(request)
	default:
		return projectionJSONResponse(http.StatusNotFound, `{"error":{"type":"route_not_found"}}`), nil
	}
}

func (transport *projectionTransport) write(request *http.Request) (*http.Response, error) {
	version, err := strconv.ParseUint(request.URL.Query().Get("version"), 10, 64)
	if err != nil || request.URL.Query().Get("version_type") != "external" ||
		request.URL.Query().Get("refresh") != "wait_for" || request.URL.Query().Get("require_alias") != "true" {
		return projectionJSONResponse(http.StatusBadRequest, `{"error":{"type":"illegal_argument_exception"}}`), nil
	}
	id, err := url.PathUnescape(strings.TrimPrefix(request.URL.Path, "/tenant-a-tracking-projections-write/_doc/"))
	if err != nil {
		return nil, err
	}
	if version <= transport.version {
		body := fmt.Sprintf(`{"_index":"tenant-a-tracking-projections-v1","_id":%q,"_version":%d,"error":{"type":"version_conflict_engine_exception"}}`, id, transport.version)
		return projectionJSONResponse(http.StatusConflict, body), nil
	}
	source, err := io.ReadAll(io.LimitReader(request.Body, 16<<10))
	if err != nil {
		return nil, err
	}
	transport.version = version
	transport.source = append(json.RawMessage(nil), source...)
	body := fmt.Sprintf(`{"_index":"tenant-a-tracking-projections-v1","_id":%q,"_version":%d,"result":"created"}`, id, version)
	return projectionJSONResponse(http.StatusCreated, body), nil
}

func (transport *projectionTransport) search(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(io.LimitReader(request.Body, 16<<10))
	if err != nil {
		return nil, err
	}
	if !bytes.Contains(body, []byte(`"location.code"`)) || !bytes.Contains(body, []byte(`"HEL"`)) {
		return projectionJSONResponse(http.StatusBadRequest, `{"error":{"type":"query_not_supported"}}`), nil
	}
	response := fmt.Sprintf(`{"took":1,"timed_out":false,"_shards":{"total":1,"successful":1,"skipped":0,"failed":0},"hits":{"total":{"value":1,"relation":"eq"},"hits":[{"_index":"tenant-a-tracking-projections-v1","_id":"tracking-123","_version":%d,"_source":%s,"sort":["tracking-123"]}]}}`, transport.version, transport.source)
	return projectionJSONResponse(http.StatusOK, response), nil
}

func (transport *projectionTransport) CloseIdleConnections() {
	transport.mu.Lock()
	transport.closed = true
	transport.mu.Unlock()
}

func (transport *projectionTransport) closedIdleConnections() bool {
	transport.mu.Lock()
	defer transport.mu.Unlock()
	return transport.closed
}

func projectionJSONResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
