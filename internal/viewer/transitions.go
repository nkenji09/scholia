package viewer

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func registerTransitionRoutes(mux *http.ServeMux, s *store.Store, c *indexCache) {
	mux.HandleFunc("GET /api/transitions", listTransitionsHandler(s, c))
	mux.HandleFunc("GET /api/transitions/{id}", getTransitionHandler(c))
	mux.HandleFunc("DELETE /api/transitions/{id}", deleteTransitionHandler(s))
}

type transitionsResponse struct {
	Transitions []model.Transition `json:"transitions,omitempty"`
	Facet       string             `json:"facet,omitempty"`
	Roots       []index.FacetNode  `json:"roots,omitempty"`
	Untagged    []model.Transition `json:"untagged,omitempty"`
	// Details は `?detail=1` のときだけ載る（decision
	// 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）。transition 1件につき
	// `/api/transitions/{id}` を1本ずつ投げていたのを、一覧と同じ1本に畳む。
	// 値は `GET /api/transitions/{id}` の応答と同じ形（静的書き出しの
	// `staticData.transitionDetail` とも同一・面間整合原則 D10b-2）。
	// omitempty なので、`detail` を渡さない既存の呼び手には JSON が1バイトも
	// 変わらない（後方互換）。
	Details map[string]index.TransitionDetail `json:"details,omitempty"`
}

// buildTransitionsResponse is shared by the live handler and the static
// export bake (§7 scholia export --html); callers are responsible for
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

func listTransitionsHandler(s *store.Store, c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}

		q := r.URL.Query()
		facet, tagID, kind := q.Get("facet"), q.Get("tag"), q.Get("kind")

		if facet != "" && !containsStr(snap.Config.TagKindIDs(), facet) {
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

		out := buildTransitionsResponse(ix, facet, tagID, kind)
		if q.Get("detail") != "" {
			details, err := index.AllTransitionDetails(&snap, ix)
			if err != nil {
				writeStoreError(w, err)
				return
			}
			out.Details = details
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getTransitionHandler(c *indexCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		snap, ix, err := c.load()
		if err != nil {
			writeStoreError(w, err)
			return
		}
		detail, ok, err := index.BuildTransitionDetail(&snap, ix, id)
		if err != nil {
			writeStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("transition %q が実在しません", id))
			return
		}
		writeJSON(w, http.StatusOK, detail)
	}
}

type deleteTransitionResponse struct {
	ID string `json:"id"`
}

// deleteTransitionHandler serves DELETE /api/transitions/{id}: the「削除
// (提案)」write (§8.8 P5・M-5「削除」・G-1′ 拡張). Removes only the
// transition's own working-tree file (store.RemoveTransitionUnlinked —
// uncommitted, never touches git), refusing with 409 if any decision still
// targets it (that would otherwise leave `scholia lint`'s decision-target rule
// — error severity — pointing at a dangling reference; see
// RemoveTransitionUnlinked's doc comment for why this doesn't cascade the
// way `scholia tx rm` does).
func deleteTransitionHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if !validTransitionID(id) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("id %q は不正です（'/' '\\' や '.'/'..' は使えません）", id))
			return
		}
		if !s.TransitionExists(id) {
			writeError(w, http.StatusNotFound, fmt.Sprintf("transition %q が実在しません", id))
			return
		}
		if err := s.RemoveTransitionUnlinked(id); err != nil {
			var refErr *store.TransitionReferencedError
			if errors.As(err, &refErr) {
				writeError(w, http.StatusConflict, refErr.Error())
				return
			}
			writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, deleteTransitionResponse{ID: id})
	}
}
