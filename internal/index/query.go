package index

import (
	"fmt"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// FacetNode is a facet-tree node with the transitions that land on it
// (§3.8 faceted hierarchy: a facet axis's tag nesting becomes a tree with
// transitions at the leaves). CLI (`scholia list --facet`) and the viewer
// (`GET /api/transitions?facet=`) share this exact shape — including its
// JSON field names — so both surfaces present the same derived view.
type FacetNode struct {
	Tag         model.Tag          `json:"tag"`
	Transitions []model.Transition `json:"transitions,omitempty"`
	Children    []FacetNode        `json:"children,omitempty"`
}

// FilterTransitions applies --tag/--kind as an AND filter: tagID (if set)
// must be in the transition's effective tags (§3.7 ancestor expansion);
// kind (if set) is matched against the transition's action's vocab kind.
func FilterTransitions(ix *Index, all []model.Transition, tagID, kind string) []model.Transition {
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

// BuildFacetNodes builds the facet tree for the given kind (§3.8), attaching
// to each node only the transitions present in filtered (so callers can
// combine facet grouping with a --tag/--kind pre-filter).
func BuildFacetNodes(ix *Index, facet string, filtered []model.Transition) []FacetNode {
	inSet := make(map[string]bool, len(filtered))
	for _, t := range filtered {
		inSet[t.ID] = true
	}

	var build func(node *TagNode) FacetNode
	build = func(node *TagNode) FacetNode {
		fn := FacetNode{Tag: node.Tag}
		for _, t := range ix.TransitionsByTag(node.Tag.ID) {
			if inSet[t.ID] {
				fn.Transitions = append(fn.Transitions, t)
			}
		}
		for _, c := range node.Children {
			fn.Children = append(fn.Children, build(c))
		}
		return fn
	}

	roots := ix.FacetTree(facet)
	out := make([]FacetNode, 0, len(roots))
	for _, root := range roots {
		out = append(out, build(root))
	}
	return out
}

// UntaggedTransitions returns the subset of filtered with no effective tag
// of the given facet kind — the trailing "untagged" group in faceted views
// (§3.8).
func UntaggedTransitions(ix *Index, filtered []model.Transition, facet string) []model.Transition {
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

// SelectRulesDecisions implements the `scholia rules` / `GET /api/rules`
// selector semantics (§3.8 rules: cross-cutting decisions aggregated by
// target) — exactly one of tagID/txID/facet may be set by the caller:
//   - txID: decisions on the transition itself, plus decisions on any tag in
//     its effective tag set (§3.7 ancestor expansion) — a parent tag's
//     cross-cutting rule also governs a child-tagged transition.
//   - tagID: decisions on the tag itself and its ancestors (a parent's rule
//     governs its descendants).
//   - facet: decisions on every tag whose kind equals facet.
//   - none: all decisions.
func SelectRulesDecisions(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	switch {
	case txID != "":
		tx, ok := findTransitionByID(snap.Transitions, txID)
		if !ok {
			return nil, fmt.Errorf("transition %q が実在しません", txID)
		}
		targetTags := make(map[string]bool)
		for _, id := range EffectiveTags(snap, &tx) {
			targetTags[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			if d.Target.Type == model.DecisionTargetTransition {
				return d.Target.ID == txID
			}
			return d.Target.Type == model.DecisionTargetTag && targetTags[d.Target.ID]
		}), nil

	case tagID != "":
		if !tagExistsByID(snap.Tags, tagID) {
			return nil, fmt.Errorf("tag %q が実在しません", tagID)
		}
		ancestors := make(map[string]bool)
		for _, id := range TagAncestors(snap, tagID) {
			ancestors[id] = true
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && ancestors[d.Target.ID]
		}), nil

	case facet != "":
		facetTags := make(map[string]bool)
		for _, t := range snap.Tags {
			if t.Kind == facet {
				facetTags[t.ID] = true
			}
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			return d.Target.Type == model.DecisionTargetTag && facetTags[d.Target.ID]
		}), nil

	default:
		return append([]model.Decision{}, snap.Decisions...), nil
	}
}

func findTransitionByID(transitions []model.Transition, id string) (model.Transition, bool) {
	for _, t := range transitions {
		if t.ID == id {
			return t, true
		}
	}
	return model.Transition{}, false
}

func tagExistsByID(tags []model.Tag, id string) bool {
	for _, t := range tags {
		if t.ID == id {
			return true
		}
	}
	return false
}

func filterDecisions(decisions []model.Decision, keep func(model.Decision) bool) []model.Decision {
	out := make([]model.Decision, 0, len(decisions))
	for _, d := range decisions {
		if keep(d) {
			out = append(out, d)
		}
	}
	return out
}
