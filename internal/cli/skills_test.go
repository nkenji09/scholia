package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runSkills は scholia skills サブコマンドをテスト用に実行するヘルパ。
// skills install は --dir を持たないルートコマンドなので run() は使わない。
func runSkills(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(args)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

// expectedSkillFiles は agents/skills/ 配下から embed されるはずのファイル一覧
// （_scholia-shared を含む相対構造を保つこと自体を検証する）。
var expectedSkillFiles = []string{
	filepath.Join("scholia", "SKILL.md"),
	filepath.Join("scholia", "README.md"),
	filepath.Join("scholia-change", "SKILL.md"),
	filepath.Join("scholia-config-setup", "SKILL.md"),
	filepath.Join("_scholia-shared", "references", "modeling-principles.md"),
}

func TestSkillsInstall_Project_MaterializesFiles(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := runSkills(t, "skills", "install", "--project")
	if err != nil {
		t.Fatalf("skills install --project failed: %v\noutput:\n%s", err, out)
	}

	skillsRoot := filepath.Join(dir, ".claude", "skills")
	for _, rel := range expectedSkillFiles {
		full := filepath.Join(skillsRoot, rel)
		data, err := os.ReadFile(full)
		if err != nil {
			t.Fatalf("expected file %s to exist: %v", full, err)
		}
		if len(data) == 0 {
			t.Fatalf("expected file %s to be non-empty", full)
		}
	}

	// 元の SKILL.md 内容と一致すること（コピーが内容を変えていないことの確認）。
	srcSkillMD, err := os.ReadFile(filepath.Join(cwd, "..", "..", "agents", "skills", "scholia", "SKILL.md"))
	if err != nil {
		t.Skipf("source SKILL.md not found relative to test (repo layout dependent): %v", err)
	}
	dstSkillMD, err := os.ReadFile(filepath.Join(skillsRoot, "scholia", "SKILL.md"))
	if err != nil {
		t.Fatalf("dest SKILL.md missing: %v", err)
	}
	if string(srcSkillMD) != string(dstSkillMD) {
		t.Fatalf("installed scholia/SKILL.md content differs from source")
	}
}

func TestSkillsInstall_DefaultDoesNotOverwrite_ForceDoes(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	if out, err := runSkills(t, "skills", "install", "--project"); err != nil {
		t.Fatalf("first install failed: %v\noutput:\n%s", err, out)
	}

	target := filepath.Join(dir, ".claude", "skills", "scholia", "SKILL.md")
	// 展開後のファイルを改変し、次回既定実行で消えない（スキップされる）ことを確認する。
	sentinel := []byte("LOCAL EDIT SENTINEL")
	if err := os.WriteFile(target, sentinel, 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runSkills(t, "skills", "install", "--project")
	if err != nil {
		t.Fatalf("second install (default, no --force) failed: %v\noutput:\n%s", err, out)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(sentinel) {
		t.Fatalf("expected existing file to be left untouched without --force, got: %s", string(data))
	}
	if !strings.Contains(out, "警告") || !strings.Contains(out, "スキップ") {
		t.Fatalf("expected skip warning in output, got:\n%s", out)
	}

	// --force で上書きされること。
	out, err = runSkills(t, "skills", "install", "--project", "--force")
	if err != nil {
		t.Fatalf("third install (--force) failed: %v\noutput:\n%s", err, out)
	}
	data, err = os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == string(sentinel) {
		t.Fatalf("expected --force to overwrite existing file, but sentinel content remained")
	}
}

func TestSkillsInstall_UserAndProjectTogetherIsError(t *testing.T) {
	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := runSkills(t, "skills", "install", "--user", "--project")
	if err == nil {
		t.Fatalf("expected error when both --user and --project are given, output:\n%s", out)
	}
}

func TestSkillsInstall_User_TargetsHomeDir(t *testing.T) {
	fakeHome := t.TempDir()
	t.Setenv("HOME", fakeHome)

	dir := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	out, err := runSkills(t, "skills", "install", "--user")
	if err != nil {
		t.Fatalf("skills install --user failed: %v\noutput:\n%s", err, out)
	}

	target := filepath.Join(fakeHome, ".claude", "skills", "scholia", "SKILL.md")
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected %s to exist under fake HOME: %v", target, err)
	}
}
