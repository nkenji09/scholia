package viewer

import (
	"fmt"
	"net/http"

	"github.com/nkenji09/product-memory/internal/review"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerReviewsRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/reviews", getReviewsHandler(s))
	mux.HandleFunc("DELETE /api/reviews/{id}", deleteReviewHandler(s))
}

// getReviewsHandler serves GET /api/reviews: the AI-comment delivery sidecar
// (§8.4). Reviews are written by `pmem review add` under .pmem/reviews/ —
// not records, so they go through internal/review's own reader instead of
// store.LoadAll (§8.4 grounding: LoadAll only opens the four fixed
// subdirectories and never sees reviews/). This is a read-only route; the
// viewer never writes (creates) a review — G-3 is not reversed. DELETE below
// is the one write this file has: it only ever removes an overlay comment
// the frontend has already folded into a decision (§35 T-review-adopt/
// -reject cleanup step), never adds or edits one.
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

type deleteReviewResponse struct {
	ID string `json:"id"`
}

// deleteReviewHandler serves DELETE /api/reviews/{id}: the cleanup half of
// adopt/reject (§35 T-review-adopt/T-review-reject — eff.storage.
// delete-review, called by the frontend only after its POST /api/decision
// has already succeeded, so a proposal's why is never lost). Server-mode
// only, like every other viewer write (§7 narrow rule) — a static export
// has no write API at all, so this route simply doesn't exist there.
func deleteReviewHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validTransitionID(id) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("id %q は不正です（'/' '\\' や '.'/'..' は使えません）", id))
			return
		}
		if err := review.Delete(s.Dir, id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, deleteReviewResponse{ID: id})
	}
}
