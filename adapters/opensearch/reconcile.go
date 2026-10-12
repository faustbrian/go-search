package opensearch

import (
	"context"
	"errors"
	"time"

	"github.com/faustbrian/go-search"
)

// ReconciliationPITKeepAlive requests a two-minute PIT deadline; smaller configured
// cursor limits reject Read rather than silently changing this request.
const ReconciliationPITKeepAlive = 2 * time.Minute

// ErrPartialResults identifies a partial cursor search, which cannot advance reconciliation
// safely.
var ErrPartialResults = errors.New("search/opensearch: partial results cannot be reconciled")

// Read implements search.ReconciliationReader with a stable ID-ordered PIT
// projection. Partial cursor results are refused; source digests are calculated
// locally and source bytes are not retained in records or diagnostics. A full page
// retains a PIT continuation, so abandoned readers rely on expiry/application policy.
func (c *Client) Read(ctx context.Context, tenant, index, cursor string, pageSize int) (search.ReconciliationPage, error) {
	result, err := c.Search(ctx, search.Request{
		Tenant: tenant, Index: index, Query: search.MatchAllQuery{},
		Sort: []search.Sort{{Field: search.DocumentIDSortField, Direction: search.Ascending}},
		Page: search.CursorPage{Size: pageSize, Cursor: cursor, KeepAlive: ReconciliationPITKeepAlive},
	})
	if err != nil {
		return search.ReconciliationPage{}, err
	}
	hits := result.Hits()
	records := make([]search.ReconciliationRecord, len(hits))
	for position, hit := range hits {
		if hit.Version == 0 || len(hit.Source) == 0 {
			return search.ReconciliationPage{}, malformedFailure(OperationSearch, ErrMalformedResponse)
		}
		records[position] = search.IndexRecord(hit.ID, hit.Version, search.SourceDigest(hit.Source))
	}
	return search.ReconciliationPage{Records: records, Cursor: result.NextCursor(), Done: result.NextCursor() == ""}, nil
}

var _ search.ReconciliationReader = (*Client)(nil)
