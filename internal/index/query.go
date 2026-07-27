package index

import (
	"fmt"
	"sort"

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
// target) — exactly one of tagID/txID/vocabID/facet may be set by the caller:
//   - txID: decisions on the transition itself, plus decisions on any tag in
//     its effective tag set (§3.7 ancestor expansion) — a parent tag's
//     cross-cutting rule also governs a child-tagged transition.
//   - tagID: decisions on the tag itself and its ancestors (a parent's rule
//     governs its descendants).
//   - vocabID: decisions on the vocab itself (#45 D5 vocab-target), plus
//     decisions on the tags that vocab directly carries (VocabEntry.Tags) and
//     those tags' ancestors — the same "own ∪ effective-tag closure" shape
//     `--tx` uses, applied to a vocab entry (#45 D10b-1 governs 並置・D10b-3
//     `rules --vocab`). Unknown vocab is an error.
//   - facet: decisions on every tag whose kind equals facet.
//   - none: all decisions.
//
// This is the single query core both `scholia rules` (all selectors) and the
// viewer's per-record governs (D10b-1) call, so CLI and viewer never diverge
// on "which decisions govern this record" (面間整合原則 D10b-2).
func SelectRulesDecisions(snap *store.Snapshot, tagID, txID, facet string) ([]model.Decision, error) {
	return SelectRulesDecisionsFor(snap, tagID, txID, "", facet)
}

// SelectRulesDecisionsFor is SelectRulesDecisions with the additional vocabID
// selector. SelectRulesDecisions delegates here with vocabID="" so existing
// callers (which never pass a vocab) keep their exact three-selector API.
func SelectRulesDecisionsFor(snap *store.Snapshot, tagID, txID, vocabID, facet string) ([]model.Decision, error) {
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

	case vocabID != "":
		v, ok := findVocabByID(snap.Vocab, vocabID)
		if !ok {
			return nil, fmt.Errorf("vocab %q が実在しません", vocabID)
		}
		// own（target が vocab:<id>）∪ その vocab が直接持つタグ（VocabEntry.Tags）
		// とその祖先タグ（TagAncestors）への decision。`--tx` の「自身＋実効タグ」と
		// 同型だが、vocab は「参照タグ＋祖先」（実効タグの祖先展開なし・vocab は
		// 遷移の実効タグ導出のシードにはなるが自身の実効タグは持たない）。
		govTags := make(map[string]bool)
		for _, tg := range v.Tags {
			for _, anc := range TagAncestors(snap, tg) {
				govTags[anc] = true
			}
		}
		return filterDecisions(snap.Decisions, func(d model.Decision) bool {
			if d.Target.Type == model.DecisionTargetVocab {
				return d.Target.ID == vocabID
			}
			return d.Target.Type == model.DecisionTargetTag && govTags[d.Target.ID]
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

func findVocabByID(vocab []model.VocabEntry, id string) (model.VocabEntry, bool) {
	for _, v := range vocab {
		if v.ID == id {
			return v, true
		}
	}
	return model.VocabEntry{}, false
}

// GovernsProvenance names how a governing decision reaches a record (#45
// D10b-1). "own" = the decision targets this exact record; "effective-tag" =
// it targets a tag the record carries directly (own tag / vocab tag);
// "parent" = it targets an ancestor tag reached only by walking ParentIDs.
type GovernsProvenance string

const (
	GovernsOwn          GovernsProvenance = "own"           // decision の target がこのレコード自身
	GovernsEffectiveTag GovernsProvenance = "effective-tag" // レコードが直接持つ実効タグへの decision
	GovernsViaParent    GovernsProvenance = "parent"        // 祖先タグ（ParentIDs 展開）への decision
)

// GovernsEntry is one governing decision plus its provenance and, when the
// provenance is a tag path, which tag id carried it — the per-record governs
// payload the viewer's TagCard/SpecCard/VocabCard render (#45 D10b-1). Shared
// by `GET /api/*` governs and the static export bake so both surfaces present
// the same set with the same provenance labels (面間整合原則 D10b-2).
type GovernsEntry struct {
	Decision   model.Decision    `json:"decision"`
	Provenance GovernsProvenance `json:"provenance"`
	// ViaTag is the tag id the decision was reached through (only set for
	// effective-tag / parent provenance; empty for own).
	ViaTag string `json:"viaTag,omitempty"`
}

// GovernsForTag returns the decisions governing a tag record — the tag's own
// decisions plus those on its ancestors — each tagged with provenance (#45
// D10b-1). A decision whose target IS this tag is "own"; strict ancestors
// reached via ParentIDs are "parent". Chronological.
//
// The tag's own decisions were previously reported as "effective-tag", which
// made a rule written directly ON this tag read as if it arrived through a tag
// — "own" never appeared on a tag card at all. That broke the invariant
// 01KXYED61J6QBEX75H2XHVHW7Y kept explicitly ("自身の判断がどれか曖昧にしない")
// and it now also decides what the inherited-rules disclosure counts
// (01KYHW4NBNVN9BFXYZMBX8MPF8 条項3 discloses *inherited* rules, i.e. the
// non-own ones), so the distinction has to be right at the source.
func GovernsForTag(snap *store.Snapshot, tagID string) ([]GovernsEntry, error) {
	if !tagExistsByID(snap.Tags, tagID) {
		return nil, fmt.Errorf("tag %q が実在しません", tagID)
	}
	ownDecisions := filterDecisions(snap.Decisions, func(d model.Decision) bool {
		return d.Target.Type == model.DecisionTargetTag && d.Target.ID == tagID
	})
	// The tag itself stays in directTags so its ancestors are walked from it;
	// governsFromTagSets dedupes by decision id keeping the strongest
	// provenance, so the own entries above win over the effective-tag ones.
	directTags := map[string]bool{tagID: true}
	return governsFromTagSets(snap, ownFromDecisions(ownDecisions), directTags), nil
}

// GovernsForTransition returns the decisions governing a transition — its own
// decisions ("own") plus decisions on its effective tags, split into
// effective-tag (a directly-carried tag: own/vocab source) vs parent (reached
// only by ancestor expansion) provenance (#45 D10b-1). Chronological.
func GovernsForTransition(snap *store.Snapshot, txID string) ([]GovernsEntry, error) {
	tx, ok := findTransitionByID(snap.Transitions, txID)
	if !ok {
		return nil, fmt.Errorf("transition %q が実在しません", txID)
	}
	directTags := make(map[string]bool)
	for _, et := range EffectiveTagsWithProvenance(snap, &tx) {
		for _, src := range et.Sources {
			if src == SourceOwn || src == SourceVocab {
				directTags[et.ID] = true
			}
		}
	}
	ownDecisions := filterDecisions(snap.Decisions, func(d model.Decision) bool {
		return d.Target.Type == model.DecisionTargetTransition && d.Target.ID == txID
	})
	entries := governsFromTagSets(snap, ownFromDecisions(ownDecisions), directTags)
	return entries, nil
}

// GovernsForVocab returns the decisions governing a vocab entry — its own
// vocab-target decisions ("own") plus decisions on the tags it directly
// carries and their ancestors (#45 D10b-1). Directly-carried tags are
// "effective-tag"; ancestors reached only via ParentIDs are "parent".
// Chronological.
func GovernsForVocab(snap *store.Snapshot, vocabID string) ([]GovernsEntry, error) {
	v, ok := findVocabByID(snap.Vocab, vocabID)
	if !ok {
		return nil, fmt.Errorf("vocab %q が実在しません", vocabID)
	}
	directTags := make(map[string]bool, len(v.Tags))
	for _, tg := range v.Tags {
		directTags[tg] = true
	}
	ownDecisions := filterDecisions(snap.Decisions, func(d model.Decision) bool {
		return d.Target.Type == model.DecisionTargetVocab && d.Target.ID == vocabID
	})
	entries := governsFromTagSets(snap, ownFromDecisions(ownDecisions), directTags)
	return entries, nil
}

// ownFromDecisions wraps a record's own decisions as GovernsOwn entries.
func ownFromDecisions(decisions []model.Decision) []GovernsEntry {
	out := make([]GovernsEntry, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, GovernsEntry{Decision: d, Provenance: GovernsOwn})
	}
	return out
}

// governsFromTagSets assembles the tag-path governs entries: for every tag in
// directTags plus its ancestors, any tag-target decision on it is attributed
// effective-tag (if the tag is direct) or parent (ancestor-only). Prepends the
// supplied own entries, dedupes by decision id (a decision reachable by several
// tag paths appears once, at its strongest — own > effective-tag > parent),
// and sorts the whole list chronologically.
func governsFromTagSets(snap *store.Snapshot, own []GovernsEntry, directTags map[string]bool) []GovernsEntry {
	// Build the full governing-tag closure and remember, per tag, whether it is
	// direct or only an ancestor, and the nearest direct tag it descends from.
	type tagInfo struct {
		direct bool
		via    string // for ancestor tags: a direct tag that reaches it
	}
	closure := make(map[string]tagInfo)
	for tg := range directTags {
		for _, anc := range TagAncestors(snap, tg) {
			info := closure[anc]
			if anc == tg {
				info.direct = true
			} else if info.via == "" {
				info.via = tg
			}
			closure[anc] = info
		}
	}

	// strength ranks provenance so a decision reached by several paths keeps
	// the most direct one.
	strength := map[GovernsProvenance]int{GovernsOwn: 3, GovernsEffectiveTag: 2, GovernsViaParent: 1}

	byID := make(map[string]GovernsEntry)
	order := make([]string, 0)
	upsert := func(e GovernsEntry) {
		if prev, ok := byID[e.Decision.ID]; ok {
			if strength[e.Provenance] > strength[prev.Provenance] {
				byID[e.Decision.ID] = e
			}
			return
		}
		byID[e.Decision.ID] = e
		order = append(order, e.Decision.ID)
	}
	for _, e := range own {
		upsert(e)
	}
	for _, d := range snap.Decisions {
		if d.Target.Type != model.DecisionTargetTag {
			continue
		}
		info, ok := closure[d.Target.ID]
		if !ok {
			continue
		}
		if info.direct {
			upsert(GovernsEntry{Decision: d, Provenance: GovernsEffectiveTag, ViaTag: d.Target.ID})
		} else {
			upsert(GovernsEntry{Decision: d, Provenance: GovernsViaParent, ViaTag: d.Target.ID})
		}
	}

	out := make([]GovernsEntry, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Decision.At < out[j].Decision.At })
	return out
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
