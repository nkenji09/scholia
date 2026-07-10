package store

import (
	"testing"

	"github.com/nkenji09/product-memory/internal/model"
)

func TestRemoveVocab_RejectsReferencedThenDeletesUnreferenced(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.a", Category: model.CategoryAction, Label: "a"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "eff.a", Category: model.CategoryEffect, Label: "a"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}

	if _, err := s.RemoveVocab("act.a"); err == nil {
		t.Fatalf("expected error removing a referenced vocab")
	}
	if !s.VocabExists("act.a") {
		t.Fatalf("referenced vocab should not have been deleted")
	}

	if _, err := s.RemoveVocab("eff.a"); err == nil {
		t.Fatalf("expected error removing a referenced vocab (then slot)")
	}

	if err := s.SaveVocab(model.VocabEntry{ID: "cond.unused", Category: model.CategoryCondition, Label: "u"}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	result, err := s.RemoveVocab("cond.unused")
	if err != nil {
		t.Fatalf("RemoveVocab unreferenced: %v", err)
	}
	if result.ID != "cond.unused" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if s.VocabExists("cond.unused") {
		t.Fatalf("unreferenced vocab should have been deleted")
	}
}

func TestRemoveTag_UnreferencedDeletesDirectly(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "t1", Name: "t1"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}

	result, err := s.RemoveTag("t1", false)
	if err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	if result.Forced {
		t.Fatalf("expected Forced=false for a non-force removal")
	}
	if s.TagExists("t1") {
		t.Fatalf("tag should have been deleted")
	}
}

func TestRemoveTag_RejectsReferencedWithoutForceThenDetachesWithForce(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "parent", Name: "parent"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "child", Name: "child", ParentIDs: []string{"parent"}}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.a", Category: model.CategoryAction, Label: "a", Tags: []string{"parent"}}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"parent"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}

	if _, err := s.RemoveTag("parent", false); err == nil {
		t.Fatalf("expected error removing a referenced tag without --force")
	}
	if !s.TagExists("parent") {
		t.Fatalf("referenced tag should not have been deleted without --force")
	}

	result, err := s.RemoveTag("parent", true)
	if err != nil {
		t.Fatalf("RemoveTag --force: %v", err)
	}
	if len(result.DetachedTransitions) != 1 || result.DetachedTransitions[0] != "T-1" {
		t.Fatalf("expected T-1 detached, got %v", result.DetachedTransitions)
	}
	if len(result.DetachedVocab) != 1 || result.DetachedVocab[0] != "act.a" {
		t.Fatalf("expected act.a detached, got %v", result.DetachedVocab)
	}
	if len(result.DetachedTags) != 1 || result.DetachedTags[0] != "child" {
		t.Fatalf("expected child detached, got %v", result.DetachedTags)
	}
	if s.TagExists("parent") {
		t.Fatalf("tag should have been deleted after force detach")
	}

	tx, err := s.LoadTransition("T-1")
	if err != nil {
		t.Fatalf("LoadTransition: %v", err)
	}
	if len(tx.Tags) != 0 {
		t.Fatalf("expected T-1.Tags to be empty after detach, got %v", tx.Tags)
	}

	v, err := s.LoadVocab("act.a")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if len(v.Tags) != 0 {
		t.Fatalf("expected act.a.Tags to be empty after detach, got %v", v.Tags)
	}

	child, err := s.LoadTag("child")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if len(child.ParentIDs) != 0 {
		t.Fatalf("expected child.ParentIDs to be empty after detach, got %v", child.ParentIDs)
	}
}

func TestRemoveTag_RejectsDecisionTargetEvenWithForce(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTag(model.Tag{ID: "t1", Name: "t1"}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	if err := s.SaveDecision(model.Decision{
		ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "t1"},
		Why: "w", At: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	if _, err := s.RemoveTag("t1", true); err == nil {
		t.Fatalf("expected error removing a decision-targeted tag even with force")
	}
	if !s.TagExists("t1") {
		t.Fatalf("decision-targeted tag should not have been deleted")
	}
}

func TestRemoveTransition_CascadesOnlyItsOwnDecisions(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}
	if err := s.SaveDecision(model.Decision{
		ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-1"},
		Why: "w", At: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}
	if err := s.SaveDecision(model.Decision{
		ID: "01D2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why: "w", At: "2026-01-01T00:00:00Z",
	}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	result, err := s.RemoveTransition("T-1", "no longer needed")
	if err != nil {
		t.Fatalf("RemoveTransition: %v", err)
	}
	if len(result.RemovedDecisions) != 1 || result.RemovedDecisions[0] != "01D1" {
		t.Fatalf("expected 01D1 removed, got %v", result.RemovedDecisions)
	}
	if result.Why != "no longer needed" {
		t.Fatalf("expected Why echoed back, got %q", result.Why)
	}

	if s.TransitionExists("T-1") {
		t.Fatalf("transition should have been deleted")
	}

	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(snap.Decisions) != 1 || snap.Decisions[0].ID != "01D2" {
		t.Fatalf("expected only 01D2 to survive, got %v", snap.Decisions)
	}
}
