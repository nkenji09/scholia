package cli

import (
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
