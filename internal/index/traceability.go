package index

import "github.com/nkenji09/product-memory/internal/model"

// TraceabilityEntry is one requirement-traceability row (§7): a tag of a
// traceabilityKind, the transitions that satisfy it, and whether it's a gap
// (0 satisfied — mirrors lint's requirement-gap rule, §5).
type TraceabilityEntry struct {
	Tag         model.Tag `json:"tag"`
	SatisfiedBy []string  `json:"satisfiedBy"`
	Gap         bool      `json:"gap"`
}

// Traceability lists, for each kind in kinds, every tag of that kind in
// facet-tree order (§3.8) together with the transitions whose effective tags
// (§3.7) satisfy it. Satisfaction reuses TransitionsByTag, which is built
// from the same EffectiveTags closure lint's requirement-gap rule checks
// (internal/lint/rules_advisory.go) — so a transition satisfies a
// requirement tag whether tagged directly, via a child tag (ancestor
// expansion), or via a tagged vocab entry it references.
//
// FacetTree can list a tag under more than one parent for multi-parent tags
// (§3.8 "多重所属可"); Traceability inherits that and may repeat such a tag
// as multiple entries, one per parent path — an accepted consequence of
// reusing FacetTree's ordering rather than a reason to hide the duplicate.
func Traceability(ix *Index, kinds []string) []TraceabilityEntry {
	var out []TraceabilityEntry
	for _, kind := range kinds {
		for _, root := range ix.FacetTree(kind) {
			out = append(out, flattenTraceability(ix, root)...)
		}
	}
	return out
}

func flattenTraceability(ix *Index, node *TagNode) []TraceabilityEntry {
	txs := ix.TransitionsByTag(node.Tag.ID)
	satisfiedBy := make([]string, 0, len(txs))
	for _, t := range txs {
		satisfiedBy = append(satisfiedBy, t.ID)
	}
	out := []TraceabilityEntry{{Tag: node.Tag, SatisfiedBy: satisfiedBy, Gap: len(satisfiedBy) == 0}}
	for _, c := range node.Children {
		out = append(out, flattenTraceability(ix, c)...)
	}
	return out
}
