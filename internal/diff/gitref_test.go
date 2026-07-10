package diff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/nkenji09/product-memory/internal/store"
)

// gitTestRepo は t.TempDir() に git リポジトリ + `pmem init` 済みの .pmem/ を用意する。
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
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
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
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
	commitAll(t, dir, "seed")

	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "cond.b.json"), `{"id":"cond.b","category":"condition","label":"b"}`+"\n")

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
	txPath := filepath.Join(dir, ".pmem", "transitions", "T-1.json")
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "act.a.json"), `{"id":"act.a","category":"action","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "eff.a.json"), `{"id":"eff.a","category":"effect","label":"a"}`+"\n")
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "eff.b.json"), `{"id":"eff.b","category":"effect","label":"b"}`+"\n")
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
	decPath := filepath.Join(dir, ".pmem", "decisions", "d1.json")
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
	writeFile(t, filepath.Join(dir, ".pmem", "vocab", "cond.a.json"), `{"id":"cond.a","category":"condition","label":"a"}`+"\n")
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

func TestDiff_RefWithoutPmemDirIsClearError(t *testing.T) {
	dir, s := gitTestRepo(t)
	// .pmem/ をまだ何もコミットしていない状態で commit を作る（.pmem 自体を含めない）。
	writeFile(t, filepath.Join(dir, "README.md"), "hello\n")
	runGitT(t, dir, "add", "README.md")
	runGitT(t, dir, "commit", "-q", "-m", "no pmem yet")

	if _, err := Diff(s, "HEAD"); err == nil {
		t.Fatalf("expected error when ref has no .pmem/ path")
	}
}
