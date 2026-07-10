package index

import (
	"reflect"
	"testing"
	"time"

	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
)

func TestEffectiveTags_UnionOfOwnAndVocabTags(t *testing.T) {
	snap := &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a", Tags: []string{"subject.a"}},
			{ID: "cond.a", Category: model.CategoryCondition, Label: "a", Tags: []string{"concern.security"}},
			{ID: "eff.a", Category: model.CategoryEffect, Label: "a"},
		},
	}
	tx := &model.Transition{ID: "T-1", Action: "act.a", Given: []string{"cond.a"}, Then: []string{"eff.a"}, Tags: []string{"req.happy"}}

	got := EffectiveTags(snap, tx)
	want := []string{"concern.security", "req.happy", "subject.a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTags = %v, want %v", got, want)
	}
}

func TestEffectiveTags_AncestorExpansion(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "req.auth", Name: "auth"},
			{ID: "req.auth-happy-path", Name: "happy", ParentIDs: []string{"req.auth"}},
		},
	}
	tx := &model.Transition{ID: "T-1", Tags: []string{"req.auth-happy-path"}}

	got := EffectiveTags(snap, tx)
	want := []string{"req.auth", "req.auth-happy-path"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTags = %v, want %v", got, want)
	}
}

func TestEffectiveTags_DedupesAcrossOwnAndVocabPaths(t *testing.T) {
	snap := &store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a", Tags: []string{"subject.auth"}},
		},
	}
	tx := &model.Transition{ID: "T-1", Action: "act.a", Tags: []string{"subject.auth"}}

	got := EffectiveTags(snap, tx)
	want := []string{"subject.auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EffectiveTags = %v, want %v (expected dedup)", got, want)
	}
}

func TestEffectiveTags_ToleratesCyclicParentIDs(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "a", Name: "a", ParentIDs: []string{"b"}},
			{ID: "b", Name: "b", ParentIDs: []string{"a"}},
		},
	}
	tx := &model.Transition{ID: "T-1", Tags: []string{"a"}}

	done := make(chan []string, 1)
	go func() { done <- EffectiveTags(snap, tx) }()

	select {
	case got := <-done:
		want := []string{"a", "b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("EffectiveTags = %v, want %v", got, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("EffectiveTags did not terminate on cyclic parentIds (expected visited-set guard)")
	}
}

func TestTagAncestors_SelfPlusAncestors(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "subject.auth", Name: "auth"},
			{ID: "req.auth", Name: "req-auth", ParentIDs: []string{"subject.auth"}},
			{ID: "req.auth-happy-path", Name: "happy", ParentIDs: []string{"req.auth"}},
		},
	}
	got := TagAncestors(snap, "req.auth-happy-path")
	want := []string{"req.auth", "req.auth-happy-path", "subject.auth"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TagAncestors = %v, want %v", got, want)
	}
}

func TestTagAncestors_ToleratesCycles(t *testing.T) {
	snap := &store.Snapshot{
		Tags: []model.Tag{
			{ID: "a", Name: "a", ParentIDs: []string{"b"}},
			{ID: "b", Name: "b", ParentIDs: []string{"a"}},
		},
	}
	got := TagAncestors(snap, "a")
	want := []string{"a", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TagAncestors = %v, want %v", got, want)
	}
}

func TestEffectiveTags_EmptyWhenNoTagsAnywhere(t *testing.T) {
	snap := &store.Snapshot{}
	tx := &model.Transition{ID: "T-1", Action: "act.a"}
	got := EffectiveTags(snap, tx)
	if len(got) != 0 {
		t.Fatalf("expected no effective tags, got %v", got)
	}
}
