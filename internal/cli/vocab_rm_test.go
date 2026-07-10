package cli

import "testing"

func TestCLI_VocabRmDeletesUnreferencedEntry(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a")

	mustRun(t, dir, "vocab", "rm", "cond.a")

	if _, err := run(t, dir, "vocab", "add", "condition", "cond.a", "--label", "re-add"); err != nil {
		t.Fatalf("expected cond.a to be gone so re-adding succeeds: %v", err)
	}
}

func TestCLI_VocabRmRejectsReferencedEntry(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if _, err := run(t, dir, "vocab", "rm", "act.user.submit-login"); err == nil {
		t.Fatalf("expected error removing a vocab referenced by a transition")
	}

	if _, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint to remain green after a rejected vocab rm")
	}
}

func TestCLI_VocabRmRejectsCategoryMismatch(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a")

	if _, err := run(t, dir, "vocab", "rm", "cond.a", "--category", "action"); err == nil {
		t.Fatalf("expected error for --category mismatch")
	}
}
