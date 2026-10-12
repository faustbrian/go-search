// Package search defines bounded contracts for rebuildable derived indexes,
// typed queries, authenticated cursors, projections, migration, and reconciliation.
// It does not own authoritative application data, durable outboxes, authorization,
// retries, or backend deployment. Identifiers are validated as UTF-8, not normalized
// or authenticated; adapters and application callbacks enforce tenant authority.
//
// Validated immutable values copy retained mutable input, but public request,
// document, hit, and report structs remain mutable. Keep borrowed input stable
// during calls. Callbacks are synchronous and must cooperate with context
// cancellation; the core does not contain their panics or impose execution timeouts.
//
// Start with DefaultLimits and NewDocument, then construct typed Request values
// for a chosen Searcher/Indexer. This module uses only Go standard-library code;
// the optional OpenSearch adapter is a separate module and owns its SDK/service boundary.
package search

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"unicode/utf8"
)

var (
	// ErrTenantRequired rejects an empty or invalid UTF-8 tenant label.
	ErrTenantRequired = errors.New("search: tenant is required")
	// ErrTenantTooLarge rejects tenant bytes beyond MaxTenantBytes.
	ErrTenantTooLarge = errors.New("search: tenant exceeds limit")
	// ErrIndexRequired rejects an empty or invalid UTF-8 logical index.
	ErrIndexRequired = errors.New("search: index is required")
	// ErrIndexTooLarge rejects index bytes beyond MaxIndexBytes.
	ErrIndexTooLarge = errors.New("search: index exceeds limit")
	// ErrIDRequired rejects an empty or invalid UTF-8 document ID.
	ErrIDRequired = errors.New("search: document ID is required")
	// ErrIDTooLarge rejects document ID bytes beyond MaxIDBytes.
	ErrIDTooLarge = errors.New("search: document ID exceeds limit")
	// ErrVersionRequired rejects zero external version; authoritative ordering is caller-owned.
	ErrVersionRequired = errors.New("search: external version is required")
	// ErrSourceRequired rejects nil source; nonnil empty source is instead invalid JSON.
	ErrSourceRequired = errors.New("search: document source is required")
	// ErrSourceTooLarge rejects source bytes beyond MaxSourceBytes.
	ErrSourceTooLarge = errors.New("search: document source exceeds limit")
	// ErrInvalidSource classifies non-object or malformed source, possibly joined with a
	// structural cause.
	ErrInvalidSource = errors.New("search: document source must be a JSON object")
	// ErrJSONDepthLimit identifies nesting deeper than the selected JSON limit.
	ErrJSONDepthLimit = errors.New("search: JSON nesting depth exceeds limit")
	// ErrJSONNodeLimit identifies member/element count beyond the selected shared JSON budget.
	ErrJSONNodeLimit = errors.New("search: JSON object-field or array-element count exceeds limit")
	// ErrDuplicateJSONKey identifies a repeated object key at any validated nesting level.
	ErrDuplicateJSONKey    = errors.New("search: JSON object contains a duplicate key")
	errMalformedJSONObject = errors.New("search: malformed JSON object")
)

// Document is a mutable source-versioned search document. NewDocument copies
// Source; struct literals and subsequent field assignments do not establish ownership.
type Document struct {
	// Tenant is the application tenant label, not proof of authorization.
	Tenant string
	// Index is the logical index identity interpreted by an adapter.
	Index string
	// ID is the stable document identity within Tenant and Index.
	ID string
	// Version is a positive authoritative external version; callers own monotonic allocation.
	Version uint64
	// Source is JSON object data. NewDocument retains an independent copy.
	Source json.RawMessage
}

// NewDocument validates nonempty UTF-8 identities, a positive version, and a bounded
// JSON object without duplicate keys at any depth. It copies the original source
// bytes, including whitespace; it does not normalize identifiers or authorize writes.
func NewDocument(tenant, index, id string, version uint64, source json.RawMessage, limits Limits) (Document, error) {
	if tenant == "" || !utf8.ValidString(tenant) {
		return Document{}, ErrTenantRequired
	}
	if len(tenant) > limits.MaxTenantBytes {
		return Document{}, ErrTenantTooLarge
	}
	if index == "" || !utf8.ValidString(index) {
		return Document{}, ErrIndexRequired
	}
	if len(index) > limits.MaxIndexBytes {
		return Document{}, ErrIndexTooLarge
	}
	if id == "" || !utf8.ValidString(id) {
		return Document{}, ErrIDRequired
	}
	if len(id) > limits.MaxIDBytes {
		return Document{}, ErrIDTooLarge
	}
	if version == 0 {
		return Document{}, ErrVersionRequired
	}
	if source == nil {
		return Document{}, ErrSourceRequired
	}
	if len(source) > limits.MaxSourceBytes {
		return Document{}, ErrSourceTooLarge
	}
	if limits.MaxJSONDepth <= 0 || limits.MaxJSONNodes <= 0 {
		return Document{}, ErrInvalidLimits
	}
	trimmed := bytes.TrimSpace(source)
	if len(trimmed) < 2 {
		return Document{}, ErrInvalidSource
	}
	if trimmed[0] != '{' {
		return Document{}, ErrInvalidSource
	}
	if trimmed[len(trimmed)-1] != '}' {
		return Document{}, ErrInvalidSource
	}
	remainingNodes := limits.MaxJSONNodes
	if err := validateBoundedJSONObject(trimmed, limits.MaxJSONDepth, &remainingNodes); err != nil {
		return Document{}, errors.Join(ErrInvalidSource, err)
	}

	return Document{Tenant: tenant, Index: index, ID: id, Version: version, Source: bytes.Clone(source)}, nil
}

func validateBoundedJSONObject(value json.RawMessage, maximumDepth int, remainingNodes *int) error {
	if !utf8.Valid(value) {
		return errMalformedJSONObject
	}
	decoder := json.NewDecoder(bytes.NewReader(value))
	// Preserve decimal spelling rather than rounding through float64 while walking.
	decoder.UseNumber()
	opening, err := decoder.Token()
	if err != nil {
		return errMalformedJSONObject
	}
	delimiter, _ := opening.(json.Delim)
	if delimiter != '{' {
		return errMalformedJSONObject
	}
	if err := validateJSONContainer(decoder, delimiter, 1, maximumDepth, remainingNodes); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errMalformedJSONObject
	}
	return nil
}

func validateJSONContainer(decoder *json.Decoder, opening json.Delim, depth, maximumDepth int, remainingNodes *int) error {
	if depth > maximumDepth {
		return ErrJSONDepthLimit
	}
	var keys map[string]struct{}
	if opening == '{' {
		keys = make(map[string]struct{})
	}
	for decoder.More() {
		if keys != nil {
			token, err := decoder.Token()
			if err != nil {
				return errMalformedJSONObject
			}
			key := token.(string)
			if _, duplicate := keys[key]; duplicate {
				return ErrDuplicateJSONKey
			}
			keys[key] = struct{}{}
		}
		if *remainingNodes == 0 {
			return ErrJSONNodeLimit
		}
		*remainingNodes = *remainingNodes - 1
		value, err := decoder.Token()
		if err != nil {
			return errMalformedJSONObject
		}
		if delimiter, nested := value.(json.Delim); nested {
			if err := validateJSONContainer(decoder, delimiter, depth+1, maximumDepth, remainingNodes); err != nil {
				return err
			}
		}
	}
	_, err := decoder.Token()
	if err != nil {
		return errMalformedJSONObject
	}
	return nil
}
