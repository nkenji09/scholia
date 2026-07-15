package refs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExecute_DryRunLeavesSourceUnchanged(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "// see req.foo for the decision behind this\npackage main\n")

	before, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, false)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.Matches) != 1 {
		t.Fatalf("expected 1 match, got %+v", report.Matches)
	}
	if len(report.RewrittenFiles) != 0 {
		t.Fatalf("dry-run must not report rewritten files, got %v", report.RewrittenFiles)
	}

	after, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("dry-run must not modify the file:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestExecute_ApplyRewritesOnlyMatchedSpan(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go",
		"// see req.foo for context, but req.foobar and req.foo-bar are unrelated\npackage main\n")

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.baz"}}, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.RewrittenFiles) != 1 {
		t.Fatalf("expected 1 rewritten file, got %v", report.RewrittenFiles)
	}

	got, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "// see req.baz for context, but req.foobar and req.foo-bar are unrelated\npackage main\n"
	if string(got) != want {
		t.Fatalf("Execute apply:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExecute_IdempotentSecondRunIsNoOp(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "// see req.foo\npackage main\n")

	pairs := []Pair{{OldID: "req.foo", NewID: "req.bar"}}
	if _, err := Execute(root, pairs, true); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	report2, err := Execute(root, pairs, true)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if len(report2.Matches) != 0 || len(report2.RewrittenFiles) != 0 {
		t.Fatalf("second run should be a no-op, got %+v", report2)
	}
}

func TestExecute_CascadePairsApplyIndependently(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go",
		"// pmem ids: req.foo top-level, req.foo-child nested, req.foo-child-grandchild deeper\n")

	pairs := []Pair{
		{OldID: "req.foo", NewID: "req.top"},
		{OldID: "req.foo-child", NewID: "req.top-child"},
		{OldID: "req.foo-child-grandchild", NewID: "req.top-child-grandchild"},
	}
	report, err := Execute(root, pairs, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.RewrittenFiles) != 1 {
		t.Fatalf("expected 1 rewritten file, got %v", report.RewrittenFiles)
	}

	got, err := os.ReadFile(filepath.Join(root, "main.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "// pmem ids: req.top top-level, req.top-child nested, req.top-child-grandchild deeper\n"
	if string(got) != want {
		t.Fatalf("Execute cascade apply:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestExecute_PartialFailureReportsAndDoesNotAbortOtherFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	root := t.TempDir()
	writeFile(t, root, "ok.go", "// see req.foo here\n")
	writeFile(t, root, "locked/blocked.go", "// see req.foo here too\n")

	lockedDir := filepath.Join(root, "locked")
	if err := os.Chmod(lockedDir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(lockedDir, 0o755) })

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.Failed) != 1 || report.Failed[0].Path != "locked/blocked.go" {
		t.Fatalf("expected locked/blocked.go to fail, got %+v", report.Failed)
	}
	if len(report.RewrittenFiles) != 1 || report.RewrittenFiles[0] != "ok.go" {
		t.Fatalf("expected ok.go to still be rewritten, got %v", report.RewrittenFiles)
	}

	got, err := os.ReadFile(filepath.Join(root, "ok.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "// see req.bar here\n" {
		t.Fatalf("ok.go should be rewritten, got %q", got)
	}

	// Re-running after fixing permissions completes the partial failure
	// (idempotent: ok.go, already rewritten, is untouched the second time).
	os.Chmod(lockedDir, 0o755)
	report2, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if len(report2.Failed) != 0 {
		t.Fatalf("expected no failures after fixing permissions, got %+v", report2.Failed)
	}
	if len(report2.RewrittenFiles) != 1 || report2.RewrittenFiles[0] != "locked/blocked.go" {
		t.Fatalf("expected only locked/blocked.go rewritten on retry, got %v", report2.RewrittenFiles)
	}
}

func TestScanIDs_ReportsOccurrencesWithoutModifyingSource(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "// see req.foo and act.user.submit-login\n")

	report, err := ScanIDs(root, []string{"req.foo", "act.user.submit-login", "req.gone"})
	if err != nil {
		t.Fatalf("ScanIDs: %v", err)
	}
	if len(report.Matches) != 2 {
		t.Fatalf("expected 2 matches, got %+v", report.Matches)
	}
	if len(report.RewrittenFiles) != 0 {
		t.Fatalf("ScanIDs must never rewrite files, got %v", report.RewrittenFiles)
	}
}
