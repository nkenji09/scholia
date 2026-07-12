package cli

import (
	"strings"
	"testing"
)

func TestCLI_TxEditUpdatesOnlyGivenFields(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	mustRun(t, dir, "vocab", "add", "effect", "eff.other", "--label", "other")
	mustRun(t, dir, "tx", "edit", "T-login", "--then", "eff.other")

	out := mustRun(t, dir, "show", "tx", "T-login")
	if !strings.Contains(out, "eff.other") {
		t.Fatalf("expected --then to be updated:\n%s", out)
	}
	if !strings.Contains(out, "act.user.submit-login") {
		t.Fatalf("expected action to remain untouched by a --then-only edit:\n%s", out)
	}
}

func TestCLI_TxEditRejectsEmptyThenAndDanglingRefs(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if _, err := run(t, dir, "tx", "edit", "T-login", "--then", ""); err == nil {
		t.Fatalf("expected error clearing --then to empty")
	}
	if _, err := run(t, dir, "tx", "edit", "T-login", "--action", "act.missing"); err == nil {
		t.Fatalf("expected error for dangling action reference")
	}
	if _, err := run(t, dir, "tx", "edit", "T-login", "--tags", "tag.missing"); err == nil {
		t.Fatalf("expected error for dangling tag reference")
	}

	if _, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint to remain green after rejected tx edits")
	}
}

func TestCLI_TxTagAddRmAndSet(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "tag", "create", "concern.perf", "--name", "perf", "--kind", "concern")

	mustRun(t, dir, "tx", "tag", "T-login", "--add", "concern.perf")
	out := mustRun(t, dir, "show", "tx", "T-login")
	if !strings.Contains(out, "concern.perf") {
		t.Fatalf("expected --add to attach concern.perf:\n%s", out)
	}

	mustRun(t, dir, "tx", "tag", "T-login", "--rm", "concern.perf")
	out = mustRun(t, dir, "show", "tx", "T-login")
	if strings.Contains(out, "concern.perf") {
		t.Fatalf("expected --rm to detach concern.perf:\n%s", out)
	}

	mustRun(t, dir, "tx", "tag", "T-login", "--set", "concern.perf")
	out = mustRun(t, dir, "show", "tx", "T-login", "--json")
	if !strings.Contains(out, "concern.perf") || strings.Contains(out, "req.auth") {
		t.Fatalf("expected --set to fully replace tags with just concern.perf:\n%s", out)
	}
}

func TestCLI_TxTagRejectsMissingTagAndConflictingFlags(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if _, err := run(t, dir, "tx", "tag", "T-login", "--add", "tag.missing"); err == nil {
		t.Fatalf("expected error for missing tag on --add")
	}
	if _, err := run(t, dir, "tx", "tag", "T-login", "--set", "tag.missing"); err == nil {
		t.Fatalf("expected error for missing tag on --set")
	}
	if _, err := run(t, dir, "tx", "tag", "T-login", "--add", "req.auth", "--set", "req.auth"); err == nil {
		t.Fatalf("expected error combining --add and --set")
	}
	if _, err := run(t, dir, "tx", "tag", "T-login"); err == nil {
		t.Fatalf("expected error when neither --add/--rm nor --set is given")
	}
}

func TestCLI_TxRmRequiresWhyAndForce(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if _, err := run(t, dir, "tx", "rm", "T-login"); err == nil {
		t.Fatalf("expected error without --why/--force")
	}
	if _, err := run(t, dir, "tx", "rm", "T-login", "--why", "cleanup"); err == nil {
		t.Fatalf("expected error without --force")
	}
	if _, err := run(t, dir, "tx", "rm", "T-login", "--force"); err == nil {
		t.Fatalf("expected error without --why")
	}
}

func TestCLI_TxRmCascadesTargetingDecisionsAndStaysLintGreen(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "transition:T-login", "--why", "初期実装")
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "共通規則")

	out := mustRun(t, dir, "tx", "rm", "T-login", "--why", "not needed anymore", "--force", "--json")
	if !strings.Contains(out, "removedDecisions") {
		t.Fatalf("expected removedDecisions in tx rm --json output:\n%s", out)
	}

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green after tx rm cascade, got:\n%s", out)
	}

	rulesOut := mustRun(t, dir, "rules", "--tag", "subject.auth")
	if !strings.Contains(rulesOut, "共通規則") {
		t.Fatalf("expected the tag-targeted decision to survive tx rm (only tx-targeted decisions cascade):\n%s", rulesOut)
	}
}
