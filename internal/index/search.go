package index

import (
	"strings"

	"github.com/nkenji09/product-memory/internal/model"
)

// SearchResult is the outcome of Search: every matching transition plus,
// for each, which fields matched (for UI highlighting / debugging).
type SearchResult struct {
	Transitions []model.Transition  `json:"transitions"`
	MatchedOn   map[string][]string `json:"matchedOn"`
}

// Search finds transitions whose (a) effective tag id/name (§3.7), (b)
// action/given/then vocab id/label, (c) own id, or (d) action's kind name
// contains query (case-insensitive substring match). An empty/blank query
// matches nothing (the UI is expected to skip the call for an empty box;
// this guards direct API callers the same way). Results and matchedOn are
// always non-nil so callers see `[]`/`{}`, never `null`, on no hits.
func Search(ix *Index, query string) SearchResult {
	result := SearchResult{Transitions: []model.Transition{}, MatchedOn: map[string][]string{}}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return result
	}

	for _, t := range ix.AllTransitions() {
		var fields []string

		if strings.Contains(strings.ToLower(t.ID), q) {
			fields = append(fields, "id")
		}
		for _, tagID := range ix.EffectiveTags[t.ID] {
			tag := ix.TagByID[tagID]
			if strings.Contains(strings.ToLower(tag.ID), q) || strings.Contains(strings.ToLower(tag.Name), q) {
				fields = append(fields, "tag:"+tag.ID)
			}
		}
		for _, ref := range vocabRefs(t) {
			v := ix.VocabByID[ref]
			if strings.Contains(strings.ToLower(v.ID), q) || strings.Contains(strings.ToLower(v.Label), q) {
				fields = append(fields, "vocab:"+v.ID)
			}
		}
		if kind := ix.VocabByID[t.Action].Kind; kind != "" && strings.Contains(strings.ToLower(kind), q) {
			fields = append(fields, "kind:"+kind)
		}

		if len(fields) > 0 {
			result.Transitions = append(result.Transitions, t)
			result.MatchedOn[t.ID] = fields
		}
	}
	return result
}

// vocabRefs lists the vocab ids a transition references (action/given/then),
// mirroring the refs slice Build assembles for vocabTransitions.
func vocabRefs(t model.Transition) []string {
	refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
	refs = append(refs, t.Action)
	refs = append(refs, t.Given...)
	refs = append(refs, t.Then...)
	return refs
}
