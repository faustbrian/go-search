package search

import (
	"context"
	"encoding/json"
	"errors"
	"unicode/utf8"
)

// ErrInvalidProjectionEvent rejects invalid event kind, identity, key, version, source, or
// limits.
var ErrInvalidProjectionEvent = errors.New("search: invalid projection event")

// ProjectionKind selects full-document upsert or source-free deletion.
type ProjectionKind string

const (
	// ProjectionUpsert projects a complete source document.
	ProjectionUpsert ProjectionKind = "upsert"
	// ProjectionDelete projects a durable deletion version without source JSON.
	ProjectionDelete ProjectionKind = "delete"
)

// ProjectionEvent retains a validated immutable projection intent and outbox key.
// Replay safety depends on authoritative version allocation and adapter policy;
// the event itself implements no deduplication or exactly-once delivery.
type ProjectionEvent struct {
	tenant, index, id string
	version           uint64
	kind              ProjectionKind
	source            json.RawMessage
	idempotencyKey    string
}

// NewProjectionEvent validates a positive source version, bounded identity/key,
// and the upsert/delete source contract, then copies retained source bytes.
// The idempotency key is caller metadata, not a durable deduplication store.
func NewProjectionEvent(tenant, index, id string, version uint64, kind ProjectionKind, source json.RawMessage, idempotencyKey string, limits Limits) (ProjectionEvent, error) {
	if limits.Validate() != nil || idempotencyKey == "" || len(idempotencyKey) > limits.MaxIDBytes ||
		!utf8.ValidString(tenant) || !utf8.ValidString(index) || !utf8.ValidString(id) || !utf8.ValidString(idempotencyKey) {
		return ProjectionEvent{}, ErrInvalidProjectionEvent
	}
	switch kind {
	case ProjectionUpsert:
		document, err := NewDocument(tenant, index, id, version, source, limits)
		if err != nil {
			return ProjectionEvent{}, errors.Join(ErrInvalidProjectionEvent, err)
		}
		return ProjectionEvent{tenant: tenant, index: index, id: id, version: version, kind: kind, source: document.Source, idempotencyKey: idempotencyKey}, nil
	case ProjectionDelete:
		if len(source) != 0 || tenant == "" || index == "" || id == "" || version == 0 || len(tenant) > limits.MaxTenantBytes || len(index) > limits.MaxIndexBytes || len(id) > limits.MaxIDBytes {
			return ProjectionEvent{}, ErrInvalidProjectionEvent
		}
		return ProjectionEvent{tenant: tenant, index: index, id: id, version: version, kind: kind, idempotencyKey: idempotencyKey}, nil
	default:
		return ProjectionEvent{}, ErrInvalidProjectionEvent
	}
}

// IdempotencyKey returns the retained application outbox identity.
func (event ProjectionEvent) IdempotencyKey() string { return event.idempotencyKey }

// Operation returns a mutation with an independent source copy; a zero event is not a
// validated operation.
func (event ProjectionEvent) Operation() WriteOperation {
	if event.kind == ProjectionDelete {
		return DeleteDocument(event.tenant, event.index, event.id, event.version)
	}
	return WriteOperation{Action: ActionUpsert, Tenant: event.tenant, Index: event.index, ID: event.id, Version: event.version, Source: append(json.RawMessage(nil), event.source...)}
}

// ProjectionOutbox is implemented by an application's source-of-truth
// transaction boundary. The search package never opens or owns that
// transaction.
type ProjectionOutbox interface {
	// Enqueue must persist the event in the application source-of-truth transaction; the core
	// supplies no outbox implementation.
	Enqueue(context.Context, ProjectionEvent) error
}

// ProjectionConsumer applies at-least-once events through external versions.
// It returns conflicts and unknown outcomes to the caller for policy decisions.
type ProjectionConsumer struct{ indexer Indexer }

// NewProjectionConsumer borrows the Indexer without starting work or taking lifecycle
// ownership.
// It rejects a nil interface but does not diagnose typed-nil implementations.
func NewProjectionConsumer(indexer Indexer) (*ProjectionConsumer, error) {
	if indexer == nil {
		return nil, ErrInvalidProjectionEvent
	}
	return &ProjectionConsumer{indexer: indexer}, nil
}

// Handle synchronously delegates one event to Write with the supplied context and refresh.
// It does not retry, deduplicate, contain callback panics, or persist a checkpoint.
func (consumer *ProjectionConsumer) Handle(ctx context.Context, event ProjectionEvent, refresh RefreshPolicy) (ItemOutcome, error) {
	return consumer.indexer.Write(ctx, event.Operation(), refresh)
}
