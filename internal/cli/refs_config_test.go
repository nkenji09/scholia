package cli

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// setSourceRefsConfig sets config.sourceRefs for dir's store — there is no
// `scholia config set` key for it (yet), so this goes through the store's Go
// API directly, the same way `scholia config set` itself would under the
// hood, rather than hand-editing the JSON file.
func setSourceRefsConfig(t *testing.T, dir string, scan, exclude []string) {
	t.Helper()
	s, err := store.Open(dir)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.SourceRefs = &model.SourceRefs{Scan: scan, Exclude: exclude}
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
}

func TestCLI_RefsScanRespectsSourceRefsScan(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "app/handler.go", "// see req.foo\n")
	writeSourceFile(t, dir, "apps/other.go", "// see req.foo\n")
	setSourceRefsConfig(t, dir, []string{"app"}, nil)

	out := mustRun(t, dir, "refs", "scan", "--id", "req.foo")
	if !strings.Contains(out, "app/handler.go") {
		t.Fatalf("expected app/handler.go in scan output, got:\n%s", out)
	}
	if strings.Contains(out, "apps/other.go") {
		t.Fatalf("sourceRefs.scan=[\"app\"] must not swallow apps/, got:\n%s", out)
	}
}

func TestCLI_RefsScanRespectsSourceRefsExclude(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "app/handler.go", "// see req.foo\n")
	writeSourceFile(t, dir, "vendor/thirdparty.go", "// see req.foo\n")
	setSourceRefsConfig(t, dir, nil, []string{"vendor"})

	out := mustRun(t, dir, "refs", "scan", "--id", "req.foo")
	if !strings.Contains(out, "app/handler.go") {
		t.Fatalf("expected app/handler.go in scan output, got:\n%s", out)
	}
	if strings.Contains(out, "vendor/thirdparty.go") {
		t.Fatalf("sourceRefs.exclude=[\"vendor\"] must remove it, got:\n%s", out)
	}
}

func TestCLI_RefsRewriteRespectsSourceRefsScan(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "app/handler.go", "// see req.foo\n")
	writeSourceFile(t, dir, "apps/other.go", "// see req.foo\n")
	setSourceRefsConfig(t, dir, []string{"app"}, nil)

	mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")

	if got := readSourceFile(t, dir, "app/handler.go"); got != "// see req.bar\n" {
		t.Fatalf("expected app/handler.go rewritten, got %q", got)
	}
	if got := readSourceFile(t, dir, "apps/other.go"); got != "// see req.foo\n" {
		t.Fatalf("expected apps/other.go untouched by scan scope, got %q", got)
	}
}

func TestCLI_TagRenameImplicitScanRespectsSourceRefs(t *testing.T) {
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "app/handler.go", "// see req.auth\n")
	writeSourceFile(t, dir, "apps/other.go", "// see req.auth\n")
	setSourceRefsConfig(t, dir, []string{"app"}, nil)

	out := mustRun(t, dir, "tag", "rename", "req.auth", "req.authn")
	if !strings.Contains(out, "app/handler.go") {
		t.Fatalf("expected app/handler.go in dry-run output, got:\n%s", out)
	}
	if strings.Contains(out, "apps/other.go") {
		t.Fatalf("sourceRefs.scan should keep rename's implicit scan out of apps/, got:\n%s", out)
	}
}

// TestCLI_RefsScanWithoutSourceRefsConfigIsUnchanged is the regression
// check for the config-wiring fix: with no sourceRefs set at all (the
// pre-existing, still-default case), scanning must cover the whole project
// root exactly as before this field was wired in.
func TestCLI_RefsScanWithoutSourceRefsConfigIsUnchanged(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "app/handler.go", "// see req.foo\n")
	writeSourceFile(t, dir, "apps/other.go", "// see req.foo\n")

	out := mustRun(t, dir, "refs", "scan", "--id", "req.foo")
	for _, want := range []string{"app/handler.go", "apps/other.go"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q with no sourceRefs config set, got:\n%s", want, out)
		}
	}
}
