package search

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"unicode/utf8"
)

// ErrInvalidResult rejects inconsistent hit/JSON/diagnostic shape, not all possible backend
// protocol violations.
var ErrInvalidResult = errors.New("search: invalid backend result")

// TotalRelation states whether a backend count is exact or a lower bound.
type TotalRelation string

const (
	// TotalExact denotes an exact backend hit count.
	TotalExact TotalRelation = "eq"
	// TotalLowerBound denotes a lower-bound count, not total traversal completeness.
	TotalLowerBound TotalRelation = "gte"
)

// Total is backend count evidence, which must not be smaller than returned hits.
type Total struct {
	// Value is the backend hit count or lower bound.
	Value uint64
	// Relation qualifies Value as exact or a lower bound.
	Relation TotalRelation
}

// Hit is a mutable hit description. Result constructors/accessors copy its
// nested bytes, slices, maps, and score pointer. Score is backend-provided relevance
// and is intentionally not claimed to be portable across adapters.
type Hit struct {
	// Index is the adapter-attributed logical index identity.
	Index string
	// ID is the document identity.
	ID string
	// Version is positive backend external-version evidence.
	Version uint64
	// Score is optional backend relevance, not comparable across adapters.
	Score *float64
	// Source is returned JSON; shaping, byte bounds, and disclosure are adapter responsibilities.
	Source json.RawMessage
	// SortValues is the ordered JSON tuple used for continuation.
	SortValues []json.RawMessage
	// Highlights is field-to-fragment data; render as untrusted content, not safe HTML.
	Highlights map[string][]string
}

// Failure is a shared partial-result diagnostic. Its text is not automatically redacted.
type Failure struct {
	// Scope and Code classify the failure; Message is caller-supplied text that may contain
	// sensitive data.
	Scope, Code, Message string
	// Retryable is backend advice, not an automatic retry policy.
	Retryable bool
}

// ShardDiagnostics records backend shard counts, not replica durability or whole-cluster
// health.
type ShardDiagnostics struct {
	// Total, Successful, Skipped, and Failed describe shard participation and must be
	// internally consistent.
	Total, Successful, Skipped, Failed int
}

// Diagnostics preserves backend timing and partial-result evidence; consumers must inspect
// Partial.
type Diagnostics struct {
	// Backend is a bounded nonempty backend identifier.
	Backend string
	// RequestID is an optional bounded diagnostic identifier.
	RequestID string
	// Took is nonnegative backend-reported duration, not client end-to-end time.
	Took time.Duration
	// TimedOut indicates backend timeout and requires Partial.
	TimedOut bool
	// Partial marks incomplete execution; false is not a persistence or full-cluster guarantee.
	Partial bool
	// Shards reports internally consistent shard counts.
	Shards ShardDiagnostics
	// Failures retains independently copied classifications in a Result.
	Failures []Failure
	// Warnings retains copied text; the constructor does not redact it.
	Warnings []string
}

// Result retains partial-result diagnostics rather than converting a successful
// HTTP status into an unconditional success claim.
type Result struct {
	hits         []Hit
	total        Total
	aggregations map[string]json.RawMessage
	suggestions  map[string]json.RawMessage
	diagnostics  Diagnostics
	nextCursor   string
}

// NewResult validates hit identities, positive versions, finite scores, JSON values,
// and diagnostic consistency, then deeply copies retained mutable data. It has no
// Limits parameter: byte/count/depth admission and privacy remain adapter concerns.
func NewResult(hits []Hit, total Total, aggregations, suggestions map[string]json.RawMessage, diagnostics Diagnostics, nextCursor string) (Result, error) {
	if total.Relation != TotalExact && total.Relation != TotalLowerBound || total.Value < uint64(len(hits)) {
		return Result{}, ErrInvalidResult
	}
	copyHits := make([]Hit, len(hits))
	for index, hit := range hits {
		if hit.Index == "" || hit.ID == "" || !utf8.ValidString(hit.Index) || !utf8.ValidString(hit.ID) ||
			hit.Version == 0 || hit.Score != nil && (math.IsNaN(*hit.Score) || math.IsInf(*hit.Score, 0)) {
			return Result{}, ErrInvalidResult
		}
		if len(hit.Source) > 0 && !json.Valid(hit.Source) {
			return Result{}, ErrInvalidResult
		}
		for _, value := range hit.SortValues {
			if !json.Valid(value) {
				return Result{}, ErrInvalidResult
			}
		}
		copyHits[index] = cloneHit(hit)
	}
	if !validRawResultMap(aggregations) || !validRawResultMap(suggestions) || !validDiagnostics(diagnostics) {
		return Result{}, ErrInvalidResult
	}

	return Result{
		hits: copyHits, total: total, aggregations: cloneRawMap(aggregations), suggestions: cloneRawMap(suggestions),
		diagnostics: cloneDiagnostics(diagnostics), nextCursor: nextCursor,
	}, nil
}

func validRawResultMap(values map[string]json.RawMessage) bool {
	for name, value := range values {
		if !validField(name) || !json.Valid(value) {
			return false
		}
	}
	return true
}

func validDiagnostics(value Diagnostics) bool {
	shards := value.Shards
	if value.Took < 0 || !validDiagnosticIdentifier(value.Backend, MaxFieldNameBytes, true) ||
		!validDiagnosticIdentifier(value.RequestID, DefaultLimits().MaxIDBytes, false) ||
		shards.Total < 0 || shards.Successful < 0 || shards.Skipped < 0 || shards.Failed < 0 ||
		shards.Successful > shards.Total ||
		shards.Failed != shards.Total-shards.Successful-shards.Skipped ||
		(value.TimedOut || shards.Failed > 0 || len(value.Failures) > 0) && !value.Partial {
		return false
	}
	for _, failure := range value.Failures {
		if !validDiagnosticIdentifier(failure.Scope, MaxFieldNameBytes, true) ||
			!validDiagnosticIdentifier(failure.Code, MaxFieldNameBytes, true) || !validDiagnosticText(failure.Message) {
			return false
		}
	}
	for _, warning := range value.Warnings {
		if !validDiagnosticText(warning) {
			return false
		}
	}
	return true
}

func validDiagnosticIdentifier(value string, maximumBytes int, required bool) bool {
	if value == "" {
		return !required
	}
	return len(value) <= maximumBytes && utf8.ValidString(value) && !strings.ContainsAny(value, "\x00\r\n")
}

func validDiagnosticText(value string) bool {
	return len(value) <= DefaultLimits().MaxSourceBytes && utf8.ValidString(value) && !strings.ContainsRune(value, '\x00')
}

// Hits returns independently copied nested hit data and score pointers.
func (r Result) Hits() []Hit {
	result := make([]Hit, len(r.hits))
	for index, hit := range r.hits {
		result[index] = cloneHit(hit)
	}
	return result
}

// Total returns backend count evidence by value.
func (r Result) Total() Total { return r.total }

// Aggregations returns an independent map and JSON byte copies.
func (r Result) Aggregations() map[string]json.RawMessage { return cloneRawMap(r.aggregations) }

// Suggestions returns an independent map and JSON byte copies.
func (r Result) Suggestions() map[string]json.RawMessage { return cloneRawMap(r.suggestions) }

// Diagnostics returns an independent copy of failure and warning slices.
func (r Result) Diagnostics() Diagnostics { return cloneDiagnostics(r.diagnostics) }

// NextCursor returns opaque continuation bytes without validating or consuming them.
func (r Result) NextCursor() string { return r.nextCursor }

func cloneHit(hit Hit) Hit {
	hit.Source = append(json.RawMessage(nil), hit.Source...)
	hit.SortValues = cloneRawMessages(hit.SortValues)
	if hit.Score != nil {
		score := *hit.Score
		hit.Score = &score
	}
	if hit.Highlights != nil {
		result := make(map[string][]string, len(hit.Highlights))
		for field, fragments := range hit.Highlights {
			result[field] = append([]string(nil), fragments...)
		}
		hit.Highlights = result
	}
	return hit
}

func cloneRawMap(values map[string]json.RawMessage) map[string]json.RawMessage {
	if values == nil {
		return nil
	}
	result := make(map[string]json.RawMessage, len(values))
	for name, value := range values {
		result[name] = append(json.RawMessage(nil), value...)
	}
	return result
}

func cloneDiagnostics(value Diagnostics) Diagnostics {
	value.Failures = append([]Failure(nil), value.Failures...)
	value.Warnings = append([]string(nil), value.Warnings...)
	return value
}

// Searcher is the context-aware query boundary implemented by adapters.
type Searcher interface {
	// Capabilities reports supported semantics or an operation error; false features must not
	// silently degrade.
	Capabilities(context.Context) (Capabilities, error)
	// Search executes one typed request and retains partial-result diagnostics; callers own
	// policy and continuation handling.
	Search(context.Context, Request) (Result, error)
}
