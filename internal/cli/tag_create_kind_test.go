package cli

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

// --kind 省略時の既定/必須化（handoff: tag-kind 衛生）。

func TestCLI_TagCreateDefaultsKindWhenSingleTagKindDeclared(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "config", "set", "tagKinds", "concern")

	out := mustRun(t, dir, "tag", "create", "t1", "--name", "t1", "--json")
	if !strings.Contains(out, `"kind": "concern"`) {
		t.Fatalf("expected kind to default to the sole declared tagKind, got:\n%s", out)
	}

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	tag, err := s.LoadTag("t1")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if tag.Kind != "concern" {
		t.Fatalf("Kind = %q, want %q", tag.Kind, "concern")
	}
}

func TestCLI_TagCreateRequiresKindWhenMultipleTagKindsDeclared(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init") // default tagKinds = requirement,concern,subject (3)

	out, err := run(t, dir, "tag", "create", "t1", "--name", "t1")
	if err == nil {
		t.Fatalf("expected error for --kind omitted with multiple declared tagKinds, got output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "--kind が必須です") {
		t.Fatalf("expected error to explain --kind is required, got: %v", err)
	}
	if s, serr := store.Open(dir); serr == nil {
		if s.TagExists("t1") {
			t.Fatalf("tag must not be created when the required --kind is missing")
		}
	}
}

func TestCLI_TagCreateAllowsEmptyKindWhenNoTagKindsDeclared(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "config", "set", "tagKinds", "")

	mustRun(t, dir, "tag", "create", "t1", "--name", "t1")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	tag, err := s.LoadTag("t1")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if tag.Kind != "" {
		t.Fatalf("Kind = %q, want empty for degenerate (0 tagKinds) config", tag.Kind)
	}

	lintOut := mustRun(t, dir, "lint")
	if !strings.Contains(lintOut, "kind-missing") {
		t.Fatalf("expected lint to surface a kind-missing advisory for the null-kind tag, got:\n%s", lintOut)
	}
}
