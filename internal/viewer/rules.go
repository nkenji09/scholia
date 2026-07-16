package viewer

import (
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func registerRulesRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/rules", getRulesHandler(s))
}

type rulesResponse struct {
	Decisions []model.Decision `json:"decisions"`
}

// getRulesHandler serves GET /api/rules?tag=|tx=|facet=. Exactly one of the
// three selectors may be given; with none given at all, index.SortedRulesFor
// falls through to index.SelectRulesDecisions's "no selector" case and
// returns every decision in the project, chronologically ascending (oldest
// first) — this is the "全件モード" HomeView's recent-decisions widget uses
// (it takes the tail of the list; see .concierge/decision.md §F). That mode
// needed no new query-selection logic, only this doc comment plus
// TestGetRules_NoSelector locking the behavior in with a test.
func getRulesHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		tagID, txID, facet := q.Get("tag"), q.Get("tx"), q.Get("facet")

		selected := 0
		for _, v := range []string{tagID, txID, facet} {
			if v != "" {
				selected++
			}
		}
		if selected > 1 {
			writeError(w, http.StatusBadRequest, "tag / tx / facet は同時に指定できません")
			return
		}

		snap, _, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		decisions, err := index.SortedRulesFor(&snap, tagID, txID, facet)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rulesResponse{Decisions: decisions})
	}
}
