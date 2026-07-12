package cli

import (
	"strings"
	"testing"
)

// TestCLISmoke_Phase1FullFlow chains the acceptance-checklist scenario end to
// end: init → vocab add → vocab tag → tag create → tx add → decide (both a
// transition target and a tag target) → rules → lint → rename (both kinds)
// → lint. Each individual command already has focused tests elsewhere; this
// one guards the flow as a whole (item 9f of the handoff).
func TestCLISmoke_Phase1FullFlow(t *testing.T) {
	dir := t.TempDir()

	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "condition", "cond.credentials-valid", "--label", "資格情報が正当")
	mustRun(t, dir, "vocab", "add", "action", "act.user.submit-login", "--label", "ログイン送信", "--kind", "user")
	mustRun(t, dir, "vocab", "add", "effect", "eff.session.issue-token", "--label", "セッショントークン発行", "--kind", "state", "--owner", "server")
	mustRun(t, dir, "tag", "create", "subject.auth", "--name", "認証")
	mustRun(t, dir, "tag", "create", "req.auth-happy-path", "--name", "正常系ログイン", "--kind", "requirement", "--parent", "subject.auth")
	mustRun(t, dir, "vocab", "tag", "act.user.submit-login", "--add", "subject.auth")
	mustRun(t, dir, "tx", "add", "T-login-submit-valid",
		"--action", "act.user.submit-login",
		"--given", "cond.credentials-valid",
		"--then", "eff.session.issue-token",
		"--tags", "req.auth-happy-path",
	)

	txDecide := mustRun(t, dir, "decide", "--on", "transition:T-login-submit-valid", "--why", "トークンは httpOnly cookie で発行（XSS対策）", "--ref", "https://example.com/pr/42", "--json")
	if !strings.Contains(txDecide, `"id"`) {
		t.Fatalf("expected decide --json to emit the created record, got %s", txDecide)
	}
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "認証まわりの共通不変条件")

	rulesOut := mustRun(t, dir, "rules", "--tx", "T-login-submit-valid")
	if !strings.Contains(rulesOut, "httpOnly cookie") || !strings.Contains(rulesOut, "共通不変条件") {
		t.Fatalf("expected rules --tx to surface both the direct and the vocab-tag-derived decision, got:\n%s", rulesOut)
	}

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green before rename, got error. output:\n%s", out)
	}

	mustRun(t, dir, "vocab", "rename", "act.user.submit-login", "--to", "act.user.login-submit")
	mustRun(t, dir, "tx", "rename", "T-login-submit-valid", "--to", "T-login-submit")

	if _, err := run(t, dir, "lint"); err != nil {
		out, _ := run(t, dir, "lint")
		t.Fatalf("expected lint green after both renames, got error. output:\n%s", out)
	}

	rulesAfterRename := mustRun(t, dir, "rules", "--tx", "T-login-submit")
	if !strings.Contains(rulesAfterRename, "httpOnly cookie") {
		t.Fatalf("expected decision to still surface for the renamed transition, got:\n%s", rulesAfterRename)
	}
}
