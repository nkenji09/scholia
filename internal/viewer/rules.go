package viewer

import (
	"net/http"

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

		decisions, err := index.SortedRulesFor(&snap, tagID, txID, facet)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, rulesResponse{Decisions: decisions})
	}
}
