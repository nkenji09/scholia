package refs

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"
)

func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestEnumerateFiles_WalkFallbackExcludesReservedDirs(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, "sub/thing.go", "package sub\n")
	writeFile(t, root, ".scholia/tags/req.foo.json", `{"id":"req.foo"}`)
	writeFile(t, root, ".git/HEAD", "ref: refs/heads/main\n")
	writeFile(t, root, "_workspace/note.md", "scratch\n")
	writeFile(t, root, ".concierge/decision.md", "draft\n")

	got, err := walkFiles(root)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	want := []string{"main.go", "sub/thing.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkFiles = %v, want %v", got, want)
	}
}

func TestEnumerateFiles_GitLsFilesHonorsGitignore(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	root := t.TempDir()
	runGitT(t, root, "init", "-q")
	runGitT(t, root, "config", "user.email", "test@example.com")
	runGitT(t, root, "config", "user.name", "test")
	writeFile(t, root, ".gitignore", "ignored.txt\n")
	writeFile(t, root, "tracked.go", "package main\n")
	writeFile(t, root, "ignored.txt", "should not appear\n")
	writeFile(t, root, ".scholia/tags/req.foo.json", `{"id":"req.foo"}`)
	runGitT(t, root, "add", "tracked.go", ".gitignore")

	got, err := EnumerateFiles(root)
	if err != nil {
		t.Fatalf("EnumerateFiles: %v", err)
	}
	sort.Strings(got)
	want := []string{".gitignore", "tracked.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("EnumerateFiles = %v, want %v", got, want)
	}
}

func runGitT(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestReadSourceFile_SkipsBinaryAndOversized(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "text.go", "package main\n")
	if err := os.WriteFile(filepath.Join(root, "bin.dat"), []byte{0x00, 0x01, 0x02}, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	big := make([]byte, maxScanFileSize+1)
	if err := os.WriteFile(filepath.Join(root, "big.txt"), big, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, skip := readSourceFile(root, "text.go"); skip != nil {
		t.Fatalf("text.go should read cleanly, got skip=%v", skip)
	}
	if _, skip := readSourceFile(root, "bin.dat"); skip == nil || skip.Reason != "binary" {
		t.Fatalf("expected binary skip note, got %v", skip)
	}
	if _, skip := readSourceFile(root, "big.txt"); skip == nil || skip.Reason != "too-large" {
		t.Fatalf("expected too-large skip note, got %v", skip)
	}
}

// TestWalkFiles_UnwalkableRootStaysFatal is the other half of the line
// walkFiles draws: individual paths below root are never fatal, but failing to
// walk root itself is. Without this, the guard above has no counterweight and
// making root non-fatal too passes every test — while producing the worst
// possible output for an inventory tool: exit 0, zero skips, zero matches, a
// complete-looking empty answer.
func TestWalkFiles_UnwalkableRootStaysFatal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not deny anything")
	}
	root := filepath.Join(t.TempDir(), "proj")
	writeFile(t, root, "main.go", "// req.foo\npackage main\n")
	// --x: root can be stat'd but not listed.
	if err := os.Chmod(root, 0o100); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(root, 0o755) })

	got, err := walkFiles(root)
	if err == nil {
		t.Fatalf("walkFiles must fail when root cannot be walked, got %v candidates and no error", got)
	}

	// And the failure must reach the caller instead of becoming an empty report.
	if _, gitErr := gitLsFiles(root); gitErr == nil {
		t.Skip("temp dir is inside a git repo: the walk fallback would not be exercised")
	}
	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, false)
	if err == nil {
		t.Fatalf("Execute must not report a complete-looking empty inventory, got %+v", report)
	}
}

// TestWalkFiles_UnlistableExcludedDirIsNotACandidate pins an ordering that is
// load-bearing: the always-excluded check runs *before* the unclassifiable-path
// branch. Reverse them and an unlistable .git/.scholia/_workspace/.concierge
// leaks into the inventory as a skip line for a directory the scan is supposed
// to ignore entirely.
func TestWalkFiles_UnlistableExcludedDirIsNotACandidate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not deny anything")
	}
	root := t.TempDir()
	writeFile(t, root, "main.go", "package main\n")
	writeFile(t, root, ".git/HEAD", "ref: refs/heads/main\n")
	gitDir := filepath.Join(root, ".git")
	if err := os.Chmod(gitDir, 0o100); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(gitDir, 0o755) })

	got, err := walkFiles(root)
	if err != nil {
		t.Fatalf("walkFiles: %v", err)
	}
	want := []string{"main.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkFiles = %v, want %v (an excluded dir must stay excluded even when it cannot be listed)", got, want)
	}
}

// TestReadSourceFile_NonRegularAndUnreadableCandidatesAreSkips pins the
// property that lets a scan survive a bad candidate: readSourceFile answers
// read-it-or-skip-it for every path enumeration hands it, and has no error
// return a caller could turn into an abort.
//
// Scope: this checks the *classification* of candidate paths — anything whose
// resolved mode is not a regular file, and anything that cannot be stat'd or
// read, comes back as a SkipNote. It claims nothing about which paths
// enumeration produces (that is the two Execute tests below), and nothing about
// non-regular candidates never being opened (that is the FIFO test, which is
// the only case where opening is observably different from failing to read).
func TestReadSourceFile_NonRegularAndUnreadableCandidatesAreSkips(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "pkg/thing.go", "package pkg\n")

	// A directory reached as a candidate — what `git ls-files` reports for a
	// gitlink (submodule), and what the walk fallback hands on for a directory
	// it could not list.
	if _, skip := readSourceFile(root, "pkg"); skip == nil || skip.Reason != "not-regular" {
		t.Fatalf("directory candidate: expected not-regular skip, got %v", skip)
	}

	if err := os.Symlink(filepath.Join(root, "pkg"), filepath.Join(root, "linked")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	// A symlink whose target is a directory: one entry for `git ls-files`, a
	// non-directory for the walk fallback's Lstat, a directory for os.Stat.
	if _, skip := readSourceFile(root, "linked"); skip == nil || skip.Reason != "not-regular" {
		t.Fatalf("symlink-to-dir candidate: expected not-regular skip, got %v", skip)
	}

	// A dangling symlink — enumerated by `git ls-files`, stat fails.
	if err := os.Symlink(filepath.Join(root, "gone"), filepath.Join(root, "dangling")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if _, skip := readSourceFile(root, "dangling"); skip == nil || skip.Reason != "unreadable" {
		t.Fatalf("dangling symlink candidate: expected unreadable skip, got %v", skip)
	}

	// A path that is not there at all (enumerated, then deleted).
	if _, skip := readSourceFile(root, "vanished.go"); skip == nil || skip.Reason != "unreadable" {
		t.Fatalf("missing candidate: expected unreadable skip, got %v", skip)
	}

	// A symlink to a regular file still reads: what is rejected is the
	// resolved mode, not symlinks.
	if err := os.Symlink(filepath.Join(root, "pkg/thing.go"), filepath.Join(root, "alias.go")); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	data, skip := readSourceFile(root, "alias.go")
	if skip != nil {
		t.Fatalf("symlink-to-regular-file should read cleanly, got skip=%v", skip)
	}
	if string(data) != "package pkg\n" {
		t.Fatalf("symlink-to-regular-file should read the target's bytes, got %q", data)
	}
}

// TestWalkFiles_UnclassifiablePathBecomesCandidateNotAnError covers the second
// abort surface: the walk fallback used to return the first per-path error it
// saw, so one directory it may not list (or one entry whose Lstat fails) killed
// enumeration for the whole tree before any file was read. Such a path is now
// handed on as a candidate for readSourceFile to classify.
//
// Scope: only paths *below* root. Failing to walk root itself is still an
// error, and this test does not cover that.
func TestWalkFiles_UnclassifiablePathBecomesCandidateNotAnError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: directory permissions do not deny anything")
	}
	root := t.TempDir()
	writeFile(t, root, "ok.go", "package ok\n")
	writeFile(t, root, "unlistable/hidden.go", "package hidden\n")
	writeFile(t, root, "unstattable/inner.go", "package inner\n")

	// --x: Lstat of the directory works, listing it does not.
	unlistable := filepath.Join(root, "unlistable")
	if err := os.Chmod(unlistable, 0o100); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unlistable, 0o755) })
	// r--: listing the directory works, Lstat of the entries inside does not.
	unstattable := filepath.Join(root, "unstattable")
	if err := os.Chmod(unstattable, 0o400); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(unstattable, 0o755) })

	got, err := walkFiles(root)
	if err != nil {
		t.Fatalf("walkFiles aborted on a path it could not classify: %v", err)
	}
	want := []string{"ok.go", "unlistable", "unstattable/inner.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("walkFiles = %v, want %v", got, want)
	}
}
