package search

import (
	"errors"
	"time"
)

// ErrInvalidLimits rejects nonpositive limits or an overflowing page/item product.
var ErrInvalidLimits = errors.New("search: limits must be positive")

// Limits bounds caller-controlled identifiers, payloads, traversal, and
// response decoding. Adapters may advertise stricter limits.
type Limits struct {
	// MaxTenantBytes bounds UTF-8 tenant identifier bytes.
	MaxTenantBytes int
	// MaxIndexBytes bounds logical index identifier bytes.
	MaxIndexBytes int
	// MaxIDBytes bounds document and selected lifecycle identifier bytes.
	MaxIDBytes int
	// MaxSourceBytes bounds source bytes and combined definition JSON bytes.
	MaxSourceBytes int
	// MaxQueryBytes bounds request input and encoded fingerprint payload bytes.
	MaxQueryBytes int
	// MaxBulkItems bounds operations in one bulk request.
	MaxBulkItems int
	// MaxBulkBytes bounds the shared input estimate; adapters must also bound final wire bytes.
	MaxBulkBytes int
	// MaxPageItems bounds requested hits or records per page.
	MaxPageItems int
	// MaxPages bounds traversal pages; its product with MaxPageItems must fit int.
	MaxPages int
	// MaxResultBytes bounds adapter traversal bytes and reconciliation retained-state estimates.
	MaxResultBytes int64
	// MaxCursorDuration bounds remaining signed search-cursor lifetime at decode.
	MaxCursorDuration time.Duration
	// MaxQueryDepth bounds nested typed query depth.
	MaxQueryDepth int
	// MaxQueryClauses bounds query nodes and selected request collection sizes.
	MaxQueryClauses int
	// MaxJSONDepth bounds JSON object/array nesting at validated source boundaries.
	MaxJSONDepth int
	// MaxJSONNodes bounds JSON member/element counts; definitions share the budget across both
	// objects.
	MaxJSONNodes int
}

// DefaultLimits returns conservative bounded defaults.
func DefaultLimits() Limits {
	return Limits{
		MaxTenantBytes:    128,
		MaxIndexBytes:     255,
		MaxIDBytes:        512,
		MaxSourceBytes:    1 << 20,
		MaxQueryBytes:     1 << 20,
		MaxBulkItems:      500,
		MaxBulkBytes:      5 << 20,
		MaxPageItems:      1_000,
		MaxPages:          1_000,
		MaxResultBytes:    10 << 20,
		MaxCursorDuration: 15 * time.Minute,
		MaxQueryDepth:     32,
		MaxQueryClauses:   1_024,
		MaxJSONDepth:      32,
		MaxJSONNodes:      4_096,
	}
}

// Validate requires positive limits and a representable page/item product.
// It does not allocate resources, apply defaults, or bound arbitrary callbacks.
func (l Limits) Validate() error {
	if l.MaxTenantBytes <= 0 || l.MaxIndexBytes <= 0 || l.MaxIDBytes <= 0 ||
		l.MaxSourceBytes <= 0 || l.MaxQueryBytes <= 0 || l.MaxBulkItems <= 0 || l.MaxBulkBytes <= 0 ||
		l.MaxPageItems <= 0 || l.MaxPages <= 0 || l.MaxResultBytes <= 0 ||
		l.MaxCursorDuration <= 0 ||
		l.MaxQueryDepth <= 0 || l.MaxQueryClauses <= 0 || l.MaxJSONDepth <= 0 ||
		l.MaxJSONNodes <= 0 || l.MaxPages > int(^uint(0)>>1)/l.MaxPageItems {
		return ErrInvalidLimits
	}

	return nil
}
