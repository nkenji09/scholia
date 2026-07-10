// Package index provides the derived query index (in-memory → SQLite; §3.9).
// Phase 1 seeds it with the pure read-time joins that other derived views need
// (effective tags, §3.7) so they have one shared, tested implementation.
package index

import (
	"sort"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

// EffectiveTags computes the effective tag set of a transition (§3.7):
//
//	effective(t) = 祖先展開( t.tags ∪ ⋃ tags(t が参照する vocab: action/given/then) )
//
// The result is deduplicated and sorted. Cyclic tag parentIds do not cause an
// infinite loop (visited set).
func EffectiveTags(snap *store.Snapshot, t *model.Transition) []string {
	vocabByID := make(map[string]model.VocabEntry, len(snap.Vocab))
	for _, v := range snap.Vocab {
		vocabByID[v.ID] = v
	}

	seeds := make(map[string]bool, len(t.Tags))
	for _, id := range t.Tags {
		seeds[id] = true
	}
	refs := make([]string, 0, 1+len(t.Given)+len(t.Then))
	refs = append(refs, t.Action)
	refs = append(refs, t.Given...)
	refs = append(refs, t.Then...)
	for _, ref := range refs {
		if v, ok := vocabByID[ref]; ok {
			for _, id := range v.Tags {
				seeds[id] = true
			}
		}
	}

	closure := ancestorClosure(tagIndex(snap.Tags), seeds)
	return sortedKeys(closure)
}

// TagAncestors returns tagID itself plus every ancestor reachable via
// Tag.ParentIDs (sorted, deduplicated, cycle-safe). Used by `pmem rules
// --tag` (§3.8): a decision on an ancestor tag also governs its descendants.
func TagAncestors(snap *store.Snapshot, tagID string) []string {
	closure := ancestorClosure(tagIndex(snap.Tags), map[string]bool{tagID: true})
	return sortedKeys(closure)
}

// ancestorClosure expands seeds along Tag.ParentIDs until fixpoint, tolerating
// cycles and dangling parent references (those are surfaced by lint's tag-ref
// rule, not here).
func ancestorClosure(tagByID map[string]model.Tag, seeds map[string]bool) map[string]bool {
	visited := make(map[string]bool, len(seeds))
	var expand func(id string)
	expand = func(id string) {
		if visited[id] {
			return
		}
		visited[id] = true
		tag, ok := tagByID[id]
		if !ok {
			return
		}
		for _, p := range tag.ParentIDs {
			expand(p)
		}
	}
	for id := range seeds {
		expand(id)
	}
	return visited
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
