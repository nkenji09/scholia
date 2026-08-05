package cli

import (
	"strings"
	"testing"
)

func TestGaps_TextOutputOmitsMatrixButKeepsGapsAndScope(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a")
	mustRun(t, dir, "vocab", "add", "condition", "cond.x", "--label", "x")
	mustRun(t, dir, "vocab", "add", "condition", "cond.y", "--label", "y")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "eff-a", "--kind", "log")
	mustRun(t, dir, "vocab", "add", "effect", "eff.b", "--label", "eff-b", "--kind", "log")
	mustRun(t, dir, "tx", "add", "T-general", "--action", "act.a", "--given", "cond.x", "--then", "eff.a")
	mustRun(t, dir, "tx", "add", "T-specific", "--action", "act.a", "--given", "cond.x,cond.y", "--then", "eff.b")

	out, err := run(t, dir, "gaps", "act.a")
	if err != nil {
		t.Fatalf("gaps: %v\n%s", err, out)
	}
	for _, want := range []string{"subset-shadow", "T-general ⊊ T-specific", "抜け（L-total", "重なり（宣言軸", "scope-disclosure"} {
		if !strings.Contains(out, want) {
			t.Fatalf("gaps output missing %q:\n%s", want, out)
		}
	}
	for _, notWant := range []string{"マトリクス（可視化", "宣言軸: 0 件（この action", "## acknowledged-remainder"} {
		if strings.Contains(out, notWant) {
			t.Fatalf("gaps output must be gap-only, unexpectedly contains %q:\n%s", notWant, out)
		}
	}
}

func TestGaps_TextOutputNeverBareNoGapsEvenWithZeroFindings(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a")
	mustRun(t, dir, "vocab", "add", "condition", "cond.x", "--label", "x")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "eff-a", "--kind", "log")
	mustRun(t, dir, "tx", "add", "T-1", "--action", "act.a", "--given", "cond.x", "--then", "eff.a")

	out, err := run(t, dir, "gaps", "act.a")
	if err != nil {
		t.Fatalf("gaps: %v\n%s", err, out)
	}
	for _, want := range []string{"subset-shadow（証明可能な重複）: 0 件", "抜け（L-total・唯一 clean に sound）: 0 件", "重なり（宣言軸に相対的に sound な ambiguity）: 0 件", "scope-disclosure（保証の外・削れない必須項目）", "列挙した軸", "don't-care 扱いの条件"} {
		if !strings.Contains(out, want) {
			t.Fatalf("gaps output with zero findings must still print scope-disclosure and section counts, missing %q:\n%s", want, out)
		}
	}
}

func TestGaps_JSONOutputRoundTripsGapFieldsButOmitsMatrix(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "config", "set", "tagKinds", "requirement,concern,subject,axis")
	mustRun(t, dir, "tag", "create", "axis.mode", "--name", "mode", "--kind", "axis", "--total")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a")
	mustRun(t, dir, "vocab", "add", "condition", "cond.check", "--label", "check")
	mustRun(t, dir, "vocab", "add", "condition", "cond.apply", "--label", "apply")
	mustRun(t, dir, "vocab", "tag", "cond.check", "--add", "axis.mode")
	mustRun(t, dir, "vocab", "tag", "cond.apply", "--add", "axis.mode")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "eff-a", "--kind", "log")
	mustRun(t, dir, "tx", "add", "T-check", "--action", "act.a", "--given", "cond.check", "--then", "eff.a")

	out, err := run(t, dir, "gaps", "act.a", "--json")
	if err != nil {
		t.Fatalf("gaps --json: %v\n%s", err, out)
	}
	for _, want := range []string{`"axisId":"axis.mode"`, `"value":"cond.apply"`, `"declaredAxes"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("gaps --json output missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, `"matrix"`) {
		t.Fatalf("gaps --json output must not include the full matrix:\n%s", out)
	}
}

func TestGaps_UnknownActionYieldsEmptyGapsNotError(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	out, err := run(t, dir, "gaps", "act.does-not-exist")
	if err != nil {
		t.Fatalf("gaps on unknown action should not error (empty report), got: %v\n%s", err, out)
	}
	if !strings.Contains(out, "scope-disclosure") {
		t.Fatalf("expected scope-disclosure even for unknown action, got:\n%s", out)
	}
}
