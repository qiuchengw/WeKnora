package handler

import (
	"github.com/Tencent/WeKnora/internal/types"
)

// hybridSearchResultResponse is the HTTP projection of a retrieval result for
// `hybrid-search`.
//
// Why a projection instead of exposing types.SearchResult.ContentRevision directly:
// that field is deliberately excluded from JSON (`json:"-"`). SearchResult values are
// persisted inside message records, and replaying them from JSON must keep
// ContentRevision at zero so the merge pipeline fails open to the position path
// (see chunkTrusted). A retrieval caller has the opposite requirement: a citation
// needs the revision that was current **at retrieval time**, and it never replays a
// stored payload.
//
// Embedding keeps every existing field byte-for-byte identical — only
// `content_revision` is added.
type hybridSearchResultResponse struct {
	*types.SearchResult
	// ContentRevision is emitted **always** (no omitempty): 0 means "never edited",
	// which a consumer must be able to tell apart from "the server did not say".
	ContentRevision int `json:"content_revision"`
}

// projectHybridSearchResults projects retrieval results for the `hybrid-search`
// response. Nil slots are preserved (they marshal to `null` exactly as before), so
// the projection changes nothing but the added field.
func projectHybridSearchResults(results []*types.SearchResult) []*hybridSearchResultResponse {
	projected := make([]*hybridSearchResultResponse, len(results))
	for i, result := range results {
		if result == nil {
			continue
		}
		projected[i] = &hybridSearchResultResponse{
			SearchResult:    result,
			ContentRevision: result.ContentRevision,
		}
	}
	return projected
}
