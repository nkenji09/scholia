package index

import "github.com/nkenji09/scholia/internal/model"

// TraceabilityEntry is one requirement-traceability row (§7): a tag of a
// traceabilityKind, the transitions that satisfy it, and whether it's a gap
// (0 satisfied — mirrors lint's requirement-gap rule, §5).
type TraceabilityEntry struct {
	Tag         model.Tag `json:"tag"`
	SatisfiedBy []string  `json:"satisfiedBy"`
	Gap         bool      `json:"gap"`
}

// Traceability lists, for each kind in kinds, every tag of that kind —
// exactly once each (§7: traceability is "which requirement is satisfied",
// one row per tag, not a tree nav) — together with the transitions whose
// effective tags (§3.7) satisfy it. Satisfaction reuses TransitionsByTag,
// which is built from the same EffectiveTags closure lint's requirement-gap
// rule checks (internal/lint/rules_advisory.go) — so a transition satisfies
// a requirement tag whether tagged directly, via a child tag (ancestor
// expansion), or via a tagged vocab entry it references.
//
// FacetTree can list a tag under more than one parent for multi-parent tags
// (§3.8 "多重所属可"); walking it would repeat such a tag once per parent
// path, which double-counts it in any summary over the result (gap counts,
// total counts, §review finding: "Traceability のサマリ件数が多親タグで水増し
// される"). Traceability dedupes by tag.ID, keeping the first occurrence —
// i.e. FacetTree's DFS order, so entries stay tree-ordered even though each
// tag now appears exactly once. SatisfiedBy/Gap don't depend on which parent
// path a tag was reached through (both come from the tag-id-keyed
// EffectiveTags closure), so keeping the first occurrence changes nothing
// about their values — only removes the duplicate rows.
func Traceability(ix *Index, kinds []string) []TraceabilityEntry {
	seen := make(map[string]bool)
	var out []TraceabilityEntry
	for _, kind := range kinds {
		for _, root := range ix.FacetTree(kind) {
			out = append(out, flattenTraceability(ix, root, seen)...)
		}
	}
	return out
}

// flattenTraceability walks node in DFS order, skipping (subtree included)
// any tag already in seen so a multi-parent tag contributes its entry and
// its own subtree only once, at its first-encountered path.
func flattenTraceability(ix *Index, node *TagNode, seen map[string]bool) []TraceabilityEntry {
	if seen[node.Tag.ID] {
		return nil
	}
	seen[node.Tag.ID] = true

	txs := ix.TransitionsByTag(node.Tag.ID)
	satisfiedBy := make([]string, 0, len(txs))
	for _, t := range txs {
		satisfiedBy = append(satisfiedBy, t.ID)
	}
	out := []TraceabilityEntry{{Tag: node.Tag, SatisfiedBy: satisfiedBy, Gap: len(satisfiedBy) == 0}}
	for _, c := range node.Children {
		out = append(out, flattenTraceability(ix, c, seen)...)
	}
	return out
}
