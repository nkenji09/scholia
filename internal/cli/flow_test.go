package cli

import (
	"strings"
	"testing"
)

func TestFlow_TextOutputListsMatrixSubsetShadowAndScope(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")
	mustRun(t, dir, "vocab", "add", "action", "act.a", "--label", "a")
	mustRun(t, dir, "vocab", "add", "condition", "cond.x", "--label", "x")
	mustRun(t, dir, "vocab", "add", "condition", "cond.y", "--label", "y")
	mustRun(t, dir, "vocab", "add", "effect", "eff.a", "--label", "eff-a", "--kind", "log")
	mustRun(t, dir, "vocab", "add", "effect", "eff.b", "--label", "eff-b", "--kind", "log")
	mustRun(t, dir, "tx", "add", "T-general", "--action", "act.a", "--given", "cond.x", "--then", "eff.a")
	mustRun(t, dir, "tx", "add", "T-specific", "--action", "act.a", "--given", "cond.x,cond.y", "--then", "eff.b")

	out, err := run(t, dir, "flow", "act.a")
	if err != nil {
		t.Fatalf("flow: %v\n%s", err, out)
	}
	for _, want := range []string{"T-general", "T-specific", "subset-shadow", "T-general ⊊ T-specific", "scope-disclosure", "宣言軸: 0"} {
		if !strings.Contains(out, want) {
			t.Fatalf("flow output missing %q:\n%s", want, out)
		}
	}
}

func TestFlow_JSONOutputRoundTripsTotalGapAndOverlap(t *testing.T) {
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

	out, err := run(t, dir, "flow", "act.a", "--json")
	if err != nil {
		t.Fatalf("flow --json: %v\n%s", err, out)
	}
	for _, want := range []string{`"axisId":"axis.mode"`, `"value":"cond.apply"`, `"declaredAxes"`} {
		if !strings.Contains(out, want) {
			t.Fatalf("flow --json output missing %q:\n%s", want, out)
		}
	}
}

func TestFlow_UnknownActionYieldsEmptyMatrixNotError(t *testing.T) {
	dir := t.TempDir()
	mustRun(t, dir, "init")

	out, err := run(t, dir, "flow", "act.does-not-exist")
	if err != nil {
		t.Fatalf("flow on unknown action should not error (empty report), got: %v\n%s", err, out)
	}
	if !strings.Contains(out, "この action を持つ遷移はありません") {
		t.Fatalf("expected empty-matrix message, got:\n%s", out)
	}
}
