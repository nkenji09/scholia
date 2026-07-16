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
	WriteText(&buf, r)
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
	WriteText(&buf, r)
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
	WriteText(&buf, r)
	out := buf.String()

	if !strings.Contains(out, "this action に効いていません") {
		t.Fatalf("case (b) output missing hint that the axis doesn't reach this action:\n%s", out)
	}
	if strings.Contains(out, "tag create --kind axis") {
		t.Fatalf("case (b) output must not use case (a)'s wording:\n%s", out)
	}
}
