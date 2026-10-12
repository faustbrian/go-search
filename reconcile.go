package search

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"unicode/utf8"
)

var (
	// ErrInvalidReconciler requires nonnil dependencies and positive valid limits.
	ErrInvalidReconciler = errors.New("search: reconciliation dependencies and limits are required")
	// ErrInvalidReconciliation rejects invalid tenant/index or traversal request bounds.
	ErrInvalidReconciliation = errors.New("search: invalid reconciliation request")
	// ErrReconciliationLimit rejects page, combined record, or retained-state byte estimates
	// beyond limits.
	ErrReconciliationLimit = errors.New("search: reconciliation record limit exceeded")
	// ErrMalformedReconciliation rejects non-progressing, oversized, misordered, or
	// inconsistent reader pages.
	ErrMalformedReconciliation = errors.New("search: malformed or non-progressing reconciliation page")
	// ErrReconciliationDeletionGuard refuses orphan repair without a valid durable deletion
	// reservation; context sentinels may be joined.
	ErrReconciliationDeletionGuard = errors.New("search: orphan repair requires a durable source deletion guard")
	// ErrRepairPartial identifies partial, ambiguous, or misattributed repair result evidence.
	ErrRepairPartial = errors.New("search: reconciliation repair was partial or ambiguous")
)

// ReconciliationRecord is mutable snapshot evidence. Source records carry a copied
// Document; index records must not carry one.
type ReconciliationRecord struct {
	// ID is the stable nonempty UTF-8 identity used for strict ordering.
	ID string
	// Version is positive source/index external-version evidence.
	Version uint64
	// Digest is nonempty content identity; source digests are checked against Document.
	Digest string
	// Document is required only for authoritative source records; retained source bytes are
	// copied.
	Document *Document
}

// SourceRecord copies document source and computes its canonical digest without validating
// the document.
func SourceRecord(document Document) ReconciliationRecord {
	copyDocument := document
	copyDocument.Source = append(json.RawMessage(nil), document.Source...)
	return ReconciliationRecord{ID: document.ID, Version: document.Version, Digest: SourceDigest(document.Source), Document: &copyDocument}
}

// IndexRecord constructs metadata-only index evidence without validation.
func IndexRecord(id string, version uint64, digest string) ReconciliationRecord {
	return ReconciliationRecord{ID: id, Version: version, Digest: digest}
}

// SourceDigest hashes UTF-8 object JSON using Go JSON key ordering and number text.
// It returns empty for invalid/non-object input, applies no Limits budget, and alone
// does not reject duplicate keys or establish RFC 8785 canonicalization.
func SourceDigest(source json.RawMessage) string {
	if !utf8.Valid(source) {
		return ""
	}
	canonical, err := canonicalJSONObject(source)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// ReconciliationPage carries borrowed page data; readers must supply one stable, strictly
// ID-ordered traversal.
type ReconciliationPage struct {
	// Records is a borrowed bounded page, strictly ordered across the complete traversal.
	Records []ReconciliationRecord
	// Cursor must progress on nonfinal pages and be empty when Done.
	Cursor string
	// Done marks the terminal page; it does not independently prove snapshot consistency.
	Done bool
}

// ReconciliationReader owns snapshot identity and consistency across pages; the core checks
// shape/progress, not isolation.
type ReconciliationReader interface {
	// Read returns the next tenant/index-bound stable ID page. Empty nonfinal pages and
	// repeated cursors are rejected.
	Read(context.Context, string, string, string, int) (ReconciliationPage, error)
}

// ReconciliationDeletion identifies an indexed document that was absent from
// a bounded source snapshot. ObservedIndexVersion is the minimum version that
// a guarded deletion must supersede.
type ReconciliationDeletion struct {
	// Tenant, Index, and ID bind a durable deletion reservation to one authoritative identity.
	Tenant, Index, ID string
	// ObservedIndexVersion is the indexed version the reserved tombstone must supersede.
	ObservedIndexVersion uint64
}

// ReconciliationDeletionGuard atomically confirms authoritative source
// deletion and reserves a durable tombstone version. The returned version must
// be greater than ObservedIndexVersion, and every later source write for the
// same identity must use a still greater version. Implementations must make a
// repeated reservation safe after an ambiguous or interrupted repair run and
// must be safe for concurrent calls.
type ReconciliationDeletionGuard interface {
	// ReserveDeletion atomically confirms current source deletion and reserves a repeat-safe
	// version above the observed index and below terminal uint64.
	ReserveDeletion(context.Context, ReconciliationDeletion) (uint64, error)
}

// DriftKind classifies source/index differences without treating the index as authority.
type DriftKind string

const (
	// DriftMissing means an authoritative source ID is absent from the index.
	DriftMissing DriftKind = "missing"
	// DriftStale means the index version is below the source version.
	DriftStale DriftKind = "stale"
	// DriftDivergent means equal-version digest mismatch or an index version ahead of source;
	// it is never repaired automatically.
	DriftDivergent DriftKind = "divergent"
	// DriftOrphaned means an indexed ID is absent from the separately scanned source snapshot,
	// not proof of current deletion.
	DriftOrphaned DriftKind = "orphaned"
)

// Drift records one ID difference; versions are evidence, not permission to overwrite
// equal/newer index state.
type Drift struct {
	// ID is the compared document identity.
	ID string
	// Kind identifies missing, stale, divergent, or orphaned state.
	Kind DriftKind
	// SourceVersion and IndexVersion are observed versions; zero means absent on that side.
	SourceVersion, IndexVersion uint64
}

// ReconciliationRequest bounds one combined source/index scan and optionally dispatches
// repairs.
type ReconciliationRequest struct {
	// Tenant and Index select logical scope; readers/indexer own authorization.
	Tenant, Index string
	// PageSize bounds each page; MaxRecords bounds the combined source and index record count.
	PageSize, MaxRecords int
	// Repair enables missing/stale upserts and guarded orphan deletes; divergence remains
	// reported only.
	Repair bool
}

// ReconciliationReport retains mutable drift data and acknowledged repair counts; Complete
// does not mean zero drift.
type ReconciliationReport struct {
	// SourceRecords counts retained authoritative records.
	SourceRecords int
	// IndexRecords counts retained index records.
	IndexRecords int
	// Drift is caller-owned difference evidence, possibly nonempty even when Complete.
	Drift []Drift
	// Repaired counts acknowledged applied items, not a new zero-drift proof.
	Repaired int
	// Complete marks completed scanning/selected dispatch; it is false on partial repair or
	// failed orphan guards.
	Complete bool
}

// Reconciler borrows readers, indexer, and optional deletion guard. It creates no goroutine
// or durable checkpoint;
// readers/callbacks must be concurrency-safe and cooperate with caller cancellation.
type Reconciler struct {
	source        ReconciliationReader
	index         ReconciliationReader
	repair        Indexer
	deletionGuard ReconciliationDeletionGuard
	limits        Limits
}

// NewReconciler constructs a reconciler that repairs missing and stale
// documents but fails explicitly before dispatch if repair encounters an
// orphan. Use NewReconcilerWithDeletionGuard to authorize orphan deletion.
func NewReconciler(source, index ReconciliationReader, repair Indexer, limits Limits) (*Reconciler, error) {
	if source == nil || index == nil || repair == nil || limits.Validate() != nil {
		return nil, ErrInvalidReconciler
	}
	return &Reconciler{source: source, index: index, repair: repair, limits: limits}, nil
}

// NewReconcilerWithDeletionGuard constructs a reconciler that may repair
// orphaned index records after the guard durably authorizes each deletion.
func NewReconcilerWithDeletionGuard(source, index ReconciliationReader, repair Indexer, guard ReconciliationDeletionGuard, limits Limits) (*Reconciler, error) {
	if guard == nil {
		return nil, ErrInvalidReconciler
	}
	reconciler, err := NewReconciler(source, index, repair, limits)
	if err != nil {
		return nil, err
	}
	reconciler.deletionGuard = guard
	return reconciler, nil
}

// Run fully reads bounded stable source and index snapshots before merging IDs.
// MaxRecords and retained-byte estimates are shared across both scans. Missing/stale
// records may be repaired; divergent records (equal-version digest mismatch or
// index-ahead version) are never overwritten. Orphans require a durable deletion
// guard before any bulk dispatch. Complete means the scans and selected repair
// batches completed, not zero drift or an atomic snapshot across both readers.
func (r *Reconciler) Run(ctx context.Context, request ReconciliationRequest) (ReconciliationReport, error) {
	maximumRecords := r.limits.MaxPages * r.limits.MaxPageItems
	if request.Tenant == "" || !utf8.ValidString(request.Tenant) {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	if len(request.Tenant) > r.limits.MaxTenantBytes {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	if request.Index == "" || !utf8.ValidString(request.Index) {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	if len(request.Index) > r.limits.MaxIndexBytes {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	if request.PageSize <= 0 || request.PageSize > r.limits.MaxPageItems {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	if request.MaxRecords <= 0 || request.MaxRecords > maximumRecords {
		return ReconciliationReport{}, ErrInvalidReconciliation
	}
	remainingBytes := r.limits.MaxResultBytes
	source, err := readReconciliation(ctx, r.source, request, true, r.limits, &remainingBytes)
	if err != nil {
		return ReconciliationReport{}, err
	}
	indexRequest := request
	indexRequest.MaxRecords -= len(source)
	indexed, err := readReconciliation(ctx, r.index, indexRequest, false, r.limits, &remainingBytes)
	if err != nil {
		return ReconciliationReport{}, err
	}
	report := ReconciliationReport{SourceRecords: len(source), IndexRecords: len(indexed), Complete: true}
	operations := make([]WriteOperation, 0)
	type guardedDeletion struct {
		position int
		request  ReconciliationDeletion
	}
	guardedDeletions := make([]guardedDeletion, 0)
	left, right := 0, 0
	for left < len(source) || right < len(indexed) {
		switch {
		case right >= len(indexed) || left < len(source) && source[left].ID < indexed[right].ID:
			report.Drift = append(report.Drift, Drift{ID: source[left].ID, Kind: DriftMissing, SourceVersion: source[left].Version})
			if request.Repair {
				operations = append(operations, IndexDocument(*source[left].Document))
			}
			left++
		case left >= len(source) || indexed[right].ID < source[left].ID:
			report.Drift = append(report.Drift, Drift{ID: indexed[right].ID, Kind: DriftOrphaned, IndexVersion: indexed[right].Version})
			if request.Repair {
				guardedDeletions = append(guardedDeletions, guardedDeletion{
					position: len(operations),
					request:  ReconciliationDeletion{Tenant: request.Tenant, Index: request.Index, ID: indexed[right].ID, ObservedIndexVersion: indexed[right].Version},
				})
				operations = append(operations, WriteOperation{})
			}
			right++
		default:
			sourceRecord, indexRecord := source[left], indexed[right]
			if sourceRecord.Version > indexRecord.Version {
				report.Drift = append(report.Drift, Drift{ID: sourceRecord.ID, Kind: DriftStale, SourceVersion: sourceRecord.Version, IndexVersion: indexRecord.Version})
				if request.Repair {
					operations = append(operations, IndexDocument(*sourceRecord.Document))
				}
			} else if sourceRecord.Version != indexRecord.Version || sourceRecord.Digest != indexRecord.Digest {
				report.Drift = append(report.Drift, Drift{ID: sourceRecord.ID, Kind: DriftDivergent, SourceVersion: sourceRecord.Version, IndexVersion: indexRecord.Version})
			}
			left++
			right++
		}
	}
	if len(guardedDeletions) != 0 {
		report.Complete = false
		if r.deletionGuard == nil {
			return report, ErrReconciliationDeletionGuard
		}
		for _, deletion := range guardedDeletions {
			if deletion.request.ObservedIndexVersion == math.MaxUint64 {
				return report, ErrReconciliationDeletionGuard
			}
		}
		for _, deletion := range guardedDeletions {
			version, err := r.deletionGuard.ReserveDeletion(ctx, deletion.request)
			if err != nil {
				return report, classifiedReconciliationDeletionGuardError(err)
			}
			if version <= deletion.request.ObservedIndexVersion || version == math.MaxUint64 {
				return report, ErrReconciliationDeletionGuard
			}
			operations[deletion.position] = DeleteDocument(deletion.request.Tenant, deletion.request.Index, deletion.request.ID, version)
		}
	}
	if len(operations) == 0 {
		return report, nil
	}
	report.Complete = false
	for start := 0; start < len(operations); {
		end := min(start+r.limits.MaxBulkItems, len(operations))
		batchOperations := append([]WriteOperation(nil), operations[start:end]...)
		bulk := BulkRequest{Operations: batchOperations, Refresh: RefreshWaitFor}
		if err := bulk.Validate(AllCapabilities(), r.limits); err != nil {
			return report, err
		}
		result, err := r.repair.Bulk(ctx, bulk)
		if err != nil {
			return report, err
		}
		items := result.Items()
		if result.ValidateRequest(bulk) != nil {
			return report, ErrRepairPartial
		}
		for _, item := range items {
			if item.State == OutcomeApplied {
				report.Repaired++
			}
		}
		if result.Partial() {
			return report, ErrRepairPartial
		}
		start = end
	}
	report.Complete = true
	return report, nil
}

func classifiedReconciliationDeletionGuardError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return errors.Join(ErrReconciliationDeletionGuard, context.Canceled)
	case errors.Is(err, context.DeadlineExceeded):
		return errors.Join(ErrReconciliationDeletionGuard, context.DeadlineExceeded)
	default:
		return ErrReconciliationDeletionGuard
	}
}

func readReconciliation(ctx context.Context, reader ReconciliationReader, request ReconciliationRequest, requireDocuments bool, limits Limits, sharedRemainingBytes ...*int64) ([]ReconciliationRecord, error) {
	localRemainingBytes := limits.MaxResultBytes
	remainingBytes := &localRemainingBytes
	if len(sharedRemainingBytes) == 1 && sharedRemainingBytes[0] != nil {
		remainingBytes = sharedRemainingBytes[0]
	}
	result := make([]ReconciliationRecord, 0)
	cursor := ""
	pages := 0
	for {
		if pages == limits.MaxPages {
			return nil, ErrReconciliationLimit
		}
		page, err := reader.Read(ctx, request.Tenant, request.Index, cursor, request.PageSize)
		if err != nil {
			return nil, err
		}
		pages++
		if len(page.Cursor) > limits.MaxQueryBytes || page.Done && page.Cursor != "" {
			return nil, ErrMalformedReconciliation
		}
		if !page.Done && (len(page.Records) == 0 || page.Cursor == "" || page.Cursor == cursor) {
			return nil, ErrMalformedReconciliation
		}
		if len(page.Records) > request.PageSize {
			return nil, ErrMalformedReconciliation
		}
		if len(page.Records) > request.MaxRecords-len(result) {
			return nil, ErrReconciliationLimit
		}
		for _, record := range page.Records {
			if record.ID == "" || len(record.ID) > limits.MaxIDBytes || !utf8.ValidString(record.ID) || record.Version == 0 ||
				record.Digest == "" || len(record.Digest) > limits.MaxIDBytes || !utf8.ValidString(record.Digest) ||
				requireDocuments && record.Document == nil || !requireDocuments && record.Document != nil {
				return nil, ErrMalformedReconciliation
			}
			if record.Document != nil && (record.Document.Tenant != request.Tenant || record.Document.Index != request.Index || record.Document.ID != record.ID || record.Document.Version != record.Version) {
				return nil, ErrMalformedReconciliation
			}
			if requireDocuments {
				document, err := NewDocument(record.Document.Tenant, record.Document.Index, record.Document.ID, record.Document.Version, record.Document.Source, limits)
				if err != nil || record.Digest != SourceDigest(document.Source) {
					return nil, ErrMalformedReconciliation
				}
				record.Document = &document
			}
			if !reserveReconciliationRecord(record, remainingBytes) {
				return nil, ErrReconciliationLimit
			}
			result = append(result, cloneReconciliationRecord(record))
		}
		if page.Done {
			break
		}
		if len(result) == request.MaxRecords {
			return nil, ErrReconciliationLimit
		}
		cursor = page.Cursor
	}
	for index := 1; index < len(result); index++ {
		if result[index-1].ID >= result[index].ID {
			return nil, ErrMalformedReconciliation
		}
	}
	return result, nil
}

func reserveReconciliationRecord(record ReconciliationRecord, remaining *int64) bool {
	// Reserve conservatively for the retained record, drift/report metadata,
	// and a possible repair operation. Source bytes can be owned once by the
	// snapshot and once by an index repair operation.
	const fixedOverhead int64 = 256
	available := *remaining
	sizes := []int64{fixedOverhead, int64(len(record.ID)), int64(len(record.Digest))}
	if record.Document != nil {
		sizes = append(sizes,
			int64(len(record.Document.Tenant)), int64(len(record.Document.Index)), int64(len(record.Document.ID)),
			int64(len(record.Document.Source)), int64(len(record.Document.Source)),
		)
	}
	for _, size := range sizes {
		if size > available {
			return false
		}
		available -= size
	}
	*remaining = available
	return true
}

func cloneReconciliationRecord(record ReconciliationRecord) ReconciliationRecord {
	if record.Document != nil {
		document := *record.Document
		document.Source = append(json.RawMessage(nil), document.Source...)
		record.Document = &document
	}
	return record
}
