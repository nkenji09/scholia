package cli

import (
	"strings"
	"testing"
)

// seedListFixture は list/spec テスト共通の題材（多重所属＋untagged の 1 遷移を含む）を作る。
func seedListFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(args ...string) {
		t.Helper()
		if out, err := run(t, dir, args...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", args, err, out)
		}
	}
	must("init")
	must("vocab", "add", "condition", "cond.valid", "--label", "資格情報が正当")
	must("vocab", "add", "action", "act.submit", "--label", "ログイン送信", "--kind", "user")
	must("vocab", "add", "action", "act.other", "--label", "その他アクション", "--kind", "system")
	must("vocab", "add", "effect", "eff.token", "--label", "トークン発行", "--kind", "state")

	must("tag", "create", "subject.auth", "--name", "認証", "--kind", "subject")
	must("tag", "create", "req.auth", "--name", "認証要件", "--kind", "requirement")
	must("tag", "create", "req.auth-happy", "--name", "正常系", "--kind", "requirement", "--parent", "req.auth")

	must("tx", "add", "T-happy", "--action", "act.submit", "--given", "cond.valid", "--then", "eff.token",
		"--tags", "req.auth-happy,subject.auth")
	must("tx", "add", "T-untagged", "--action", "act.other", "--then", "eff.token")
}

func TestList_FlatListsAllTransitions(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "list")
	if err != nil {
		t.Fatalf("list: %v\n%s", err, out)
	}
	for _, want := range []string{"T-happy", "T-untagged"} {
		if !strings.Contains(out, want) {
			t.Fatalf("list output missing %q:\n%s", want, out)
		}
	}
}

func TestList_TagFilterHitsDescendantViaAncestorExpansion(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	// req.auth 自身には T-happy はタグ付けされていない（req.auth-happy のみ）が、
	// 実効タグの祖先展開で req.auth もヒットするはず（§3.7）。
	out, err := run(t, dir, "list", "--tag", "req.auth")
	if err != nil {
		t.Fatalf("list --tag: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-happy") {
		t.Fatalf("expected T-happy to match --tag req.auth via ancestor expansion:\n%s", out)
	}
	if strings.Contains(out, "T-untagged") {
		t.Fatalf("T-untagged should not match --tag req.auth:\n%s", out)
	}
}

func TestList_KindFilterIsActionKind(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "list", "--kind", "system")
	if err != nil {
		t.Fatalf("list --kind: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-untagged") {
		t.Fatalf("expected T-untagged (act.other, kind=system):\n%s", out)
	}
	if strings.Contains(out, "T-happy") {
		t.Fatalf("T-happy (kind=user) should not match --kind system:\n%s", out)
	}
}

func TestList_FacetTreeGroupsMultiMembershipAndUntagged(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "list", "--facet", "requirement", "--json")
	if err != nil {
		t.Fatalf("list --facet: %v\n%s", err, out)
	}
	// req.auth の下にも req.auth-happy の下にも T-happy が現れる（多重所属可・§3.8）。
	if strings.Count(out, "T-happy") != 2 {
		t.Fatalf("expected T-happy to appear under both req.auth and req.auth-happy (multi-membership), got:\n%s", out)
	}
	if !strings.Contains(out, `"untagged"`) {
		t.Fatalf("expected untagged bucket for T-untagged (no requirement-kind tag):\n%s", out)
	}
	if !strings.Contains(out, "T-untagged") {
		t.Fatalf("expected T-untagged in output:\n%s", out)
	}
}

func TestList_FacetAndTagFiltersCombineWithAND(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	out, err := run(t, dir, "list", "--facet", "requirement", "--tag", "subject.auth")
	if err != nil {
		t.Fatalf("list --facet --tag: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-happy") {
		t.Fatalf("expected T-happy (has both req.auth-happy and subject.auth):\n%s", out)
	}
	if strings.Contains(out, "T-untagged") {
		t.Fatalf("T-untagged has no subject.auth tag, should be filtered out entirely:\n%s", out)
	}
}

func TestList_RejectsUndeclaredFacetAndKind(t *testing.T) {
	dir := t.TempDir()
	seedListFixture(t, dir)

	if _, err := run(t, dir, "list", "--facet", "not-declared"); err == nil {
		t.Fatalf("expected error for undeclared facet tagKind")
	}
	if _, err := run(t, dir, "list", "--kind", "not-declared"); err == nil {
		t.Fatalf("expected error for undeclared action kind")
	}
	if _, err := run(t, dir, "list", "--tag", "does.not.exist"); err == nil {
		t.Fatalf("expected error for unknown tag")
	}
}
