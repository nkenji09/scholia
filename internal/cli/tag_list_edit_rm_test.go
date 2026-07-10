package cli

import (
	"strings"
	"testing"
)

func TestCLI_TagListFlatAndKindFilter(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out := mustRun(t, dir, "tag", "list")
	for _, want := range []string{"subject.auth", "req.auth", "req.auth-happy"} {
		if !strings.Contains(out, want) {
			t.Fatalf("tag list missing %q:\n%s", want, out)
		}
	}

	filtered := mustRun(t, dir, "tag", "list", "--kind", "requirement")
	if strings.Contains(filtered, "subject.auth") {
		t.Fatalf("tag list --kind requirement should not include subject.auth (kind=subject):\n%s", filtered)
	}
	if !strings.Contains(filtered, "req.auth") {
		t.Fatalf("tag list --kind requirement should include req.auth:\n%s", filtered)
	}
}

func TestCLI_TagListTreeNestsByParentAndShowsMultiParentTwice(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "p1", "--name", "p1")
	mustRun(t, dir, "tag", "create", "p2", "--name", "p2")
	mustRun(t, dir, "tag", "create", "child", "--name", "child", "--parent", "p1", "--parent", "p2")

	out := mustRun(t, dir, "tag", "list", "--tree")
	if strings.Count(out, "- child ") != 2 {
		t.Fatalf("expected multi-parent tag to appear under both parents in --tree output, got:\n%s", out)
	}
}

func TestCLI_TagEditUpdatesOnlyGivenFields(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "t1", "--name", "one", "--desc", "orig")

	mustRun(t, dir, "tag", "edit", "t1", "--color", "#3b82f6")

	out := mustRun(t, dir, "tag", "list", "--json")
	if !strings.Contains(out, "orig") {
		t.Fatalf("tag edit of --color should not clear --desc, got:\n%s", out)
	}
	if !strings.Contains(out, "#3b82f6") {
		t.Fatalf("expected --color to be applied:\n%s", out)
	}
}

func TestCLI_TagEditRejectsUndeclaredKindMissingParentAndCycle(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "a", "--name", "a")
	mustRun(t, dir, "tag", "create", "b", "--name", "b", "--parent", "a")

	if _, err := run(t, dir, "tag", "edit", "a", "--kind", "not-declared"); err == nil {
		t.Fatalf("expected error for undeclared kind")
	}
	if _, err := run(t, dir, "tag", "edit", "a", "--parent", "does.not.exist"); err == nil {
		t.Fatalf("expected error for missing parent")
	}
	if _, err := run(t, dir, "tag", "edit", "a", "--parent", "b"); err == nil {
		t.Fatalf("expected error: a -> parent b -> parent a would cycle")
	}

	if _, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint to remain green after rejected tag edits")
	}
}

func TestCLI_TagRmRejectsReferencedTagWithoutForce(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if _, err := run(t, dir, "tag", "rm", "req.auth"); err == nil {
		t.Fatalf("expected error removing a tag referenced by a transition")
	}
	if _, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint to remain green after a rejected tag rm")
	}
}

func TestCLI_TagRmForceDetachesFromAllReferencesAndLintStaysGreen(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "tag", "create", "req.auth-child", "--name", "child", "--kind", "requirement", "--parent", "req.auth")

	mustRun(t, dir, "tag", "rm", "req.auth", "--force")

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green after tag rm --force detach cascade, got:\n%s", out)
	}

	txOut := mustRun(t, dir, "show", "tx", "T-login", "--json")
	if strings.Contains(txOut, "req.auth") {
		t.Fatalf("expected req.auth to be detached from T-login's tags:\n%s", txOut)
	}
}

func TestCLI_TagRmRejectsWhenTagIsDecisionTarget(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "cross-cutting rule")

	if _, err := run(t, dir, "tag", "rm", "subject.auth", "--force"); err == nil {
		t.Fatalf("expected error removing a tag that is a decision target, even with --force")
	}
}
