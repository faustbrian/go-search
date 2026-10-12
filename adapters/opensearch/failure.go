package opensearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

var (
	// ErrOverloaded identifies retryable 429 or 503 admission failure.
	ErrOverloaded = errors.New("search/opensearch: cluster is overloaded")
	// ErrClusterBlocked identifies a cluster-level write or metadata block.
	ErrClusterBlocked = errors.New("search/opensearch: cluster is blocked")
	// ErrRejected identifies a deterministic request rejection.
	ErrRejected = errors.New("search/opensearch: request was rejected")
	// ErrVersionConflict classifies the recognized external-version rejection, without backend
	// reason text.
	ErrVersionConflict = errors.New("search/opensearch: external version conflict")
	// ErrMappingRejected classifies recognized mapping rejection, without backend reason text.
	ErrMappingRejected = errors.New("search/opensearch: document mapping was rejected")
	// ErrPITExpired classifies missing or expired PIT search context.
	ErrPITExpired = errors.New("search/opensearch: point in time expired")
	// ErrBackpressure identifies instance-local admission exhaustion, not global backend capacity.
	ErrBackpressure = errors.New("search/opensearch: local capacity exhausted")
	// ErrCircuitOpen identifies instance-local circuit refusal, not proof of backend outage.
	ErrCircuitOpen = errors.New("search/opensearch: circuit is open")
)

// Operation identifies the stable adapter operation in diagnostics.
type Operation string

const (
	// OperationInfo reads cluster identity and supported version.
	OperationInfo Operation = "info"
	// OperationSearch identifies typed query execution.
	OperationSearch Operation = "search"
	// OperationCreatePIT identifies PIT creation.
	OperationCreatePIT Operation = "create_pit"
	// OperationDeletePIT identifies PIT deletion.
	OperationDeletePIT Operation = "delete_pit"
	// OperationBulk identifies the positional NDJSON mutation request.
	OperationBulk Operation = "bulk"
	// OperationWrite identifies one document mutation.
	OperationWrite Operation = "write"
	// OperationLifecycle identifies discovery or lifecycle policy scope.
	OperationLifecycle Operation = "lifecycle"
	// OperationCreateIndex identifies generation creation.
	OperationCreateIndex Operation = "create_index"
	// OperationReindex identifies task submission/polling.
	OperationReindex Operation = "reindex"
	// OperationVerifyIndex identifies semantic/count verification.
	OperationVerifyIndex Operation = "verify_index"
	// OperationResolveAlias identifies alias lookup.
	OperationResolveAlias Operation = "resolve_alias"
	// OperationSwapAlias identifies atomic alias mutation.
	OperationSwapAlias Operation = "swap_alias"
	// OperationDeleteIndex identifies irreversible generation deletion.
	OperationDeleteIndex Operation = "delete_index"
	// OperationHealth identifies operator-wide health inspection.
	OperationHealth Operation = "health"
	// OperationCapacity identifies operator-wide capacity inspection.
	OperationCapacity Operation = "capacity"
	// OperationTemplate identifies composable template mutation.
	OperationTemplate Operation = "template"
)

// FailureCategory is a stable operational classification independent of
// OpenSearch's human-readable reason strings.
type FailureCategory string

const (
	// FailureCancelled identifies caller cancellation or deadline expiry.
	FailureCancelled FailureCategory = "cancelled"
	// FailureTransport identifies transport failure without raw error disclosure.
	FailureTransport FailureCategory = "transport"
	// FailureOverloaded identifies classified 429/503 overload.
	FailureOverloaded FailureCategory = "overloaded"
	// FailureClusterBlocked identifies a cluster block classification.
	FailureClusterBlocked FailureCategory = "cluster_blocked"
	// FailureRejected identifies other deterministic rejection.
	FailureRejected FailureCategory = "rejected"
	// FailureMalformed identifies invalid or incomplete response evidence.
	FailureMalformed FailureCategory = "malformed_response"
	// FailureVersionConflict identifies external-version conflict.
	FailureVersionConflict FailureCategory = "version_conflict"
	// FailureMappingRejected identifies recognized mapping rejection.
	FailureMappingRejected FailureCategory = "mapping_rejected"
	// FailurePITExpired identifies expired/missing search context.
	FailurePITExpired FailureCategory = "pit_expired"
	// FailureBackpressure identifies process-local admission refusal.
	FailureBackpressure FailureCategory = "backpressure"
	// FailureCircuitOpen identifies process-local circuit refusal.
	FailureCircuitOpen FailureCategory = "circuit_open"
)

// Failure is mutable classified adapter error metadata. Adapter-created failures
// exclude backend reason text and provider details, retaining only bounded codes
// and classified causes. A mutation guard cancellation before callback execution
// retains the guard error chain through Unwrap, so that chain must be secret-free.
// Public literals/field mutation do not receive adapter redaction guarantees, and
// Retryable is advice rather than permission to replay mutations.
type Failure struct {
	// Operation identifies the adapter request category.
	Operation Operation
	// Category is the stable classification, not raw backend reason text.
	Category FailureCategory
	// Status is the observed HTTP status, or zero without response evidence.
	Status int
	// Code is the adapter-sanitized backend identifier; public assignments are not sanitized.
	Code string
	// Retryable indicates classified retry potential; callers still own ambiguity reconciliation.
	Retryable bool
	// OutcomeKnown states whether receipt is classified as known, not whether the mutation
	// succeeded.
	OutcomeKnown bool
	cause        error
}

func unknownTransportFailure(operation Operation, cause error) *Failure {
	failure := transportFailure(operation, cause)
	failure.OutcomeKnown = false
	return failure
}

func unknownMalformedFailure(operation Operation, cause error) *Failure {
	failure := malformedFailure(operation, cause)
	failure.OutcomeKnown = false
	return failure
}

// Error formats operation/category/status/code without private cause text.
// Publicly mutated fields are formatted verbatim and must not contain secrets.
func (failure *Failure) Error() string {
	message := fmt.Sprintf("search/opensearch: %s failed (%s", failure.Operation, failure.Category)
	if failure.Status != 0 {
		message += fmt.Sprintf(", HTTP %d", failure.Status)
	}
	if failure.Code != "" {
		message += ", code " + failure.Code
	}

	return message + ")"
}

// Unwrap exposes the retained cause for errors.Is/errors.As. Provider/transport
// details are reduced to owned classifications, but a pre-callback mutation guard
// error wrapping cancellation/deadline is retained as supplied. Do not log that
// chain without application sanitization; Error itself omits private cause text.
func (failure *Failure) Unwrap() error { return failure.cause }

func cancelledFailure(operation Operation, cause error) *Failure {
	return &Failure{
		Operation: operation, Category: FailureCancelled,
		OutcomeKnown: true, cause: cause,
	}
}

func transportFailure(operation Operation, cause error) *Failure {
	if errors.Is(cause, context.Canceled) {
		return cancelledFailure(operation, context.Canceled)
	}
	if errors.Is(cause, context.DeadlineExceeded) {
		return cancelledFailure(operation, context.DeadlineExceeded)
	}
	if errors.Is(cause, ErrBackpressure) {
		return &Failure{Operation: operation, Category: FailureBackpressure, Retryable: true, OutcomeKnown: false, cause: ErrBackpressure}
	}
	if errors.Is(cause, ErrCircuitOpen) {
		return &Failure{Operation: operation, Category: FailureCircuitOpen, Retryable: true, OutcomeKnown: false, cause: ErrCircuitOpen}
	}

	return &Failure{
		Operation: operation, Category: FailureTransport,
		Retryable: true, OutcomeKnown: true, cause: ErrTransport,
	}
}

func responseFailure(operation Operation, status int, body []byte) *Failure {
	code := responseErrorCode(body)
	category, retryable, sentinel := FailureRejected, false, ErrRejected
	switch {
	case code == "cluster_block_exception":
		category, sentinel = FailureClusterBlocked, ErrClusterBlocked
	case status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable:
		category, retryable, sentinel = FailureOverloaded, true, ErrOverloaded
	case code == "version_conflict_engine_exception":
		category, sentinel = FailureVersionConflict, ErrVersionConflict
	case code == "mapper_parsing_exception" || code == "strict_dynamic_mapping_exception":
		category, sentinel = FailureMappingRejected, ErrMappingRejected
	case operation == OperationSearch && status == http.StatusNotFound &&
		responseHasErrorCode(body, "search_context_missing_exception"):
		category, sentinel = FailurePITExpired, ErrPITExpired
	}

	return &Failure{
		Operation: operation, Category: category, Status: status, Code: code,
		Retryable: retryable, OutcomeKnown: true,
		cause: errors.Join(ErrUnexpectedStatus, sentinel),
	}
}

func classifyPITSearchFailure(err error) error {
	failure := new(Failure)
	if !errors.As(err, &failure) || failure.Operation != OperationSearch ||
		failure.Status != http.StatusNotFound || failure.Code != "resource_not_found_exception" {
		return err
	}
	copyFailure := *failure
	copyFailure.Category = FailurePITExpired
	copyFailure.Retryable = false
	copyFailure.cause = errors.Join(ErrUnexpectedStatus, ErrPITExpired)
	return &copyFailure
}

func responseHasErrorCode(body []byte, expected string) bool {
	var payload struct {
		Error struct {
			Type      string `json:"type"`
			RootCause []struct {
				Type string `json:"type"`
			} `json:"root_cause"`
			FailedShards []struct {
				Reason struct {
					Type string `json:"type"`
				} `json:"reason"`
			} `json:"failed_shards"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil || expected == "" {
		return false
	}
	if payload.Error.Type == expected {
		return true
	}
	for _, cause := range payload.Error.RootCause {
		if cause.Type == expected {
			return true
		}
	}
	for _, shard := range payload.Error.FailedShards {
		if shard.Reason.Type == expected {
			return true
		}
	}
	return false
}

func malformedFailure(operation Operation, cause error) *Failure {
	return &Failure{
		Operation: operation, Category: FailureMalformed,
		OutcomeKnown: true, cause: cause,
	}
}

func responseErrorCode(body []byte) string {
	var payload struct {
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return "unknown"
	}
	if len(payload.Error) == 0 {
		return "unknown"
	}
	var object struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(payload.Error, &object) != nil || !safeErrorCode(object.Type) {
		return "unknown"
	}

	return object.Type
}

func safeErrorCode(code string) bool {
	if code == "" || len(code) > 128 {
		return false
	}
	for _, character := range code {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '_' {
			return false
		}
	}

	return !strings.Contains(code, "__")
}
