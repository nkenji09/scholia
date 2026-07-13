package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/product-memory/internal/store"
)

func withFakeEditor(t *testing.T, capture func(editorCmd string) string) {
	t.Helper()
	orig := runEditor
	runEditor = func(editorCmd, path string) error {
		return os.WriteFile(path, []byte(capture(editorCmd)), 0o644)
	}
	t.Cleanup(func() { runEditor = orig })
}

// --- descSource 単体: 排他制御と優先度 ---

func TestDescSource_ResolveNoneSetReturnsUnchanged(t *testing.T) {
	value, changed, err := descSource{}.resolve()
	if err != nil || changed || value != "" {
		t.Fatalf("resolve() = (%q, %v, %v), want (\"\", false, nil)", value, changed, err)
	}
}

func TestDescSource_ResolveDirectOnly(t *testing.T) {
	value, changed, err := descSource{direct: "hello", directSet: true}.resolve()
	if err != nil || !changed || value != "hello" {
		t.Fatalf("resolve() = (%q, %v, %v), want (\"hello\", true, nil)", value, changed, err)
	}
}

func TestDescSource_ResolveFileOnlyTrimsTrailingNewlinePreservesMarkdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "d.md")
	content := "# 見出し\n\n段落1。\n\n段落2 with \"quotes\" and 改行\nさらに続く。\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	value, changed, err := descSource{file: path}.resolve()
	if err != nil || !changed {
		t.Fatalf("resolve() err=%v changed=%v", err, changed)
	}
	want := strings.TrimRight(content, "\n")
	if value != want {
		t.Fatalf("value = %q, want %q", value, want)
	}
}

func TestDescSource_ResolveFileMissingReturnsError(t *testing.T) {
	_, _, err := descSource{file: "/no/such/path.md"}.resolve()
	if err == nil {
		t.Fatalf("expected error for missing --desc-file path")
	}
}

func TestDescSource_ResolveEditOnlyUsesSeam(t *testing.T) {
	withFakeEditor(t, func(editorCmd string) string { return "編集内容\n" })
	value, changed, err := descSource{edit: true}.resolve()
	if err != nil || !changed || value != "編集内容" {
		t.Fatalf("resolve() = (%q, %v, %v), want (\"編集内容\", true, nil)", value, changed, err)
	}
}

func TestDescSource_ResolveRejectsMultipleSources(t *testing.T) {
	cases := []descSource{
		{direct: "a", directSet: true, file: "/tmp/x"},
		{direct: "a", directSet: true, edit: true},
		{file: "/tmp/x", edit: true},
		{direct: "a", directSet: true, file: "/tmp/x", edit: true},
	}
	for i, d := range cases {
		if _, _, err := d.resolve(); err == nil {
			t.Fatalf("case %d: expected exclusivity error for %+v", i, d)
		}
	}
}

// --- $EDITOR seam: 実 editor 無しで --edit の配線を検証 ---

func TestCLI_EditFlagRespectsEditorEnvAndSeam(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	var gotEditorCmd string
	withFakeEditor(t, func(editorCmd string) string {
		gotEditorCmd = editorCmd
		return "段落1\n\n段落2\n"
	})
	t.Setenv("EDITOR", "my-fake-editor")

	mustRun(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern", "--edit")

	if gotEditorCmd != "my-fake-editor" {
		t.Fatalf("$EDITOR not respected: got %q", gotEditorCmd)
	}

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	tag, err := s.LoadTag("t1")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if tag.Description != "段落1\n\n段落2" {
		t.Fatalf("Description = %q, want multi-paragraph markdown preserved", tag.Description)
	}
}

func TestCLI_EditFlagDefaultsToViWhenEditorUnset(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	var gotEditorCmd string
	withFakeEditor(t, func(editorCmd string) string {
		gotEditorCmd = editorCmd
		return "x"
	})
	t.Setenv("EDITOR", "")

	mustRun(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern", "--edit")

	if gotEditorCmd != "vi" {
		t.Fatalf("expected default editor \"vi\", got %q", gotEditorCmd)
	}
}

// --- --desc-file 配線: tag create / tag edit / vocab add ---

func TestCLI_TagCreateDescFileRoundTripsMultiParagraphMarkdown(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	descPath := filepath.Join(t.TempDir(), "desc.md")
	content := "# タイトル\n\n1段落目。\n\n2段落目 \"引用符\" と改行\nもう一行。\n"
	if err := os.WriteFile(descPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	mustRun(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern", "--desc-file", descPath)

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	tag, err := s.LoadTag("t1")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if tag.Description != strings.TrimRight(content, "\n") {
		t.Fatalf("Description = %q, want file content preserved verbatim (minus trailing newline)", tag.Description)
	}
}

func TestCLI_TagEditDescFileUpdatesDescriptionKeepsOtherFields(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "t1", "--name", "one", "--kind", "concern", "--color", "#3b82f6")

	descPath := filepath.Join(t.TempDir(), "desc.md")
	content := "更新後の説明。\n\n複数段落。\n"
	if err := os.WriteFile(descPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mustRun(t, dir, "tag", "edit", "t1", "--desc-file", descPath)

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	tag, err := s.LoadTag("t1")
	if err != nil {
		t.Fatalf("LoadTag: %v", err)
	}
	if tag.Description != strings.TrimRight(content, "\n") {
		t.Fatalf("Description = %q, want updated content", tag.Description)
	}
	if tag.Color != "#3b82f6" {
		t.Fatalf("Color = %q, want preserved", tag.Color)
	}
	if tag.Name != "one" {
		t.Fatalf("Name = %q, want preserved", tag.Name)
	}
}

func TestCLI_VocabAddDescFileRoundTripsMarkdown(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	descPath := filepath.Join(t.TempDir(), "desc.md")
	content := "**重要**: httpOnly cookie で管理する。\n\n詳細説明。\n"
	if err := os.WriteFile(descPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a", "--desc-file", descPath)

	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	v, err := s.LoadVocab("act.a")
	if err != nil {
		t.Fatalf("LoadVocab: %v", err)
	}
	if v.Description != strings.TrimRight(content, "\n") {
		t.Fatalf("Description = %q, want file content preserved", v.Description)
	}
}

// --- 排他エラー: CLI 配線でも表面化することを確認 ---

func TestCLI_TagCreateRejectsDescAndDescFileTogether(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	descPath := filepath.Join(t.TempDir(), "d.md")
	if err := os.WriteFile(descPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern", "--desc", "a", "--desc-file", descPath); err == nil {
		t.Fatalf("expected error for --desc + --desc-file together")
	}
}

func TestCLI_TagEditRejectsDescAndEditTogether(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern")
	if _, err := run(t, dir, "tag", "edit", "t1", "--desc", "a", "--edit"); err == nil {
		t.Fatalf("expected error for --desc + --edit together")
	}
}

func TestCLI_VocabAddRejectsDescFileAndEditTogether(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	descPath := filepath.Join(t.TempDir(), "d.md")
	if err := os.WriteFile(descPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a", "--desc-file", descPath, "--edit"); err == nil {
		t.Fatalf("expected error for --desc-file + --edit together")
	}
}
