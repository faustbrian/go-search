package search

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"time"
	"unicode/utf8"
)

const (
	// MaxFieldNameBytes bounds individual field names before backend encoding.
	MaxFieldNameBytes = 256
	// MaxNumberBytes bounds an exact decimal representation.
	MaxNumberBytes = 256
)

var (
	errInvalidNumber   = errors.New("search: invalid exact number")
	errInvalidGeoPoint = errors.New("search: invalid geo point")
	exactNumberPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)
)

// ValueKind identifies a lossless search document value representation.
type ValueKind uint8

const (
	// KindNull represents explicit JSON null and is the zero kind.
	KindNull ValueKind = iota
	// KindString retains text without Unicode normalization.
	KindString
	// KindNumber retains bounded decimal spelling without binary floating-point conversion.
	KindNumber
	// KindBool represents JSON true or false.
	KindBool
	// KindTime encodes a UTC RFC3339Nano string.
	KindTime
	// KindGeo encodes latitude and longitude coordinates.
	KindGeo
	// KindArray retains an independently copied sequence of Values.
	KindArray
	// KindObject retains an independently copied name-to-Value map.
	KindObject
)

// GeoPoint is a WGS84 latitude and longitude pair.
type GeoPoint struct {
	// Latitude is a finite degree coordinate in [-90, 90] for GeoValue.
	Latitude float64
	// Longitude is a finite degree coordinate in [-180, 180] for GeoValue.
	Longitude float64
}

// Value is an immutable, JSON-compatible document value. Numbers retain their
// decimal spelling and timestamps are normalized to UTC without losing
// nanoseconds. A missing field is represented only by absence from a field map;
// NullValue represents an explicitly present null.
type Value struct {
	kind   ValueKind
	text   string
	truth  bool
	point  GeoPoint
	array  []Value
	object map[string]Value
}

// NullValue returns explicit JSON null; an absent object key is different.
func NullValue() Value { return Value{kind: KindNull} }

// StringValue retains text as supplied. UTF-8 validity is checked on JSON encoding.
func StringValue(value string) Value { return Value{kind: KindString, text: value} }

// NumberValue accepts at most 256 bytes of decimal JSON syntax without an exponent.
// It preserves spelling, including negative zero, and rejects leading-zero integers.
func NumberValue(value string) (Value, error) {
	if len(value) == 0 || len(value) > MaxNumberBytes || !exactNumberPattern.MatchString(value) {
		return Value{}, errInvalidNumber
	}

	return Value{kind: KindNumber, text: value}, nil
}

// BoolValue returns a JSON boolean.
func BoolValue(value bool) Value { return Value{kind: KindBool, truth: value} }

// TimeValue formats the supplied time in UTC with RFC3339Nano; it does not validate an
// application time range.
func TimeValue(value time.Time) Value {
	return Value{kind: KindTime, text: value.UTC().Format(time.RFC3339Nano)}
}

// GeoValue rejects nonfinite or out-of-range degree coordinates.
func GeoValue(latitude, longitude float64) (Value, error) {
	if math.IsNaN(latitude) || math.IsInf(latitude, 0) || latitude < -90 || latitude > 90 ||
		math.IsNaN(longitude) || math.IsInf(longitude, 0) || longitude < -180 || longitude > 180 {
		return Value{}, errInvalidGeoPoint
	}

	return Value{kind: KindGeo, point: GeoPoint{Latitude: latitude, Longitude: longitude}}, nil
}

// ArrayValue recursively copies values. It applies no Limits budget; nil encodes as null.
func ArrayValue(values []Value) Value {
	return Value{kind: KindArray, array: cloneValues(values)}
}

// ObjectValue recursively copies fields. It applies no Limits budget; nil encodes as null.
func ObjectValue(fields map[string]Value) Value {
	return Value{kind: KindObject, object: cloneFields(fields)}
}

// Kind returns the retained discriminator; the zero Value has KindNull.
func (v Value) Kind() ValueKind { return v.kind }

// MarshalJSON rejects invalid UTF-8 recursively and encodes the retained representation.
// It imposes no size/depth budget and does not redact values.
func (v Value) MarshalJSON() ([]byte, error) {
	if !validUTF8Value(v) {
		return nil, errors.New("search: value contains invalid UTF-8")
	}
	switch v.kind {
	case KindNull:
		return []byte("null"), nil
	case KindString, KindTime:
		return json.Marshal(v.text)
	case KindNumber:
		return []byte(v.text), nil
	case KindBool:
		return json.Marshal(v.truth)
	case KindGeo:
		return json.Marshal(struct {
			Latitude  float64 `json:"lat"`
			Longitude float64 `json:"lon"`
		}{Latitude: v.point.Latitude, Longitude: v.point.Longitude})
	case KindArray:
		return json.Marshal(v.array)
	case KindObject:
		return json.Marshal(v.object)
	default:
		return nil, fmt.Errorf("search: unknown value kind %d", v.kind)
	}
}

func validUTF8Value(value Value) bool {
	if !utf8.ValidString(value.text) {
		return false
	}
	for _, child := range value.array {
		if !validUTF8Value(child) {
			return false
		}
	}
	for key, child := range value.object {
		if !utf8.ValidString(key) || !validUTF8Value(child) {
			return false
		}
	}
	return true
}

func cloneFields(fields map[string]Value) map[string]Value {
	if fields == nil {
		return nil
	}
	result := make(map[string]Value, len(fields))
	for name, value := range fields {
		result[name] = value.clone()
	}
	return result
}

func cloneValues(values []Value) []Value {
	if values == nil {
		return nil
	}
	result := make([]Value, len(values))
	for index, value := range values {
		result[index] = value.clone()
	}
	return result
}

func (v Value) clone() Value {
	v.array = cloneValues(v.array)
	v.object = cloneFields(v.object)
	return v
}
