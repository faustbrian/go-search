package opensearch_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/faustbrian/go-search"
	adapter "github.com/faustbrian/go-search/adapters/opensearch"
)

const privateCoordinatorMarker = "synthetic-private-coordinator-detail"

type privateCoordinatorError struct{ cause error }

func (err *privateCoordinatorError) Error() string { return privateCoordinatorMarker }
func (err *privateCoordinatorError) Unwrap() error { return err.cause }

func TestLifecycleGuardCancellationOwnsDiagnosticCause(t *testing.T) {
	for _, cancellation := range []error{context.Canceled, context.DeadlineExceeded} {
		for _, shape := range []string{"bare", "wrapped", "joined", "typed"} {
			t.Run(cancellation.Error()+"/"+shape, func(t *testing.T) {
				private := errors.New(privateCoordinatorMarker)
				cause := cancellation
				var originalTyped *privateCoordinatorError
				switch shape {
				case "wrapped":
					cause = fmt.Errorf("%s: %w", privateCoordinatorMarker, cancellation)
				case "joined":
					cause = errors.Join(private, cancellation)
				case "typed":
					originalTyped = &privateCoordinatorError{cause: cancellation}
					cause = originalTyped
				}
				dispatched := false
				client, err := adapter.New(adapter.Config{
					Endpoints: []string{"https://search.example.test"},
					Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
						dispatched = true
						return nil, errors.New("unexpected dispatch")
					}),
					TransportOwnership: adapter.TransportBorrowed,
					RequestTimeout:     time.Second, MaximumResponseBytes: 4096,
					Lifecycle: &adapter.LifecycleConfig{
						Authorizer:    adapter.LifecycleAuthorizerFunc(func(context.Context, string, []string) error { return nil }),
						MutationGuard: adapter.LifecycleMutationGuardFunc(func(context.Context, adapter.LifecycleMutationRequest, func() error) error { return cause }),
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = client.Close() })
				definition, err := search.NewIndexDefinition("documents-v1", []byte(`{}`), []byte(`{}`), search.DefaultLimits())
				if err != nil {
					t.Fatal(err)
				}
				operationErr := client.CreateIndex(t.Context(), "tenant", definition)
				var failure *adapter.Failure
				if !errors.As(operationErr, &failure) || failure.Category != adapter.FailureCancelled || !failure.OutcomeKnown || !errors.Is(operationErr, cancellation) || dispatched {
					t.Fatal("guard cancellation lost classification, outcome knowledge, or dispatch refusal")
				}
				var privateTyped *privateCoordinatorError
				if !errors.Is(operationErr, cause) {
					t.Fatal("guard cancellation lost the original inspectable cause identity")
				}
				if shape == "joined" && !errors.Is(operationErr, private) {
					t.Fatal("guard cancellation lost the joined inspectable cause")
				}
				if shape == "typed" && (!errors.As(operationErr, &privateTyped) || privateTyped != originalTyped) {
					t.Fatal("guard cancellation lost the original inspectable typed cause")
				}
				// Inspect adapter-owned presentation, not deliberately extracted caller causes.
				for current := operationErr; current != nil; current = errors.Unwrap(current) {
					if strings.Contains(current.Error(), privateCoordinatorMarker) {
						t.Fatal("guard cancellation exposed private wrapped cause text")
					}
				}
			})
		}
	}
}

func TestLifecycleResponseFixtureReachesDecoderAndClassifiesOutcomes(t *testing.T) {
	for _, test := range []struct {
		name  string
		body  string
		valid bool
	}{
		{"acknowledged", `{"acknowledged":true,"shards_acknowledged":true}`, true},
		{"incomplete acknowledgement", `{"acknowledged":true,"shards_acknowledged":false}`, false},
		{"trailing response", `{"acknowledged":true,"shards_acknowledged":true}{}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := fuzzSearchClient(t, test.body, &adapter.LifecycleConfig{
				Authorizer:    adapter.LifecycleAuthorizerFunc(func(context.Context, string, []string) error { return nil }),
				MutationGuard: allowLifecycleMutationGuard(),
			})
			definition, err := search.NewIndexDefinition("documents-v1", []byte(`{}`), []byte(`{}`), search.DefaultLimits())
			if err != nil {
				t.Fatal(err)
			}
			err = client.CreateIndex(t.Context(), "tenant", definition)
			if test.valid {
				if err != nil {
					t.Fatalf("complete acknowledgement was not accepted: %v", err)
				}
				return
			}
			var failure *adapter.Failure
			if !errors.As(err, &failure) || failure.Category != adapter.FailureMalformed || failure.OutcomeKnown || !errors.Is(err, adapter.ErrLifecycleRejected) {
				t.Fatal("malformed response lost classified unknown outcome after admission")
			}
		})
	}
}
