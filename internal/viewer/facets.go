package viewer

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func registerFacetRoutes(mux *http.ServeMux, s *store.Store) {
	mux.HandleFunc("GET /api/facets", getFacetsHandler(s))
	mux.HandleFunc("GET /api/tags", getTagsHandler(s))
	mux.HandleFunc("GET /api/vocab", getVocabHandler(s))
}

type facetsResponse struct {
	FacetKinds []string                         `json:"facetKinds"`
	Trees      map[string][]index.FacetTreeNode `json:"trees"`
}

// buildFacetsResponse is shared by the live handler and the static export
// bake (§7 pmem export --html) so both serialize the same derived tree.
func buildFacetsResponse(snap store.Snapshot, ix *index.Index) facetsResponse {
	trees := make(map[string][]index.FacetTreeNode, len(snap.Config.FacetKinds))
	for _, kind := range snap.Config.FacetKinds {
		trees[kind] = index.BuildFacetTreeNodes(ix.FacetTree(kind))
	}
	return facetsResponse{FacetKinds: snap.Config.FacetKinds, Trees: trees}
}

func getFacetsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildFacetsResponse(snap, ix))
	}
}

func getTagsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, _, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		kind := r.URL.Query().Get("kind")
		if kind != "" && !containsStr(snap.Config.TagKinds, kind) {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("kind %q は config.tagKinds に未宣言です", kind))
			return
		}
		out := make([]model.Tag, 0, len(snap.Tags))
		for _, t := range snap.Tags {
			if kind == "" || t.Kind == kind {
				out = append(out, t)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		writeJSON(w, http.StatusOK, out)
	}
}

func getVocabHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, _, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		category := r.URL.Query().Get("category")
		if category != "" && category != "condition" && category != "action" && category != "effect" {
			writeError(w, http.StatusBadRequest,
				fmt.Sprintf("category %q は condition|action|effect のいずれかである必要があります", category))
			return
		}
		out := make([]model.VocabEntry, 0, len(snap.Vocab))
		for _, v := range snap.Vocab {
			if category == "" || v.Category == category {
				out = append(out, v)
			}
		}
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		writeJSON(w, http.StatusOK, out)
	}
}
