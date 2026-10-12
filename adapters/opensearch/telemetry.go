package opensearch

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// TelemetryCategory is a fixed transport/admission classification, not a full
// operation-success assertion.
type TelemetryCategory string

const (
	// TelemetrySuccess means no transport/admission error or HTTP >=400, not a validated
	// semantic result.
	TelemetrySuccess TelemetryCategory = "success"
	// TelemetryTransport identifies an unclassified transport error.
	TelemetryTransport TelemetryCategory = "transport"
	// TelemetryCredentials identifies sanitized provider/signing refusal.
	TelemetryCredentials TelemetryCategory = "credentials"
	// TelemetryBackpressure identifies local admission refusal.
	TelemetryBackpressure TelemetryCategory = "backpressure"
	// TelemetryCircuitOpen identifies local circuit refusal.
	TelemetryCircuitOpen TelemetryCategory = "circuit_open"
	// TelemetryOverloaded identifies observed 429/503.
	TelemetryOverloaded TelemetryCategory = "overloaded"
	// TelemetryHTTPFailure identifies another HTTP status >=400.
	TelemetryHTTPFailure TelemetryCategory = "http_failure"
	// TelemetrySemantic accompanies a separate semantic Signal.
	TelemetrySemantic TelemetryCategory = "semantic"
)

// TelemetrySignal names an additional semantic event after transport completion.
type TelemetrySignal string

const (
	// TelemetryUnknownWriteOutcome reports uncertain mutation receipt.
	TelemetryUnknownWriteOutcome TelemetrySignal = "unknown_write_outcomes"
	// TelemetryPartialSearch reports partial search evidence.
	TelemetryPartialSearch TelemetrySignal = "partial_searches"
	// TelemetryPITCleanupFailure reports a failed PIT deletion acknowledgement.
	TelemetryPITCleanupFailure TelemetrySignal = "pit_cleanup_failures"
	// TelemetryCutoverFailure reports cutover failure, not proof the alias was unchanged.
	TelemetryCutoverFailure TelemetrySignal = "cutover_failures"
)

// TelemetryEvent is mutable telemetry metadata. Adapter-created events contain
// only fixed low-cardinality state; user-constructed events are not sanitized.
// Adapter-created events never
// contains endpoints, tenant labels, index names, query fields, request bodies,
// credentials, OpenSearch reason strings, or node IDs.
type TelemetryEvent struct {
	// Operation identifies the fixed adapter request category.
	Operation Operation
	// Category classifies transport/admission or semantic signal emission.
	Category TelemetryCategory
	// Signal identifies an optional semantic condition; ordinary transport events leave it empty.
	Signal TelemetrySignal
	// Status is the observed response status, zero without response.
	Status int
	// Duration covers transport admission/body completion and callback clock timing, not full
	// public-operation latency; signals leave it zero.
	Duration time.Duration
	// InFlight is the local permit count at observation.
	InFlight int
	// Queued is the local waiter count at observation.
	Queued int
	// CircuitOpen is the local cooldown observation.
	CircuitOpen bool
}

// TelemetryObserver is a borrowed synchronous operator callback. Adapter invocation ignores
// its errors and contains panics, not blocking.
type TelemetryObserver interface {
	// Observe is called inline with request context; it must return promptly and be
	// concurrency-safe. Observer errors/panics do not fail the operation.
	Observe(context.Context, TelemetryEvent) error
}

// TelemetryObserverFunc adapts a function to TelemetryObserver; nil invocation panics.
type TelemetryObserverFunc func(context.Context, TelemetryEvent) error

// Observe delegates to the wrapped function directly; panic/error containment occurs only
// at adapter invocation.
func (observe TelemetryObserverFunc) Observe(ctx context.Context, event TelemetryEvent) error {
	return observe(ctx, event)
}

// TelemetryConfig enables synchronous telemetry with borrowed observer and clock; both are
// required when supplied.
type TelemetryConfig struct {
	// Observer is borrowed; blocking delays operation completion even though errors/panics are
	// ignored.
	Observer TelemetryObserver
	// Clock is borrowed for nonnegative duration calculation; its panics are not contained by
	// observer recovery.
	Clock func() time.Time
}

type telemetry struct {
	observer TelemetryObserver
	clock    func() time.Time
}

func newTelemetry(config *TelemetryConfig) (*telemetry, error) {
	if config == nil {
		return &telemetry{clock: time.Now}, nil
	}
	if config.Observer == nil || config.Clock == nil {
		return nil, ErrInvalidConfig
	}
	return &telemetry{observer: config.Observer, clock: config.Clock}, nil
}

func (telemetry *telemetry) now() time.Time { return telemetry.clock() }

func (telemetry *telemetry) observe(ctx context.Context, event TelemetryEvent) {
	if telemetry.observer == nil {
		return
	}
	defer func() { _ = recover() }()
	_ = telemetry.observer.Observe(ctx, event)
}

func (telemetry *telemetry) signal(ctx context.Context, operation Operation, signal TelemetrySignal) {
	telemetry.observe(ctx, TelemetryEvent{Operation: operation, Category: TelemetrySemantic, Signal: signal})
}

func (telemetry *telemetry) event(operation Operation, started time.Time, status int, err error, resilience ResilienceSnapshot) TelemetryEvent {
	category := TelemetrySuccess
	switch {
	case errors.Is(err, ErrCredentials):
		category = TelemetryCredentials
	case errors.Is(err, ErrBackpressure):
		category = TelemetryBackpressure
	case errors.Is(err, ErrCircuitOpen):
		category = TelemetryCircuitOpen
	case err != nil:
		category = TelemetryTransport
	case status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable:
		category = TelemetryOverloaded
	case status >= 400:
		category = TelemetryHTTPFailure
	}
	duration := telemetry.now().Sub(started)
	duration = max(duration, 0)
	return TelemetryEvent{Operation: operation, Category: category, Status: status, Duration: duration, InFlight: resilience.InFlight, Queued: resilience.Queued, CircuitOpen: resilience.CircuitOpen}
}

type operationContextKey struct{}

func withOperation(ctx context.Context, operation Operation) context.Context {
	return context.WithValue(ctx, operationContextKey{}, operation)
}
func operationFromContext(ctx context.Context) Operation {
	if operation, ok := ctx.Value(operationContextKey{}).(Operation); ok {
		return operation
	}
	return Operation("unknown")
}
