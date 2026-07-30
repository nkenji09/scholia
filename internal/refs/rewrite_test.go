package refs

import (
	"os"
	"os/exec"
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
		"// scholia ids: req.foo top-level, req.foo-child nested, req.foo-child-grandchild deeper\n")

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
	want := "// scholia ids: req.top top-level, req.top-child nested, req.top-child-grandchild deeper\n"
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

// skipReasons collapses a report's skips into path->reason for assertions.
func skipReasons(report Report) map[string]string {
	out := map[string]string{}
	for _, sk := range report.Skipped {
		out[sk.Path] = sk.Reason
	}
	return out
}

// TestExecute_BadCandidateFromGitLsFilesDoesNotAbortScan reproduces the
// reported failure on the default enumeration path: `git ls-files` reports a
// symlink-to-directory as a single entry, and reading it used to fail with
// `read <path>: is a directory`, which Execute returned as fatal — losing every
// match in every other file. The scan must complete, keep the real match, and
// leave the omission visible.
func TestExecute_BadCandidateFromGitLsFilesDoesNotAbortScan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runGitT(t, root, "init", "-q")
	writeFile(t, root, "hooks/impl.go", "// tx.foo.bar\npackage hooks\n")
	// Both sort before "hooks/impl.go", so a bad candidate is read first and an
	// abort would swallow the match that follows it.
	if err := os.Symlink(filepath.Join(root, "hooks"), filepath.Join(root, "aaa-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "aab-dangling")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	files, err := EnumerateFiles(root)
	if err != nil {
		t.Fatalf("EnumerateFiles: %v", err)
	}
	// Precondition: the bad entries really do come back as candidates on this
	// path. Without this the test could pass by never seeing them.
	enumerated := map[string]bool{}
	for _, f := range files {
		enumerated[f] = true
	}
	for _, want := range []string{"aaa-link", "aab-dangling"} {
		if !enumerated[want] {
			t.Fatalf("precondition: expected git ls-files to yield %q as a candidate, got %v", want, files)
		}
	}

	report, err := Execute(root, []Pair{{OldID: "tx.foo.bar", NewID: "tx.foo.baz"}}, false)
	if err != nil {
		t.Fatalf("Execute aborted on a non-regular candidate: %v", err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Path != "hooks/impl.go" {
		t.Fatalf("expected the real file's match to survive, got %+v", report.Matches)
	}
	got := skipReasons(report)
	if got["aaa-link"] != "not-regular" || got["aab-dangling"] != "unreadable" {
		t.Fatalf("skipped candidates must stay visible in the report, got %+v", report.Skipped)
	}
}

// TestExecute_BadCandidateFromWalkFallbackDoesNotAbortScan is the same
// acceptance criterion on the *other* enumeration path. The walk fallback runs
// when there is no git (or no repo), classifies with Lstat — so a
// symlink-to-directory is a candidate there too — and additionally meets paths
// it cannot classify at all, which used to abort enumeration before any file
// was read.
func TestExecute_BadCandidateFromWalkFallbackDoesNotAbortScan(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not deny anything")
	}
	root := t.TempDir()
	if _, err := gitLsFiles(root); err == nil {
		t.Skip("temp dir is inside a git repo: the walk fallback would not be exercised")
	}
	writeFile(t, root, "hooks/impl.go", "// tx.foo.bar\npackage hooks\n")
	if err := os.Symlink(filepath.Join(root, "hooks"), filepath.Join(root, "aaa-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	writeFile(t, root, "aab-unlistable/hidden.go", "package hidden\n")
	unlistable := filepath.Join(root, "aab-unlistable")
	if err := os.Chmod(unlistable, 0o100); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unlistable, 0o755) })
	writeFile(t, root, "aac-unstattable/inner.go", "package inner\n")
	unstattable := filepath.Join(root, "aac-unstattable")
	if err := os.Chmod(unstattable, 0o400); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unstattable, 0o755) })

	report, err := Execute(root, []Pair{{OldID: "tx.foo.bar", NewID: "tx.foo.baz"}}, false)
	if err != nil {
		t.Fatalf("Execute aborted on the walk-fallback path: %v", err)
	}
	if len(report.Matches) != 1 || report.Matches[0].Path != "hooks/impl.go" {
		t.Fatalf("expected the real file's match to survive, got %+v", report.Matches)
	}
	got := skipReasons(report)
	if got["aaa-link"] != "not-regular" {
		t.Fatalf("symlink-to-dir must be a visible not-regular skip, got %+v", report.Skipped)
	}
	if got["aab-unlistable"] != "not-regular" {
		t.Fatalf("unlistable dir must be a visible skip, got %+v", report.Skipped)
	}
	if got["aac-unstattable/inner.go"] != "unreadable" {
		t.Fatalf("unstattable entry must be a visible skip, got %+v", report.Skipped)
	}
}

// TestExecute_ApplyFaceBadCandidateRewritesTheGoodFilesAndDisclosesTheSkip is
// the same acceptance criterion on the face that actually writes. The two tests
// above only exercise apply=false, so a caller that promoted a skip to fatal —
// or dropped it from the report — *only when apply is true* stayed green: the
// reported bug, restored on the half of the code that mutates source.
//
// That face is the more expensive one to lose. applyRenameRefs runs
// Execute(apply=true) after the `.scholia` rename has already committed, so
// aborting there leaves the store renamed and every source reference untouched.
//
// Scope: error-or-not, the bytes on disk, and the skip's presence in the report,
// all with apply=true. Exit codes are the CLI's job (cli/refs_test.go).
func TestExecute_ApplyFaceBadCandidateRewritesTheGoodFilesAndDisclosesTheSkip(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "hooks/impl.go", "// tx.foo.bar\npackage hooks\n")
	// Sorts before "hooks/impl.go": an abort here would swallow the rewrite.
	if err := os.Symlink(filepath.Join(root, "hooks"), filepath.Join(root, "aaa-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	files, err := EnumerateFiles(root)
	if err != nil {
		t.Fatalf("EnumerateFiles: %v", err)
	}
	var sawCandidate bool
	for _, f := range files {
		if f == "aaa-link" {
			sawCandidate = true
		}
	}
	if !sawCandidate {
		t.Fatalf("precondition: the bad candidate must be enumerated, got %v", files)
	}

	report, err := Execute(root, []Pair{{OldID: "tx.foo.bar", NewID: "tx.foo.baz"}}, true)
	if err != nil {
		t.Fatalf("Execute(apply=true) aborted on a non-regular candidate: %v", err)
	}
	if len(report.RewrittenFiles) != 1 || report.RewrittenFiles[0] != "hooks/impl.go" {
		t.Fatalf("expected hooks/impl.go to be rewritten, got %v", report.RewrittenFiles)
	}
	// The bytes, not just the report: an abort or a lost write is only visible here.
	got, err := os.ReadFile(filepath.Join(root, "hooks/impl.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "// tx.foo.baz\npackage hooks\n" {
		t.Fatalf("the good file must be rewritten despite the bad candidate, got %q", got)
	}
	if skipReasons(report)["aaa-link"] != SkipNotRegular {
		t.Fatalf("the skip must stay visible on the apply face too, got %+v", report.Skipped)
	}
}

// TestExecute_ApplyFaceUnreadableCandidateIsReportedAsUnfinishedWork pins the
// property the CLI's exit code is built on: a candidate that could not be read
// is unfinished work (its old ids are still in the file), while a candidate this
// package is designed not to read is not. Both are skips; only one is counted by
// UnreadableSkips.
func TestExecute_ApplyFaceUnreadableCandidateIsReportedAsUnfinishedWork(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not deny anything")
	}
	root := t.TempDir()
	writeFile(t, root, "ok.go", "// tx.foo.bar\n")
	writeFile(t, root, "unreadable.go", "// tx.foo.bar\n")
	locked := filepath.Join(root, "unreadable.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })
	// A by-design skip alongside it, to show the two are not the same bucket.
	if err := os.Symlink(root, filepath.Join(root, "self-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	report, err := Execute(root, []Pair{{OldID: "tx.foo.bar", NewID: "tx.foo.baz"}}, true)
	if err != nil {
		t.Fatalf("Execute(apply=true): %v", err)
	}
	unreadable := report.UnreadableSkips()
	if len(unreadable) != 1 || unreadable[0].Path != "unreadable.go" {
		t.Fatalf("expected exactly the unreadable file to count as unfinished work, got %+v (all skips %+v)", unreadable, report.Skipped)
	}
	if skipReasons(report)["self-link"] != SkipNotRegular {
		t.Fatalf("expected the by-design skip to still be reported, got %+v", report.Skipped)
	}
	// ...and the readable file was still rewritten.
	got, err := os.ReadFile(filepath.Join(root, "ok.go"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "// tx.foo.baz\n" {
		t.Fatalf("expected ok.go to be rewritten, got %q", got)
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
