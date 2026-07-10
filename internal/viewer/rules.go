package viewer

import (
	"net/http"
	"sort"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerRulesRoute(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/rules", getRulesHandler(s))
}

type rulesResponse struct {
	Decisions []model.Decision `json:"decisions"`
}

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

		decisions, err := sortedRulesFor(&snap, tagID, txID, facet)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rulesResponse{Decisions: decisions})
	}
}

// sortedRulesFor wraps index.SelectRulesDecisions (the shared §3.8 rules
// selector, also used by `pmem rules`) with the viewer's chronological
// presentation order — a viewer-local formatting choice, not derived-view
// logic, so it stays here rather than in internal/index.
func sortedRulesFor(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	decisions, err := index.SelectRulesDecisions(snap, tagID, txID, facet)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(decisions, func(i, j int) bool { return decisions[i].At < decisions[j].At })
	return decisions, nil
}
