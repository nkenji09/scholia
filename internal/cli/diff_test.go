package cli

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func gitInitT(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func gitCommitAllT(t *testing.T, dir, msg string) {
	t.Helper()
	for _, args := range [][]string{{"add", "-A"}, {"commit", "-q", "-m", msg}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestDiff_NoChanges(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	out, err := run(t, dir, "diff")
	if err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if !strings.Contains(out, "差分なし") {
		t.Fatalf("expected 差分なし, got:\n%s", out)
	}
}

func TestDiff_VocabAddDetected(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	if out, err := run(t, dir, "vocab", "add", "condition", "cond.new", "--label", "新しい条件"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff", "--json")
	if err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if !strings.Contains(out, "cond.new") {
		t.Fatalf("expected diff to report added vocab cond.new:\n%s", out)
	}
}

func TestDiff_ThenReorderDetected(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	if out, err := run(t, dir, "vocab", "add", "effect", "eff.second", "--label", "2 つめの効果"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tx", "add", "T-reorder", "--action", "act.submit", "--then", "eff.token,eff.second"); err != nil {
		t.Fatalf("tx add: %v\n%s", err, out)
	}
	gitCommitAllT(t, dir, "seed")

	txPath := filepath.Join(dir, ".scholia", "transitions", "T-reorder.json")
	writeRawJSON(t, txPath, `{"id":"T-reorder","action":"act.submit","given":[],"then":["eff.second","eff.token"]}`)

	out, err := run(t, dir, "diff")
	if err != nil {
		t.Fatalf("diff: %v\n%s", err, out)
	}
	if !strings.Contains(out, "並び替え") {
		t.Fatalf("expected then-reorder to be reported:\n%s", out)
	}
}

func TestDiff_DecisionRemovalIsErrorExit(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	decPath := filepath.Join(dir, ".scholia", "decisions", "d1.json")
	writeRawJSON(t, decPath, `{"id":"d1","target":{"type":"transition","id":"T-happy"},"why":"why","at":"2026-01-01T00:00:00Z"}`)
	gitCommitAllT(t, dir, "seed")

	if err := os.Remove(decPath); err != nil {
		t.Fatal(err)
	}

	out, err := run(t, dir, "diff")
	if err == nil {
		t.Fatalf("expected diff to fail (exit non-zero) on decision removal, output:\n%s", out)
	}
	if !strings.Contains(out, "append-only 違反") {
		t.Fatalf("expected append-only violation message, got:\n%s", out)
	}
}

// gap G8: scholia diff はベースライン（.scholia）が無い初回でも詰まらない。

func TestDiff_NoCommitsIsGracefulOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)

	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("scholia init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff")
	if err != nil {
		t.Fatalf("expected diff to succeed (exit 0) on first run with zero commits: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ベースライン") {
		t.Fatalf("expected a baseline-missing notice, got:\n%s", out)
	}
	if !strings.Contains(out, "cond.a") {
		t.Fatalf("expected current records to show up as added, got:\n%s", out)
	}
}

func TestDiff_ScholiaUncommittedIsGracefulOnFirstRun(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)

	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("scholia init: %v\n%s", err, out)
	}
	// .scholia は git add せず、README だけ commit して HEAD を作る。
	writeRawJSON(t, filepath.Join(dir, "README.md"), "hello")
	cmd := exec.Command("git", "add", "README.md")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add README.md: %v\n%s", err, out)
	}
	cmd = exec.Command("git", "commit", "-q", "-m", "no scholia yet")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff")
	if err != nil {
		t.Fatalf("expected diff to succeed (exit 0) when .scholia/ isn't committed yet: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ベースライン") {
		t.Fatalf("expected a baseline-missing notice, got:\n%s", out)
	}
	if !strings.Contains(out, "cond.a") {
		t.Fatalf("expected current records to show up as added, got:\n%s", out)
	}
}

func TestDiff_ExplicitInvalidRefStillErrors(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	out, err := run(t, dir, "diff", "no-such-ref")
	if err == nil {
		t.Fatalf("expected diff --ref no-such-ref to fail (exit non-zero), output:\n%s", out)
	}
}

// R-2: `scholia diff A B`（ref 対 ref・2引数）。

func TestDiff_TwoRefsShowsCommitDiff(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	if out, err := run(t, dir, "vocab", "add", "condition", "cond.new", "--label", "新しい条件"); err != nil {
		t.Fatalf("vocab add: %v\n%s", out, err)
	}
	gitCommitAllT(t, dir, "add cond.new")

	out, err := run(t, dir, "diff", "HEAD^", "HEAD")
	if err != nil {
		t.Fatalf("diff HEAD^ HEAD: %v\n%s", err, out)
	}
	if !strings.Contains(out, "cond.new") {
		t.Fatalf("expected diff to report cond.new added in the last commit:\n%s", out)
	}
	if !strings.Contains(out, "diff: HEAD^ vs HEAD") {
		t.Fatalf("expected header to show both refs:\n%s", out)
	}
}

func TestDiff_TwoRefsJSONValidAndSameSchema(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	if out, err := run(t, dir, "vocab", "add", "condition", "cond.new", "--label", "新しい条件"); err != nil {
		t.Fatalf("vocab add: %v\n%s", out, err)
	}
	gitCommitAllT(t, dir, "add cond.new")

	out, err := run(t, dir, "diff", "HEAD^", "HEAD", "--json")
	if err != nil {
		t.Fatalf("diff HEAD^ HEAD --json: %v\n%s", err, out)
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error %v:\n%s", err, out)
	}
	for _, key := range []string{"ref", "afterRef", "vocab", "tags", "transitions", "decisions"} {
		if _, ok := parsed[key]; !ok {
			t.Fatalf("expected JSON key %q (same schema as 0/1-arg path), got: %v", key, parsed)
		}
	}
}

func TestDiff_TwoRefsNoChanges(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")
	cmd := exec.Command("git", "commit", "-q", "--allow-empty", "-m", "empty")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit --allow-empty: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff", "HEAD^", "HEAD")
	if err != nil {
		t.Fatalf("diff HEAD^ HEAD: %v\n%s", err, out)
	}
	if !strings.Contains(out, "差分なし") {
		t.Fatalf("expected 差分なし, got:\n%s", out)
	}
}

func TestDiff_TwoRefsUnknownRefIsError(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	out, err := run(t, dir, "diff", "HEAD", "no-such-ref")
	if err == nil {
		t.Fatalf("expected diff HEAD no-such-ref to fail (exit non-zero), output:\n%s", out)
	}
}

func TestDiff_ThreeArgsRejected(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	out, err := run(t, dir, "diff", "HEAD", "HEAD", "HEAD")
	if err == nil {
		t.Fatalf("expected diff with 3 args to fail (exit non-zero), output:\n%s", out)
	}
}

func TestDiff_DefaultsToHEADAndAcceptsExplicitRef(t *testing.T) {
	dir := t.TempDir()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	gitCommitAllT(t, dir, "seed")

	out, err := run(t, dir, "diff", "HEAD")
	if err != nil {
		t.Fatalf("diff HEAD: %v\n%s", err, out)
	}
	if !strings.Contains(out, "差分なし") {
		t.Fatalf("expected 差分なし for explicit HEAD ref:\n%s", out)
	}
}

// --- #45 U4: diff --check（CI ゲート）と正規化・逃し弁 ---

func seedDiffCheckFixture(t *testing.T, dir string) string {
	t.Helper()
	gitInitT(t, dir)
	seedListFixture(t, dir)
	decPath := filepath.Join(dir, ".scholia", "decisions", "d1.json")
	writeRawJSON(t, decPath, `{"id":"d1","target":{"type":"transition","id":"T-happy"},"why":"why 本文","at":"2026-01-01T00:00:00Z","commits":["aaa111"]}`)
	gitCommitAllT(t, dir, "seed")
	return decPath
}

func TestDiffCheck_NoChangesIsOK(t *testing.T) {
	dir := t.TempDir()
	seedDiffCheckFixture(t, dir)

	out, err := run(t, dir, "diff", "--check", "HEAD")
	if err != nil {
		t.Fatalf("diff --check: %v\n%s", err, out)
	}
	if !strings.Contains(out, "append-only OK") {
		t.Fatalf("expected append-only OK line:\n%s", out)
	}
}

func TestDiffCheck_CommitsAppendIsGreen(t *testing.T) {
	dir := t.TempDir()
	seedDiffCheckFixture(t, dir)

	// decision add-commit（正規操作）で commits を追記 → --check は緑のまま。
	if out, err := run(t, dir, "decision", "add-commit", "d1", "bbb222"); err != nil {
		t.Fatalf("decision add-commit: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff", "--check", "HEAD")
	if err != nil {
		t.Fatalf("commits 追記だけの diff --check が失敗（正規操作の偽陽性）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "changed 1〔許容欄位のみ〕") {
		t.Fatalf("expected allowed-changed count:\n%s", out)
	}
}

func TestDiffCheck_TxRenameRepointIsGreen(t *testing.T) {
	dir := t.TempDir()
	seedDiffCheckFixture(t, dir)

	// tx rename（正規操作・decision の target.id 追随）→ --check は緑のまま。
	if out, err := run(t, dir, "tx", "rename", "T-happy", "--to", "T-happy-renamed", "--no-refs"); err != nil {
		t.Fatalf("tx rename: %v\n%s", err, out)
	}

	out, err := run(t, dir, "diff", "--check", "HEAD")
	if err != nil {
		t.Fatalf("tx rename 追随だけの diff --check が失敗（正規操作の偽陽性）: %v\n%s", err, out)
	}
}

func TestDiffCheck_JudgmentFieldChangeIsRed(t *testing.T) {
	dir := t.TempDir()
	decPath := seedDiffCheckFixture(t, dir)

	writeRawJSON(t, decPath, `{"id":"d1","target":{"type":"transition","id":"T-happy"},"why":"書き換えた","at":"2026-01-01T00:00:00Z","commits":["aaa111"]}`)

	out, err := run(t, dir, "diff", "--check", "HEAD")
	if err == nil {
		t.Fatalf("why 改変の diff --check が exit 0 になった:\n%s", out)
	}
	if !strings.Contains(out, "[violation] decision d1: 判断欄位 why が改変されています") {
		t.Fatalf("expected violation line with field name:\n%s", out)
	}
}

func TestDiffCheck_RetrofitValveDowngradesWithReason(t *testing.T) {
	dir := t.TempDir()
	decPath := seedDiffCheckFixture(t, dir)
	writeRawJSON(t, decPath, `{"id":"d1","target":{"type":"transition","id":"T-happy"},"why":"書き換えた","at":"2026-01-01T00:00:00Z","commits":["aaa111"]}`)

	// flag 経由（理由必須・出力に記録）。
	out, err := run(t, dir, "diff", "--check", "HEAD", "--allow-decision-retrofit", "全店 retrofit の例外承認")
	if err != nil {
		t.Fatalf("逃し弁つき diff --check が失敗: %v\n%s", err, out)
	}
	if !strings.Contains(out, "警告へ降格") || !strings.Contains(out, "全店 retrofit の例外承認") {
		t.Fatalf("expected downgrade note with the reason recorded:\n%s", out)
	}

	// env 経由（値＝理由）。
	t.Setenv("SCHOLIA_ALLOW_DECISION_RETROFIT", "PR label decision-retrofit-approved")
	out, err = run(t, dir, "diff", "--check", "HEAD")
	if err != nil {
		t.Fatalf("env 逃し弁つき diff --check が失敗: %v\n%s", err, out)
	}
	if !strings.Contains(out, "PR label decision-retrofit-approved") {
		t.Fatalf("expected env reason recorded:\n%s", out)
	}

	// --json にも記録される。
	var parsed map[string]any
	out, err = run(t, dir, "diff", "--check", "HEAD", "--json")
	if err != nil {
		t.Fatalf("env 逃し弁つき diff --check --json が失敗: %v\n%s", err, out)
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if parsed["retrofitAllowed"] != true || parsed["retrofitReason"] != "PR label decision-retrofit-approved" {
		t.Fatalf("expected retrofitAllowed/retrofitReason in JSON: %v", parsed)
	}
}

func TestDiffCheck_RetrofitFlagRequiresReason(t *testing.T) {
	dir := t.TempDir()
	seedDiffCheckFixture(t, dir)

	out, err := run(t, dir, "diff", "--check", "HEAD", "--allow-decision-retrofit", "")
	if err == nil {
		t.Fatalf("理由なしの --allow-decision-retrofit が通った:\n%s", out)
	}
	if !strings.Contains(out, "理由が必須") {
		t.Fatalf("expected reason-required error:\n%s", out)
	}
}

func TestDiff_DefaultModeAlsoNormalizes(t *testing.T) {
	dir := t.TempDir()
	seedDiffCheckFixture(t, dir)

	// 既定 diff にも正規化が適用される: commits 追記は exit 0（従来は撃墜していた）。
	if out, err := run(t, dir, "decision", "add-commit", "d1", "bbb222"); err != nil {
		t.Fatalf("decision add-commit: %v\n%s", err, out)
	}
	out, err := run(t, dir, "diff")
	if err != nil {
		t.Fatalf("既定 diff が commits 追記で失敗（正規操作の偽陽性）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "許容欄位のみ") {
		t.Fatalf("expected allowed-change marker in default report:\n%s", out)
	}
}
