package viewer

import (
	"net/http"

	"github.com/nkenji09/product-memory/internal/review"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerReviewsRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/reviews", getReviewsHandler(s))
}

// getReviewsHandler serves GET /api/reviews: the AI-comment delivery sidecar
// (§8.4). Reviews are written by `pmem review add` under .pmem/reviews/ —
// not records, so they go through internal/review's own reader instead of
// store.LoadAll (§8.4 grounding: LoadAll only opens the four fixed
// subdirectories and never sees reviews/). This is a read-only route; the
// viewer never writes a review (G-3 is not reversed — see §8.4).
func getReviewsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reviews, err := review.List(s.Dir)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if reviews == nil {
			reviews = []review.Review{}
		}
		writeJSON(w, http.StatusOK, reviews)
	}
}
