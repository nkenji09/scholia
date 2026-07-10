package cli

import (
	"strings"
	"testing"
)

// setupAuthFixture creates a store with a vocab-tagged action, a nested tag
// pair, and one transition — the shared fixture for decide/rules tests.
func setupAuthFixture(t *testing.T, dir string) {
	t.Helper()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "subject.auth", "--name", "認証")
	mustRun(t, dir, "tag", "create", "req.auth", "--name", "要件-auth", "--kind", "requirement", "--parent", "subject.auth")
	mustRun(t, dir, "vocab", "add", "action", "act.user.submit-login", "--label", "ログイン送信")
	mustRun(t, dir, "vocab", "add", "effect", "eff.session.issue-token", "--label", "トークン発行")
	mustRun(t, dir, "vocab", "tag", "act.user.submit-login", "--add", "subject.auth")
	mustRun(t, dir, "tx", "add", "T-login",
		"--action", "act.user.submit-login",
		"--then", "eff.session.issue-token",
		"--tags", "req.auth",
	)
}

func mustRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := run(t, dir, args...)
	if err != nil {
		t.Fatalf("run %v failed: %v\noutput:\n%s", args, err, out)
	}
	return out
}

func TestCLI_DecideThenRulesSurfacesTxAndTagDecisions(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	// T-login の実効タグは {req.auth, subject.auth}（vocab 経路＋祖先展開）。
	txDecide := mustRun(t, dir, "decide", "--on", "transition:T-login", "--why", "httpOnly cookie でトークン発行", "--json")
	if !strings.Contains(txDecide, `"transition"`) {
		t.Fatalf("expected transition target in decide output, got %s", txDecide)
	}

	tagDecide := mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "null と空文字は同一の未入力として扱う", "--json")
	if !strings.Contains(tagDecide, `"subject.auth"`) {
		t.Fatalf("expected tag target in decide output, got %s", tagDecide)
	}

	out := mustRun(t, dir, "rules", "--tx", "T-login")
	if !strings.Contains(out, "httpOnly cookie") {
		t.Fatalf("rules --tx should surface the decision made directly on T-login:\n%s", out)
	}
	if !strings.Contains(out, "null と空文字") {
		t.Fatalf("rules --tx should surface the decision on subject.auth (in T-login's effective tags via vocab tag):\n%s", out)
	}
}

func TestCLI_RulesTagSelectorIncludesAncestors(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "認証まわりの共通規則")

	out := mustRun(t, dir, "rules", "--tag", "req.auth")
	if !strings.Contains(out, "認証まわりの共通規則") {
		t.Fatalf("rules --tag req.auth should surface a decision on its ancestor subject.auth:\n%s", out)
	}
}

func TestCLI_RulesFacetSelector(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	mustRun(t, dir, "decide", "--on", "tag:req.auth", "--why", "要件 facet の規則")

	out := mustRun(t, dir, "rules", "--facet", "requirement")
	if !strings.Contains(out, "要件 facet の規則") {
		t.Fatalf("rules --facet requirement should surface decisions on requirement-kind tags:\n%s", out)
	}
}

func TestCLI_RulesRejectsMultipleSelectors(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	if _, err := run(t, dir, "rules", "--tag", "subject.auth", "--tx", "T-login"); err == nil {
		t.Fatalf("expected error when --tag and --tx are both given")
	}
}

func TestCLI_DecideRejectsMissingTarget(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	if _, err := run(t, dir, "decide", "--on", "transition:T-missing", "--why", "x"); err == nil {
		t.Fatalf("expected error for decide on a nonexistent transition")
	}
	if _, err := run(t, dir, "decide", "--on", "bogus:foo", "--why", "x"); err == nil {
		t.Fatalf("expected error for an unrecognized --on target type")
	}
}
