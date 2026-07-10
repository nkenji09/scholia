package index

import (
	"reflect"
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func traceabilitySnapshot() *store.Snapshot {
	return &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.submit", Category: model.CategoryAction, Label: "送信", Kind: "user", Tags: []string{"req.vocab-tagged"}},
			{ID: "eff.token", Category: model.CategoryEffect, Label: "トークン発行"},
		},
		Tags: []model.Tag{
			{ID: "req.auth", Name: "認証要件", Kind: "requirement"},
			{ID: "req.auth-happy", Name: "正常系", Kind: "requirement", ParentIDs: []string{"req.auth"}},
			{ID: "req.vocab-tagged", Name: "vocab 経由", Kind: "requirement"},
			{ID: "req.unmet", Name: "未充足要件", Kind: "requirement"},
		},
		Transitions: []model.Transition{
			// req.auth-happy に直接タグ。祖先展開で req.auth も充足する（子タグ経由の充足）。
			{ID: "T-child", Action: "act.submit", Then: []string{"eff.token"}, Tags: []string{"req.auth-happy"}},
			// タグなし。act.submit の vocab タグ（req.vocab-tagged）経由でのみ充足する（vocab 経由の充足）。
			{ID: "T-vocab", Action: "act.submit", Then: []string{"eff.token"}},
		},
	}
}

func TestTraceability_ChildTagSatisfiesAncestorRequirement(t *testing.T) {
	snap := traceabilitySnapshot()
	ix := Build(snap)

	entries := Traceability(ix, []string{"requirement"})
	byID := traceabilityByTagID(entries)

	entry, ok := byID["req.auth"]
	if !ok {
		t.Fatalf("req.auth entry missing: %+v", entries)
	}
	if entry.Gap {
		t.Fatalf("req.auth.Gap = true, want false (satisfied via child tag req.auth-happy on T-child)")
	}
	if !reflect.DeepEqual(entry.SatisfiedBy, []string{"T-child"}) {
		t.Fatalf("req.auth.SatisfiedBy = %v, want [T-child]", entry.SatisfiedBy)
	}
}

func TestTraceability_VocabTagSatisfiesRequirement(t *testing.T) {
	snap := traceabilitySnapshot()
	ix := Build(snap)

	entries := Traceability(ix, []string{"requirement"})
	byID := traceabilityByTagID(entries)

	entry, ok := byID["req.vocab-tagged"]
	if !ok {
		t.Fatalf("req.vocab-tagged entry missing: %+v", entries)
	}
	if entry.Gap {
		t.Fatalf("req.vocab-tagged.Gap = true, want false (satisfied via vocab tag on act.submit)")
	}
	want := []string{"T-child", "T-vocab"} // どちらも act.submit を参照するので両方 vocab 経由で充足
	if !reflect.DeepEqual(entry.SatisfiedBy, want) {
		t.Fatalf("req.vocab-tagged.SatisfiedBy = %v, want %v", entry.SatisfiedBy, want)
	}
}

func TestTraceability_ZeroSatisfiedIsGap(t *testing.T) {
	snap := traceabilitySnapshot()
	ix := Build(snap)

	entries := Traceability(ix, []string{"requirement"})
	byID := traceabilityByTagID(entries)

	entry, ok := byID["req.unmet"]
	if !ok {
		t.Fatalf("req.unmet entry missing: %+v", entries)
	}
	if !entry.Gap {
		t.Fatalf("req.unmet.Gap = false, want true (0 satisfying transitions)")
	}
	if len(entry.SatisfiedBy) != 0 {
		t.Fatalf("req.unmet.SatisfiedBy = %v, want empty", entry.SatisfiedBy)
	}
}

func TestTraceability_FiltersByRequestedKindsOnly(t *testing.T) {
	snap := traceabilitySnapshot()
	snap.Tags = append(snap.Tags, model.Tag{ID: "concern.perf", Name: "性能", Kind: "concern"})
	ix := Build(snap)

	entries := Traceability(ix, []string{"requirement"})
	for _, e := range entries {
		if e.Tag.Kind != "requirement" {
			t.Fatalf("entries contains non-requirement tag %s (kind=%s)", e.Tag.ID, e.Tag.Kind)
		}
	}
}

func traceabilityByTagID(entries []TraceabilityEntry) map[string]TraceabilityEntry {
	out := make(map[string]TraceabilityEntry, len(entries))
	for _, e := range entries {
		out[e.Tag.ID] = e
	}
	return out
}
