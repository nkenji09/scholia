package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeSourceFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func readSourceFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	return string(b)
}

func TestCLI_TagRenameDefaultDryRunLeavesSourceUnchanged(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth for the requirement this satisfies\npackage handler\n")

	out := mustRun(t, dir, "tag", "rename", "req.auth", "req.authn")

	if !strings.Contains(out, "handler.go:1") {
		t.Fatalf("expected dry-run to report handler.go:1, got:\n%s", out)
	}
	if !strings.Contains(out, "scholia refs rewrite req.auth req.authn --apply") {
		t.Fatalf("expected rewrite suggestion, got:\n%s", out)
	}
	got := readSourceFile(t, dir, "handler.go")
	want := "// see req.auth for the requirement this satisfies\npackage handler\n"
	if got != want {
		t.Fatalf("dry-run must not modify source, got %q", got)
	}
}

func TestCLI_TagRenameRewriteRefsAppliesBoundarySafeReplace(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go",
		"// see req.auth here, but req.auth-sibling and req.authbogus are unrelated\npackage handler\n")

	out := mustRun(t, dir, "tag", "rename", "req.auth", "req.authn", "--rewrite-refs")
	if !strings.Contains(out, "書き換えました") {
		t.Fatalf("expected rewrite confirmation, got:\n%s", out)
	}

	got := readSourceFile(t, dir, "handler.go")
	want := "// see req.authn here, but req.auth-sibling and req.authbogus are unrelated\npackage handler\n"
	if got != want {
		t.Fatalf("rewrite:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCLI_TagRenameNoRefsSkipsScanEntirely(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth\npackage handler\n")

	out := mustRun(t, dir, "tag", "rename", "req.auth", "req.authn", "--no-refs")
	if strings.Contains(out, "handler.go") {
		t.Fatalf("--no-refs must not scan source at all, got:\n%s", out)
	}

	got := readSourceFile(t, dir, "handler.go")
	if got != "// see req.auth\npackage handler\n" {
		t.Fatalf("--no-refs must leave source untouched, got %q", got)
	}
}

func TestCLI_TagRenameCascadeRewritesAllPlanPairsAndSparesSiblings(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "tag", "create", "req.foo", "--name", "foo", "--kind", "requirement")
	mustRun(t, dir, "tag", "create", "req.foo-child", "--name", "child", "--kind", "requirement", "--parent", "req.foo")
	mustRun(t, dir, "tag", "create", "req.foobar", "--name", "unrelated sibling", "--kind", "requirement")
	writeSourceFile(t, dir, "doc.md", "req.foo top, req.foo-child nested, req.foobar unrelated\n")

	mustRun(t, dir, "tag", "rename", "req.foo", "req.top", "--cascade", "--rewrite-refs")

	got := readSourceFile(t, dir, "doc.md")
	want := "req.top top, req.top-child nested, req.foobar unrelated\n"
	if got != want {
		t.Fatalf("cascade rewrite:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestCLI_VocabRenameRefsJSONOutputShape(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// act.user.submit-login\n")

	out := mustRun(t, dir, "vocab", "rename", "act.user.submit-login", "--to", "act.user.login-submit", "--json")
	for _, want := range []string{`"rename"`, `"refs"`, `"matches"`, "handler.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected JSON output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestCLI_TxRenameRewriteRefsAppliesReplace(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see T-login for the flow\n")

	mustRun(t, dir, "tx", "rename", "T-login", "--to", "T-login-submit", "--rewrite-refs")

	got := readSourceFile(t, dir, "handler.go")
	if got != "// see T-login-submit for the flow\n" {
		t.Fatalf("tx rename rewrite, got %q", got)
	}
}

// TestCLI_TagRenamePartialSourceFailureStaysCommittedAndReportsNonZero covers
// the handoff's non-atomic-failure acceptance criterion at the CLI layer
// (not just internal/refs): the `.scholia` rename must succeed and stick even
// when source rewriting can't write one of the files, the command must
// exit non-zero to signal that, and `scholia refs rewrite --apply` must be
// able to finish the job afterward (idempotently) once the obstruction is
// removed.
func TestCLI_TagRenamePartialSourceFailureStaysCommittedAndReportsNonZero(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "locked/handler.go", "// see req.auth here\n")

	lockedDir := filepath.Join(dir, "locked")
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(lockedDir, 0o755) })

	out, err := run(t, dir, "tag", "rename", "req.auth", "req.authn", "--rewrite-refs")
	if err == nil {
		t.Fatalf("expected non-zero exit when source rewrite fails, got success:\n%s", out)
	}

	// The `.scholia` rename itself must have committed regardless.
	list := mustRun(t, dir, "tag", "list")
	if strings.Contains(list, "req.auth\t") || !strings.Contains(list, "req.authn") {
		t.Fatalf("expected .scholia rename to stay committed despite source-rewrite failure, got:\n%s", list)
	}

	os.Chmod(lockedDir, 0o755)
	retryOut := mustRun(t, dir, "refs", "rewrite", "req.auth", "req.authn", "--apply")
	if !strings.Contains(retryOut, "書き換えました") {
		t.Fatalf("expected retry via `scholia refs rewrite --apply` to finish the job, got:\n%s", retryOut)
	}
	got := readSourceFile(t, dir, "locked/handler.go")
	if got != "// see req.authn here\n" {
		t.Fatalf("expected retry to complete the rewrite, got %q", got)
	}

	// A further retry is idempotent (nothing left to rewrite).
	idemOut := mustRun(t, dir, "refs", "rewrite", "req.auth", "req.authn", "--apply")
	if !strings.Contains(idemOut, "見つかりませんでした") {
		t.Fatalf("expected idempotent no-op on second retry, got:\n%s", idemOut)
	}
}
