package cli

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

// setupScrollFixture creates the scroll state-chain fixture: an effect that
// establishes a condition (mirrors eff.state.save-scroll-to-session →
// cond.view-scroll-in-session).
func setupScrollFixture(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.view-scroll-in-session", "--label", "セッション内のスクロール位置")
	mustRun(t, dir, "vocab", "add", "effect", "eff.state.save-scroll-to-session", "--label", "スクロール位置を保存",
		"--establishes", "cond.view-scroll-in-session")
}

func TestCLI_VocabAddEstablishes_ValidatesEffectOnlyAndExistence(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.x", "--label", "x")

	// establishes on a non-effect vocab is rejected.
	if _, err := run(t, dir, "vocab", "add", "action", "act.y", "--label", "y", "--establishes", "cond.x"); err == nil {
		t.Fatalf("expected --establishes on action to be rejected")
	}
	// establishes pointing at a missing id is rejected.
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.z", "--label", "z", "--establishes", "cond.missing"); err == nil {
		t.Fatalf("expected --establishes to a missing id to be rejected")
	}
	// establishes pointing at a non-condition is rejected.
	mustRun(t, dir, "vocab", "add", "effect", "eff.other", "--label", "o")
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.z", "--label", "z", "--establishes", "eff.other"); err == nil {
		t.Fatalf("expected --establishes to a non-condition to be rejected")
	}
	// valid establishes on an effect goes through.
	mustRun(t, dir, "vocab", "add", "effect", "eff.ok", "--label", "ok", "--establishes", "cond.x",
		"--ref", "DESIGN.md §x", "--alt-label", "別名A", "--alt-label", "別名B")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("eff.ok")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if len(v.Establishes) != 1 || v.Establishes[0] != "cond.x" {
		t.Fatalf("Establishes = %v, want [cond.x]", v.Establishes)
	}
	if v.Ref != "DESIGN.md §x" {
		t.Fatalf("Ref = %q", v.Ref)
	}
	if len(v.AltLabels) != 2 {
		t.Fatalf("AltLabels = %v, want 2", v.AltLabels)
	}
}

func TestCLI_VocabEditEstablishesRefAltLabelsAndClears(t *testing.T) {
	dir := t.TempDir()
	setupScrollFixture(t, dir)

	// edit ref + alt-label
	mustRun(t, dir, "vocab", "edit", "eff.state.save-scroll-to-session",
		"--ref", "https://example/spec", "--alt-label", "スクロール保存")
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("eff.state.save-scroll-to-session")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Ref != "https://example/spec" || len(v.AltLabels) != 1 {
		t.Fatalf("after edit: ref=%q altLabels=%v", v.Ref, v.AltLabels)
	}
	if len(v.Establishes) != 1 {
		t.Fatalf("establishes should survive an unrelated edit, got %v", v.Establishes)
	}

	// editing establishes to a missing id is rejected.
	if _, err := run(t, dir, "vocab", "edit", "eff.state.save-scroll-to-session", "--establishes", "cond.missing"); err == nil {
		t.Fatalf("expected edit --establishes to a missing id to be rejected")
	}

	// clear establishes + alt-labels
	mustRun(t, dir, "vocab", "edit", "eff.state.save-scroll-to-session", "--clear-establishes", "--clear-alt-labels")
	v, err = s.LoadVocab("eff.state.save-scroll-to-session")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if len(v.Establishes) != 0 || len(v.AltLabels) != 0 {
		t.Fatalf("after clears: establishes=%v altLabels=%v", v.Establishes, v.AltLabels)
	}

	// --clear-X and --X together are rejected.
	if _, err := run(t, dir, "vocab", "edit", "eff.state.save-scroll-to-session",
		"--clear-establishes", "--establishes", "cond.view-scroll-in-session"); err == nil {
		t.Fatalf("expected --clear-establishes with --establishes to be rejected")
	}
}

func TestCLI_DecideOnVocab(t *testing.T) {
	dir := t.TempDir()
	setupScrollFixture(t, dir)

	// dry-run first (advisory preview, append-only recommended flow).
	dry := mustRun(t, dir, "decide", "--on", "vocab:cond.view-scroll-in-session",
		"--why", "# テスト用の見出し\n\nスクロール復元遷移の前提条件として materialize する", "--dry-run")
	if !strings.Contains(dry, "dry-run") {
		t.Fatalf("dry-run output unexpected: %s", dry)
	}

	// real decide on a vocab.
	out := mustRun(t, dir, "decide", "--on", "vocab:cond.view-scroll-in-session",
		"--why", "# テスト用の見出し\n\nスクロール復元遷移の前提条件として materialize する", "--json")
	if !strings.Contains(out, `"vocab"`) {
		t.Fatalf("decide --on vocab JSON should record vocab target: %s", out)
	}

	// decide on a missing vocab is rejected.
	if _, err := run(t, dir, "decide", "--on", "vocab:cond.missing", "--why", "# テスト用の見出し\n\nx"); err == nil {
		t.Fatalf("expected decide on missing vocab to be rejected")
	}

	// decision list --on vocab: surfaces it.
	list := mustRun(t, dir, "decision", "list", "--on", "vocab:cond.view-scroll-in-session")
	if !strings.Contains(list, "vocab:cond.view-scroll-in-session") {
		t.Fatalf("decision list --on vocab did not surface the decision: %s", list)
	}
}

func TestCLI_ShowVocabBidirectionalEstablishesAndDecisions(t *testing.T) {
	dir := t.TempDir()
	setupScrollFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "vocab:cond.view-scroll-in-session", "--why", "# テスト用の見出し\n\n前提条件の正本")

	// The effect side shows what it establishes.
	effOut := mustRun(t, dir, "show", "vocab", "eff.state.save-scroll-to-session")
	if !strings.Contains(effOut, "この効果が成立させる条件") || !strings.Contains(effOut, "cond.view-scroll-in-session") {
		t.Fatalf("effect show vocab missing establishes forward: %s", effOut)
	}

	// The condition side shows what establishes it (reverse) + its decision.
	condOut := mustRun(t, dir, "show", "vocab", "cond.view-scroll-in-session")
	if !strings.Contains(condOut, "この条件を成立させる効果") || !strings.Contains(condOut, "eff.state.save-scroll-to-session") {
		t.Fatalf("condition show vocab missing establishedBy reverse: %s", condOut)
	}
	if !strings.Contains(condOut, "decisions (1)") {
		t.Fatalf("condition show vocab missing vocab-target decision: %s", condOut)
	}
}

func TestCLI_SearchIncludesAltLabels(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "effect", "eff.emit.scope-disclosure", "--label", "スコープ開示",
		"--alt-label", "保証の外の印字")

	out := mustRun(t, dir, "search", "保証の外", "--type", "vocab")
	if !strings.Contains(out, "eff.emit.scope-disclosure") {
		t.Fatalf("search should match on altLabels, got: %s", out)
	}
}
