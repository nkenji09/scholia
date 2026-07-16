package index

import (
	"strings"

	"github.com/nkenji09/scholia/internal/model"
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
		seen := make(map[string]bool)
		for _, c := range searchCandidates(ix, t) {
			if seen[c.Label] || !strings.Contains(c.Text, q) {
				continue
			}
			seen[c.Label] = true
			fields = append(fields, c.Label)
		}

		if len(fields) > 0 {
			result.Transitions = append(result.Transitions, t)
			result.MatchedOn[t.ID] = fields
		}
	}
	return result
}

// SearchCandidate is one substring-matchable (label, lowercased text) pair
// Search() tests query against for a given transition.
type SearchCandidate struct {
	Label string `json:"label"`
	Text  string `json:"text"`
}

// TransitionSearchDoc is a transition's full search corpus — every candidate
// Search() would test a query against, precomputed. A static export bakes
// this once (§7 scholia export --html) so a JS client can re-run the same
// per-candidate substring test per query without re-deriving effective tags
// or vocab labels — the derivation (this function) stays the single source
// of truth; only the trivial substring test itself is re-run client-side.
type TransitionSearchDoc struct {
	TransitionID string            `json:"transitionId"`
	Candidates   []SearchCandidate `json:"candidates"`
}

// SearchCorpus returns every transition's search candidates, unfiltered, in
// the same order Search() iterates them (id-ascending via AllTransitions).
func SearchCorpus(ix *Index) []TransitionSearchDoc {
	all := ix.AllTransitions()
	docs := make([]TransitionSearchDoc, 0, len(all))
	for _, t := range all {
		docs = append(docs, TransitionSearchDoc{TransitionID: t.ID, Candidates: searchCandidates(ix, t)})
	}
	return docs
}

// searchCandidates builds transition t's full candidate list (own id;
// effective tag id/name; action/given/then vocab id/label; action's kind
// name) — the shared basis for both live Search() and SearchCorpus().
func searchCandidates(ix *Index, t model.Transition) []SearchCandidate {
	var out []SearchCandidate
	out = append(out, SearchCandidate{Label: "id", Text: strings.ToLower(t.ID)})
	for _, tagID := range ix.EffectiveTags[t.ID] {
		tag := ix.TagByID[tagID]
		out = append(out,
			SearchCandidate{Label: "tag:" + tag.ID, Text: strings.ToLower(tag.ID)},
			SearchCandidate{Label: "tag:" + tag.ID, Text: strings.ToLower(tag.Name)},
		)
	}
	for _, ref := range vocabRefs(t) {
		v := ix.VocabByID[ref]
		out = append(out,
			SearchCandidate{Label: "vocab:" + v.ID, Text: strings.ToLower(v.ID)},
			SearchCandidate{Label: "vocab:" + v.ID, Text: strings.ToLower(v.Label)},
		)
	}
	if kind := ix.VocabByID[t.Action].Kind; kind != "" {
		out = append(out, SearchCandidate{Label: "kind:" + kind, Text: strings.ToLower(kind)})
	}
	return out
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
