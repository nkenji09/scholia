package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLI_RefsScanListsOccurrencesForSpecificID(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth for the requirement\npackage handler\n")

	out := mustRun(t, dir, "refs", "scan", "--id", "req.auth")
	if !strings.Contains(out, "handler.go:1") {
		t.Fatalf("expected handler.go:1 in scan output, got:\n%s", out)
	}
}

// TestCLI_RefsScanDoesNotSuggestNoOpRewrite covers the nit: `refs scan`'s
// matches carry Old==New (ScanIDs' placeholder), so the human-readable
// output must not print a useless `scholia refs rewrite req.auth req.auth
// --apply` suggestion.
func TestCLI_RefsScanDoesNotSuggestNoOpRewrite(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth\n")

	out := mustRun(t, dir, "refs", "scan", "--id", "req.auth")
	if strings.Contains(out, "scholia refs rewrite req.auth req.auth") {
		t.Fatalf("expected no self-rewrite suggestion in scan output, got:\n%s", out)
	}
}

func TestCLI_RefsScanAllKnownIDsWithoutFlag(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// act.user.submit-login and req.auth are both referenced here\n")

	out := mustRun(t, dir, "refs", "scan")
	for _, want := range []string{"act.user.submit-login", "req.auth"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected scan (all ids) to surface %q, got:\n%s", want, out)
		}
	}
}

func TestCLI_RefsScanNeverModifiesSource(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth\n")

	mustRun(t, dir, "refs", "scan", "--id", "req.auth")

	got := readSourceFile(t, dir, "handler.go")
	if got != "// see req.auth\n" {
		t.Fatalf("refs scan must never modify source, got %q", got)
	}
}

// TestCLI_RefsScanSurvivesNonRegularCandidateAndNamesTheSkip is the acceptance
// criterion at the CLI layer: a symlink-to-directory in the tree used to make
// `scholia refs scan` exit non-zero with `read <path>: is a directory` and print
// nothing at all. The command must now succeed, still list the real occurrence,
// and print the skip so the omission is not silent — the reasons only exist if a
// human can read them (printRenameRefsReport is the only place that renders
// them).
func TestCLI_RefsScanSurvivesNonRegularCandidateAndNamesTheSkip(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "handler.go", "// see req.auth for the requirement\npackage handler\n")
	writeSourceFile(t, dir, "realhooks/pre.sh", "echo hi\n")
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "realhooks"), filepath.Join(dir, ".claude", "hooks")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	out := mustRun(t, dir, "refs", "scan", "--id", "req.auth")

	if !strings.Contains(out, "handler.go:1") {
		t.Fatalf("expected the real occurrence to survive the bad candidate, got:\n%s", out)
	}
	if !strings.Contains(out, "skip (not-regular)") || !strings.Contains(out, filepath.ToSlash(filepath.Join(".claude", "hooks"))) {
		t.Fatalf("expected the skipped candidate and its reason in human-readable output, got:\n%s", out)
	}
}

func TestCLI_RefsRewriteDefaultIsDryRun(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "handler.go", "// see req.foo here\n")

	out := mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar")
	if !strings.Contains(out, "handler.go:1") {
		t.Fatalf("expected dry-run listing, got:\n%s", out)
	}
	got := readSourceFile(t, dir, "handler.go")
	if got != "// see req.foo here\n" {
		t.Fatalf("dry-run must not modify source, got %q", got)
	}
}

func TestCLI_RefsRewriteApplyIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "handler.go", "// see req.foo here, but req.foobar is unrelated\n")

	mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	got := readSourceFile(t, dir, "handler.go")
	want := "// see req.bar here, but req.foobar is unrelated\n"
	if got != want {
		t.Fatalf("rewrite --apply:\ngot:  %q\nwant: %q", got, want)
	}

	out := mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	if !strings.Contains(out, "見つかりませんでした") {
		t.Fatalf("second --apply run should be a no-op (nothing left to rewrite), got:\n%s", out)
	}
	got2 := readSourceFile(t, dir, "handler.go")
	if got2 != want {
		t.Fatalf("second run must not change content again, got %q", got2)
	}
}

func TestCLI_RefsRewriteJSONOutputShape(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "handler.go", "// see req.foo here\n")

	out := mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--json")
	for _, want := range []string{`"matches"`, "handler.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected JSON output to contain %q, got:\n%s", want, out)
		}
	}
}
