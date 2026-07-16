package index

import (
	"sort"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// FacetTreeNode is the tag-tree shape for sidebar navigation (§3.8 faceted
// hierarchy) — a value-typed, JSON-friendly mirror of TagNode carrying no
// transitions (those are fetched separately once a tag is selected). Shared
// by the viewer's GET /api/facets handler and `scholia export --html`'s static
// bake (§7) so both serialize the same derived tree.
type FacetTreeNode struct {
	Tag      model.Tag       `json:"tag"`
	Children []FacetTreeNode `json:"children,omitempty"`
}

// BuildFacetTreeNodes converts a FacetTree() forest into its JSON shape.
func BuildFacetTreeNodes(nodes []*TagNode) []FacetTreeNode {
	out := make([]FacetTreeNode, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, FacetTreeNode{Tag: n.Tag, Children: BuildFacetTreeNodes(n.Children)})
	}
	return out
}

// SortedRulesFor wraps SelectRulesDecisions with the chronological
// presentation order shared by `GET /api/rules`, a transition's detail
// panel (TransitionDetail.Rules), and the static export bake.
func SortedRulesFor(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	decisions, err := SelectRulesDecisions(snap, tagID, txID, facet)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(decisions, func(i, j int) bool { return decisions[i].At < decisions[j].At })
	return decisions, nil
}

// TransitionDetail mirrors `scholia show tx --resolve`'s output plus effective
// tags and the decisions on this transition itself — the detail panel's data
// (§7). Shared by GET /api/transitions/{id} and the static export bake.
type TransitionDetail struct {
	model.Transition
	ActionLabel string   `json:"actionLabel,omitempty"`
	GivenLabels []string `json:"givenLabels,omitempty"`
	ThenLabels  []string `json:"thenLabels,omitempty"`
	// EffectiveTags carries provenance (own/vocab/ancestor, §3.7) so the
	// viewer can show *why* a tag is effective instead of a flat id set
	// (gap G11) — computed once by EffectiveTagsWithProvenance, not
	// re-derived client-side (§9 single source of truth).
	EffectiveTags []EffectiveTag `json:"effectiveTags,omitempty"`
	// Rules は「この transition 自身に直接ぶら下がる decision」だけを載せる
	// （祖先タグの cross-cutting decision は含めない）。カードは「そのレコード
	// 自身の意思決定」を出す表示なので、cross-cutting 集約（§3.8・祖先展開）を
	// 提供する `scholia rules` / GET /api/rules（SelectRulesDecisions）とは
	// 意図的に別扱い。append-only の保存は不変で、これは表示の絞り込み。
	Rules []model.Decision `json:"rules,omitempty"`
}

// BuildTransitionDetail builds TransitionDetail for id. ok is false if no
// such transition exists.
func BuildTransitionDetail(snap *store.Snapshot, ix *Index, id string) (detail TransitionDetail, ok bool, err error) {
	t, ok := ix.TransitionByID[id]
	if !ok {
		return TransitionDetail{}, false, nil
	}

	label := func(vocabID string) string {
		if v, ok := ix.VocabByID[vocabID]; ok {
			return v.Label
		}
		return "?"
	}

	detail = TransitionDetail{Transition: t, ActionLabel: label(t.Action)}
	for _, g := range t.Given {
		detail.GivenLabels = append(detail.GivenLabels, label(g))
	}
	for _, e := range t.Then {
		detail.ThenLabels = append(detail.ThenLabels, label(e))
	}
	detail.EffectiveTags = EffectiveTagsWithProvenance(snap, &t)

	// この transition 自身の decision のみ（祖先タグの cross-cutting は除く）。
	own := filterDecisions(snap.Decisions, func(d model.Decision) bool {
		return d.Target.Type == model.DecisionTargetTransition && d.Target.ID == id
	})
	sort.SliceStable(own, func(i, j int) bool { return own[i].At < own[j].At })
	detail.Rules = own

	return detail, true, nil
}
