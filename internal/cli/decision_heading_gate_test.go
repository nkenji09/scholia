package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// decision を新しく作る面それぞれについて、見出しの無い why が保存されないことを
// 押さえる（01KZ06SYR3APGF3JD4NQRFTEEN）。
//
// ⚠️ **これは「面を列挙する歯止め」ではない。** 歯止めそのものは
// store.TestNoThirdDecisionWritePort（decision を書ける口が 2 つしかないこと）に
// あり、ここは各面が実際にその口を通っていることの振る舞い検査である。
// 4 面目を足した人がここに 1 行足し忘れても、口を通る限り拒否は効く。

func decisionCount(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(dir, ".scholia", "decisions"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatalf("read decisions dir: %v", err)
	}
	return len(entries)
}

// 面1: scholia decide。
func TestCLI_DecideRejectsMissingHeading(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	before := decisionCount(t, dir)

	out, err := run(t, dir, "decide", "--on", "tag:subject.auth", "--why", "見出しの無い why\n\n本文")
	if err == nil {
		t.Fatalf("見出しの無い why は保存されないはず:\n%s", out)
	}
	if !strings.Contains(out, "見出し") {
		t.Fatalf("何を直せばよいかが出力にあるべき:\n%s", out)
	}
	if got := decisionCount(t, dir); got != before {
		t.Fatalf("拒んだのに decision が増えた: %d → %d", before, got)
	}
}

// 上限（80 字）まで含めて拒否規則である——「1 行目が # で始まる」だけだと、
// `# ` に続けて長文を 1 行で書けば素通りする。
func TestCLI_DecideRejectsOverlongHeading(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	before := decisionCount(t, dir)

	out, err := run(t, dir, "decide", "--on", "tag:subject.auth",
		"--why", "# "+strings.Repeat("長", 431)+"\n\n本文")
	if err == nil {
		t.Fatalf("81 字以上の見出しは保存されないはず:\n%s", out)
	}
	if got := decisionCount(t, dir); got != before {
		t.Fatalf("拒んだのに decision が増えた: %d → %d", before, got)
	}
}

// 逃し弁 --allow/--reason（変更4）。理由が必須で、使用は stdout と --json に残る。
func TestCLI_DecideAllowEscapeRecordsReason(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)

	if out, err := run(t, dir, "decide", "--on", "tag:subject.auth", "--why", "見出し無し\n\n本文",
		"--allow", "decision-heading"); err == nil {
		t.Fatalf("--allow に --reason が無いなら拒むべき:\n%s", out)
	}

	out := mustRun(t, dir, "decide", "--on", "tag:subject.auth", "--why", "見出し無し\n\n本文",
		"--allow", "decision-heading", "--reason", "移行中の一件だけ例外にする", "--json")
	var env struct {
		Record  map[string]any `json:"record"`
		Allowed []struct {
			Rule   string `json:"rule"`
			Reason string `json:"reason"`
		} `json:"allowed"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(env.Allowed) != 1 || env.Allowed[0].Rule != "decision-heading" {
		t.Fatalf("--allow の使用が記録されるべき: %+v", env.Allowed)
	}
	if env.Allowed[0].Reason != "移行中の一件だけ例外にする" {
		t.Fatalf("理由が記録されるべき: %+v", env.Allowed)
	}
}

// 面2: review adopt（review 本文がそのまま why になる経路）。
func TestCLI_ReviewAdoptRejectsMissingHeading(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixtureWithBody(t, dir, "見出しの無い提案本文")
	before := decisionCount(t, dir)

	out, err := run(t, dir, "review", "adopt", id)
	if err == nil {
		t.Fatalf("見出しの無い review 本文の昇格は止まるはず:\n%s", out)
	}
	if got := decisionCount(t, dir); got != before {
		t.Fatalf("拒んだのに decision が増えた: %d → %d", before, got)
	}
	// review は消えていない——書いた本文を失わない（昇格が先・掃除が後）。
	if out := mustRun(t, dir, "review", "list"); !strings.Contains(out, id) {
		t.Fatalf("昇格に失敗したら review は残るべき:\n%s", out)
	}
	// --why で見出しを付ければ通る（逃し弁ではなく why の側を直す）。
	mustRun(t, dir, "review", "adopt", id, "--why", "# 見出しを付けて昇格する\n\n見出しの無い提案本文")
	if got := decisionCount(t, dir); got != before+1 {
		t.Fatalf("--why で見出しを付けたら通るべき: %d → %d", before, got)
	}
}

// 却下は「却下: 」を素で前置きしない——前置きすると 1 行目が見出しでなくなり、
// 見出しのある提案を却下しただけでゲートに落ちる。
func TestCLI_ReviewRejectKeepsHeadingFirst(t *testing.T) {
	dir := t.TempDir()
	id := setupReviewFixtureWithBody(t, dir, "# A ではなく B とする\n\n提案の本文")

	out := mustRun(t, dir, "review", "reject", id, "--json")
	var d struct {
		Why string `json:"why"`
	}
	if err := json.Unmarshal([]byte(out), &d); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	first, _, _ := strings.Cut(d.Why, "\n")
	if !strings.HasPrefix(first, "# ") {
		t.Fatalf("却下の why の 1 行目が見出しでない: %q", first)
	}
	if !strings.Contains(first, "却下") || !strings.Contains(first, "A ではなく B とする") {
		t.Fatalf("却下であることと元の見出しの両方が 1 行目に残るべき: %q", first)
	}
}

func setupReviewFixtureWithBody(t *testing.T, dir, body string) string {
	t.Helper()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "a")
	mustRun(t, dir, "tx", "add", "T-1", "--action", "act.a", "--then", "eff.a", "--tags", "subject.auth")
	out := mustRun(t, dir, "review", "add", "--on", "transition:T-1", "--body", body, "--json")
	var added struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(out), &added); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	return added.ID
}
