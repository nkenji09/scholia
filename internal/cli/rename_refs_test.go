package cli

import (
	"os"
	"path/filepath"
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
	if !strings.Contains(out, "pmem refs rewrite req.auth req.authn --apply") {
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
