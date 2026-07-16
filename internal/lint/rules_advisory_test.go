package lint

import (
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func TestRequirementGapRedAndGreen(t *testing.T) {
	cfg := model.DefaultConfig() // traceabilityKinds = ["requirement"]

	red := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.auth", Name: "auth", Kind: "requirement"}},
	}
	got := checkRequirementGap(red)
	if !hasRule(got, "requirement-gap") {
		t.Fatalf("expected requirement-gap finding for uncovered requirement tag, got %+v", got)
	}
	for _, f := range got {
		if f.Severity != SeverityWarn {
			t.Fatalf("requirement-gap must be warn severity, got %s", f.Severity)
		}
	}

	green := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.auth", Name: "auth", Kind: "requirement"}},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"req.auth"}},
		},
	}
	if got := checkRequirementGap(green); hasRule(got, "requirement-gap") {
		t.Fatalf("expected no requirement-gap finding once a transition carries the tag, got %+v", got)
	}
}

func TestRequirementGapCoversViaAncestorAndVocabPath(t *testing.T) {
	cfg := model.DefaultConfig()
	snap := store.Snapshot{
		Config: cfg,
		Tags: []model.Tag{
			{ID: "req.auth", Name: "auth", Kind: "requirement"},
			{ID: "req.auth.happy", Name: "happy", Kind: "requirement", ParentIDs: []string{"req.auth"}},
		},
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a"},
		},
		Transitions: []model.Transition{
			// carries only the child tag; req.auth must still be considered
			// covered because ancestor expansion is part of effective tags.
			{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"req.auth.happy"}},
		},
	}
	if got := checkRequirementGap(snap); hasRule(got, "requirement-gap") {
		t.Fatalf("expected ancestor tag req.auth to be covered via child's effective tags, got %+v", got)
	}
}

func TestKindMissingRedAndGreen(t *testing.T) {
	red := store.Snapshot{
		Tags: []model.Tag{{ID: "t.orphan", Name: "orphan"}}, // Kind == ""
	}
	got := checkKindMissing(red)
	if len(got) != 1 || got[0].Target != "t.orphan" {
		t.Fatalf("expected kind-missing finding for null-kind tag, got %+v", got)
	}
	if got[0].Severity != SeverityWarn {
		t.Fatalf("kind-missing must be warn severity, got %s", got[0].Severity)
	}

	green := store.Snapshot{
		Tags: []model.Tag{{ID: "t.typed", Name: "typed", Kind: "concern"}},
	}
	if got := checkKindMissing(green); hasRule(got, "kind-missing") {
		t.Fatalf("did not expect kind-missing finding for a tag with kind set, got %+v", got)
	}
}

func TestRefFreshnessRedAndGreen(t *testing.T) {
	fileLine := store.Snapshot{
		Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "tag", ID: "t"}, Why: "w", Ref: "foo.go:42", At: "2026-01-01T00:00:00Z"}},
	}
	if got := checkRefFreshness(fileLine); !hasRule(got, "ref-freshness") {
		t.Fatalf("expected ref-freshness finding for file:line ref, got %+v", got)
	}

	for _, ref := range []string{"https://example.com/pull/42", "PR#42", "a1b2c3d", ""} {
		green := store.Snapshot{
			Decisions: []model.Decision{{ID: "d1", Target: model.DecisionTarget{Type: "tag", ID: "t"}, Why: "w", Ref: ref, At: "2026-01-01T00:00:00Z"}},
		}
		if got := checkRefFreshness(green); hasRule(got, "ref-freshness") {
			t.Fatalf("did not expect ref-freshness finding for ref %q, got %+v", ref, got)
		}
	}
}

func TestDecisionCoverageInfo(t *testing.T) {
	snap := store.Snapshot{
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}},
			{ID: "T-2", Action: "act.a", Then: []string{"eff.a"}},
		},
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-1"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	got := checkDecisionCoverage(snap)
	if len(got) != 1 || got[0].Target != "T-2" {
		t.Fatalf("expected decision-coverage finding only for T-2, got %+v", got)
	}
	if got[0].Severity != SeverityInfo {
		t.Fatalf("decision-coverage must be info severity, got %s", got[0].Severity)
	}
}

func TestUnusedVocabInfo(t *testing.T) {
	snap := store.Snapshot{
		Vocab: []model.VocabEntry{
			{ID: "act.used", Category: model.CategoryAction, Label: "used"},
			{ID: "act.unused", Category: model.CategoryAction, Label: "unused"},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.used", Then: []string{"eff.a"}},
		},
	}
	got := checkUnusedVocab(snap)
	if len(got) != 1 || got[0].Target != "act.unused" {
		t.Fatalf("expected unused-vocab finding only for act.unused, got %+v", got)
	}
}

func TestExclusiveViolationRedAndGreen(t *testing.T) {
	axis := model.Tag{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}
	vocab := []model.VocabEntry{
		{ID: "cond.check", Category: model.CategoryCondition, Label: "check", Tags: []string{"axis.mode"}},
		{ID: "cond.apply", Category: model.CategoryCondition, Label: "apply", Tags: []string{"axis.mode"}},
	}

	red := store.Snapshot{
		Tags:  []model.Tag{axis},
		Vocab: vocab,
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.check", "cond.apply"}, Then: []string{"eff.a"}},
		},
	}
	got := checkExclusiveViolation(red)
	if len(got) != 1 || got[0].Target != "T-1" {
		t.Fatalf("expected exclusive-violation for T-1 (same axis, 2 values in one given), got %+v", got)
	}
	if got[0].Severity != SeverityWarn {
		t.Fatalf("exclusive-violation must be warn severity, got %s", got[0].Severity)
	}

	green := store.Snapshot{
		Tags:  []model.Tag{axis},
		Vocab: vocab,
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}},
			{ID: "T-2", Action: "act.a", Given: []string{"cond.apply"}, Then: []string{"eff.b"}},
		},
	}
	if got := checkExclusiveViolation(green); hasRule(got, "exclusive-violation") {
		t.Fatalf("did not expect exclusive-violation when each given pins at most one axis value, got %+v", got)
	}
}

func TestExclusiveViolationNoAxisTagsIsNoOp(t *testing.T) {
	snap := store.Snapshot{
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.a", "cond.b"}, Then: []string{"eff.a"}},
		},
	}
	if got := checkExclusiveViolation(snap); len(got) != 0 {
		t.Fatalf("expected no findings without any axis tag declared, got %+v", got)
	}
}

func TestComplementMissingRedAndGreen(t *testing.T) {
	red := store.Snapshot{
		Tags: []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}},
		Vocab: []model.VocabEntry{
			{ID: "cond.check", Category: model.CategoryCondition, Label: "check", Tags: []string{"axis.mode"}},
		},
	}
	got := checkComplementMissing(red)
	if len(got) != 1 || got[0].Target != "axis.mode" {
		t.Fatalf("expected complement-missing for axis.mode (only 1 materialized value), got %+v", got)
	}
	if got[0].Severity != SeverityWarn {
		t.Fatalf("complement-missing must be warn severity, got %s", got[0].Severity)
	}

	green := store.Snapshot{
		Tags: []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}},
		Vocab: []model.VocabEntry{
			{ID: "cond.check", Category: model.CategoryCondition, Label: "check", Tags: []string{"axis.mode"}},
			{ID: "cond.apply", Category: model.CategoryCondition, Label: "apply", Tags: []string{"axis.mode"}},
		},
	}
	if got := checkComplementMissing(green); hasRule(got, "complement-missing") {
		t.Fatalf("did not expect complement-missing once 2 values are materialized, got %+v", got)
	}
}

func TestComplementMissingIgnoresNonTotalAxis(t *testing.T) {
	snap := store.Snapshot{
		Tags: []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: false}},
		Vocab: []model.VocabEntry{
			{ID: "cond.check", Category: model.CategoryCondition, Label: "check", Tags: []string{"axis.mode"}},
		},
	}
	if got := checkComplementMissing(snap); hasRule(got, "complement-missing") {
		t.Fatalf("non-total axis must not trigger complement-missing, got %+v", got)
	}
}

func TestAdvisoryRulesDoNotAffectHasError(t *testing.T) {
	snap := store.Snapshot{
		Config: model.DefaultConfig(),
		Tags:   []model.Tag{{ID: "req.auth", Name: "auth", Kind: "requirement"}}, // triggers requirement-gap (warn)
	}
	got := Run(snap)
	if !hasRule(got, "requirement-gap") {
		t.Fatalf("expected requirement-gap to fire, got %+v", got)
	}
	if HasError(got) {
		t.Fatalf("warn/info findings must not make HasError true, got %+v", got)
	}
}
