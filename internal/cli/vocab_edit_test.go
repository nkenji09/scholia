package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

func TestCLI_VocabEditUpdatesDescriptionKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "ラベル", "--owner", "server", "--description", "orig")

	mustRun(t, dir, "vocab", "edit", "eff.a", "--description", "更新後の説明")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("eff.a")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Description != "更新後の説明" {
		t.Fatalf("Description = %q, want updated", v.Description)
	}
	if v.Label != "ラベル" || v.Owner != "server" || v.Category != "effect" {
		t.Fatalf("other fields not preserved: %+v", v)
	}
}

func TestCLI_VocabEditDescFile(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a")

	descPath := filepath.Join(t.TempDir(), "desc.md")
	content := "複数段落の説明。\n\n2段落目。\n"
	if err := os.WriteFile(descPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mustRun(t, dir, "vocab", "edit", "cond.a", "--desc-file", descPath)

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("cond.a")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Description != strings.TrimRight(content, "\n") {
		t.Fatalf("Description = %q, want file content", v.Description)
	}
}

func TestCLI_VocabEditRejectsUnknownID(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "vocab", "edit", "cond.missing", "--description", "x"); err == nil {
		t.Fatalf("expected error for unknown vocab id")
	}
}

func TestCLI_VocabEditRequiresOneDescFlag(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a")
	if _, err := run(t, dir, "vocab", "edit", "cond.a"); err == nil {
		t.Fatalf("expected error when no --description/--desc-file/--edit given")
	}
}

func TestCLI_VocabEditUpdatesLabelKeepsDescription(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.foo", "--label", "旧ラベル", "--description", "説明本文")

	mustRun(t, dir, "vocab", "edit", "act.foo", "--label", "新ラベル")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("act.foo")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Label != "新ラベル" {
		t.Fatalf("Label = %q, want 新ラベル", v.Label)
	}
	if v.Description != "説明本文" {
		t.Fatalf("Description = %q, want unchanged", v.Description)
	}
}

func TestCLI_VocabEditUpdatesLabelAndDescriptionTogether(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.foo", "--label", "旧ラベル", "--description", "旧説明")

	mustRun(t, dir, "vocab", "edit", "act.foo", "--label", "新ラベル", "--description", "新説明")

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("act.foo")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Label != "新ラベル" {
		t.Fatalf("Label = %q, want 新ラベル", v.Label)
	}
	if v.Description != "新説明" {
		t.Fatalf("Description = %q, want 新説明", v.Description)
	}
}

func TestCLI_VocabEditRejectsEmptyLabel(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.foo", "--label", "ラベル")
	if _, err := run(t, dir, "vocab", "edit", "act.foo", "--label", ""); err == nil {
		t.Fatalf("expected error for empty --label")
	}
}

func TestCLI_VocabEditRejectsUnknownIDWithLabel(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "vocab", "edit", "act.missing", "--label", "x"); err == nil {
		t.Fatalf("expected error for unknown vocab id")
	}
}

func TestCLI_VocabEditRejectsDescriptionAndDescFileTogether(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a")
	descPath := filepath.Join(t.TempDir(), "d.md")
	if err := os.WriteFile(descPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run(t, dir, "vocab", "edit", "cond.a", "--description", "a", "--desc-file", descPath); err == nil {
		t.Fatalf("expected error for --description + --desc-file together")
	}
}
