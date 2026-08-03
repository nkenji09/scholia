package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

func TestRenameVocab_UpdatesTransitionReferencesAndFile(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.old", Category: model.CategoryAction, Label: "old"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "eff.a", Category: model.CategoryEffect, Label: "a"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.old", Given: []string{"act.old"}, Then: []string{"eff.a"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}

	result, err := s.RenameVocab("act.old", "act.new")
	if err != nil {
		t.Fatalf("RenameVocab: %v", err)
	}
	if result.OldID != "act.old" || result.NewID != "act.new" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.UpdatedTransitions) != 1 || result.UpdatedTransitions[0] != "T-1" {
		t.Fatalf("expected T-1 in UpdatedTransitions, got %v", result.UpdatedTransitions)
	}

	if s.VocabExists("act.old") {
		t.Fatalf("old vocab file should be removed")
	}
	if !s.VocabExists("act.new") {
		t.Fatalf("new vocab file should exist")
	}
	if _, err := os.Stat(filepath.Join(s.Dir, "vocab", "act.old.json")); !os.IsNotExist(err) {
		t.Fatalf("act.old.json should no longer exist on disk: %v", err)
	}

	tx, err := s.LoadTransition("T-1")
	if err != nil {
		t.Fatalf("LoadTransition: %v", err)
	}
	if tx.Action != "act.new" {
		t.Fatalf("expected action reference updated to act.new, got %q", tx.Action)
	}
	if len(tx.Given) != 1 || tx.Given[0] != "act.new" {
		t.Fatalf("expected given reference updated to act.new, got %v", tx.Given)
	}
}

func TestRenameVocab_FollowsDecisionTargetAndEstablishes(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// cond.old is the condition being renamed; it is a decision target and is
	// referenced by another effect's establishes[].
	if err := s.SaveVocab(model.VocabEntry{ID: "cond.old", Category: model.CategoryCondition, Label: "old"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "eff.mk", Category: model.CategoryEffect, Label: "mk", Establishes: []string{"cond.old"}}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.CreateDecision(model.Decision{
		ID: "01DV", Target: model.DecisionTarget{Type: model.DecisionTargetVocab, ID: "cond.old"},
		Why: "# テスト用の見出し\n\nw", At: "2026-01-01T00:00:00Z",
	}, DecisionCreateOptions{}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	// Unrelated tag decision must be left untouched.
	if err := s.CreateDecision(model.Decision{
		ID: "01DT", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why: "# テスト用の見出し\n\nw", At: "2026-01-01T00:00:00Z",
	}, DecisionCreateOptions{}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	result, err := s.RenameVocab("cond.old", "cond.new")
	if err != nil {
		t.Fatalf("RenameVocab: %v", err)
	}
	if len(result.UpdatedDecisions) != 1 || result.UpdatedDecisions[0] != "01DV" {
		t.Fatalf("expected 01DV in UpdatedDecisions, got %v", result.UpdatedDecisions)
	}
	if len(result.UpdatedVocab) != 1 || result.UpdatedVocab[0] != "eff.mk" {
		t.Fatalf("expected eff.mk in UpdatedVocab, got %v", result.UpdatedVocab)
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	byID := make(map[string]model.Decision, len(snap.Decisions))
	for _, d := range snap.Decisions {
		byID[d.ID] = d
	}
	if byID["01DV"].Target.ID != "cond.new" {
		t.Fatalf("expected decision 01DV target repointed to cond.new, got %+v", byID["01DV"])
	}
	if byID["01DT"].Target.ID != "subject.x" {
		t.Fatalf("expected unrelated tag decision 01DT untouched, got %+v", byID["01DT"])
	}
	eff, err := s.LoadVocab("eff.mk")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if len(eff.Establishes) != 1 || eff.Establishes[0] != "cond.new" {
		t.Fatalf("expected eff.mk.establishes repointed to cond.new, got %v", eff.Establishes)
	}
}

func TestRenameVocab_RejectsSameOrExistingID(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.a", Category: model.CategoryAction, Label: "a"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.b", Category: model.CategoryAction, Label: "b"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if _, err := s.RenameVocab("act.a", "act.a"); err == nil {
		t.Fatalf("expected error when newId == oldId")
	}
	if _, err := s.RenameVocab("act.a", "act.b"); err == nil {
		t.Fatalf("expected error when newId already exists")
	}
	if _, err := s.RenameVocab("act.missing", "act.c"); err == nil {
		t.Fatalf("expected error when oldId does not exist")
	}
}

func TestRenameTransition_UpdatesDecisionTargetsAndFile(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-old", Action: "act.a", Then: []string{"eff.a"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}
	if err := s.CreateDecision(model.Decision{
		ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-old"},
		Why: "# テスト用の見出し\n\nw", At: "2026-01-01T00:00:00Z",
	}, DecisionCreateOptions{}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	// Decision on an unrelated tag must be left untouched.
	if err := s.CreateDecision(model.Decision{
		ID: "01D2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why: "# テスト用の見出し\n\nw", At: "2026-01-01T00:00:00Z",
	}, DecisionCreateOptions{}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	result, err := s.RenameTransition("T-old", "T-new")
	if err != nil {
		t.Fatalf("RenameTransition: %v", err)
	}
	if len(result.UpdatedDecisions) != 1 || result.UpdatedDecisions[0] != "01D1" {
		t.Fatalf("expected 01D1 in UpdatedDecisions, got %v", result.UpdatedDecisions)
	}

	if s.TransitionExists("T-old") {
		t.Fatalf("old transition file should be removed")
	}
	if !s.TransitionExists("T-new") {
		t.Fatalf("new transition file should exist")
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	byID := make(map[string]model.Decision, len(snap.Decisions))
	for _, d := range snap.Decisions {
		byID[d.ID] = d
	}
	if byID["01D1"].Target.ID != "T-new" {
		t.Fatalf("expected decision 01D1 target updated to T-new, got %+v", byID["01D1"])
	}
	if byID["01D2"].Target.ID != "subject.x" {
		t.Fatalf("expected unrelated tag decision 01D2 untouched, got %+v", byID["01D2"])
	}
}
