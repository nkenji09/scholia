package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/product-memory/internal/store"
)

// run は cobra コマンドをテスト用に直接実行するヘルパ（バイナリを介さない統合スモーク）。
func run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := newRootCmd()
	cmd.SetArgs(append([]string{"--dir", dir}, args...))
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	err := cmd.Execute()
	return out.String(), err
}

func TestCLISmoke_InitVocabTagTxLintShow(t *testing.T) {
	dir := t.TempDir()

	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init failed: %v\noutput:\n%s", err, out)
	}
	// 冪等性: 2 回目の init も成功すること。
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("second init failed: %v\noutput:\n%s", err, out)
	}

	if out, err := run(t, dir, "vocab", "add", "condition", "cond.credentials-valid", "--label", "資格情報が正当"); err != nil {
		t.Fatalf("vocab add condition failed: %v\noutput:\n%s", err, out)
	}
	if out, err := run(t, dir, "vocab", "add", "action", "act.user.submit-login", "--label", "ログイン送信", "--kind", "user"); err != nil {
		t.Fatalf("vocab add action failed: %v\noutput:\n%s", err, out)
	}
	if out, err := run(t, dir, "vocab", "add", "effect", "eff.session.issue-token", "--label", "セッショントークン発行", "--kind", "state", "--owner", "server"); err != nil {
		t.Fatalf("vocab add effect failed: %v\noutput:\n%s", err, out)
	}

	if out, err := run(t, dir, "tag", "create", "subject.auth", "--name", "認証", "--kind", "subject"); err != nil {
		t.Fatalf("tag create parent failed: %v\noutput:\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "req.auth-happy-path", "--name", "正常系ログイン", "--kind", "requirement", "--parent", "subject.auth"); err != nil {
		t.Fatalf("tag create child failed: %v\noutput:\n%s", err, out)
	}

	if out, err := run(t, dir, "tx", "add", "T-login-submit-valid",
		"--action", "act.user.submit-login",
		"--given", "cond.credentials-valid",
		"--then", "eff.session.issue-token",
		"--tags", "req.auth-happy-path,subject.auth",
	); err != nil {
		t.Fatalf("tx add failed: %v\noutput:\n%s", err, out)
	}

	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("expected lint to be green, got error: %v\noutput:\n%s", err, out)
	}

	out, err := run(t, dir, "show", "tx", "T-login-submit-valid", "--resolve")
	if err != nil {
		t.Fatalf("show tx failed: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{"T-login-submit-valid", "ログイン送信", "資格情報が正当", "セッショントークン発行"} {
		if !strings.Contains(out, want) {
			t.Fatalf("show tx --resolve output missing %q:\n%s", want, out)
		}
	}
}

func TestCLI_InitNoGitignoreSkipsGitignoreWrite(t *testing.T) {
	dir := t.TempDir()

	if out, err := run(t, dir, "init", "--no-gitignore"); err != nil {
		t.Fatalf("init --no-gitignore failed: %v\noutput:\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(dir, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf(".gitignore should not be created with --no-gitignore, stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".pmem")); err != nil {
		t.Fatalf(".pmem should still be created with --no-gitignore: %v", err)
	}
}

func TestCLI_InitWithoutFlagWritesGitignore(t *testing.T) {
	dir := t.TempDir()

	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init failed: %v\noutput:\n%s", err, out)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("read .gitignore: %v", err)
	}
	if !strings.Contains(string(data), ".pmem/index.db") {
		t.Fatalf(".gitignore should contain .pmem/index.db by default, got %q", string(data))
	}
}

func TestCLI_VocabAddRejectsDuplicateAndInvalidKindAndOwner(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "condition", "cond.a", "--label", "a"); err != nil {
		t.Fatalf("first vocab add: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "condition", "cond.a", "--label", "dup"); err == nil {
		t.Fatalf("expected error for duplicate vocab id")
	}
	if _, err := run(t, dir, "vocab", "add", "condition", "cond.b", "--label", "b", "--kind", "not-declared"); err == nil {
		t.Fatalf("expected error for undeclared kind")
	}
	if _, err := run(t, dir, "vocab", "add", "condition", "cond.c", "--label", "c", "--owner", "server"); err == nil {
		t.Fatalf("expected error for --owner on non-effect category")
	}
}

func TestCLI_TxAddRejectsEmptyThenAndDanglingRefs(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "vocab", "add", "action", "act.a", "--label", "a"); err != nil {
		t.Fatalf("vocab add: %v", err)
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.a"); err == nil {
		t.Fatalf("expected error for missing --then")
	}
	if _, err := run(t, dir, "tx", "add", "T-1", "--action", "act.missing", "--then", "eff.a"); err == nil {
		t.Fatalf("expected error for dangling action reference")
	}
}

func TestCLI_TagCreateRejectsMissingParentAndCycle(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v", err)
	}
	if _, err := run(t, dir, "tag", "create", "t1", "--name", "t1", "--kind", "concern", "--parent", "t.missing"); err == nil {
		t.Fatalf("expected error for missing parent")
	}
}

// --- 壊れた fixture: lint が error 相当（RunE がエラーを返す＝exit 1 相当）になること ---

func TestCLI_LintFailsOnDanglingVocabRef(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	writeRawJSON(t, filepath.Join(s.Dir, "transitions", "T-1.json"),
		`{"id":"T-1","action":"act.missing","given":[],"then":["eff.missing"]}`)

	if _, err := run(t, dir, "lint"); err == nil {
		t.Fatalf("expected lint to fail on dangling vocab-ref")
	}
}

func TestCLI_LintFailsOnCyclicTags(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	writeRawJSON(t, filepath.Join(s.Dir, "tags", "a.json"), `{"id":"a","name":"a","parentIds":["b"]}`)
	writeRawJSON(t, filepath.Join(s.Dir, "tags", "b.json"), `{"id":"b","name":"b","parentIds":["a"]}`)

	if _, err := run(t, dir, "lint"); err == nil {
		t.Fatalf("expected lint to fail on cyclic tags")
	}
}

func TestCLI_LintFailsOnIDFilenameMismatch(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	writeRawJSON(t, filepath.Join(s.Dir, "vocab", "cond.wrong-filename.json"),
		`{"id":"cond.actual-id","category":"condition","label":"x"}`)

	if _, err := run(t, dir, "lint"); err == nil {
		t.Fatalf("expected lint to fail on filename/id mismatch")
	}
}

func writeRawJSON(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatalf("write fixture %s: %v", path, err)
	}
}
