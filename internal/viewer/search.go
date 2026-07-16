package viewer

import (
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/store"
)

func registerSearchRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/search", getSearchHandler(s))
}

// getSearchHandler serves §3.8's cross-cutting search: effective tag id/name,
// action/given/then vocab id/label, transition id, and action kind name
// (index.Search). An empty q returns an empty result the same way the UI
// skips the call for an empty search box.
func getSearchHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		result := index.Search(ix, r.URL.Query().Get("q"))
		writeJSON(w, http.StatusOK, result)
	}
}
