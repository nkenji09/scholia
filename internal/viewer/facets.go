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

// facetTreeNode is the tag-tree shape for sidebar navigation (§3.8 faceted
// hierarchy). Transitions are fetched separately via /api/transitions once a
// tag is selected, so nodes here carry no transitions.
type facetTreeNode struct {
	Tag      model.Tag       `json:"tag"`
	Children []facetTreeNode `json:"children,omitempty"`
}

type facetsResponse struct {
	FacetKinds []string                   `json:"facetKinds"`
	Trees      map[string][]facetTreeNode `json:"trees"`
}

func getFacetsHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		trees := make(map[string][]facetTreeNode, len(snap.Config.FacetKinds))
		for _, kind := range snap.Config.FacetKinds {
			trees[kind] = buildFacetTreeNodes(ix.FacetTree(kind))
		}
		writeJSON(w, http.StatusOK, facetsResponse{FacetKinds: snap.Config.FacetKinds, Trees: trees})
	}
}

func buildFacetTreeNodes(nodes []*index.TagNode) []facetTreeNode {
	out := make([]facetTreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, facetTreeNode{Tag: n.Tag, Children: buildFacetTreeNodes(n.Children)})
	}
	return out
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
