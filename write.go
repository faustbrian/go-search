package search

import (
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

var (
	// ErrBulkLimit rejects invalid bulk limits or item/input-byte budget overflow.
	ErrBulkLimit = errors.New("search: bulk operation exceeds item or byte limit")
	// ErrTenantMismatch rejects a bulk unit containing different exact tenant labels.
	ErrTenantMismatch = errors.New("search: bulk operation crosses tenant boundary")
	// ErrInvalidOperation rejects malformed identities, versions, action, refresh, or source
	// shape.
	ErrInvalidOperation = errors.New("search: invalid write operation")
	// ErrInvalidBulkResult rejects invalid positional/classification shape or request attribution.
	ErrInvalidBulkResult = errors.New("search: invalid bulk result")
)

// WriteAction identifies a document mutation.
type WriteAction string

const (
	// ActionIndex writes a complete externally versioned document.
	ActionIndex WriteAction = "index"
	// ActionUpdate replaces an existing document; adapters may reject this capability.
	ActionUpdate WriteAction = "update"
	// ActionUpsert writes a complete document whether or not it already exists.
	ActionUpsert WriteAction = "upsert"
	// ActionDelete removes a document under an authoritative external version.
	ActionDelete WriteAction = "delete"
)

// RefreshPolicy makes post-write visibility explicit.
type RefreshPolicy string

const (
	// RefreshNone requests no visibility wait.
	RefreshNone RefreshPolicy = "none"
	// RefreshWaitFor requests waiting for backend refresh visibility.
	RefreshWaitFor RefreshPolicy = "wait_for"
	// RefreshImmediate requests immediate refresh, with backend-dependent cost.
	RefreshImmediate RefreshPolicy = "immediate"
)

// WriteOperation is a source-versioned mutation. Source is absent only for a
// delete. The document helper functions copy Source; literals and assignments
// remain caller-owned mutable input until an implementation copies them.
type WriteOperation struct {
	// Action selects the full-document mutation semantics.
	Action WriteAction
	// Tenant scopes the application identity, not authentication.
	Tenant string
	// Index is the logical index resolved by the backend adapter.
	Index string
	// ID identifies one document within tenant/index scope.
	ID string
	// Version is the positive authoritative external version.
	Version uint64
	// Source is required object JSON except for delete; keep it stable during calls.
	Source json.RawMessage
}

// IndexDocument creates an index operation with independently copied source bytes; it does
// not revalidate the document.
func IndexDocument(document Document) WriteOperation {
	return operationFromDocument(ActionIndex, document)
}

// UpdateDocument creates an update-existing operation with copied source; support is
// adapter-specific.
func UpdateDocument(document Document) WriteOperation {
	return operationFromDocument(ActionUpdate, document)
}

// UpsertDocument creates a full-document upsert with copied source; it does not revalidate
// the document.
func UpsertDocument(document Document) WriteOperation {
	return operationFromDocument(ActionUpsert, document)
}

// DeleteDocument creates a source-free deletion intent without validating identities or
// version.
func DeleteDocument(tenant, index, id string, version uint64) WriteOperation {
	return WriteOperation{Action: ActionDelete, Tenant: tenant, Index: index, ID: id, Version: version}
}

func operationFromDocument(action WriteAction, document Document) WriteOperation {
	return WriteOperation{Action: action, Tenant: document.Tenant, Index: document.Index, ID: document.ID, Version: document.Version, Source: append(json.RawMessage(nil), document.Source...)}
}

// BulkRequest is one bounded, single-tenant write unit. A backend may partially
// apply it; callers must inspect every returned item.
type BulkRequest struct {
	// Operations is an ordered borrowed input sequence; implementations may copy it before
	// callbacks.
	Operations []WriteOperation
	// Refresh requests post-write visibility, not a transaction across items.
	Refresh RefreshPolicy
}

// Validate checks external versioning, tenant isolation, and encoded source
// bounds before network execution. Its byte charge includes a fixed per-item
// allowance, not final protocol encoding; adapters must check their encoded body.
func (r BulkRequest) Validate(capabilities Capabilities, limits Limits) error {
	if limits.Validate() != nil {
		return ErrBulkLimit
	}
	if !capabilities.ExternalVersion || !capabilities.BulkPartialOutcomes {
		return ErrUnsupported
	}
	if len(r.Operations) == 0 || len(r.Operations) > limits.MaxBulkItems {
		return ErrBulkLimit
	}
	if r.Refresh != RefreshNone && r.Refresh != RefreshWaitFor && r.Refresh != RefreshImmediate {
		return ErrInvalidOperation
	}
	tenant := r.Operations[0].Tenant
	remainingBytes := limits.MaxBulkBytes
	for _, operation := range r.Operations {
		if operation.Action == ActionUpdate && !capabilities.UpdateExisting {
			return unsupported("update existing")
		}
		if operation.Tenant != tenant {
			return ErrTenantMismatch
		}
		if err := operation.validate(limits); err != nil {
			return err
		}
		for _, size := range []int{len(operation.Tenant), len(operation.Index), len(operation.ID), len(operation.Source), 64} {
			if size > remainingBytes {
				return ErrBulkLimit
			}
			remainingBytes -= size
		}
	}
	return nil
}

func (o WriteOperation) validate(limits Limits) error {
	if o.Tenant == "" || len(o.Tenant) > limits.MaxTenantBytes || o.Index == "" || len(o.Index) > limits.MaxIndexBytes ||
		o.ID == "" || len(o.ID) > limits.MaxIDBytes || o.Version == 0 || !utf8.ValidString(o.Tenant) ||
		!utf8.ValidString(o.Index) || !utf8.ValidString(o.ID) {
		return ErrInvalidOperation
	}
	switch o.Action {
	case ActionIndex, ActionUpdate, ActionUpsert:
		trimmed := bytesTrimSpace(o.Source)
		if len(o.Source) == 0 || len(o.Source) > limits.MaxSourceBytes || len(trimmed) < 2 || trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' {
			return ErrInvalidOperation
		}
		remainingNodes := limits.MaxJSONNodes
		if err := validateBoundedJSONObject(trimmed, limits.MaxJSONDepth, &remainingNodes); err != nil {
			return errors.Join(ErrInvalidOperation, err)
		}
	case ActionDelete:
		if len(o.Source) != 0 {
			return ErrInvalidOperation
		}
	default:
		return ErrInvalidOperation
	}
	return nil
}

func bytesTrimSpace(value []byte) []byte {
	start, end := 0, len(value)
	for start < end && (value[start] == ' ' || value[start] == '\n' || value[start] == '\r' || value[start] == '\t') {
		start++
	}
	for end > start && (value[end-1] == ' ' || value[end-1] == '\n' || value[end-1] == '\r' || value[end-1] == '\t') {
		end--
	}
	return value[start:end]
}

// OutcomeState classifies each write without flattening partial or ambiguous
// bulk execution into a Boolean.
type OutcomeState string

const (
	// OutcomeApplied identifies acknowledged application of the item.
	OutcomeApplied OutcomeState = "applied"
	// OutcomeNotFound identifies an absent target, including a missing delete.
	OutcomeNotFound OutcomeState = "not_found"
	// OutcomeVersionConflict identifies external version rejection.
	OutcomeVersionConflict OutcomeState = "version_conflict"
	// OutcomeRejected identifies admission or deterministic rejection as classified by the
	// adapter.
	OutcomeRejected OutcomeState = "rejected"
	// OutcomeFailed identifies a classified non-applied item failure.
	OutcomeFailed OutcomeState = "failed"
	// OutcomeUnknown means receipt cannot safely be inferred from the response or error.
	OutcomeUnknown OutcomeState = "unknown"
)

// ItemOutcome is mutable per-operation evidence, not a retry command.
type ItemOutcome struct {
	// Position is the zero-based original request offset.
	Position int
	// ID is the attributed document identity.
	ID string
	// Action is the attributed original mutation.
	Action WriteAction
	// State is the classified item result; Unknown requires receipt reconciliation.
	State OutcomeState
	// Version is adapter-returned version evidence; applied attribution is checked by
	// ValidateRequest.
	Version uint64
	// Code is a bounded classification identifier, not arbitrary backend reason text.
	Code string
	// Diagnostic is bounded text; NewBulkResult does not redact caller-supplied content.
	Diagnostic string
	// Retryable is adapter advice, not proof that automatic replay is safe.
	Retryable bool
}

// BulkResult retains independently copied positional outcomes; its zero value is empty.
type BulkResult struct{ items []ItemOutcome }

// NewBulkResult validates positional shape, action/state, and bounded diagnostic text,
// then copies items. It does not establish attribution to a request or redact text;
// use ValidateRequest before relying on an adapter result.
func NewBulkResult(items []ItemOutcome) (BulkResult, error) {
	if len(items) == 0 {
		return BulkResult{}, ErrInvalidBulkResult
	}
	copyItems := append([]ItemOutcome(nil), items...)
	for position, item := range copyItems {
		if item.Position != position || item.ID == "" || !utf8.ValidString(item.ID) ||
			!validAction(item.Action) || !validOutcome(item.State) ||
			!validDiagnosticIdentifier(item.Code, MaxFieldNameBytes, false) || !validDiagnosticText(item.Diagnostic) {
			return BulkResult{}, ErrInvalidBulkResult
		}
	}
	return BulkResult{items: copyItems}, nil
}

// Items returns an independent copy in original operation order.
func (r BulkResult) Items() []ItemOutcome { return append([]ItemOutcome(nil), r.items...) }

// ValidateRequest verifies that every outcome is attributed to the operation
// at the same position in request and that applied versions match.
func (r BulkResult) ValidateRequest(request BulkRequest) error {
	if len(r.items) == 0 || len(r.items) != len(request.Operations) {
		return ErrInvalidBulkResult
	}
	for position, item := range r.items {
		operation := request.Operations[position]
		if item.Position != position || item.ID != operation.ID || item.Action != operation.Action ||
			item.State == OutcomeApplied && item.Version != operation.Version {
			return ErrInvalidBulkResult
		}
	}
	return nil
}

// Partial reports whether any retained item is not OutcomeApplied; an empty zero result
// reports false.
func (r BulkResult) Partial() bool {
	for _, item := range r.items {
		if item.State != OutcomeApplied {
			return true
		}
	}
	return false
}

// HasUnknown reports whether any retained item has OutcomeUnknown.
func (r BulkResult) HasUnknown() bool {
	for _, item := range r.items {
		if item.State == OutcomeUnknown {
			return true
		}
	}
	return false
}

func validAction(action WriteAction) bool {
	return action == ActionIndex || action == ActionUpdate || action == ActionUpsert || action == ActionDelete
}
func validOutcome(state OutcomeState) bool {
	return state == OutcomeApplied || state == OutcomeNotFound || state == OutcomeVersionConflict || state == OutcomeRejected || state == OutcomeFailed || state == OutcomeUnknown
}

// Indexer is the context-aware document write boundary implemented by adapters.
type Indexer interface {
	// Write applies one mutation and returns its classification; an error may coexist with an
	// unknown outcome.
	Write(context.Context, WriteOperation, RefreshPolicy) (ItemOutcome, error)
	// Bulk applies an ordered unit whose items may succeed independently; inspect every result.
	Bulk(context.Context, BulkRequest) (BulkResult, error)
}
