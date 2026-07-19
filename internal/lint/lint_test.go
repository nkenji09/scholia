package lint

import (
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func baseConfig() model.Config {
	cfg := model.DefaultConfig()
	cfg.Kinds.Condition = []model.KindDecl{{ID: "foo"}}
	return cfg
}

func hasRule(findings []Finding, rule string) bool {
	for _, f := range findings {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestVocabRefGreenAndRed(t *testing.T) {
	green := store.Snapshot{
		Config: baseConfig(),
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a"},
			{ID: "cond.a", Category: model.CategoryCondition, Label: "a"},
			{ID: "eff.a", Category: model.CategoryEffect, Label: "a"},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.a"}, Then: []string{"eff.a"}},
		},
	}
	if got := checkVocabRef(green); len(got) != 0 {
		t.Fatalf("expected no vocab-ref findings, got %+v", got)
	}

	red := green
	red.Transitions = []model.Transition{
		{ID: "T-1", Action: "act.missing", Given: []string{"cond.a"}, Then: []string{"eff.a"}},
	}
	if got := checkVocabRef(red); !hasRule(got, "vocab-ref") {
		t.Fatalf("expected vocab-ref finding for dangling action, got %+v", got)
	}

	wrongCategory := green
	wrongCategory.Transitions = []model.Transition{
		{ID: "T-1", Action: "cond.a" /* condition, not action */, Given: nil, Then: []string{"eff.a"}},
	}
	if got := checkVocabRef(wrongCategory); !hasRule(got, "vocab-ref") {
		t.Fatalf("expected vocab-ref finding for wrong category, got %+v", got)
	}
}

func TestVocabRefEstablishesGreenAndRed(t *testing.T) {
	green := store.Snapshot{
		Config: baseConfig(),
		Vocab: []model.VocabEntry{
			{ID: "cond.scroll", Category: model.CategoryCondition, Label: "s"},
			{ID: "eff.save", Category: model.CategoryEffect, Label: "save", Establishes: []string{"cond.scroll"}},
		},
	}
	if got := checkVocabRef(green); len(got) != 0 {
		t.Fatalf("expected no vocab-ref findings for valid establishes, got %+v", got)
	}

	dangling := green
	dangling.Vocab = []model.VocabEntry{
		{ID: "eff.save", Category: model.CategoryEffect, Label: "save", Establishes: []string{"cond.missing"}},
	}
	if got := checkVocabRef(dangling); !hasRule(got, "vocab-ref") {
		t.Fatalf("expected vocab-ref finding for dangling establishes, got %+v", got)
	}

	notCondition := green
	notCondition.Vocab = []model.VocabEntry{
		{ID: "eff.a", Category: model.CategoryEffect, Label: "a"},
		{ID: "eff.save", Category: model.CategoryEffect, Label: "save", Establishes: []string{"eff.a"}},
	}
	if got := checkVocabRef(notCondition); !hasRule(got, "vocab-ref") {
		t.Fatalf("expected vocab-ref finding for establishes pointing at non-condition, got %+v", got)
	}

	onNonEffect := store.Snapshot{
		Config: baseConfig(),
		Vocab: []model.VocabEntry{
			{ID: "cond.scroll", Category: model.CategoryCondition, Label: "s"},
			{ID: "act.x", Category: model.CategoryAction, Label: "x", Establishes: []string{"cond.scroll"}},
		},
	}
	if got := checkVocabRef(onNonEffect); !hasRule(got, "vocab-ref") {
		t.Fatalf("expected vocab-ref finding for establishes on a non-effect vocab, got %+v", got)
	}
}

func TestRefFreshnessCoversVocabRef(t *testing.T) {
	snap := store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "eff.a", Category: model.CategoryEffect, Label: "a", Ref: "internal/foo.go:42"},
			{ID: "eff.b", Category: model.CategoryEffect, Label: "b", Ref: "DESIGN.md §3.4 scope-disclosure 契約"},
		},
	}
	got := checkRefFreshness(snap)
	if !hasRule(got, "ref-freshness") {
		t.Fatalf("expected ref-freshness finding for vocab file:line ref, got %+v", got)
	}
	for _, f := range got {
		if f.Target == "eff.b" {
			t.Fatalf("versioned § ref must not be flagged, got %+v", f)
		}
	}
}

func TestKindValidGreenAndRed(t *testing.T) {
	cfg := baseConfig()
	green := store.Snapshot{
		Config: cfg,
		Vocab:  []model.VocabEntry{{ID: "cond.a", Category: model.CategoryCondition, Label: "a", Kind: "foo"}},
		Tags:   []model.Tag{{ID: "t1", Name: "t1", Kind: "requirement"}},
	}
	if got := checkKindValid(green); len(got) != 0 {
		t.Fatalf("expected no kind-valid findings, got %+v", got)
	}

	red := store.Snapshot{
		Config: cfg,
		Vocab:  []model.VocabEntry{{ID: "cond.a", Category: model.CategoryCondition, Label: "a", Kind: "not-declared"}},
	}
	if got := checkKindValid(red); !hasRule(got, "kind-valid") {
		t.Fatalf("expected kind-valid finding for undeclared vocab kind, got %+v", got)
	}

	redTag := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "t1", Name: "t1", Kind: "not-declared"}},
	}
	if got := checkKindValid(redTag); !hasRule(got, "kind-valid") {
		t.Fatalf("expected kind-valid finding for undeclared tag kind, got %+v", got)
	}
}

func TestTagRefGreenAndRed(t *testing.T) {
	green := store.Snapshot{
		Tags: []model.Tag{
			{ID: "subject.parent", Name: "parent"},
			{ID: "subject.child", Name: "child", ParentIDs: []string{"subject.parent"}},
		},
		Vocab:       []model.VocabEntry{{ID: "act.a", Category: model.CategoryAction, Label: "a", Tags: []string{"subject.child"}}},
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"subject.parent"}}},
	}
	if got := checkTagRef(green); len(got) != 0 {
		t.Fatalf("expected no tag-ref findings, got %+v", got)
	}

	danglingParent := store.Snapshot{
		Tags: []model.Tag{{ID: "subject.child", Name: "child", ParentIDs: []string{"subject.missing"}}},
	}
	if got := checkTagRef(danglingParent); !hasRule(got, "tag-ref") {
		t.Fatalf("expected tag-ref finding for dangling parentIds, got %+v", got)
	}

	danglingTxTag := store.Snapshot{
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"subject.missing"}}},
	}
	if got := checkTagRef(danglingTxTag); !hasRule(got, "tag-ref") {
		t.Fatalf("expected tag-ref finding for dangling transition tag, got %+v", got)
	}

	cyclic := store.Snapshot{
		Tags: []model.Tag{
			{ID: "a", Name: "a", ParentIDs: []string{"b"}},
			{ID: "b", Name: "b", ParentIDs: []string{"a"}},
		},
	}
	if got := checkTagRef(cyclic); !hasRule(got, "tag-ref") {
		t.Fatalf("expected tag-ref finding for cyclic parentIds, got %+v", got)
	}
}

func TestCycleMembersSelfLoop(t *testing.T) {
	members := CycleMembers(map[string][]string{"a": {"a"}})
	if len(members) != 1 || members[0] != "a" {
		t.Fatalf("expected self-loop cycle for a, got %v", members)
	}
}

func TestDecisionTargetGreenAndRed(t *testing.T) {
	green := store.Snapshot{
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}}},
		Tags:        []model.Tag{{ID: "subject.x", Name: "x"}},
		Vocab:       []model.VocabEntry{{ID: "cond.a", Category: model.CategoryCondition, Label: "a"}},
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d3", Target: model.DecisionTarget{Type: model.DecisionTargetVocab, ID: "cond.a"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	if got := checkDecisionTarget(green); len(got) != 0 {
		t.Fatalf("expected no decision-target findings, got %+v", got)
	}

	red := store.Snapshot{
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-missing"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	if got := checkDecisionTarget(red); !hasRule(got, "decision-target") {
		t.Fatalf("expected decision-target finding for dangling transition target, got %+v", got)
	}

	redVocab := store.Snapshot{
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetVocab, ID: "cond.missing"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	if got := checkDecisionTarget(redVocab); !hasRule(got, "decision-target") {
		t.Fatalf("expected decision-target finding for dangling vocab target, got %+v", got)
	}
}

func TestEmptyThenGreenAndRed(t *testing.T) {
	green := store.Snapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}}}}
	if got := checkEmptyThen(green); len(got) != 0 {
		t.Fatalf("expected no empty-then findings, got %+v", got)
	}

	red := store.Snapshot{Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: nil}}}
	if got := checkEmptyThen(red); !hasRule(got, "empty-then") {
		t.Fatalf("expected empty-then finding, got %+v", got)
	}
}

func TestIDUniqueGreenAndRed(t *testing.T) {
	green := store.Snapshot{
		Vocab: []model.VocabEntry{{ID: "cond.a", Category: model.CategoryCondition, Label: "a"}},
	}
	if got := checkIDUnique(green); len(got) != 0 {
		t.Fatalf("expected no id-unique findings, got %+v", got)
	}

	dupIDs := store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "cond.a", Category: model.CategoryCondition, Label: "a"},
			{ID: "cond.a", Category: model.CategoryCondition, Label: "a-dup"},
		},
	}
	if got := checkIDUnique(dupIDs); !hasRule(got, "id-unique") {
		t.Fatalf("expected id-unique finding for duplicate id, got %+v", got)
	}

	mismatch := store.Snapshot{
		IDMismatches: []store.IDMismatch{{Category: "vocab", File: "cond.other.json", RecordID: "cond.a"}},
	}
	if got := checkIDUnique(mismatch); !hasRule(got, "id-unique") {
		t.Fatalf("expected id-unique finding for filename/id mismatch, got %+v", got)
	}
}

func TestRunAndHasError(t *testing.T) {
	green := store.Snapshot{
		Config: baseConfig(),
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a"},
			{ID: "eff.a", Category: model.CategoryEffect, Label: "a"},
		},
		Transitions: []model.Transition{{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}}},
	}
	if got := Run(green); HasError(got) {
		t.Fatalf("expected fully green snapshot, got findings: %+v", got)
	}

	red := store.Snapshot{
		Transitions: []model.Transition{{ID: "T-1", Action: "act.missing", Then: nil}},
	}
	got := Run(red)
	if !HasError(got) {
		t.Fatalf("expected errors for broken snapshot, got none")
	}
}
