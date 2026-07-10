package viewer

import (
	"fmt"
	"net/http"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerTransitionRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/transitions", listTransitionsHandler(s))
	mux.HandleFunc("GET /api/transitions/{id}", getTransitionHandler(s))
}

type transitionsResponse struct {
	Transitions []model.Transition `json:"transitions,omitempty"`
	Facet       string             `json:"facet,omitempty"`
	Roots       []index.FacetNode  `json:"roots,omitempty"`
	Untagged    []model.Transition `json:"untagged,omitempty"`
}

// buildTransitionsResponse is shared by the live handler and the static
// export bake (§7 pmem export --html); callers are responsible for
// validating facet/tagID/kind beforehand (the live handler does this via
// HTTP 400s, the export bake only ever passes ids it already knows exist).
func buildTransitionsResponse(ix *index.Index, facet, tagID, kind string) transitionsResponse {
	filtered := index.FilterTransitions(ix, ix.AllTransitions(), tagID, kind)

	var out transitionsResponse
	if facet != "" {
		out.Facet = facet
		out.Roots = index.BuildFacetNodes(ix, facet, filtered)
		out.Untagged = index.UntaggedTransitions(ix, filtered, facet)
	} else {
		out.Transitions = filtered
	}
	return out
}

func listTransitionsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		q := r.URL.Query()
		facet, tagID, kind := q.Get("facet"), q.Get("tag"), q.Get("kind")

		if facet != "" && !containsStr(snap.Config.TagKinds, facet) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("facet %q は config.tagKinds に未宣言です", facet))
			return
		}
		if tagID != "" && !s.TagExists(tagID) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("tag %q が実在しません", tagID))
			return
		}
		if kind != "" && !containsStr(snap.Config.Kinds.Action, kind) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("kind %q は config.kinds.action に未宣言です", kind))
			return
		}

		writeJSON(w, http.StatusOK, buildTransitionsResponse(ix, facet, tagID, kind))
	}
}

func getTransitionHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		detail, ok, err := index.BuildTransitionDetail(&snap, ix, id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("transition %q が実在しません", id))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}
