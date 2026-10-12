package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	// ErrInvalidQuery rejects malformed, over-budget, or unrecognized typed request input.
	ErrInvalidQuery = errors.New("search: invalid query")
	// ErrUnsupported identifies a capability or adapter-bound node that cannot be executed
	// without degradation.
	ErrUnsupported = errors.New("search: unsupported capability")
	// ErrUnstableSort requires the final cursor sort component to be DocumentIDSortField.
	ErrUnstableSort = errors.New("search: cursor pagination requires a final document ID sort")
	// ErrPageLimit rejects invalid pagination shape or exhausted traversal bounds.
	ErrPageLimit = errors.New("search: pagination limit exceeded")
)

// DocumentIDSortField is the backend-neutral virtual field for a document's
// stable ID. Adapters must translate it to their native metadata field and
// must not expose that backend spelling through the shared request contract.
const DocumentIDSortField = "$document_id"

// Capabilities declares adapter behavior. False means the adapter must reject
// the feature before network execution rather than silently degrading it.
type Capabilities struct {
	// Boolean, Term, FullText, Prefix, Range, Exists, and Geo advertise their respective typed
	// query families.
	Boolean, Term, FullText, Prefix, Range, Exists, Geo bool
	// Cursor and PointInTime jointly enable PIT pagination; Offset enables bounded
	// non-snapshot windows.
	Cursor, PointInTime, Offset bool
	// Projection, Highlight, Aggregation, and Suggestion advertise result-shaping families.
	Projection, Highlight, Aggregation, Suggestion bool
	// ExternalVersion, UpdateExisting, and BulkPartialOutcomes advertise versioned
	// write/result semantics.
	ExternalVersion, UpdateExisting, BulkPartialOutcomes bool
	// Lifecycle and Templates advertise management surfaces; RawExtensions advertises explicit
	// adapter-native query nodes.
	Lifecycle, Templates, RawExtensions bool
}

// AllCapabilities returns the complete shared capability set, primarily for
// conformance tests. Production adapters should report only proven features.
func AllCapabilities() Capabilities {
	return Capabilities{
		Boolean: true, Term: true, FullText: true, Prefix: true, Range: true,
		Exists: true, Geo: true, Cursor: true, PointInTime: true, Offset: true,
		Projection: true, Highlight: true, Aggregation: true, Suggestion: true,
		ExternalVersion: true, UpdateExisting: true, BulkPartialOutcomes: true, Lifecycle: true,
		Templates: true, RawExtensions: true,
	}
}

// Query is a typed query node supported by capability-aware adapters.
type Query interface{ queryNode() }

// MatchAllQuery selects all documents in the resolved scope; it does not bypass tenant policy.
type MatchAllQuery struct{}

func (MatchAllQuery) queryNode() {}

// BoolQuery combines nested clauses without executing or scoring them in the core.
// Adapters define ranking and zero-minimum should semantics; do not assume analyzer parity.
type BoolQuery struct {
	// Must and Filter require matches; MustNot excludes matches; Should contributes
	// optional/required alternatives according to MinimumShouldMatch and adapter semantics.
	Must, Should, Filter, MustNot []Query
	// MinimumShouldMatch is explicit and nonnegative, no greater than the number of Should
	// clauses.
	MinimumShouldMatch int
}

func (BoolQuery) queryNode() {}

// TermQuery matches an exact scalar indexed value, not full-text analysis.
type TermQuery struct {
	// Field is a bounded indexed field name.
	Field string
	// Value must be string, number, boolean, or time; null and containers are rejected.
	Value Value
}

func (TermQuery) queryNode() {}

// FullTextQuery carries backend-analyzed text; mappings, locale analyzers, and relevance
// remain application/adapter policy.
type FullTextQuery struct {
	// Fields contains bounded field names with optional positive finite boosts.
	Fields []string
	// Text is nonblank input; Analyzer and Locale select explicit backend analysis policy,
	// without Unicode normalization in the core.
	Text, Analyzer, Locale string
}

func (FullTextQuery) queryNode() {}

// PrefixQuery matches an indexed string prefix; the core does not fold case or normalize
// Unicode.
type PrefixQuery struct {
	// Field names the indexed string field; Prefix is nonempty and compared by the adapter.
	Field, Prefix string
}

func (PrefixQuery) queryNode() {}

// RangeQuery supplies at least one scalar bound; comparison semantics follow the field mapping.
type RangeQuery struct {
	// Field names the mapped comparison field.
	Field string
	// GT and GTE are exclusive lower-bound choices; LT and LTE are exclusive upper-bound
	// choices. Nil omits a bound.
	GT, GTE, LT, LTE *Value
}

func (RangeQuery) queryNode() {}

// ExistsQuery tests indexed-value presence, which differs from JSON-key presence for
// null/empty arrays.
type ExistsQuery struct {
	// Field names an indexed-value presence check.
	Field string
}

func (ExistsQuery) queryNode() {}

// GeoDistanceQuery supplies finite degree coordinates and a positive numeric kilometer radius.
type GeoDistanceQuery struct {
	// Field names a mapped geographic field.
	Field string
	// Origin is a finite WGS84 latitude/longitude degree pair.
	Origin GeoPoint
	// DistanceKM is a positive numeric radius validated through float64 parsing, not a
	// unit-bearing string.
	DistanceKM Value
}

func (GeoDistanceQuery) queryNode() {}

// RawExtensionQuery carries one bounded backend-specific query object. Adapter
// identifies the only adapter allowed to interpret Payload. Applications must
// authorize construction of these nodes and must not pass unrestricted caller
// JSON through this type.
type RawExtensionQuery struct {
	// Adapter explicitly identifies the only intended interpreter.
	Adapter string
	// Payload is a bounded object without duplicate keys; callers must authorize its
	// semantics, not just its syntax.
	Payload json.RawMessage
}

func (RawExtensionQuery) queryNode() {}

// SortDirection defines ascending or descending ordering.
type SortDirection string

const (
	// Ascending requests increasing backend field order.
	Ascending SortDirection = "asc"
	// Descending requests decreasing backend field order.
	Descending SortDirection = "desc"
)

// MissingPlacement defines where missing sort values appear.
type MissingPlacement string

const (
	// MissingDefault leaves missing-value placement to the backend.
	MissingDefault MissingPlacement = ""
	// MissingFirst requests missing values before present values.
	MissingFirst MissingPlacement = "_first"
	// MissingLast requests missing values after present values.
	MissingLast MissingPlacement = "_last"
)

// Sort is one ordered sort component; cursor requests end with DocumentIDSortField.
type Sort struct {
	// Field is a shared field or DocumentIDSortField; backend metadata spellings are rejected.
	Field string
	// Direction must be Ascending or Descending.
	Direction SortDirection
	// Missing selects explicit missing-value placement or the backend default.
	Missing MissingPlacement
}

// CursorPage requests bounded PIT traversal. Empty Cursor starts traversal; continuation
// preserves a fixed absolute deadline.
type CursorPage struct {
	// Size is positive and no greater than MaxPageItems.
	Size int
	// Cursor is an opaque sensitive token; keep it unchanged and do not log it.
	Cursor string
	// KeepAlive is a positive whole-millisecond duration no greater than MaxCursorDuration.
	KeepAlive time.Duration
}

// OffsetPage requests a bounded non-snapshot window; concurrent writes may shift results.
type OffsetPage struct {
	// Size is a positive page bound; Offset is nonnegative and must leave room within the
	// total item limit.
	Size, Offset int
}

// Projection selects source paths, not schema validation or authorization. Empty lists
// request full source.
type Projection struct {
	// Includes and Excludes are borrowed field-pattern lists; empty lists request all source
	// fields.
	Includes, Excludes []string
}

// Highlight requests backend text fragments. Tags and output remain untrusted content, not
// safe HTML.
type Highlight struct {
	// FragmentSize and MaxFragments request positive bounded backend fragment sizing.
	FragmentSize, MaxFragments int
	// PreTag and PostTag are untrusted UTF-8 delimiters; neither validation nor results
	// sanitize HTML.
	PreTag, PostTag string
}

// Aggregation is the closed set of typed aggregation nodes recognized by validation.
type Aggregation interface{ aggregationNode() }

// TermsAggregation requests a bounded backend term bucket set; results remain
// backend-native JSON.
type TermsAggregation struct {
	// Field is the mapped aggregation field.
	Field string
	// Size is a positive bucket count no greater than MaxPageItems.
	Size int
}

func (TermsAggregation) aggregationNode() {}

// RangeBucket supplies a named range with at least one bound; ordering/comparison is
// mapping-dependent.
type RangeBucket struct {
	// Key is a nonempty UTF-8 result bucket label.
	Key string
	// From and To are optional scalar bounds, at least one present; exact boundary behavior
	// follows the backend.
	From, To *Value
}

// RangeAggregation requests explicit range buckets in retained input order.
type RangeAggregation struct {
	// Field is the mapped range field.
	Field string
	// Buckets is a borrowed nonempty sequence charged to the query-node budget.
	Buckets []RangeBucket
}

func (RangeAggregation) aggregationNode() {}

// Suggestion is the closed set of typed suggestion nodes recognized by validation.
type Suggestion interface{ suggestionNode() }

// PrefixSuggestion requests a bounded completion-prefix result, not general spell correction.
type PrefixSuggestion struct {
	// Field is a completion field; Text is a nonempty UTF-8 prefix.
	Field, Text string
	// Size is a positive result count no greater than MaxPageItems.
	Size int
}

func (PrefixSuggestion) suggestionNode() {}

// Request contains a backend-neutral typed search operation.
type Request struct {
	// Tenant and Index identify logical scope, not authentication or authorization.
	Tenant, Index string
	// Query is a recognized typed value node; pointer forms are not recognized by the validator.
	Query Query
	// Sort is a borrowed ordered tuple, ending in the document-ID tie-breaker for cursor pages.
	Sort []Sort
	// Page must contain a CursorPage or OffsetPage value, not a pointer.
	Page any
	// Projection controls source disclosure and requires application policy.
	Projection Projection
	// Highlights maps requested fields to borrowed fragment settings.
	Highlights map[string]Highlight
	// Aggregations maps result names to recognized typed aggregation values.
	Aggregations map[string]Aggregation
	// Suggestions maps result names to recognized typed suggestion values.
	Suggestions map[string]Suggestion
}

// Validate checks positive limits, UTF-8, input/encoded budgets, supported value
// nodes, pagination, and shaping syntax before adapter I/O. It performs no
// normalization, authorization, mapping lookup, relevance evaluation, or HTML sanitization.
func (r Request) Validate(capabilities Capabilities, limits Limits) error {
	if limits.Validate() != nil {
		return ErrInvalidQuery
	}
	if r.Tenant == "" || len(r.Tenant) > limits.MaxTenantBytes ||
		r.Index == "" || len(r.Index) > limits.MaxIndexBytes {
		return ErrInvalidQuery
	}
	if r.Query == nil {
		return ErrInvalidQuery
	}
	if !requestInputsWithinBudget(r, limits) || !requestInputsValidUTF8(r) {
		return ErrInvalidQuery
	}
	clauses := 0
	if err := validateQuery(r.Query, capabilities, limits, 1, &clauses); err != nil {
		return err
	}
	if err := validateSort(r.Sort); err != nil {
		return err
	}
	switch page := r.Page.(type) {
	case CursorPage:
		if !capabilities.Cursor || !capabilities.PointInTime {
			return unsupported("cursor and point-in-time")
		}
		if page.Size <= 0 || page.Size > limits.MaxPageItems || page.KeepAlive < time.Millisecond ||
			page.KeepAlive > limits.MaxCursorDuration || page.KeepAlive%time.Millisecond != 0 {
			return ErrPageLimit
		}
		if len(r.Sort) == 0 || r.Sort[len(r.Sort)-1].Field != DocumentIDSortField {
			return ErrUnstableSort
		}
	case OffsetPage:
		if !capabilities.Offset {
			return unsupported("offset")
		}
		maximumItems := limits.MaxPages * limits.MaxPageItems
		if page.Size <= 0 || page.Size > limits.MaxPageItems || page.Offset < 0 ||
			page.Offset > maximumItems-page.Size {
			return ErrPageLimit
		}
	default:
		return ErrPageLimit
	}
	if (len(r.Projection.Includes) > 0 || len(r.Projection.Excludes) > 0) && !capabilities.Projection {
		return unsupported("source projection")
	}
	for _, fields := range [][]string{r.Projection.Includes, r.Projection.Excludes} {
		for _, field := range fields {
			if !validField(field) {
				return ErrInvalidQuery
			}
		}
	}
	if len(r.Highlights) > 0 && !capabilities.Highlight {
		return unsupported("highlight")
	}
	for field, highlight := range r.Highlights {
		if !validField(field) || highlight.FragmentSize <= 0 || highlight.FragmentSize > limits.MaxSourceBytes ||
			highlight.MaxFragments <= 0 || highlight.MaxFragments > limits.MaxPageItems {
			return ErrInvalidQuery
		}
	}
	if len(r.Aggregations) > 0 && !capabilities.Aggregation {
		return unsupported("aggregation")
	}
	for name, aggregation := range r.Aggregations {
		if !validField(name) || !validAggregation(aggregation, limits) {
			return ErrInvalidQuery
		}
	}
	if len(r.Suggestions) > 0 && !capabilities.Suggestion {
		return unsupported("suggestion")
	}
	for name, suggestion := range r.Suggestions {
		if !validField(name) || !validSuggestion(suggestion, limits) {
			return ErrInvalidQuery
		}
	}
	encoded, err := requestFingerprintPayload(r)
	if err != nil || len(encoded) > limits.MaxQueryBytes {
		return ErrInvalidQuery
	}

	return nil
}

func validateQuery(query Query, capabilities Capabilities, limits Limits, depth int, clauses *int) error {
	*clauses++
	if depth > limits.MaxQueryDepth || *clauses > limits.MaxQueryClauses {
		return ErrInvalidQuery
	}
	switch node := query.(type) {
	case MatchAllQuery:
		return nil
	case BoolQuery:
		if !capabilities.Boolean {
			return unsupported("boolean")
		}
		if len(node.Must) == 0 && len(node.Should) == 0 && len(node.Filter) == 0 && len(node.MustNot) == 0 ||
			node.MinimumShouldMatch < 0 || node.MinimumShouldMatch > len(node.Should) {
			return ErrInvalidQuery
		}
		for _, children := range [][]Query{node.Must, node.Should, node.Filter, node.MustNot} {
			for _, child := range children {
				if child == nil {
					return ErrInvalidQuery
				}
				if err := validateQuery(child, capabilities, limits, depth+1, clauses); err != nil {
					return err
				}
			}
		}
		return nil
	case TermQuery:
		if !capabilities.Term {
			return unsupported("term")
		}
		if !validField(node.Field) || !validTermValue(node.Value) {
			return ErrInvalidQuery
		}
	case FullTextQuery:
		if !capabilities.FullText {
			return unsupported("full text")
		}
		if len(node.Fields) == 0 || strings.TrimSpace(node.Text) == "" {
			return ErrInvalidQuery
		}
		for _, field := range node.Fields {
			if !validFullTextField(field) {
				return ErrInvalidQuery
			}
		}
	case PrefixQuery:
		if !capabilities.Prefix {
			return unsupported("prefix")
		}
		if !validField(node.Field) || node.Prefix == "" {
			return ErrInvalidQuery
		}
	case RangeQuery:
		if !capabilities.Range {
			return unsupported("range")
		}
		if !validField(node.Field) || node.GT == nil && node.GTE == nil && node.LT == nil && node.LTE == nil ||
			node.GT != nil && node.GTE != nil || node.LT != nil && node.LTE != nil {
			return ErrInvalidQuery
		}
		for _, bound := range []*Value{node.GT, node.GTE, node.LT, node.LTE} {
			if bound != nil && !validRangeValue(*bound) {
				return ErrInvalidQuery
			}
		}
	case ExistsQuery:
		if !capabilities.Exists {
			return unsupported("exists")
		}
		if !validField(node.Field) {
			return ErrInvalidQuery
		}
	case GeoDistanceQuery:
		if !capabilities.Geo {
			return unsupported("geo")
		}
		if !validField(node.Field) || math.IsNaN(node.Origin.Latitude) || math.IsInf(node.Origin.Latitude, 0) ||
			node.Origin.Latitude < -90 || node.Origin.Latitude > 90 || math.IsNaN(node.Origin.Longitude) ||
			math.IsInf(node.Origin.Longitude, 0) || node.Origin.Longitude < -180 || node.Origin.Longitude > 180 || node.DistanceKM.kind != KindNumber {
			return ErrInvalidQuery
		}
		distance, err := strconv.ParseFloat(node.DistanceKM.text, 64)
		if err != nil || distance <= 0 {
			return ErrInvalidQuery
		}
	case RawExtensionQuery:
		if !capabilities.RawExtensions {
			return unsupported("raw extension")
		}
		if !validField(node.Adapter) || len(node.Payload) == 0 || len(node.Payload) > limits.MaxSourceBytes {
			return ErrInvalidQuery
		}
		remainingNodes := limits.MaxJSONNodes
		if _, err := canonicalBoundedJSONObject(node.Payload, limits.MaxJSONDepth, &remainingNodes); err != nil {
			return ErrInvalidQuery
		}
	default:
		return ErrInvalidQuery
	}
	return nil
}

func validateSort(sorts []Sort) error {
	for _, sort := range sorts {
		if !validField(sort.Field) || backendMetadataSort(sort.Field) || sort.Direction != Ascending && sort.Direction != Descending ||
			sort.Missing != MissingDefault && sort.Missing != MissingFirst && sort.Missing != MissingLast {
			return ErrInvalidQuery
		}
	}
	return nil
}

func backendMetadataSort(field string) bool {
	switch field {
	case "_id", "_index", "_score", "_doc", "_shard_doc":
		return true
	default:
		return false
	}
}

func validAggregation(aggregation Aggregation, limits Limits) bool {
	switch value := aggregation.(type) {
	case TermsAggregation:
		return validField(value.Field) && value.Size > 0 && value.Size <= limits.MaxPageItems
	case RangeAggregation:
		if !validField(value.Field) || len(value.Buckets) == 0 || len(value.Buckets) > limits.MaxQueryClauses {
			return false
		}
		for _, bucket := range value.Buckets {
			if bucket.Key == "" || bucket.From == nil && bucket.To == nil ||
				bucket.From != nil && !validRangeValue(*bucket.From) || bucket.To != nil && !validRangeValue(*bucket.To) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func validSuggestion(suggestion Suggestion, limits Limits) bool {
	value, ok := suggestion.(PrefixSuggestion)
	return ok && validField(value.Field) && value.Text != "" && value.Size > 0 && value.Size <= limits.MaxPageItems
}

func validField(field string) bool {
	if field == "" || len(field) > MaxFieldNameBytes || strings.ContainsAny(field, "^\x00\r\n") {
		return false
	}
	return true
}

func validFullTextField(field string) bool {
	if len(field) > MaxFieldNameBytes {
		return false
	}
	base, boost, found := strings.Cut(field, "^")
	if !found {
		return validField(field)
	}
	value, err := strconv.ParseFloat(boost, 64)
	if !validField(base) || err != nil {
		return false
	}
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
		return false
	}
	return true
}

type requestInputBudget struct {
	bytes, nodes, maxCollection int
}

func requestInputsValidUTF8(request Request) bool {
	if !utf8.ValidString(request.Tenant) || !utf8.ValidString(request.Index) || !queryInputsValidUTF8(request.Query) {
		return false
	}
	for _, sort := range request.Sort {
		if !utf8.ValidString(sort.Field) {
			return false
		}
	}
	if page, ok := request.Page.(CursorPage); ok && !utf8.ValidString(page.Cursor) {
		return false
	}
	for _, fields := range [][]string{request.Projection.Includes, request.Projection.Excludes} {
		for _, field := range fields {
			if !utf8.ValidString(field) {
				return false
			}
		}
	}
	for field, highlight := range request.Highlights {
		if !utf8.ValidString(field) || !utf8.ValidString(highlight.PreTag) || !utf8.ValidString(highlight.PostTag) {
			return false
		}
	}
	for name, aggregation := range request.Aggregations {
		if !utf8.ValidString(name) || !aggregationInputsValidUTF8(aggregation) {
			return false
		}
	}
	for name, suggestion := range request.Suggestions {
		value, ok := suggestion.(PrefixSuggestion)
		if !ok || !utf8.ValidString(name) || !utf8.ValidString(value.Field) || !utf8.ValidString(value.Text) {
			return false
		}
	}
	return true
}

func queryInputsValidUTF8(query Query) bool {
	switch value := query.(type) {
	case MatchAllQuery:
		return true
	case BoolQuery:
		for _, children := range [][]Query{value.Must, value.Should, value.Filter, value.MustNot} {
			for _, child := range children {
				if !queryInputsValidUTF8(child) {
					return false
				}
			}
		}
		return true
	case TermQuery:
		return utf8.ValidString(value.Field) && validUTF8Value(value.Value)
	case FullTextQuery:
		if !utf8.ValidString(value.Text) || !utf8.ValidString(value.Analyzer) || !utf8.ValidString(value.Locale) {
			return false
		}
		for _, field := range value.Fields {
			if !utf8.ValidString(field) {
				return false
			}
		}
		return true
	case PrefixQuery:
		return utf8.ValidString(value.Field) && utf8.ValidString(value.Prefix)
	case RangeQuery:
		if !utf8.ValidString(value.Field) {
			return false
		}
		for _, bound := range []*Value{value.GT, value.GTE, value.LT, value.LTE} {
			if bound != nil && !validUTF8Value(*bound) {
				return false
			}
		}
		return true
	case ExistsQuery:
		return utf8.ValidString(value.Field)
	case GeoDistanceQuery:
		return utf8.ValidString(value.Field) && validUTF8Value(value.DistanceKM)
	case RawExtensionQuery:
		return utf8.ValidString(value.Adapter) && utf8.Valid(value.Payload)
	default:
		return false
	}
}

func aggregationInputsValidUTF8(aggregation Aggregation) bool {
	switch value := aggregation.(type) {
	case TermsAggregation:
		return utf8.ValidString(value.Field)
	case RangeAggregation:
		if !utf8.ValidString(value.Field) {
			return false
		}
		for _, bucket := range value.Buckets {
			if !utf8.ValidString(bucket.Key) {
				return false
			}
			for _, bound := range []*Value{bucket.From, bucket.To} {
				if bound != nil && !validUTF8Value(*bound) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

// Preflight borrowed collections before recursive validation and JSON construction;
// this is an input-work budget, not a process-heap or backend query-cost guarantee.
func requestInputsWithinBudget(request Request, limits Limits) bool {
	budget := requestInputBudget{bytes: limits.MaxQueryBytes, nodes: limits.MaxQueryClauses, maxCollection: limits.MaxQueryClauses}
	if !budget.consume(len(request.Tenant)) {
		return false
	}
	if !budget.consume(len(request.Index)) {
		return false
	}
	if !budget.query(request.Query, limits.MaxQueryDepth) {
		return false
	}
	for _, count := range []int{len(request.Sort), len(request.Projection.Includes), len(request.Projection.Excludes), len(request.Highlights), len(request.Aggregations), len(request.Suggestions)} {
		if count > limits.MaxQueryClauses {
			return false
		}
	}
	for _, sort := range request.Sort {
		if !budget.consume(len(sort.Field)) {
			return false
		}
	}
	if page, ok := request.Page.(CursorPage); ok {
		if !budget.consume(len(page.Cursor)) {
			return false
		}
	}
	for _, fields := range [][]string{request.Projection.Includes, request.Projection.Excludes} {
		for _, field := range fields {
			if !budget.consume(len(field)) {
				return false
			}
		}
	}
	for field, highlight := range request.Highlights {
		if !budget.consume(len(field)) {
			return false
		}
		if !budget.consume(len(highlight.PreTag)) {
			return false
		}
		if !budget.consume(len(highlight.PostTag)) {
			return false
		}
	}
	for name, aggregation := range request.Aggregations {
		if !budget.consume(len(name)) {
			return false
		}
		if !budget.aggregation(aggregation) {
			return false
		}
	}
	for name, suggestion := range request.Suggestions {
		if !budget.consume(len(name)) {
			return false
		}
		if !budget.suggestion(suggestion) {
			return false
		}
	}
	return true
}

func (b *requestInputBudget) consume(size int) bool {
	if size > b.bytes {
		return false
	}
	b.bytes -= size
	return true
}

func (b *requestInputBudget) node() bool {
	if b.nodes <= 0 {
		return false
	}
	b.nodes--
	return true
}

func (b *requestInputBudget) query(query Query, depth int) bool {
	if query == nil {
		return false
	}
	if depth <= 0 {
		return false
	}
	if !b.node() {
		return false
	}
	switch value := query.(type) {
	case MatchAllQuery:
		return true
	case BoolQuery:
		for _, children := range [][]Query{value.Must, value.Should, value.Filter, value.MustNot} {
			for _, child := range children {
				if !b.query(child, depth-1) {
					return false
				}
			}
		}
		return true
	case TermQuery:
		if !b.consume(len(value.Field)) {
			return false
		}
		return b.value(value.Value)
	case FullTextQuery:
		if len(value.Fields) > b.maxCollection {
			return false
		}
		if !b.consume(len(value.Text)) {
			return false
		}
		if !b.consume(len(value.Analyzer)) {
			return false
		}
		if !b.consume(len(value.Locale)) {
			return false
		}
		for _, field := range value.Fields {
			if !b.consume(len(field)) {
				return false
			}
		}
		return true
	case PrefixQuery:
		if !b.consume(len(value.Field)) {
			return false
		}
		return b.consume(len(value.Prefix))
	case RangeQuery:
		if !b.consume(len(value.Field)) {
			return false
		}
		for _, bound := range []*Value{value.GT, value.GTE, value.LT, value.LTE} {
			if bound != nil && !b.value(*bound) {
				return false
			}
		}
		return true
	case ExistsQuery:
		return b.consume(len(value.Field))
	case GeoDistanceQuery:
		if !b.consume(len(value.Field)) {
			return false
		}
		return b.value(value.DistanceKM)
	case RawExtensionQuery:
		if !b.consume(len(value.Adapter)) {
			return false
		}
		return b.consume(len(value.Payload))
	default:
		return false
	}
}

func (b *requestInputBudget) value(value Value) bool {
	switch value.kind {
	case KindString, KindNumber, KindTime:
		return b.consume(len(value.text))
	case KindNull, KindBool, KindGeo:
		return true
	default:
		return false
	}
}

func (b *requestInputBudget) aggregation(aggregation Aggregation) bool {
	if !b.node() {
		return false
	}
	switch value := aggregation.(type) {
	case TermsAggregation:
		return b.consume(len(value.Field))
	case RangeAggregation:
		if !b.consume(len(value.Field)) {
			return false
		}
		for _, bucket := range value.Buckets {
			if !b.node() {
				return false
			}
			if !b.consume(len(bucket.Key)) {
				return false
			}
			for _, bound := range []*Value{bucket.From, bucket.To} {
				if bound != nil && !b.value(*bound) {
					return false
				}
			}
		}
		return true
	default:
		return false
	}
}

func (b *requestInputBudget) suggestion(suggestion Suggestion) bool {
	value, ok := suggestion.(PrefixSuggestion)
	if !ok {
		return false
	}
	if !b.consume(len(value.Field)) {
		return false
	}
	return b.consume(len(value.Text))
}

func validTermValue(value Value) bool {
	switch value.kind {
	case KindString, KindNumber, KindBool, KindTime:
		return true
	default:
		return false
	}
}

func validRangeValue(value Value) bool {
	switch value.kind {
	case KindString, KindNumber, KindTime:
		return true
	default:
		return false
	}
}

func unsupported(name string) error { return fmt.Errorf("%w: %s", ErrUnsupported, name) }
