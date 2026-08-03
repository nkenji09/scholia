package cli

import (
	"strings"
	"testing"
)

func TestCLI_VocabRenameThenLintGreen(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "transition:T-login", "--why", "# テスト用の見出し\n\n初期実装")

	mustRun(t, dir, "vocab", "rename", "act.user.submit-login", "--to", "act.user.login-submit")

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green after vocab rename, got error. output:\n%s", out)
	}

	out := mustRun(t, dir, "show", "tx", "T-login")
	if !strings.Contains(out, "act.user.login-submit") {
		t.Fatalf("expected renamed vocab id reflected in transition, got:\n%s", out)
	}
}

func TestCLI_TxRenameThenLintGreenAndDecisionFollowsTarget(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "transition:T-login", "--why", "# テスト用の見出し\n\n初期実装")

	mustRun(t, dir, "tx", "rename", "T-login", "--to", "T-login-submit")

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green after tx rename, got error. output:\n%s", out)
	}

	out := mustRun(t, dir, "rules", "--tx", "T-login-submit")
	if !strings.Contains(out, "初期実装") {
		t.Fatalf("expected decision to follow renamed transition via rules --tx, got:\n%s", out)
	}
}
