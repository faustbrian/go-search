package search_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/faustbrian/go-search"
)

// BenchmarkRequestFingerprint measures one fixed two-clause/filter request with
// 50-item cursor metadata. Request construction precedes b.Loop; hashing and
// validation budget checks are measured, not backend query work.
func BenchmarkRequestFingerprint(b *testing.B) {
	request := search.Request{Tenant: "tenant-a", Index: "locations", Query: search.BoolQuery{Filter: []search.Query{search.TermQuery{Field: "country", Value: search.StringValue("FI")}, search.ExistsQuery{Field: "position"}}}, Sort: []search.Sort{{Field: search.DocumentIDSortField, Direction: search.Ascending}}, Page: search.CursorPage{Size: 50, KeepAlive: time.Minute}}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := search.RequestFingerprint(request, search.DefaultLimits()); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCursorRoundTrip measures encode plus decode of one sort scalar under
// a fixed clock/key and 4096-byte cap. Codec/state setup is outside b.Loop; this
// is not backend/PIT performance or a comparison across crypto implementations.
func BenchmarkCursorRoundTrip(b *testing.B) {
	now := time.Unix(1_800_000_000, 0)
	codec, _ := search.NewCursorCodec([]byte("0123456789abcdef0123456789abcdef"), func() time.Time { return now }, 4096)
	binding := search.CursorBinding{Tenant: "tenant-a", Index: "locations", QueryFingerprint: "query", IndexFingerprint: "mapping"}
	state := search.CursorState{PointInTime: "pit", SortValues: []json.RawMessage{json.RawMessage(`"id-1"`)}, ExpiresAt: now.Add(time.Minute)}
	b.ReportAllocs()
	for b.Loop() {
		token, err := codec.Encode(binding, state)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := codec.Decode(token, binding, search.DefaultLimits()); err != nil {
			b.Fatal(err)
		}
	}
}
