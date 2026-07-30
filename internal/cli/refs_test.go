package cli

import (
	"os"
	"path/filepath"
	"runtime"
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

	// --json is a machine-read face, so the field name and the reason string are
	// part of the contract, not just the prose above.
	jsonOut := mustRun(t, dir, "refs", "scan", "--id", "req.auth", "--json")
	if !strings.Contains(jsonOut, `"reason": "not-regular"`) {
		t.Fatalf("expected the skip reason in --json output, got:\n%s", jsonOut)
	}
}

// TestCLI_RefsRewriteApplyExitsNonZeroWhenACandidateCouldNotBeRead covers the
// exit-code half of "no candidate aborts a scan". Making read failures skips
// removed the abort, and with it the signal that `--apply` did not finish: a
// file it could not read keeps its old ids while the command prints
// 「書き換えました」and exits 0. On the apply face that is a false success, so an
// unreadable skip exits non-zero — the same treatment a failed *write* already
// gets. Deliberate skips (binary/too-large/not-regular) are not unfinished work
// and stay exit 0, which the sibling test below pins.
func TestCLI_RefsRewriteApplyExitsNonZeroWhenACandidateCouldNotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not deny anything")
	}
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "handler.go", "// see req.foo here\n")
	writeSourceFile(t, dir, "locked.go", "// see req.foo here too\n")
	locked := filepath.Join(dir, "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })

	out, err := run(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	if err == nil {
		t.Fatalf("expected non-zero exit when a source file could not be read during --apply, got success:\n%s", out)
	}
	if !strings.Contains(out, "skip (unreadable)") || !strings.Contains(out, "locked.go") {
		t.Fatalf("expected the unread file to be named in output, got:\n%s", out)
	}
	// The readable file must still have been rewritten (a non-zero exit reports
	// leftover work; it does not undo the work that succeeded).
	if got := readSourceFile(t, dir, "handler.go"); got != "// see req.bar here\n" {
		t.Fatalf("expected handler.go to be rewritten, got %q", got)
	}

	// The same tree in dry-run is an inventory read, not unfinished work: exit 0.
	dryOut, dryErr := run(t, dir, "refs", "rewrite", "req.foo", "req.bar")
	if dryErr != nil {
		t.Fatalf("dry-run must not fail on an unreadable candidate: %v\n%s", dryErr, dryOut)
	}
	// ...and so is `refs scan`, whose whole job is to report what it found.
	if scanOut, scanErr := run(t, dir, "refs", "scan", "--id", "req.foo"); scanErr != nil {
		t.Fatalf("refs scan must not fail on an unreadable candidate: %v\n%s", scanErr, scanOut)
	}

	// Once the obstruction is gone, the retry finishes the job and exits 0.
	os.Chmod(locked, 0o644)
	retryOut := mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	if !strings.Contains(retryOut, "書き換えました") {
		t.Fatalf("expected the retry to finish the job, got:\n%s", retryOut)
	}
	if got := readSourceFile(t, dir, "locked.go"); got != "// see req.bar here too\n" {
		t.Fatalf("expected locked.go to be rewritten on retry, got %q", got)
	}
}

// TestCLI_RefsRewriteApplyExitsZeroOnDeliberateSkips is the other side of that
// line: a candidate refs is designed never to read is not unfinished work, so
// `--apply` completes normally and exits 0 with the skip disclosed.
func TestCLI_RefsRewriteApplyExitsZeroOnDeliberateSkips(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "handler.go", "// see req.foo here\n")
	writeSourceFile(t, dir, "realhooks/pre.sh", "echo hi\n")
	if err := os.MkdirAll(filepath.Join(dir, ".claude"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.Symlink(filepath.Join(dir, "realhooks"), filepath.Join(dir, ".claude", "hooks")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	out := mustRun(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	if !strings.Contains(out, "skip (not-regular)") {
		t.Fatalf("expected the deliberate skip to be disclosed, got:\n%s", out)
	}
	if got := readSourceFile(t, dir, "handler.go"); got != "// see req.bar here\n" {
		t.Fatalf("expected handler.go to be rewritten, got %q", got)
	}
}

// refsApplyFaces is every command that can rewrite source references — i.e.
// every caller that passes `applied` to refsFailedErr. Adding a sixth such
// command means adding a row here, and the test below then holds it to the same
// rule; wiring one face to a wrong `applied` value fails that face's row.
//
// Enumerating the faces is not the same shape as enumerating loopholes: the
// faces are a closed set the compiler already forces to declare `applied`
// (grep refsFailedErr), and the rule under test is one property applied to all
// of them, not a list of things that might go wrong.
var refsApplyFaces = []struct {
	name string
	// setup builds a fresh store whose records contain oldID, and returns its dir.
	setup      func(t *testing.T) string
	oldID      string
	newID      string
	applyArgs  []string
	dryRunArgs []string
}{
	{
		name: "refs rewrite --apply",
		setup: func(t *testing.T) string {
			dir := t.TempDir()
			mustRun(t, dir, "init")
			return dir
		},
		oldID:      "req.foo",
		newID:      "req.bar",
		applyArgs:  []string{"refs", "rewrite", "req.foo", "req.bar", "--apply"},
		dryRunArgs: []string{"refs", "rewrite", "req.foo", "req.bar"},
	},
	{
		name: "tag rename --rewrite-refs",
		setup: func(t *testing.T) string {
			dir := t.TempDir()
			setupAuthFixture(t, dir)
			return dir
		},
		oldID:      "req.auth",
		newID:      "req.authn",
		applyArgs:  []string{"tag", "rename", "req.auth", "req.authn", "--rewrite-refs"},
		dryRunArgs: []string{"tag", "rename", "req.auth", "req.authn"},
	},
	{
		name: "vocab rename --rewrite-refs",
		setup: func(t *testing.T) string {
			dir := t.TempDir()
			setupAuthFixture(t, dir)
			return dir
		},
		oldID:      "act.user.submit-login",
		newID:      "act.user.sign-in",
		applyArgs:  []string{"vocab", "rename", "act.user.submit-login", "--to", "act.user.sign-in", "--rewrite-refs"},
		dryRunArgs: []string{"vocab", "rename", "act.user.submit-login", "--to", "act.user.sign-in"},
	},
	{
		name: "tx rename --rewrite-refs",
		setup: func(t *testing.T) string {
			dir := t.TempDir()
			setupAuthFixture(t, dir)
			return dir
		},
		oldID:      "T-login",
		newID:      "T-signin",
		applyArgs:  []string{"tx", "rename", "T-login", "--to", "T-signin", "--rewrite-refs"},
		dryRunArgs: []string{"tx", "rename", "T-login", "--to", "T-signin"},
	},
	{
		name:       "tx merge --rewrite-refs",
		setup:      setupMergeStore,
		oldID:      "T-dup",
		newID:      "T-surv",
		applyArgs:  []string{"tx", "merge", "T-dup", "--into", "T-surv", "--rewrite-refs"},
		dryRunArgs: []string{"tx", "merge", "T-dup", "--into", "T-surv"},
	},
}

// TestCLI_EveryApplyFaceExitsNonZeroWhenACandidateCouldNotBeRead holds all five
// source-rewriting commands to the same rule, because the previous round wired
// the rule into five places and only tested two of them — the other three could
// be set to "this run changed nothing" and no test noticed.
//
// The stakes are the same on every face: applyRenameRefs runs after the
// `.scholia` write has committed, so a silent exit 0 leaves the record renamed
// or merged, the source still holding the old id, and nothing saying so.
func TestCLI_EveryApplyFaceExitsNonZeroWhenACandidateCouldNotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not deny anything")
	}
	for _, face := range refsApplyFaces {
		t.Run(face.name, func(t *testing.T) {
			// Applying: the unreadable candidate is unfinished work → non-zero.
			dir := face.setup(t)
			writeSourceFile(t, dir, "readable.go", "// see "+face.oldID+" here\n")
			lockSourceFile(t, dir, "locked.go", "// see "+face.oldID+" here too\n")

			out, err := run(t, dir, face.applyArgs...)
			if err == nil {
				t.Fatalf("expected non-zero exit when a source file could not be read, got success:\n%s", out)
			}
			if !strings.Contains(out, "skip (unreadable)") || !strings.Contains(out, "locked.go") {
				t.Fatalf("expected the unread file to be named in output, got:\n%s", out)
			}
			// The work that could be done was still done.
			if got := readSourceFile(t, dir, "readable.go"); got != "// see "+face.newID+" here\n" {
				t.Fatalf("expected readable.go to be rewritten to %q, got %q", face.newID, got)
			}

			// Not applying: the same candidate is just an inventory item → exit 0.
			dryDir := face.setup(t)
			lockSourceFile(t, dryDir, "locked.go", "// see "+face.oldID+" here too\n")
			if dryOut, dryErr := run(t, dryDir, face.dryRunArgs...); dryErr != nil {
				t.Fatalf("dry-run must not fail on an unreadable candidate: %v\n%s", dryErr, dryOut)
			}
		})
	}
}

// lockSourceFile writes a source file and makes it unreadable for the rest of
// the test (restored on cleanup so the temp dir can be removed).
func lockSourceFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	writeSourceFile(t, dir, rel, content)
	path := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(path, 0o644) })
}

// TestCLI_RefsRewriteApplyReportsBothKindsOfUnfinishedWork covers the branch
// where a run leaves *both* kinds of leftover: a file it could not read, and a
// file it read and could not write back. Three of refsFailedErr's four branches
// were exercised by the tests above; this is the fourth, and without it that
// branch could return "nothing to report" with every test still green.
//
// It also pins that every unreadable file is counted, not just the first — the
// count is what tells the user how much is left.
func TestCLI_RefsRewriteApplyReportsBothKindsOfUnfinishedWork(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: permissions do not deny anything")
	}
	dir := t.TempDir()
	mustRun(t, dir, "init")
	writeSourceFile(t, dir, "ok.go", "// see req.foo here\n")
	// Two unreadable files, so a count that stops at the first one is visible.
	lockSourceFile(t, dir, "locked1.go", "// see req.foo here\n")
	lockSourceFile(t, dir, "locked2.go", "// see req.foo here\n")
	// Readable, matched, but its directory forbids the temp file the atomic
	// write needs — read succeeds, write fails.
	writeSourceFile(t, dir, "ro/frozen.go", "// see req.foo here\n")
	roDir := filepath.Join(dir, "ro")
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(roDir, 0o755) })

	out, err := run(t, dir, "refs", "rewrite", "req.foo", "req.bar", "--apply")
	if err == nil {
		t.Fatalf("expected non-zero exit when work was left unfinished, got success:\n%s", out)
	}
	for _, want := range []string{"書換に失敗したファイル 1 件", "読めなかったファイル 2 件"} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected both kinds of leftover counted in %q, got:\n%s", want, out)
		}
	}
	if got := readSourceFile(t, dir, "ok.go"); got != "// see req.bar here\n" {
		t.Fatalf("expected ok.go to be rewritten, got %q", got)
	}
}

// TestCLI_TagRenameRewriteRefsExitsNonZeroWhenACandidateCouldNotBeRead is the
// same rule on the face where losing it costs the most: applyRenameRefs runs
// after the `.scholia` rename has committed, so a silent exit 0 would leave the
// store renamed and a source file still holding the old id, with nothing saying
// so. This mirrors the existing write-failure test — the two failures are now
// symmetric.
func TestCLI_TagRenameRewriteRefsExitsNonZeroWhenACandidateCouldNotBeRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file permission semantics differ on windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("running as root: file permissions do not deny anything")
	}
	dir := t.TempDir()
	setupAuthFixture(t, dir)
	writeSourceFile(t, dir, "locked.go", "// see req.auth here\n")
	locked := filepath.Join(dir, "locked.go")
	if err := os.Chmod(locked, 0o000); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o644) })

	out, err := run(t, dir, "tag", "rename", "req.auth", "req.authn", "--rewrite-refs")
	if err == nil {
		t.Fatalf("expected non-zero exit when a source file could not be read, got success:\n%s", out)
	}
	// The `.scholia` rename itself must stay committed regardless.
	list := mustRun(t, dir, "tag", "list")
	if strings.Contains(list, "req.auth\t") || !strings.Contains(list, "req.authn") {
		t.Fatalf("expected the .scholia rename to stay committed, got:\n%s", list)
	}
	// A plain rename (no --rewrite-refs) is a dry-run scan: exit 0.
	if dryOut, dryErr := run(t, dir, "tag", "rename", "req.authn", "req.authz"); dryErr != nil {
		t.Fatalf("rename without --rewrite-refs must not fail on an unreadable candidate: %v\n%s", dryErr, dryOut)
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
