package cli

import (
	"strings"
	"testing"
)

func TestCLI_ShowTag(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create parent: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "req.auth-happy-path",
		"--name", "正常系ログイン", "--kind", "requirement", "--parent", "subject.auth",
		"--desc", "ログインが成功する経路", "--color", "#ff0000", "--ref", "https://example.com/req/1",
	); err != nil {
		t.Fatalf("tag create child: %v", err)
	}

	out, err := run(t, dir, "show", "tag", "req.auth-happy-path")
	if err != nil {
		t.Fatalf("show tag failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"req.auth-happy-path", "正常系ログイン", "requirement", "subject.auth",
		"ログインが成功する経路", "#ff0000", "https://example.com/req/1",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("show tag output missing %q:\n%s", want, out)
		}
	}

	if out, err := run(t, dir, "show", "tag", "no-such-tag"); err == nil {
		t.Fatalf("expected error for unknown tag id, got output:\n%s", out)
	}
}

func TestCLI_ShowVocab(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.session.issue-token",
		"--label", "セッショントークン発行", "--kind", "state", "--owner", "server",
	); err != nil {
		t.Fatalf("vocab add: %v", err)
	}

	out, err := run(t, dir, "show", "vocab", "eff.session.issue-token")
	if err != nil {
		t.Fatalf("show vocab failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"eff.session.issue-token", "effect", "セッショントークン発行", "state", "server",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("show vocab output missing %q:\n%s", want, out)
		}
	}

	if out, err := run(t, dir, "show", "vocab", "no-such-vocab"); err == nil {
		t.Fatalf("expected error for unknown vocab id, got output:\n%s", out)
	}
}

// vocab を参照する transition が使用箇所（逆引き）として表示される（真の影響集合・§3.3）。
func TestCLI_ShowVocabUsage(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "condition", "cond.shared", "--label", "共有条件"); err != nil {
		t.Fatalf("vocab add condition: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "アクションA"); err != nil {
		t.Fatalf("vocab add action a: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "effect", "eff.a", "--label", "エフェクトA"); err != nil {
		t.Fatalf("vocab add effect: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.a", "--given", "cond.shared", "--then", "eff.a"); err != nil {
		t.Fatalf("tx add T-1: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.b", "--label", "アクションB"); err != nil {
		t.Fatalf("vocab add action b: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-2", "--action", "act.b", "--given", "cond.shared", "--then", "eff.a"); err != nil {
		t.Fatalf("tx add T-2: %v", err)
	}

	out, err := run(t, dir, "show", "vocab", "cond.shared")
	if err != nil {
		t.Fatalf("show vocab failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"usage (2 transitions)", "T-1 (given)", "T-2 (given)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show vocab output missing %q:\n%s", want, out)
		}
	}

	// vocab を参照する transition が無ければ usage は空。
	unusedOut, err := run(t, dir, "show", "vocab", "eff.a", "--json")
	if err != nil {
		t.Fatalf("show vocab --json failed: %v\noutput:\n%s", err, unusedOut)
	}
	if !strings.Contains(unusedOut, `"txId": "T-1"`) || !strings.Contains(unusedOut, `"txId": "T-2"`) {
		t.Fatalf("show vocab --json usage missing expected txIds:\n%s", unusedOut)
	}
}

func TestCLI_ShowDecision(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create: %v", err)
	}
	decideOut, err := run(t, dir, "decide", "--on", "tag:subject.auth", "--why", "認証方式を決定", "--json")
	if err != nil {
		t.Fatalf("decide: %v\noutput:\n%s", err, decideOut)
	}
	id := extractJSONID(t, decideOut)

	out, err := run(t, dir, "show", "decision", id)
	if err != nil {
		t.Fatalf("show decision failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{id, "tag:subject.auth", "認証方式を決定"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show decision output missing %q:\n%s", want, out)
		}
	}

	if out, err := run(t, dir, "show", "decision", "no-such-decision"); err == nil {
		t.Fatalf("expected error for unknown decision id, got output:\n%s", out)
	}
}

// extractJSONID は "id": "..." 行から id を素朴に取り出す（--json 出力のテスト用ヘルパ）。
func extractJSONID(t *testing.T, jsonOut string) string {
	t.Helper()
	for _, line := range strings.Split(jsonOut, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, `"id":`) {
			v := strings.TrimPrefix(line, `"id":`)
			v = strings.TrimSpace(v)
			v = strings.TrimSuffix(v, ",")
			v = strings.Trim(v, `"`)
			return v
		}
	}
	t.Fatalf("id field not found in JSON output:\n%s", jsonOut)
	return ""
}
