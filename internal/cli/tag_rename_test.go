package cli

import (
	"strings"
	"testing"
)

// setupTagTree seeds a small subject/requirement tree plus a transition tagged
// with the leaf, so rename/cascade can be exercised end-to-end and checked for
// lint-green afterwards.
func setupTagTree(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.user.do", "--label", "do")
	mustRun(t, dir, "vocab", "add", "effect", "eff.done", "--label", "done")
	mustRun(t, dir, "tag", "create", "subject.comp", "--name", "Comp", "--kind", "subject")
	mustRun(t, dir, "tag", "create", "subject.comp-part", "--name", "Part", "--kind", "subject", "--parent", "subject.comp")
	mustRun(t, dir, "tx", "add", "T-do",
		"--action", "act.user.do", "--then", "eff.done",
		"--tags", "subject.comp-part,subject.comp")
}

func TestCLI_TagRenameThenLintGreen(t *testing.T) {
	dir := t.TempDir()
	setupTagTree(t, dir)
	// decision on the tag so the decision-target ref site is exercised too.
	mustRun(t, dir, "decide", "--on", "tag:subject.comp", "--why", "初期")

	mustRun(t, dir, "tag", "rename", "subject.comp", "subject.widget")

	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint green after tag rename, got error. output:\n%s", out)
	}

	// transition, child parent, and decision all follow the rename.
	tree := mustRun(t, dir, "tag", "list", "--tree")
	if !strings.Contains(tree, "subject.widget") || strings.Contains(tree, "subject.comp\n") {
		t.Fatalf("tree not updated after rename:\n%s", tree)
	}
	rules := mustRun(t, dir, "rules", "--tag", "subject.widget")
	if !strings.Contains(rules, "初期") {
		t.Fatalf("decision did not follow renamed tag:\n%s", rules)
	}
}

func TestCLI_TagRenameCascadeThenLintGreen(t *testing.T) {
	dir := t.TempDir()
	setupTagTree(t, dir)

	out := mustRun(t, dir, "tag", "rename", "--cascade", "subject.comp", "subject.kit")

	if !strings.Contains(out, "改名タグ 2 件") {
		t.Fatalf("expected 2 tags renamed by cascade, got:\n%s", out)
	}
	if lintOut, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint green after cascade rename, got error. output:\n%s", lintOut)
	}
	tree := mustRun(t, dir, "tag", "list", "--tree")
	for _, want := range []string{"subject.kit", "subject.kit-part"} {
		if !strings.Contains(tree, want) {
			t.Fatalf("cascade tree missing %q:\n%s", want, tree)
		}
	}
	if strings.Contains(tree, "subject.comp") {
		t.Fatalf("old prefix still present after cascade:\n%s", tree)
	}
}

func TestCLI_TagRenameCaseOnly(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "subject.uisamplerangeinput", "--name", "widget", "--kind", "subject")

	mustRun(t, dir, "tag", "rename", "subject.uisamplerangeinput", "subject.UISampleRangeInput")

	if lintOut, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint green after case-only rename. output:\n%s", lintOut)
	}
	tree := mustRun(t, dir, "tag", "list", "--tree")
	if !strings.Contains(tree, "subject.UISampleRangeInput") {
		t.Fatalf("case-only rename not reflected:\n%s", tree)
	}
}

func TestCLI_TagRenameErrors(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "subject.a", "--name", "A", "--kind", "subject")
	mustRun(t, dir, "tag", "create", "subject.b", "--name", "B", "--kind", "subject")

	if _, err := run(t, dir, "tag", "rename", "subject.a", "subject.b"); err == nil {
		t.Fatalf("expected collision error")
	}
	if _, err := run(t, dir, "tag", "rename", "subject.missing", "subject.z"); err == nil {
		t.Fatalf("expected not-found error")
	}
	// after failed renames the store is unchanged and still lints green.
	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("store should stay green after rejected renames:\n%s", out)
	}
}
