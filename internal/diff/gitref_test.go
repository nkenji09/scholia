package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

// gitTestRepo は t.TempDir() に git リポジトリ + `scholia init` 済みの .scholia/ を用意する。
func gitTestRepo(t *testing.T) (dir string, s *store.Store) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir = t.TempDir()
	runGitT(t, dir, "init", "-q")
	runGitT(t, dir, "config", "user.email", "test@example.com")
	runGitT(t, dir, "config", "user.name", "test")

	var err error
	s, err = store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	return dir, s
}

func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func commitAll(t *testing.T, dir, msg string) {
	t.Helper()
	runGitT(t, dir, "add", "-A")
	runGitT(t, dir, "commit", "-q", "-m", msg)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiff_NoChangesReportsEmpty(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	r, err := Diff(s, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !r.Empty() {
		t.Fatalf("expected no diff, got %+v", r)
	}
	if r.Ref != "HEAD" {
		t.Fatalf("Ref = %q, want default HEAD", r.Ref)
	}
}

func TestDiff_VocabAddedSinceRef(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.b.json"), `{"id":"cond.b","category":"condition","label":"b"}`+"\n")

	r, err := Diff(s, "HEAD")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.b" {
		t.Fatalf("Vocab.Added = %+v, want [cond.b]", r.Vocab.Added)
	}
}

func TestDiff_ThenReorderAcrossCommit(t *testing.T) {
	dir, s := gitTestRepo(t)
	txPath := filepath.Join(dir, ".scholia", "transitions", "T-1.json")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "act.a.json"), `{"id":"act.a","category":"action","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "eff.a.json"), `{"id":"eff.a","category":"effect","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "eff.b.json"), `{"id":"eff.b","category":"effect","label":"b"}`+"\n")
	writeFile(t, txPath, `{"id":"T-1","action":"act.a","given":[],"then":["eff.a","eff.b"]}`+"\n")
	commitAll(t, dir, "seed")

	writeFile(t, txPath, `{"id":"T-1","action":"act.a","given":[],"then":["eff.b","eff.a"]}`+"\n")

	r, err := Diff(s, "HEAD")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(r.Transitions.Changed) != 1 {
		t.Fatalf("Transitions.Changed = %+v, want 1 entry", r.Transitions.Changed)
	}
	c := r.Transitions.Changed[0]
	if !c.ThenReordered {
		t.Fatalf("ThenReordered = false, want true for then-only reorder")
	}
}

func TestDiff_DecisionRemovalIsFlaggedAsViolation(t *testing.T) {
	dir, s := gitTestRepo(t)
	decPath := filepath.Join(dir, ".scholia", "decisions", "d1.json")
	writeFile(t, decPath, `{"id":"d1","target":{"type":"transition","id":"T-1"},"why":"why","at":"2026-01-01T00:00:00Z"}`+"\n")
	commitAll(t, dir, "seed")

	if err := os.Remove(decPath); err != nil {
		t.Fatal(err)
	}

	r, err := Diff(s, "HEAD")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(r.Decisions.Removed) != 1 {
		t.Fatalf("Decisions.Removed = %+v, want 1", r.Decisions.Removed)
	}
	if !r.DecisionViolation() {
		t.Fatalf("DecisionViolation() = false, want true after a decision file was removed")
	}
}

func TestDiff_UnknownRefIsClearError(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	if _, err := Diff(s, "does-not-exist"); err == nil {
		t.Fatalf("expected error for unresolvable gitref")
	}
}

func TestDiff_NotAGitRepoIsClearError(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	if _, err := Diff(s, "HEAD"); err == nil {
		t.Fatalf("expected error when project root is not a git repository")
	}
}

func TestDiff_RefWithoutScholiaDirIsClearError(t *testing.T) {
	dir, s := gitTestRepo(t)
	// .scholia/ をまだ何もコミットしていない状態で commit を作る（.scholia 自体を含めない）。
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	runGitT(t, dir, "add", "README.md")
	runGitT(t, dir, "commit", "-q", "-m", "no scholia yet")

	if _, err := Diff(s, "HEAD"); err == nil {
		t.Fatalf("expected error when ref has no .scholia/ path")
	}
}

// gap G8: ベースライン（HEAD のコミット or ref 上の .scholia）が単に存在しない初回は、
// gitref を明示指定していない既定呼び出し（ref==""）に限り graceful に空ベースライン
// へフォールバックする（§4 scholia diff・DESIGN 矛盾なし）。

func TestDiff_NoCommitsFallsBackToEmptyBaselineOnDefaultRef(t *testing.T) {
	dir, s := gitTestRepo(t)
	// git init 直後・commit 0（HEAD が解決できない）で作業ツリーにレコードがある状態。
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")

	r, err := Diff(s, "")
	if err != nil {
		t.Fatalf("Diff: %v, want graceful fallback (no error) when HEAD has no commits yet", err)
	}
	if !r.BaselineMissing {
		t.Fatalf("BaselineMissing = false, want true when HEAD has no commits")
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.a" {
		t.Fatalf("Vocab.Added = %+v, want [cond.a] (no baseline -> everything is added)", r.Vocab.Added)
	}
	if len(r.Vocab.Removed) != 0 || len(r.Vocab.Changed) != 0 {
		t.Fatalf("expected no removed/changed against an empty baseline, got %+v", r.Vocab)
	}
}

func TestDiff_ScholiaNotYetCommittedFallsBackToEmptyBaselineOnDefaultRef(t *testing.T) {
	dir, s := gitTestRepo(t)
	// HEAD は存在するが .scholia/ をまだ git に含めていない（commit 済みなのは README のみ）。
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	runGitT(t, dir, "add", "README.md")
	runGitT(t, dir, "commit", "-q", "-m", "no scholia yet")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")

	r, err := Diff(s, "")
	if err != nil {
		t.Fatalf("Diff: %v, want graceful fallback (no error) when ref has no .scholia/ yet", err)
	}
	if !r.BaselineMissing {
		t.Fatalf("BaselineMissing = false, want true when ref has no .scholia/")
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.a" {
		t.Fatalf("Vocab.Added = %+v, want [cond.a] (no baseline -> everything is added)", r.Vocab.Added)
	}
}

func TestDiff_ExplicitRefStillErrorsWhenNoCommits(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")

	if _, err := Diff(s, "HEAD"); err == nil {
		t.Fatalf("expected error for explicit HEAD ref even though it matches the default value — user explicitly asked for it")
	}
}

// R-2: `scholia diff A B`（ref 対 ref）— タスク粒度=commit を成立させるコア。

func TestDiffRefs_VocabAddedBetweenTwoCommits(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.b.json"), `{"id":"cond.b","category":"condition","label":"b"}`+"\n")
	commitAll(t, dir, "add cond.b")

	r, err := DiffRefs(s, "HEAD^", "HEAD")
	if err != nil {
		t.Fatalf("DiffRefs: %v", err)
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.b" {
		t.Fatalf("Vocab.Added = %+v, want [cond.b]", r.Vocab.Added)
	}
	if r.Ref != "HEAD^" || r.AfterRef != "HEAD" {
		t.Fatalf("Ref/AfterRef = %q/%q, want HEAD^/HEAD", r.Ref, r.AfterRef)
	}
}

// このテストが受け入れ条件1「実データで `scholia diff <commit>^ <commit>` が
// 『その commit の変更』を出せる」の実証（複数フィールドにまたがる変更を1コミットに
// 含めても正しく検出できることまで確認する）。
func TestDiffRefs_SingleCommitChangeAcrossFields(t *testing.T) {
	dir, s := gitTestRepo(t)
	txPath := filepath.Join(dir, ".scholia", "transitions", "T-1.json")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "act.a.json"), `{"id":"act.a","category":"action","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "eff.a.json"), `{"id":"eff.a","category":"effect","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "eff.b.json"), `{"id":"eff.b","category":"effect","label":"b"}`+"\n")
	writeFile(t, txPath, `{"id":"T-1","action":"act.a","given":[],"then":["eff.a","eff.b"]}`+"\n")
	commitAll(t, dir, "seed")

	// 1コミットに: 語彙追加 + 遷移の then 変更（順序変更のみ）をまとめて含める。
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.new.json"), `{"id":"cond.new","category":"condition","label":"new"}`+"\n")
	writeFile(t, txPath, `{"id":"T-1","action":"act.a","given":[],"then":["eff.b","eff.a"]}`+"\n")
	commitAll(t, dir, "add cond.new + reorder T-1.then")

	r, err := DiffRefs(s, "HEAD^", "HEAD")
	if err != nil {
		t.Fatalf("DiffRefs: %v", err)
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.new" {
		t.Fatalf("Vocab.Added = %+v, want [cond.new]", r.Vocab.Added)
	}
	if len(r.Transitions.Changed) != 1 || !r.Transitions.Changed[0].ThenReordered {
		t.Fatalf("Transitions.Changed = %+v, want 1 entry with ThenReordered=true", r.Transitions.Changed)
	}
}

func TestDiffRefs_NoChangesReportsEmpty(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")
	runGitT(t, dir, "commit", "-q", "--allow-empty", "-m", "empty")

	r, err := DiffRefs(s, "HEAD^", "HEAD")
	if err != nil {
		t.Fatalf("DiffRefs: %v", err)
	}
	if !r.Empty() {
		t.Fatalf("expected no diff between identical snapshots, got %+v", r)
	}
}

func TestDiffRefs_UnknownBeforeRefIsError(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	if _, err := DiffRefs(s, "does-not-exist", "HEAD"); err == nil {
		t.Fatalf("expected error for unresolvable before-ref")
	}
}

func TestDiffRefs_UnknownAfterRefIsError(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	if _, err := DiffRefs(s, "HEAD", "does-not-exist"); err == nil {
		t.Fatalf("expected error for unresolvable after-ref")
	}
}

// 両側とも明示 ref なので、Diff の「既定 ref フォールバック」は適用されず、
// ベースライン欠落（.scholia を含まない ref）は常にエラーになる。
func TestDiffRefs_RefWithoutScholiaDirIsError(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	runGitT(t, dir, "add", "README.md")
	runGitT(t, dir, "commit", "-q", "-m", "no scholia yet")

	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "add scholia")

	if _, err := DiffRefs(s, "HEAD^", "HEAD"); err == nil {
		t.Fatalf("expected error when before-ref has no .scholia/ (no fallback for explicit ref-vs-ref)")
	}
}

func TestDiff_ValidBaselineOutputUnchangedByFallback(t *testing.T) {
	dir, s := gitTestRepo(t)
	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".scholia", "vocab", "cond.b.json"), `{"id":"cond.b","category":"condition","label":"b"}`+"\n")

	r, err := Diff(s, "")
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if r.BaselineMissing {
		t.Fatalf("BaselineMissing = true, want false when a valid baseline exists (no regression)")
	}
	if len(r.Vocab.Added) != 1 || r.Vocab.Added[0].ID != "cond.b" {
		t.Fatalf("Vocab.Added = %+v, want [cond.b]", r.Vocab.Added)
	}
}
