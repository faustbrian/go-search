package search

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	// ErrInvalidIndexDefinition rejects unsafe physical names or invalid bounded definition
	// objects.
	ErrInvalidIndexDefinition = errors.New("search: invalid index definition")
	// ErrSchemaLimit rejects combined settings/mappings bytes beyond MaxSourceBytes.
	ErrSchemaLimit = errors.New("search: index definition exceeds size limit")
)

// IndexDefinition is an immutable backend-neutral settings and mapping
// definition. Backend-specific settings remain explicit JSON owned by the
// adapter rather than being silently normalized into shared semantics.
type IndexDefinition struct {
	name        string
	settings    json.RawMessage
	mappings    json.RawMessage
	fingerprint string
}

// NewIndexDefinition validates and canonicalizes bounded JSON object settings
// and mappings. The fingerprint excludes the physical index name so equivalent
// generations compare equal. Names are at most 255 UTF-8 bytes and reject
// uppercase, whitespace/control and reserved path characters/prefixes; lowercase
// Unicode is accepted here, while individual adapters may impose ASCII grammar.
// Settings/mappings share one node budget; backend defaults are not materialized.
func NewIndexDefinition(name string, settings, mappings json.RawMessage, limits Limits) (IndexDefinition, error) {
	if !validIndexName(name) {
		return IndexDefinition{}, ErrInvalidIndexDefinition
	}
	if limits.MaxJSONDepth <= 0 || limits.MaxJSONNodes <= 0 {
		return IndexDefinition{}, ErrInvalidLimits
	}
	if len(settings)+len(mappings) > limits.MaxSourceBytes {
		return IndexDefinition{}, ErrSchemaLimit
	}
	remainingNodes := limits.MaxJSONNodes
	canonicalSettings, err := canonicalBoundedJSONObject(settings, limits.MaxJSONDepth, &remainingNodes)
	if err != nil {
		return IndexDefinition{}, errors.Join(ErrInvalidIndexDefinition, err)
	}
	canonicalMappings, err := canonicalBoundedJSONObject(mappings, limits.MaxJSONDepth, &remainingNodes)
	if err != nil {
		return IndexDefinition{}, errors.Join(ErrInvalidIndexDefinition, err)
	}
	// Wire contract v1 is protected by
	// TestIndexDefinitionFingerprintIsCanonicalAndCompatibilityIsExplicit. The
	// separator prevents settings/mappings concatenation ambiguity; names and
	// unmaterialized backend defaults deliberately do not enter this identity.
	hash := sha256.New()
	_, _ = hash.Write(canonicalSettings)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(canonicalMappings)
	return IndexDefinition{
		name: name, settings: canonicalSettings, mappings: canonicalMappings,
		fingerprint: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

// Name returns the retained physical name.
func (d IndexDefinition) Name() string { return d.name }

// Settings returns an independent copy of canonical object JSON.
func (d IndexDefinition) Settings() json.RawMessage {
	return append(json.RawMessage(nil), d.settings...)
}

// Mappings returns an independent copy of canonical object JSON.
func (d IndexDefinition) Mappings() json.RawMessage {
	return append(json.RawMessage(nil), d.mappings...)
}

// Fingerprint returns wire-contract-v1 lowercase SHA-256 of canonical settings,
// a zero byte, then mappings. It excludes names and implicit backend defaults;
// Go JSON number spellings remain distinct, and this is not RFC 8785 canonicalization.
func (d IndexDefinition) Fingerprint() string { return d.fingerprint }

// CompatibilityKind is the conservative byte-definition compatibility decision.
type CompatibilityKind string

const (
	// Compatible means the retained fingerprints compare equal, including two zero definitions.
	Compatible CompatibilityKind = "compatible"
	// ReindexRequired means definition fingerprints differ; no in-place compatibility
	// inference is made.
	ReindexRequired CompatibilityKind = "reindex_required"
)

// Compatibility reports canonical equality or an explicit reindex requirement; it is not
// backend mapping analysis.
type Compatibility struct {
	// Kind selects equality or an explicit migration requirement.
	Kind CompatibilityKind
	// Reasons lists changed canonical settings/mappings in a caller-owned slice.
	Reasons []string
}

// CompareDefinitions reports whether two physical generations have identical
// canonical definition bytes, excluding names. Any difference requires an explicit reindex
// plan;
// adapters may offer a more detailed backend-native compatibility analysis.
func CompareDefinitions(current, target IndexDefinition) Compatibility {
	if current.fingerprint == target.fingerprint {
		return Compatibility{Kind: Compatible}
	}
	reasons := make([]string, 0, 2)
	if !bytes.Equal(current.settings, target.settings) {
		reasons = append(reasons, "settings changed")
	}
	if !bytes.Equal(current.mappings, target.mappings) {
		reasons = append(reasons, "mappings changed")
	}
	return Compatibility{Kind: ReindexRequired, Reasons: reasons}
}

func canonicalBoundedJSONObject(value json.RawMessage, maximumDepth int, remainingNodes *int) (json.RawMessage, error) {
	if err := validateBoundedJSONObject(value, maximumDepth, remainingNodes); err != nil {
		return nil, err
	}
	return canonicalJSONObject(value)
}

func canonicalJSONObject(value json.RawMessage) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(value))
	// Preserve number text; sorted object keys remove key-order differences only.
	decoder.UseNumber()
	var decoded any
	if err := decoder.Decode(&decoded); err != nil {
		return nil, err
	}
	if _, ok := decoded.(map[string]any); !ok {
		return nil, ErrInvalidIndexDefinition
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, err
	}
	return json.Marshal(decoded)
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return ErrInvalidIndexDefinition
	}
	return nil
}

func validIndexName(name string) bool {
	if name == "" || !utf8.ValidString(name) || len(name) > 255 || name == "." || name == ".." || strings.HasPrefix(name, "_") || strings.HasPrefix(name, "-") || strings.HasPrefix(name, "+") {
		return false
	}
	for _, character := range name {
		if unicode.IsUpper(character) || unicode.IsSpace(character) || unicode.IsControl(character) || strings.ContainsRune(`\/*?"<>|,#:`, character) {
			return false
		}
	}
	return true
}
