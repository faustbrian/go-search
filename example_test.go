package search_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/faustbrian/go-search"
	"github.com/faustbrian/go-search/searchtest"
)

func Example() {
	ctx := context.Background()
	limits := search.DefaultLimits()
	engine, err := searchtest.NewFake(limits)
	if err != nil {
		log.Fatal(err)
	}
	document, err := search.NewDocument(
		"tenant-a", "locations", "hel", 1,
		json.RawMessage(`{"country":"FI"}`), limits,
	)
	if err != nil {
		log.Fatal(err)
	}
	if _, err = engine.Write(
		ctx, search.IndexDocument(document), search.RefreshWaitFor,
	); err != nil {
		log.Fatal(err)
	}
	result, err := engine.Search(ctx, search.Request{
		Tenant: "tenant-a",
		Index:  "locations",
		Query:  search.TermQuery{Field: "country", Value: search.StringValue("FI")},
		Sort:   []search.Sort{{Field: search.DocumentIDSortField, Direction: search.Ascending}},
		Page:   search.OffsetPage{Size: 10},
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(result.Hits()[0].ID)
	// Output: hel
}
