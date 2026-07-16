package diff

import (
	"reflect"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

func TestCompute_VocabAddedAndRemoved(t *testing.T) {
	before := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "a"}}}
	after := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.b", Category: "condition", Label: "b"}}}

	r := compute("HEAD", before, after)
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.b" {
		t.Fatalf("Vocab.Added = %+v, want [cond.b]", r.Vocab.Added)
	}
	if len(r.Vocab.Removed) != 1 || r.Vocab.Removed[0].ID != "cond.a" {
		t.Fatalf("Vocab.Removed = %+v, want [cond.a]", r.Vocab.Removed)
	}
	if len(r.Vocab.Changed) != 0 {
		t.Fatalf("Vocab.Changed = %+v, want none", r.Vocab.Changed)
	}
	if r.Empty() {
		t.Fatalf("Empty() = true, want false")
	}
}

func TestCompute_VocabChangedSameID(t *testing.T) {
	before := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "旧"}}}
	after := refSnapshot{Vocab: []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "新"}}}

	r := compute("HEAD", before, after)
	if len(r.Vocab.Changed) != 1 || r.Vocab.Changed[0].Before.Label != "旧" || r.Vocab.Changed[0].After.Label != "新" {
		t.Fatalf("Vocab.Changed = %+v", r.Vocab.Changed)
	}
}

func TestCompute_NoChangesIsEmpty(t *testing.T) {
	snap := refSnapshot{
		Vocab:       []model.VocabEntry{{ID: "cond.a", Category: "condition", Label: "a"}},
		Tags:        []model.Tag{{ID: "t.a", Name: "a"}},
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.a"}, Then: []string{"eff.a"}}},
		Decisions:   []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "why", At: "2026-01-01T00:00:00Z"}},
	}
	r := compute("HEAD", snap, snap)
	if !r.Empty() {
		t.Fatalf("Empty() = false, want true for identical snapshots: %+v", r)
	}
}

func TestCompute_ThenReorderIsDetectedAsChangeNotAddRemove(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.b"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.b", "eff.a"}}}}

	r := compute("HEAD", before, after)
	if len(r.Transitions.Added) != 0 || len(r.Transitions.Removed) != 0 {
		t.Fatalf("expected no add/remove for reorder, got added=%v removed=%v", r.Transitions.Added, r.Transitions.Removed)
	}
	if len(r.Transitions.Changed) != 1 {
		t.Fatalf("Transitions.Changed = %+v, want 1 entry", r.Transitions.Changed)
	}
	c := r.Transitions.Changed[0]
	if !c.ThenChanged || !c.ThenReordered {
		t.Fatalf("ThenChanged=%v ThenReordered=%v, want both true", c.ThenChanged, c.ThenReordered)
	}
}

func TestCompute_ThenElementChangeIsNotReordered(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.b"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a", "eff.c"}}}}

	r := compute("HEAD", before, after)
	c := r.Transitions.Changed[0]
	if !c.ThenChanged || c.ThenReordered {
		t.Fatalf("ThenChanged=%v ThenReordered=%v, want changed=true reordered=false", c.ThenChanged, c.ThenReordered)
	}
}

func TestCompute_GivenIsSetComparisonNotOrderSensitive(t *testing.T) {
	before := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.a", "cond.b"}, Then: []string{"eff.a"}}}}
	after := refSnapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Given: []string{"cond.b", "cond.a"}, Then: []string{"eff.a"}}}}

	r := compute("HEAD", before, after)
	if len(r.Transitions.Changed) != 0 {
		t.Fatalf("given reordering must not count as a change (given is a set, §3.2), got %+v", r.Transitions.Changed)
	}
}

func TestCompute_DecisionAddedIsNormalNotViolation(t *testing.T) {
	before := refSnapshot{}
	after := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"}}}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Added) != 1 {
		t.Fatalf("Decisions.Added = %+v, want 1", r.Decisions.Added)
	}
	if r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = true for a pure append, want false")
	}
}

func TestCompute_DecisionRemovedIsViolation(t *testing.T) {
	before := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"}}}
	after := refSnapshot{}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Removed) != 1 {
		t.Fatalf("Decisions.Removed = %+v, want 1", r.Decisions.Removed)
	}
	if !r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = false after a decision was removed, want true (append-only violation)")
	}
}

func TestCompute_DecisionModifiedIsViolation(t *testing.T) {
	before := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "旧", At: "2026-01-01T00:00:00Z"}}}
	after := refSnapshot{Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "transition", ID: "T-1"}, Why: "改変", At: "2026-01-01T00:00:00Z"}}}

	r := compute("HEAD", before, after)
	if len(r.Decisions.Changed) != 1 {
		t.Fatalf("Decisions.Changed = %+v, want 1", r.Decisions.Changed)
	}
	if !r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = false after a decision was modified, want true (append-only violation)")
	}
}

func TestSetDiff(t *testing.T) {
	added, removed := setDiff([]string{"a", "b"}, []string{"b", "c"})
	if !reflect.DeepEqual(added, []string{"c"}) {
		t.Fatalf("added = %v, want [c]", added)
	}
	if !reflect.DeepEqual(removed, []string{"a"}) {
		t.Fatalf("removed = %v, want [a]", removed)
	}
}
