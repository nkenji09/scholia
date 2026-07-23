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

// seedTagScopeFixture builds two subject subtrees that both hit the keyword
// "swap", so --tag narrowing is meaningful (the issue #1 motivating case where
// a bare OR search widens instead of narrowing):
//
//	subject.picker
//	  └─ req.picker-swap   ← T-picker-swap tagged here; uses eff.swap-range
//	subject.calendar       ← T-calendar-swap tagged here; uses eff.swap-days
func seedTagScopeFixture(t *testing.T, dir string) {
	t.Helper()
	must := func(args ...string) {
		t.Helper()
		if out, err := run(t, dir, args...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", args, err, out)
		}
	}
	must("init")
	setOwnerKind(t, dir, "subject") // 案 B: subject 表示は config.ownerKind に依存
	must("vocab", "add", "action", "act.swap-range", "--label", "範囲を swap する", "--kind", "user")
	must("vocab", "add", "effect", "eff.swap-range", "--label", "範囲 swap 効果", "--kind", "state")
	must("vocab", "add", "action", "act.swap-days", "--label", "曜日を swap する", "--kind", "user")
	must("vocab", "add", "effect", "eff.swap-days", "--label", "曜日 swap 効果", "--kind", "state")

	must("tag", "create", "subject.picker", "--name", "日付ピッカー", "--kind", "subject")
	must("tag", "create", "req.picker-swap", "--name", "範囲 swap 要件", "--kind", "requirement", "--parent", "subject.picker")
	must("tag", "create", "subject.calendar", "--name", "カレンダー", "--kind", "subject")

	must("tx", "add", "T-picker-swap",
		"--action", "act.swap-range",
		"--then", "eff.swap-range",
		"--tags", "req.picker-swap",
	)
	must("tx", "add", "T-calendar-swap",
		"--action", "act.swap-days",
		"--then", "eff.swap-days",
		"--tags", "subject.calendar",
	)
	must("decide", "--on", "tag:req.picker-swap", "--why", "picker の swap 決定")
	must("decide", "--on", "tag:subject.calendar", "--why", "calendar の swap 決定")
}

func TestSearch_TagFlagNarrowsToSubtree(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	// Bare "swap" hits both subtrees (the problem).
	wide, err := run(t, dir, "search", "swap")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, wide)
	}
	if !strings.Contains(wide, "T-picker-swap") || !strings.Contains(wide, "T-calendar-swap") {
		t.Fatalf("precondition: bare 'swap' should hit both subtrees:\n%s", wide)
	}

	// --tag subject.picker narrows to the picker subtree only.
	out, err := run(t, dir, "search", "swap", "--tag", "subject.picker")
	if err != nil {
		t.Fatalf("search --tag: %v\n%s", err, out)
	}
	for _, want := range []string{"T-picker-swap", "eff.swap-range", "req.picker-swap"} {
		if !strings.Contains(out, want) {
			t.Fatalf("search 'swap' --tag subject.picker missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range []string{"T-calendar-swap", "eff.swap-days", "subject.calendar"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("search 'swap' --tag subject.picker leaked out-of-scope %q:\n%s", notWant, out)
		}
	}
}

func TestSearch_TagFlagComposesWithType(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap", "--tag", "subject.picker", "--type", "transition")
	if err != nil {
		t.Fatalf("search --tag --type: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-picker-swap") {
		t.Fatalf("expected T-picker-swap under --type transition --tag subject.picker:\n%s", out)
	}
	for _, notWant := range []string{"T-calendar-swap", "vocab (", "decision (", "tag ("} {
		if strings.Contains(out, notWant) {
			t.Fatalf("--type transition --tag leaked %q:\n%s", notWant, out)
		}
	}
}

func TestSearch_TagFlagRepeatableIsOr(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap", "--tag", "subject.picker", "--tag", "subject.calendar")
	if err != nil {
		t.Fatalf("search repeated --tag: %v\n%s", err, out)
	}
	if !strings.Contains(out, "T-picker-swap") || !strings.Contains(out, "T-calendar-swap") {
		t.Fatalf("repeated --tag (OR) should keep both subtrees:\n%s", out)
	}
}

func TestSearch_TagFlagUnknownTagIsError(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	// Consistent with `list --tag` / `rules --tag`: a nonexistent tag is an error,
	// not a silent 0-hit (catches typos in the scope argument).
	if _, err := run(t, dir, "search", "swap", "--tag", "subject.nope"); err == nil {
		t.Fatalf("expected error for nonexistent --tag")
	}
}

func TestSearch_ShowsOwningSubjectsInline(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	// Every type's hits should carry a subject annotation for the right subtree.
	// T-picker-swap (transition) → subject.picker; T-calendar-swap → subject.calendar.
	for _, want := range []string{
		"T-picker-swap [id] T-picker-swap  · subject: subject.picker",
		"T-calendar-swap [id] T-calendar-swap  · subject: subject.calendar",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected inline subject annotation %q in:\n%s", want, out)
		}
	}
	// req.picker-swap (tag, requirement) rolls up to its subject ancestor.
	if !strings.Contains(out, "· subject: subject.picker") {
		t.Fatalf("expected a subject.picker annotation:\n%s", out)
	}
	// vocab reached only via transition (eff.swap-range) still shows the subject.
	if !strings.Contains(out, "eff.swap-range [id] eff.swap-range  · subject: subject.picker") {
		t.Fatalf("expected vocab via-transition subject annotation:\n%s", out)
	}
}

func TestSearch_TrailingMatchedSubjectsSummary(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap")
	if err != nil {
		t.Fatalf("search: %v\n%s", err, out)
	}
	if !strings.Contains(out, "matched subjects") {
		t.Fatalf("expected trailing matched-subjects summary:\n%s", out)
	}
	// Both subjects appear as --tag candidates with counts.
	for _, want := range []string{"subject.picker (", "subject.calendar ("} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected %q in matched-subjects summary:\n%s", want, out)
		}
	}
}

func TestSearch_SubjectsConsistentUnderTagScope(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap", "--tag", "subject.picker")
	if err != nil {
		t.Fatalf("search --tag: %v\n%s", err, out)
	}
	// Under --tag subject.picker, only picker subjects annotate rows, and the
	// summary must not offer subject.calendar as a candidate.
	if !strings.Contains(out, "· subject: subject.picker") {
		t.Fatalf("expected subject.picker annotations under --tag:\n%s", out)
	}
	if strings.Contains(out, "subject.calendar") {
		t.Fatalf("--tag subject.picker must not surface subject.calendar:\n%s", out)
	}
}

func TestSearch_JSONSubjectsFieldIsAdditive(t *testing.T) {
	dir := t.TempDir()
	seedTagScopeFixture(t, dir)

	out, err := run(t, dir, "search", "swap", "--json")
	if err != nil {
		t.Fatalf("search --json: %v\n%s", err, out)
	}
	var parsed struct {
		Matches []struct {
			Type     string   `json:"type"`
			ID       string   `json:"id"`
			Field    string   `json:"field"`
			Snippet  string   `json:"snippet"`
			Subjects []string `json:"subjects"`
		} `json:"matches"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, out)
	}
	if len(parsed.Matches) == 0 {
		t.Fatalf("expected matches:\n%s", out)
	}
	// Backward-compat: the original four fields still populate.
	for _, m := range parsed.Matches {
		if m.ID == "" || m.Field == "" || m.Snippet == "" || m.Type == "" {
			t.Fatalf("match missing a legacy field: %+v", m)
		}
	}
	// Additive: at least one transition hit carries its subject.
	found := false
	for _, m := range parsed.Matches {
		if m.ID == "T-picker-swap" {
			for _, s := range m.Subjects {
				if s == "subject.picker" {
					found = true
				}
			}
		}
	}
	if !found {
		t.Fatalf("expected T-picker-swap JSON match to include subjects:[subject.picker]:\n%s", out)
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
