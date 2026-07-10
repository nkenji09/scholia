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

		filtered := index.FilterTransitions(ix, ix.AllTransitions(), tagID, kind)

		var out transitionsResponse
		if facet != "" {
			out.Facet = facet
			out.Roots = index.BuildFacetNodes(ix, facet, filtered)
			out.Untagged = index.UntaggedTransitions(ix, filtered, facet)
		} else {
			out.Transitions = filtered
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// txDetail mirrors internal/cli/show_tx.go's --resolve output, plus
// effective tags and the decisions that apply to this transition (rules),
// for the detail panel (§7).
type txDetail struct {
	model.Transition
	ActionLabel   string           `json:"actionLabel,omitempty"`
	GivenLabels   []string         `json:"givenLabels,omitempty"`
	ThenLabels    []string         `json:"thenLabels,omitempty"`
	EffectiveTags []string         `json:"effectiveTags,omitempty"`
	Rules         []model.Decision `json:"rules,omitempty"`
}

func getTransitionHandler(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		snap, ix, err := loadIndexed(s)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		t, ok := ix.TransitionByID[id]
		if !ok {
			writeError(w, http.StatusNotFound, fmt.Sprintf("transition %q が実在しません", id))
			return
		}

		label := func(vocabID string) string {
			if v, ok := ix.VocabByID[vocabID]; ok {
				return v.Label
			}
			return "?"
		}
		detail := txDetail{
			Transition:  t,
			ActionLabel: label(t.Action),
		}
		for _, g := range t.Given {
			detail.GivenLabels = append(detail.GivenLabels, label(g))
		}
		for _, e := range t.Then {
			detail.ThenLabels = append(detail.ThenLabels, label(e))
		}
		detail.EffectiveTags = ix.EffectiveTags[id]

		rules, err := sortedRulesFor(&snap, "", id, "")
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		detail.Rules = rules

		writeJSON(w, http.StatusOK, detail)
	}
}
