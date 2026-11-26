package opensearch

import (
	"slices"
	"testing"
)

func TestSupportedOpenSearchVersionsIncludesCurrentRelease(t *testing.T) {
	t.Parallel()

	want := []string{"2.19.8", "3.10.1"}
	if got := SupportedOpenSearchVersions(); !slices.Equal(got, want) {
		t.Fatalf("SupportedOpenSearchVersions() = %v, want %v", got, want)
	}
}
