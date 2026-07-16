package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// snapshotDirModTimes walks dir/.scholia and records each file's mtime, so a
// caller can assert a command touched nothing (read-only).
func snapshotDirModTimes(t *testing.T, dir string) map[string]int64 {
	t.Helper()
	scholiaDir := filepath.Join(dir, ".scholia")
	snap := make(map[string]int64)
	err := filepath.Walk(scholiaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			snap[path] = info.ModTime().UnixNano()
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", scholiaDir, err)
	}
	return snap
}

// seedSearchFixture builds a store with a hit for every record type on the
// keyword "login": a tag name, a vocab label, a transition id, and a
// decision why. It also seeds an unrelated record of each type so
// --type filtering and 0-hit assertions are meaningful.
func seedSearchFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(args ...string) {
		t.Helper()
		if out, err := run(t, dir, args...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", args, err, out)
		}
	}
	must("init")
	must("vocab", "add", "condition", "cond.credentials-valid", "--label", "資格情報が正当")
	must("vocab", "add", "action", "act.user.submit-login", "--label", "ログイン送信", "--kind", "user")
	must("vocab", "add", "effect", "eff.session.issue-token", "--label", "セッショントークン発行", "--kind", "state")
	must("vocab", "add", "effect", "eff.other.noop", "--label", "無関係な効果", "--kind", "state")

	must("tag", "create", "subject.login", "--name", "ログイン", "--kind", "subject")
	must("tag", "create", "subject.other", "--name", "無関係タグ", "--kind", "subject")

	must("tx", "add", "T-login-submit-valid",
		"--action", "act.user.submit-login",
		"--given", "cond.credentials-valid",
		"--then", "eff.session.issue-token",
		"--tags", "subject.login",
	)
	must("tx", "add", "T-other",
		"--action", "act.user.submit-login",
		"--then", "eff.other.noop",
	)

	decideOut, err := run(t, dir, "decide", "--on", "tag:subject.login", "--why", "login フローの決定理由", "--json")
	if err != nil {
		t.Fatalf("decide: %v\n%s", err, decideOut)
	}
	must("decide", "--on", "tag:subject.other", "--why", "無関係な決定理由")
}

func TestSearch_MatchesAcrossAllFourTypes(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	out, err := run(t, dir, "search", "login")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	for _, want := range []string{"tag (", "vocab (", "transition (", "decision (", "subject.login", "T-login-submit-valid"} {
		if !strings.Contains(out, want) {
			t.Fatalf("search output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "subject.other") {
		t.Fatalf("search 'login' matched unrelated tag subject.other:\n%s", out)
	}
}

func TestSearch_IsCaseInsensitiveOnID(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	out, err := run(t, dir, "search", "LOGIN")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-login-submit-valid") {
		t.Fatalf("case-insensitive search failed:\n%s", out)
	}
}

func TestSearch_TypeFlagFiltersToOneType(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	out, err := run(t, dir, "search", "login", "--type", "decision")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "decision (") {
		t.Fatalf("--type decision produced no decision group:\n%s", out)
	}
	for _, notWant := range []string{"tag (", "vocab (", "transition ("} {
		if strings.Contains(out, notWant) {
			t.Fatalf("--type decision leaked other type group %q:\n%s", notWant, out)
		}
	}
}

func TestSearch_UnknownTypeIsError(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	if _, err := run(t, dir, "search", "login", "--type", "bogus"); err == nil {
		t.Fatalf("expected error for unknown --type")
	}
}

func TestSearch_NoHitsIsZeroDegradation(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	out, err := run(t, dir, "search", "zzz-no-match")
	if err != nil {
		t.Fatalf("search with no hits should not error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "該当なし") {
		t.Fatalf("expected 0-hit degradation message, got:\n%s", out)
	}
}

func TestSearch_EmptyKeywordIsError(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	if _, err := run(t, dir, "search", "   "); err == nil {
		t.Fatalf("expected error for blank keyword")
	}
}

func TestSearch_JSONShapeIsStructured(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	out, err := run(t, dir, "search", "login", "--json")
	if err != nil {
		t.Fatalf("search --json: %v\n%s", err, out)
	}
	var parsed struct {
		Matches []struct {
			Type    string `json:"type"`
			ID      string `json:"id"`
			Field   string `json:"field"`
			Snippet string `json:"snippet"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\noutput:\n%s", err, out)
	}
	if len(parsed.Matches) == 0 {
		t.Fatalf("expected matches, got none:\n%s", out)
	}
	seenTypes := make(map[string]bool)
	for _, m := range parsed.Matches {
		if m.ID == "" || m.Field == "" || m.Snippet == "" {
			t.Fatalf("match missing id/field/snippet: %+v", m)
		}
		seenTypes[m.Type] = true
	}
	for _, want := range []string{"tag", "vocab", "transition", "decision"} {
		if !seenTypes[want] {
			t.Fatalf("expected a %s match in JSON output, got types=%v", want, seenTypes)
		}
	}
}

func TestSearch_ReadOnlyDoesNotChangeScholiaDir(t *testing.T) {
	dir := t.TempDir()
	seedSearchFixture(t, dir)

	before := snapshotDirModTimes(t, dir)
	if _, err := run(t, dir, "search", "login"); err != nil {
		t.Fatalf("search: %v", err)
	}
	after := snapshotDirModTimes(t, dir)
	if len(before) != len(after) {
		t.Fatalf("search changed the number of files under .scholia: before=%d after=%d", len(before), len(after))
	}
	for path, mtime := range before {
		if after[path] != mtime {
			t.Fatalf("search modified %s (read-only violation)", path)
		}
	}
}
