package viewer

import (
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

func registerSearchRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/search", getSearchHandler(s))
}

// searchResponse is GET /api/search's envelope (#45 D10b-3). It keeps the
// existing transition-grouped fields (transitions / matchedOn) that the
// current search UI reads (additive/backward-compatible), and adds records:
// the unified core's match-level hits across all four record types (index.
// SearchRecords), which the decision search surface (D10a) reads. Both come
// from the same core so CLI `scholia search` and this endpoint answer the same
// query the same way (面間整合原則 D10b-2).
type searchResponse struct {
	// Embedded so transitions / matchedOn stay top-level fields exactly as
	// before (existing UI clients decode them unchanged).
	index.SearchResult
	Records []index.RecordMatch `json:"records"`
}

// getSearchHandler serves the cross-record search. An empty q returns an empty
// result the same way the UI skips the call for an empty search box.
func getSearchHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		q := r.URL.Query().Get("q")
		result := index.Search(ix, q)
		records := index.SearchRecords(ix, []string{q}, nil)
		writeJSON(w, http.StatusOK, searchResponse{SearchResult: result, Records: records})
	}
}
