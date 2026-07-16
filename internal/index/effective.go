// Package index provides the derived query index (in-memory → SQLite; §3.9).
// Phase 1 seeds it with the pure read-time joins that other derived views need
// (effective tags, §3.7) so they have one shared, tested implementation.
package index

import (
	"sort"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// TagSource names one of the three paths (§3.7) a tag can become effective
// through. A tag can arrive via more than one path at once (e.g. directly
// assigned AND also an ancestor of another effective tag) — EffectiveTag.
// Sources carries the full set, never just the "first" one, so provenance
// isn't silently collapsed to a single winner.
type TagSource string

const (
	SourceOwn      TagSource = "own"      // t.Tags が直接持つ
	SourceVocab    TagSource = "vocab"    // action/given/then が参照する vocab の tags
	SourceAncestor TagSource = "ancestor" // own/vocab のいずれかのタグの祖先（ParentIDs 展開）
)

// EffectiveTag is one tag in a transition's effective set, plus which of the
// three §3.7 paths produced it (possibly more than one).
type EffectiveTag struct {
	ID      string      `json:"id"`
	Sources []TagSource `json:"sources"`
}

// EffectiveTags computes the effective tag id set of a transition (§3.7):
//
//	effective(t) = 祖先展開( t.tags ∪ ⋃ tags(t が参照する vocab: action/given/then) )
//
// The result is deduplicated and sorted. This is a thin projection of
// EffectiveTagsWithProvenance's ID field (§9 single source of truth: one
// computation, provenance is not recomputed separately).
func EffectiveTags(snap *store.Snapshot, t *model.Transition) []string {
	withProvenance := EffectiveTagsWithProvenance(snap, t)
	out := make([]string, 0, len(withProvenance))
	for _, et := range withProvenance {
		out = append(out, et.ID)
	}
	return out
}

// EffectiveTagsWithProvenance computes the same effective tag set as
// EffectiveTags but keeps, per tag, which of own/vocab/ancestor path(s)
// produced it — the detail view's data for distinguishing "why is this tag
// effective" (gap G11). Deduplicated and sorted by ID. Cyclic tag parentIds
// do not cause an infinite loop (visited set, same guard as ancestorClosure).
func EffectiveTagsWithProvenance(snap *store.Snapshot, t *model.Transition) []EffectiveTag {
	vocabByID := make(map[string]model.VocabEntry, len(snap.Vocab))
	for _, v := range snap.Vocab {
		vocabByID[v.ID] = v
	}

	ownSeeds := make(map[string]bool, len(t.Tags))
	for _, id := range t.Tags {
		ownSeeds[id] = true
	}
	vocabSeeds := make(map[string]bool)
	refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
	refs = append(refs, t.Action)
	refs = append(refs, t.Given...)
	refs = append(refs, t.Then...)
	for _, ref := range refs {
		if v, ok := vocabByID[ref]; ok {
			for _, id := range v.Tags {
				vocabSeeds[id] = true
			}
		}
	}

	seeds := make(map[string]bool, len(ownSeeds)+len(vocabSeeds))
	for id := range ownSeeds {
		seeds[id] = true
	}
	for id := range vocabSeeds {
		seeds[id] = true
	}

	_, viaAncestor := ancestorClosureWithSource(tagIndex(snap.Tags), seeds)

	all := make(map[string]bool, len(seeds)+len(viaAncestor))
	for id := range seeds {
		all[id] = true
	}
	for id := range viaAncestor {
		all[id] = true
	}

	out := make([]EffectiveTag, 0, len(all))
	for id := range all {
		var sources []TagSource
		if ownSeeds[id] {
			sources = append(sources, SourceOwn)
		}
		if vocabSeeds[id] {
			sources = append(sources, SourceVocab)
		}
		if viaAncestor[id] {
			sources = append(sources, SourceAncestor)
		}
		out = append(out, EffectiveTag{ID: id, Sources: sources})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// TagAncestors returns tagID itself plus every ancestor reachable via
// Tag.ParentIDs (sorted, deduplicated, cycle-safe). Used by `scholia rules
// --tag` (§3.8): a decision on an ancestor tag also governs its descendants.
func TagAncestors(snap *store.Snapshot, tagID string) []string {
	closure := ancestorClosure(tagIndex(snap.Tags), map[string]bool{tagID: true})
	return sortedKeys(closure)
}

// ancestorClosure expands seeds along Tag.ParentIDs until fixpoint, tolerating
// cycles and dangling parent references (those are surfaced by lint's tag-ref
// rule, not here).
func ancestorClosure(tagByID map[string]model.Tag, seeds map[string]bool) map[string]bool {
	all, _ := ancestorClosureWithSource(tagByID, seeds)
	return all
}

// ancestorClosureWithSource is ancestorClosure plus viaAncestor: the subset
// of all reached strictly by walking a Tag.ParentIDs edge at least once. A
// seed can end up in viaAncestor too if some other seed's ancestor chain
// also reaches it — that's a real multi-path case (§3.7), not a bug, so
// callers must not assume seed and viaAncestor are disjoint.
func ancestorClosureWithSource(tagByID map[string]model.Tag, seeds map[string]bool) (all, viaAncestor map[string]bool) {
	all = make(map[string]bool, len(seeds))
	viaAncestor = make(map[string]bool)
	var expand func(id string)
	expand = func(id string) {
		if all[id] {
			return
		}
		all[id] = true
		tag, ok := tagByID[id]
		if !ok {
			return
		}
		for _, p := range tag.ParentIDs {
			viaAncestor[p] = true
			expand(p)
		}
	}
	for id := range seeds {
		expand(id)
	}
	return all, viaAncestor
}

func tagIndex(tags []model.Tag) map[string]model.Tag {
	m := make(map[string]model.Tag, len(tags))
	for _, t := range tags {
		m[t.ID] = t
	}
	return m
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
