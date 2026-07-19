package flow

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteText_NeverOmitsScopeSectionEvenWithZeroFindings(t *testing.T) {
	r := Report{
		Action:      "act.a",
		ActionLabel: "a",
		Scope: ScopeDisclosure{
			OutOfGuarantee: disclosureBoilerplate,
		},
	}
	var buf bytes.Buffer
	WriteText(&buf, r, false)
	out := buf.String()

	for _, want := range []string{"subset-shadow", "宣言軸", "抜け", "重なり", "acknowledged-remainder", "scope-disclosure"} {
		if !strings.Contains(out, want) {
			t.Fatalf("WriteText output missing section %q:\n%s", want, out)
		}
	}
	// The mandated out-of-guarantee captions must always print, so a
	// zero-finding run cannot be misread as a bare "no gaps".
	for _, caption := range disclosureBoilerplate {
		if !strings.Contains(out, caption) {
			t.Fatalf("WriteText output missing scope caption %q:\n%s", caption, out)
		}
	}
}

// TestWriteText_AxesAbsenceNoneDeclaredGetsActionableHint reproduces #40 ①
// case (a)'s text output: the axis mechanism is wholly unused in this
// project, so the hint must point at creating an axis tag.
func TestWriteText_AxesAbsenceNoneDeclaredGetsActionableHint(t *testing.T) {
	r := Report{
		Action:      "act.a",
		ActionLabel: "a",
		AxesAbsence: AxesAbsenceNoneDeclared,
		Scope:       ScopeDisclosure{OutOfGuarantee: disclosureBoilerplate},
	}
	var buf bytes.Buffer
	WriteText(&buf, r, false)
	out := buf.String()

	if !strings.Contains(out, "tag create --kind axis") {
		t.Fatalf("case (a) output missing actionable hint to create an axis tag:\n%s", out)
	}
	if strings.Contains(out, "this action に効いていません") {
		t.Fatalf("case (a) output must not use case (b)'s wording:\n%s", out)
	}
}

// TestWriteText_AxesAbsenceNotOnThisActionGetsActionableHint reproduces #40
// ① case (b)'s text output: axis tags exist but don't reach this action, so
// the hint must point at splitting given conditions per axis value instead.
func TestWriteText_AxesAbsenceNotOnThisActionGetsActionableHint(t *testing.T) {
	r := Report{
		Action:      "act.a",
		ActionLabel: "a",
		AxesAbsence: AxesAbsenceNotOnThisAction,
		Scope:       ScopeDisclosure{OutOfGuarantee: disclosureBoilerplate},
	}
	var buf bytes.Buffer
	WriteText(&buf, r, false)
	out := buf.String()

	if !strings.Contains(out, "this action に効いていません") {
		t.Fatalf("case (b) output missing hint that the axis doesn't reach this action:\n%s", out)
	}
	if strings.Contains(out, "tag create --kind axis") {
		t.Fatalf("case (b) output must not use case (a)'s wording:\n%s", out)
	}
}

// #45 D8: resolved overlaps/subset-shadows are folded out of the default
// count; --verbose discloses them (with the derived complement). The
// evaluation-order relativity caveat is always printed regardless.
func TestWriteText_ResolvedFindingsHiddenByDefaultShownWithVerbose(t *testing.T) {
	r := Report{
		Action:      "act.a",
		ActionLabel: "a",
		SubsetShadows: []SubsetShadow{
			{Subset: "T-a", Superset: "T-b", Resolved: true, Winner: "T-b"},
		},
		Overlaps: []Overlap{
			{Cell: map[string]string{"axis.a": "cond.a1"}, Transitions: []string{"T-1", "T-2"}, Resolved: true,
				EffectiveGiven: []EffectiveGiven{
					{TransitionID: "T-1", Priority: 1, Given: []string{"cond.a1"}},
					{TransitionID: "T-2", Priority: 2, Given: []string{"cond.a1"}, Excludes: []string{"cond.a1"}},
				}},
		},
		Scope: ScopeDisclosure{OutOfGuarantee: disclosureBoilerplate},
	}

	// default: resolved findings are not listed as holes; counts are 0.
	var def bytes.Buffer
	WriteText(&def, r, false)
	defOut := def.String()
	if !strings.Contains(defOut, "subset-shadow（証明可能な重複）: 0 件") {
		t.Fatalf("default surface must count 0 unresolved subset-shadows:\n%s", defOut)
	}
	if !strings.Contains(defOut, "重なり（宣言軸に相対的に sound な ambiguity）: 0 件") {
		t.Fatalf("default surface must count 0 unresolved overlaps:\n%s", defOut)
	}
	if !strings.Contains(defOut, "評価順で解決済み: 1 件") {
		t.Fatalf("default surface must note resolved findings exist (pointer to --verbose):\n%s", defOut)
	}
	if strings.Contains(defOut, "実効 given") {
		t.Fatalf("default surface must NOT print the derived complement:\n%s", defOut)
	}
	// The always-on evaluation-order relativity caveat must be present.
	if !strings.Contains(defOut, "実装の if/else 順との一致は検証していません") {
		t.Fatalf("evaluation-order relativity caveat must always print:\n%s", defOut)
	}

	// verbose: resolved findings and derived complement disclosed.
	var vb bytes.Buffer
	WriteText(&vb, r, true)
	vbOut := vb.String()
	if !strings.Contains(vbOut, "[解決済み]") {
		t.Fatalf("verbose must disclose resolved findings:\n%s", vbOut)
	}
	if !strings.Contains(vbOut, "実効 given") {
		t.Fatalf("verbose must print the derived complement (実効 given):\n%s", vbOut)
	}
}
