package cli

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

func TestCLI_VocabAddDescription(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.user.login", "--label", "ログイン",
		"--description", "**重要**: セッションは httpOnly cookie で管理する。")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("act.user.login")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if !strings.Contains(v.Description, "httpOnly cookie") {
		t.Fatalf("Description = %q, want it to round-trip through the store", v.Description)
	}
}

func TestCLI_VocabAddWithoutDescriptionOmitsField(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	out := mustRun(t, dir, "vocab", "add", "action", "act.user.login", "--label", "ログイン", "--json")

	if strings.Contains(out, `"description"`) {
		t.Fatalf("expected omitempty to drop description from JSON output when unset, got: %s", out)
	}
}
