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

// facetNode mirrors internal/cli/list.go's --facet output shape (that type
// is unexported in package cli, so it's reproduced here) to keep the
// grouping semantics identical to `pmem list --facet` (§7).
type facetNode struct {
	Tag         model.Tag          `json:"tag"`
	Transitions []model.Transition `json:"transitions,omitempty"`
	Children    []facetNode        `json:"children,omitempty"`
}

type transitionsResponse struct {
	Transitions []model.Transition `json:"transitions,omitempty"`
	Facet       string             `json:"facet,omitempty"`
	Roots       []facetNode        `json:"roots,omitempty"`
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

		filtered := filterTransitions(ix, ix.AllTransitions(), tagID, kind)

		var out transitionsResponse
		if facet != "" {
			out.Facet = facet
			inSet := toTxSet(filtered)
			for _, root := range ix.FacetTree(facet) {
				out.Roots = append(out.Roots, buildFacetNode(ix, root, inSet))
			}
			out.Untagged = untaggedTransitions(ix, filtered, facet)
		} else {
			out.Transitions = filtered
		}
		writeJSON(w, http.StatusOK, out)
	}
}

// filterTransitions applies --tag/--kind as an AND filter, matching
// internal/cli/list.go's filterTransitions (kind == the action's kind).
func filterTransitions(ix *index.Index, all []model.Transition, tagID, kind string) []model.Transition {
	out := make([]model.Transition, 0, len(all))
	for _, t := range all {
		if tagID != "" && !ix.HasEffectiveTag(t.ID, tagID) {
			continue
		}
		if kind != "" && ix.VocabByID[t.Action].Kind != kind {
			continue
		}
		out = append(out, t)
	}
	return out
}

func toTxSet(ts []model.Transition) map[string]bool {
	set := make(map[string]bool, len(ts))
	for _, t := range ts {
		set[t.ID] = true
	}
	return set
}

func buildFacetNode(ix *index.Index, node *index.TagNode, inSet map[string]bool) facetNode {
	fn := facetNode{Tag: node.Tag}
	for _, t := range ix.TransitionsByTag(node.Tag.ID) {
		if inSet[t.ID] {
			fn.Transitions = append(fn.Transitions, t)
		}
	}
	for _, c := range node.Children {
		fn.Children = append(fn.Children, buildFacetNode(ix, c, inSet))
	}
	return fn
}

func untaggedTransitions(ix *index.Index, filtered []model.Transition, facet string) []model.Transition {
	var out []model.Transition
	for _, t := range filtered {
		hasFacetTag := false
		for _, tagID := range ix.EffectiveTags[t.ID] {
			if ix.TagByID[tagID].Kind == facet {
				hasFacetTag = true
				break
			}
		}
		if !hasFacetTag {
			out = append(out, t)
		}
	}
	return out
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
		detail.Rules = rulesForTransition(&snap, id, detail.EffectiveTags)

		writeJSON(w, http.StatusOK, detail)
	}
}
