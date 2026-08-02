package refs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestClassifyWrite is the value-level check for the write-target rule: given
// what was learned about a path and the set of candidates this run enumerated,
// may an apply write there. Keeping the judgment out of Execute is what makes
// it checkable as input/output pairs instead of by staging a filesystem.
//
// Scope: this pins the *decision*, not the FS reading that feeds it. The rows
// below are what inspectLink can produce; TestInspectLink covers the mapping
// from real symlinks to those rows, and the Execute tests cover the two joined.
func TestClassifyWrite(t *testing.T) {
	candidates := map[string]bool{"a.ts": true, "real/impl.ts": true}

	cases := []struct {
		name string
		st   linkState
		want writeDecision
	}{
		{
			name: "plain file is written directly",
			st:   linkState{},
			want: writeDirect,
		},
		{
			name: "symlink resolving to a candidate defers to that candidate",
			st:   linkState{symlink: true, targetRel: "real/impl.ts"},
			want: writeDeferred,
		},
		{
			name: "symlink resolving inside root but outside the candidate set is refused",
			st:   linkState{symlink: true, targetRel: "vendor/x.ts"},
			want: writeRefused,
		},
		{
			name: "symlink resolving outside root is refused",
			st:   linkState{symlink: true, targetRel: "", shown: "../other/far.ts"},
			want: writeRefused,
		},
		{
			name: "broken or unresolvable symlink is refused, never written",
			st:   linkState{symlink: true, targetRel: "", shown: "nowhere.ts"},
			want: writeRefused,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyWrite(tc.st, candidates); got != tc.want {
				t.Fatalf("classifyWrite(%+v) = %d, want %d", tc.st, got, tc.want)
			}
		})
	}
}

// TestInspectLink pins the other half: turning a path on disk into the facts
// classifyWrite decides on. The chain row is the one that matters most — judging
// a link by its next hop instead of its final target would call
// chain -> alias -> real/impl.ts "outside the candidate set" and refuse a
// rewrite that is perfectly safe.
func TestInspectLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on windows")
	}
	root := t.TempDir()
	writeFile(t, root, "real/impl.ts", "// req.foo\n")
	writeFile(t, root, "plain.ts", "// req.foo\n")
	outside := filepath.Join(t.TempDir(), "far.ts")
	if err := os.WriteFile(outside, []byte("// req.foo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	symlink(t, "real/impl.ts", filepath.Join(root, "alias.ts"))
	symlink(t, "alias.ts", filepath.Join(root, "chain.ts"))
	symlink(t, outside, filepath.Join(root, "outside.ts"))
	symlink(t, "nowhere.ts", filepath.Join(root, "broken.ts"))

	cases := []struct {
		rel           string
		wantSymlink   bool
		wantTargetRel string
	}{
		{rel: "plain.ts", wantSymlink: false, wantTargetRel: ""},
		{rel: "alias.ts", wantSymlink: true, wantTargetRel: "real/impl.ts"},
		{rel: "chain.ts", wantSymlink: true, wantTargetRel: "real/impl.ts"},
		{rel: "outside.ts", wantSymlink: true, wantTargetRel: ""},
		{rel: "broken.ts", wantSymlink: true, wantTargetRel: ""},
	}
	for _, tc := range cases {
		t.Run(tc.rel, func(t *testing.T) {
			got := inspectLink(root, tc.rel)
			if got.symlink != tc.wantSymlink || got.targetRel != tc.wantTargetRel {
				t.Fatalf("inspectLink(%q) = %+v, want symlink=%v targetRel=%q",
					tc.rel, got, tc.wantSymlink, tc.wantTargetRel)
			}
		})
	}
}

// TestExecute_ApplyNeverReplacesASymlinkWithARegularFile is the guard for the
// harm itself, checked the only way that can't be faked: stat the path
// afterwards and see whether it is still a link.
//
// It fails against the pre-fix code, where writeFileAtomic's rename landed on
// the candidate path — every row below came back as a regular file, and the
// link text the user wrote was gone with nothing in the output saying so.
//
// The rows are the shapes that reach the write step at all. A symlink to a
// directory and a broken symlink never do (they are read-side skips,
// not-regular and unreadable), so they are covered in files_test.go, not here.
func TestExecute_ApplyNeverReplacesASymlinkWithARegularFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on windows")
	}
	root := t.TempDir()
	writeFile(t, root, "real/impl.ts", "// see req.foo\n")
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "far.ts")
	if err := os.WriteFile(outside, []byte("// see req.foo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	symlink(t, "real/impl.ts", filepath.Join(root, "alias.ts"))  // target is a candidate
	symlink(t, "alias.ts", filepath.Join(root, "chain.ts"))      // chain to a candidate
	symlink(t, outside, filepath.Join(root, "outside.ts"))       // target outside root
	symlink(t, "../far-rel.ts", filepath.Join(root, "uprel.ts")) // relative escape, broken
	if err := os.WriteFile(filepath.Join(outsideDir, "unused.ts"), nil, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if _, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	for _, rel := range []string{"alias.ts", "chain.ts", "outside.ts", "uprel.ts"} {
		info, err := os.Lstat(filepath.Join(root, rel))
		if err != nil {
			t.Fatalf("Lstat %s: %v", rel, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("apply replaced the symlink %s with a regular file (mode %v): "+
				"the link target the user wrote is unrecoverable", rel, info.Mode())
		}
	}
}

// TestExecute_ApplyDefersToACandidateLinkTarget pins the half of the rule that
// keeps it usable: when the link resolves to a path this same run enumerates,
// the ids do get fixed (on the target's own turn) and the run is a success.
//
// Without the deferral, "never write a symlink" would report the link as a
// failed rewrite forever — nothing to remove, so a rerun could never converge,
// and the user's only escape would be deleting their symlink.
func TestExecute_ApplyDefersToACandidateLinkTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on windows")
	}
	root := t.TempDir()
	writeFile(t, root, "real/impl.ts", "// see req.foo\n")
	symlink(t, "real/impl.ts", filepath.Join(root, "alias.ts"))

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.Failed) != 0 {
		t.Fatalf("a link whose target is itself a candidate leaves nothing undone, "+
			"so it must not be reported as failed: %+v", report.Failed)
	}
	if len(report.Links) != 1 || report.Links[0].Path != "alias.ts" || !report.Links[0].Covered {
		t.Fatalf("expected one covered link note for alias.ts, got %+v", report.Links)
	}
	if got := readFile(t, root, "real/impl.ts"); got != "// see req.bar\n" {
		t.Fatalf("the link target must still be rewritten on its own turn, got %q", got)
	}
	if got := readFile(t, root, "alias.ts"); got != "// see req.bar\n" {
		t.Fatalf("reading through the link must see the new id, got %q", got)
	}

	// And a rerun converges: nothing left to match, nothing reported, exit-worthy
	// state clean.
	report2, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if len(report2.Matches) != 0 || len(report2.Failed) != 0 || len(report2.Links) != 0 {
		t.Fatalf("rerun must converge to a clean no-op, got %+v", report2)
	}
}

// TestExecute_ApplyRefusesALinkOutOfTheCandidateSet pins the other half: when
// following the link would leave the enumerated set, nothing is written — not
// to the link, not through it — and the leftover is reported as a failed
// rewrite (the third outcome decision 01KYSG9QEE36VPSR020WV81JWW reserves for
// "read but could not be written back"), naming the target so the user can act.
func TestExecute_ApplyRefusesALinkOutOfTheCandidateSet(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on windows")
	}
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "far.ts")
	if err := os.WriteFile(outside, []byte("// see req.foo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	symlink(t, outside, filepath.Join(root, "outside.ts"))

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if len(report.RewrittenFiles) != 0 {
		t.Fatalf("nothing should have been written, got %v", report.RewrittenFiles)
	}
	if len(report.Failed) != 1 || report.Failed[0].Path != "outside.ts" {
		t.Fatalf("expected outside.ts reported as a failed rewrite, got %+v", report.Failed)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "// see req.foo\n" {
		t.Fatalf("the out-of-scope target must not be written: %q (%v)", got, err)
	}
	if len(report.Links) != 1 || report.Links[0].Covered {
		t.Fatalf("expected one uncovered link note, got %+v", report.Links)
	}
}

// TestExecute_ApplyDoesNotWriteThroughAnExcludedTarget is why the rule is about
// the *candidate set* and not merely about staying under root. filterScope
// narrows candidate paths, never resolved targets, so a single symlink is
// enough to carry a write into a directory config.sourceRefs.exclude
// deliberately shut out — the one control a user has over what this package may
// touch.
func TestExecute_ApplyDoesNotWriteThroughAnExcludedTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevation on windows")
	}
	root := t.TempDir()
	writeFile(t, root, "vendor/x.ts", "// see req.foo\n")
	symlink(t, "vendor/x.ts", filepath.Join(root, "alias.ts"))

	report, err := Execute(root, []Pair{{OldID: "req.foo", NewID: "req.bar"}}, true,
		Options{Exclude: []string{"vendor"}})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if got := readFile(t, root, "vendor/x.ts"); got != "// see req.foo\n" {
		t.Fatalf("an excluded path must not be written through a symlink, got %q", got)
	}
	if len(report.Failed) != 1 || report.Failed[0].Path != "alias.ts" {
		t.Fatalf("expected the refusal to be reported, got %+v", report.Failed)
	}
}

func symlink(t *testing.T, target, linkPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(linkPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(target, linkPath); err != nil {
		t.Fatalf("Symlink(%s, %s): %v", target, linkPath, err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("ReadFile %s: %v", rel, err)
	}
	return string(b)
}
