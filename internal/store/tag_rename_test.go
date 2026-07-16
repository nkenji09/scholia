package store

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
)

// seedTag is a small helper to save a tag with parents.
func seedTag(t *testing.T, s *Store, id, name string, parents ...string) {
	t.Helper()
	if err := s.SaveTag(model.Tag{ID: id, Name: name, ParentIDs: parents}); err != nil {
		t.Fatalf("SaveTag %s: %v", id, err)
	}
}

func TestRenameTag_RepointsAllFourReferenceSites(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// old tag with metadata that must survive; a child pointing at it; a
	// transition, a vocab entry, and a decision all referencing it.
	if err := s.SaveTag(model.Tag{
		ID: "subject.old", Name: "Old", Kind: "subject",
		Description: "keep me", Color: "#abc", Ref: "https://x",
	}); err != nil {
		t.Fatalf("SaveTag: %v", err)
	}
	seedTag(t, s, "subject.child", "Child", "subject.old", "subject.other")
	seedTag(t, s, "subject.other", "Other")
	if err := s.SaveTransition(model.Transition{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"subject.old", "subject.other"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}
	if err := s.SaveVocab(model.VocabEntry{ID: "act.a", Category: model.CategoryAction, Label: "a", Tags: []string{"subject.old"}}); err != nil {
		t.Fatalf("SaveVocab: %v", err)
	}
	if err := s.SaveDecision(model.Decision{ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.old"}, Why: "w", At: "2026-01-01T00:00:00Z"}); err != nil {
		t.Fatalf("SaveDecision: %v", err)
	}

	result, err := s.RenameTag("subject.old", "subject.new", false)
	if err != nil {
		t.Fatalf("RenameTag: %v", err)
	}

	if s.TagExists("subject.old") {
		t.Fatalf("old tag file should be gone")
	}
	if !s.TagExists("subject.new") {
		t.Fatalf("new tag file should exist")
	}
	// metadata preserved, id updated.
	nt, err := s.LoadTag("subject.new")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if nt.Name != "Old" || nt.Kind != "subject" || nt.Description != "keep me" || nt.Color != "#abc" || nt.Ref != "https://x" {
		t.Fatalf("metadata not preserved: %+v", nt)
	}

	// (1) parentIds repointed on the child (and unrelated parent untouched).
	child, _ := s.LoadTag("subject.child")
	if !containsID(child.ParentIDs, "subject.new") || containsID(child.ParentIDs, "subject.old") {
		t.Fatalf("child parentIds not repointed: %v", child.ParentIDs)
	}
	if !containsID(child.ParentIDs, "subject.other") {
		t.Fatalf("unrelated parent should be kept: %v", child.ParentIDs)
	}
	// (2) transition tags repointed, unrelated tag kept.
	tx, _ := s.LoadTransition("T-1")
	if !containsID(tx.Tags, "subject.new") || containsID(tx.Tags, "subject.old") || !containsID(tx.Tags, "subject.other") {
		t.Fatalf("transition tags not repointed: %v", tx.Tags)
	}
	// (3) vocab tags repointed.
	v, _ := s.LoadVocab("act.a")
	if !containsID(v.Tags, "subject.new") || containsID(v.Tags, "subject.old") {
		t.Fatalf("vocab tags not repointed: %v", v.Tags)
	}
	// (4) decision target repointed.
	d, _ := s.LoadDecision("01D1")
	if d.Target.ID != "subject.new" {
		t.Fatalf("decision target not repointed: %+v", d.Target)
	}

	// result summary.
	if result.RenamedTags["subject.old"] != "subject.new" || len(result.RenamedTags) != 1 {
		t.Fatalf("unexpected RenamedTags: %v", result.RenamedTags)
	}
	if len(result.UpdatedTags) != 1 || result.UpdatedTags[0] != "subject.child" {
		t.Fatalf("unexpected UpdatedTags: %v", result.UpdatedTags)
	}
	if len(result.UpdatedTransitions) != 1 || len(result.UpdatedVocab) != 1 || len(result.UpdatedDecisions) != 1 {
		t.Fatalf("unexpected updated counts: %+v", result)
	}
}

func TestRenameTag_CascadeRelabelsSubtreeAndRefs(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	// req.comp -> req.comp-a -> req.comp-a-1  (two-level subtree)
	seedTag(t, s, "req.comp", "Comp")
	seedTag(t, s, "req.comp-a", "A", "req.comp")
	seedTag(t, s, "req.comp-a-1", "A1", "req.comp-a")
	// a descendant that does NOT carry the prefix: keeps its id, parent repointed.
	seedTag(t, s, "req.loose", "Loose", "req.comp")
	// a sibling that merely shares a string prefix but is not a descendant: untouched.
	seedTag(t, s, "req.company", "Company")
	if err := s.SaveTransition(model.Transition{ID: "T-x", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"req.comp-a-1"}}); err != nil {
		t.Fatalf("SaveTransition: %v", err)
	}

	result, err := s.RenameTag("req.comp", "req.kit", true)
	if err != nil {
		t.Fatalf("RenameTag cascade: %v", err)
	}

	// subtree ids relabeled by prefix substitution.
	for old, want := range map[string]string{
		"req.comp":     "req.kit",
		"req.comp-a":   "req.kit-a",
		"req.comp-a-1": "req.kit-a-1",
	} {
		if s.TagExists(old) {
			t.Fatalf("old id %q should be gone", old)
		}
		if !s.TagExists(want) {
			t.Fatalf("new id %q should exist", want)
		}
		if result.RenamedTags[old] != want {
			t.Fatalf("RenamedTags[%q]=%q, want %q", old, result.RenamedTags[old], want)
		}
	}
	if len(result.RenamedTags) != 3 {
		t.Fatalf("expected 3 renamed tags, got %v", result.RenamedTags)
	}

	// parentIds within the subtree repointed to new ids.
	a, _ := s.LoadTag("req.kit-a")
	if !containsID(a.ParentIDs, "req.kit") {
		t.Fatalf("req.kit-a parent not repointed: %v", a.ParentIDs)
	}
	a1, _ := s.LoadTag("req.kit-a-1")
	if !containsID(a1.ParentIDs, "req.kit-a") {
		t.Fatalf("req.kit-a-1 parent not repointed: %v", a1.ParentIDs)
	}

	// loose descendant kept its id but its parent repointed comp->kit.
	if !s.TagExists("req.loose") {
		t.Fatalf("req.loose id should be unchanged")
	}
	loose, _ := s.LoadTag("req.loose")
	if !containsID(loose.ParentIDs, "req.kit") || containsID(loose.ParentIDs, "req.comp") {
		t.Fatalf("req.loose parent not repointed: %v", loose.ParentIDs)
	}

	// prefix-sharing non-descendant untouched.
	if !s.TagExists("req.company") {
		t.Fatalf("req.company must not be renamed")
	}

	// transition referencing the deepest id follows the rename.
	tx, _ := s.LoadTransition("T-x")
	if !containsID(tx.Tags, "req.kit-a-1") {
		t.Fatalf("transition tag not repointed: %v", tx.Tags)
	}
}

func TestRenameTag_CaseOnlyRenameSucceeds(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	seedTag(t, s, "subject.uisamplerangeinput", "widget")
	seedTag(t, s, "subject.child", "child", "subject.uisamplerangeinput")

	if _, err := s.RenameTag("subject.uisamplerangeinput", "subject.UISampleRangeInput", false); err != nil {
		t.Fatalf("case-only RenameTag: %v", err)
	}

	// The renamed record must be loadable under the new (cased) id with the
	// file on disk named accordingly, and the child's parent repointed.
	nt, err := s.LoadTag("subject.UISampleRangeInput")
	if err != nil {
		t.Fatalf("LoadTag new case: %v", err)
	}
	if nt.ID != "subject.UISampleRangeInput" {
		t.Fatalf("id not updated: %q", nt.ID)
	}
	// The directory entry must carry the new case exactly (case-preserving).
	entries, _ := os.ReadDir(filepath.Join(s.Dir, tagsDir))
	found := false
	for _, e := range entries {
		if e.Name() == "subject.UISampleRangeInput.json" {
			found = true
		}
		if e.Name() == "subject.uisamplerangeinput.json" {
			t.Fatalf("old-case file still present: %s", e.Name())
		}
	}
	if !found {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Fatalf("new-case file subject.UISampleRangeInput.json not found; entries=%v", names)
	}
	child, _ := s.LoadTag("subject.child")
	if !containsID(child.ParentIDs, "subject.UISampleRangeInput") {
		t.Fatalf("child parent not repointed to new case: %v", child.ParentIDs)
	}
}

func TestRenameTag_RejectsCollisionAndLeavesStoreUntouched(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	seedTag(t, s, "subject.a", "A")
	seedTag(t, s, "subject.b", "B")
	seedTag(t, s, "subject.child", "C", "subject.a")

	// direct collision: subject.a -> subject.b (exists).
	if _, err := s.RenameTag("subject.a", "subject.b", false); err == nil {
		t.Fatalf("expected collision error")
	}
	// atomicity: nothing changed.
	if !s.TagExists("subject.a") || !s.TagExists("subject.b") {
		t.Fatalf("collision must leave both tags intact")
	}
	child, _ := s.LoadTag("subject.child")
	if !containsID(child.ParentIDs, "subject.a") {
		t.Fatalf("collision must not repoint refs: %v", child.ParentIDs)
	}
}

func TestRenameTag_CascadeCollisionRejected(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	seedTag(t, s, "req.comp", "Comp")
	seedTag(t, s, "req.comp-a", "A", "req.comp")
	// A cascade-generated id (req.kit-a) already exists → must be rejected
	// before anything is written.
	seedTag(t, s, "req.kit-a", "Existing")

	if _, err := s.RenameTag("req.comp", "req.kit", true); err == nil {
		t.Fatalf("expected cascade collision error for generated id req.kit-a")
	}
	if !s.TagExists("req.comp") || !s.TagExists("req.comp-a") {
		t.Fatalf("cascade collision must leave the subtree intact")
	}
}

func TestRenameTag_RejectsMissingAndSameID(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	seedTag(t, s, "subject.a", "A")
	if _, err := s.RenameTag("subject.missing", "subject.z", false); err == nil {
		t.Fatalf("expected not-found error")
	}
	if _, err := s.RenameTag("subject.a", "subject.a", false); err == nil {
		t.Fatalf("expected same-id error")
	}
	if _, err := s.RenameTag("subject.a", "", false); err == nil {
		t.Fatalf("expected empty-newId error")
	}
}
