package flow

import (
	"reflect"
	"sort"
	"testing"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func condVocab(id string, tags ...string) model.VocabEntry {
	return model.VocabEntry{ID: id, Category: model.CategoryCondition, Label: id, Tags: tags}
}

func buildAnalyzeFixture(txs []model.Transition, vocab []model.VocabEntry, tags []model.Tag) (*store.Snapshot, *index.Index) {
	snap := &store.Snapshot{
		Config:      model.DefaultConfig(),
		Vocab:       append([]model.VocabEntry{{ID: "act.a", Category: model.CategoryAction, Label: "a"}}, vocab...),
		Tags:        tags,
		Transitions: txs,
	}
	return snap, index.Build(snap)
}

func TestAnalyze_MatrixListsAllTransitionsAndConditionsNoInference(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
		{ID: "T-2", Action: "act.a", Given: []string{"cond.y"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)

	r := Analyze(snap, ix, "act.a")
	if !reflect.DeepEqual(r.Matrix.Conditions, []string{"cond.x", "cond.y"}) {
		t.Fatalf("Matrix.Conditions = %v", r.Matrix.Conditions)
	}
	if len(r.Matrix.Rows) != 2 {
		t.Fatalf("Matrix.Rows = %+v, want 2 rows", r.Matrix.Rows)
	}
}

func TestAnalyze_SubsetShadowDetectsProperSubsetPair(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-general", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
		{ID: "T-specific", Action: "act.a", Given: []string{"cond.x", "cond.y"}, Then: []string{"eff.b"}},
		{ID: "T-unrelated", Action: "act.a", Given: []string{"cond.z"}, Then: []string{"eff.c"}},
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 1 {
		t.Fatalf("SubsetShadows = %+v, want exactly 1", r.SubsetShadows)
	}
	got := r.SubsetShadows[0]
	if got.Subset != "T-general" || got.Superset != "T-specific" {
		t.Fatalf("SubsetShadows[0] = %+v, want Subset=T-general Superset=T-specific", got)
	}
}

func TestAnalyze_SubsetShadowIgnoresEqualAndDisjointGivenSets(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.x", "cond.y"}, Then: []string{"eff.a"}},
		{ID: "T-2", Action: "act.a", Given: []string{"cond.y", "cond.x"}, Then: []string{"eff.b"}}, // same set, different order
		{ID: "T-3", Action: "act.a", Given: []string{"cond.z"}, Then: []string{"eff.c"}},           // disjoint
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 0 {
		t.Fatalf("expected no subset-shadow for equal/disjoint given sets, got %+v", r.SubsetShadows)
	}
}

func TestAnalyze_NoAxisTagsMeansNoAxisAnalysisButScopeAlwaysPresent(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, []model.VocabEntry{condVocab("cond.x")}, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.Axes) != 0 || len(r.Cells) != 0 || len(r.TotalGaps) != 0 || len(r.Overlaps) != 0 {
		t.Fatalf("expected no axis-derived signals without axis tags, got %+v", r)
	}
	if reflect.DeepEqual(r.Scope, ScopeDisclosure{}) {
		t.Fatalf("scope-disclosure must never be the zero value (no bare 'no gaps'): %+v", r.Scope)
	}
	if len(r.Scope.OutOfGuarantee) == 0 {
		t.Fatalf("scope-disclosure must always print what is out of guarantee")
	}
	if !reflect.DeepEqual(r.Scope.UndeclaredGiven, []string{"cond.x"}) {
		t.Fatalf("Scope.UndeclaredGiven = %v, want [cond.x] (used but not axis-tagged)", r.Scope.UndeclaredGiven)
	}
}

// TestAnalyze_AxesAbsenceNoneDeclaredWhenStoreHasNoAxisTagsAtAll reproduces
// #40 ①'s case (a): the store has zero kind="axis" tags anywhere, so the
// axis mechanism itself hasn't been introduced — distinct from case (b)
// below (eff.emit.scope-disclosure).
func TestAnalyze_AxesAbsenceNoneDeclaredWhenStoreHasNoAxisTagsAtAll(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, []model.VocabEntry{condVocab("cond.x")}, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.Axes) != 0 {
		t.Fatalf("expected no axes, got %+v", r.Axes)
	}
	if r.AxesAbsence != AxesAbsenceNoneDeclared {
		t.Fatalf("AxesAbsence = %q, want %q (no kind=\"axis\" tag exists anywhere in the store)", r.AxesAbsence, AxesAbsenceNoneDeclared)
	}
}

// TestAnalyze_AxesAbsenceNotOnThisActionWhenAxisTagsExistButUnreachable
// reproduces #40 ①'s case (b): the store DOES have kind="axis" tags, but the
// analyzed action's transitions never give a condition that carries one —
// the axis exists but does not reach this action.
func TestAnalyze_AxesAbsenceNotOnThisActionWhenAxisTagsExistButUnreachable(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true},
	}
	// axis.mode is declared and tags cond.other, but act.a's own transition
	// never gives a condition carrying axis.mode.
	vocab := []model.VocabEntry{
		condVocab("cond.other", "axis.mode"),
		condVocab("cond.x"),
	}
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.Axes) != 0 {
		t.Fatalf("expected no axes relevant to act.a, got %+v", r.Axes)
	}
	if r.AxesAbsence != AxesAbsenceNotOnThisAction {
		t.Fatalf("AxesAbsence = %q, want %q (axis.mode is declared but never reaches act.a's given)", r.AxesAbsence, AxesAbsenceNotOnThisAction)
	}
}

func TestAnalyze_TotalAxisGapWhenAValueNeverAppearsInGiven(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"),
	}
	txs := []model.Transition{
		{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.Axes) != 1 || r.Axes[0].ID != "axis.mode" {
		t.Fatalf("expected axis.mode to be relevant, got %+v", r.Axes)
	}
	if len(r.TotalGaps) != 1 || r.TotalGaps[0].Value != "cond.apply" {
		t.Fatalf("expected total-gap for cond.apply (never in any given), got %+v", r.TotalGaps)
	}
}

// #45 D6: total-gap の typed 容認——欠落軸タグ宛て decision が total-gap を
// acknowledge していれば AcknowledgedBy が付く（欠落値 condition 宛てでも同様）。
func TestAnalyze_TotalGapTypedAcceptance(t *testing.T) {
	tags := []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}}
	vocab := []model.VocabEntry{condVocab("cond.check", "axis.mode"), condVocab("cond.apply", "axis.mode")}
	txs := []model.Transition{{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}}}

	// 軸タグ宛て decision が total-gap を acknowledge → 容認済み。
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)
	snap.Decisions = []model.Decision{
		{ID: "01AXISACK", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "axis.mode"},
			Why: "cond.apply は意図した placeholder（rm しない）", At: "2026-01-01T00:00:00Z",
			Acknowledges: []string{"total-gap"}},
	}
	r := Analyze(snap, ix, "act.a")
	if len(r.TotalGaps) != 1 || r.TotalGaps[0].AcknowledgedBy != "01AXISACK" {
		t.Fatalf("軸タグ宛て total-gap 容認が畳まれていない: %+v", r.TotalGaps)
	}

	// 欠落値 condition 宛て decision（vocab target）でも畳む。
	snap2, ix2 := buildAnalyzeFixture(txs, vocab, tags)
	snap2.Decisions = []model.Decision{
		{ID: "01VOCABACK", Target: model.DecisionTarget{Type: model.DecisionTargetVocab, ID: "cond.apply"},
			Why: "この値は given に出さない", At: "2026-01-01T00:00:00Z",
			Acknowledges: []string{"total-gap"}},
	}
	r2 := Analyze(snap2, ix2, "act.a")
	if len(r2.TotalGaps) != 1 || r2.TotalGaps[0].AcknowledgedBy != "01VOCABACK" {
		t.Fatalf("欠落値 condition 宛て total-gap 容認が畳まれていない: %+v", r2.TotalGaps)
	}

	// 無関係 rule を acknowledge する decision では畳まない。
	snap3, ix3 := buildAnalyzeFixture(txs, vocab, tags)
	snap3.Decisions = []model.Decision{
		{ID: "01OTHER", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "axis.mode"},
			Why: "別の話", At: "2026-01-01T00:00:00Z", Acknowledges: []string{"overlap"}},
	}
	r3 := Analyze(snap3, ix3, "act.a")
	if len(r3.TotalGaps) != 1 || r3.TotalGaps[0].AcknowledgedBy != "" {
		t.Fatalf("total-gap を acknowledge しない decision で畳んではいけない: %+v", r3.TotalGaps)
	}
}

// #45 D6: subset-shadow の typed 容認——ペアのいずれかの transition 宛て decision
// が subset-shadow を acknowledge していれば畳む。
func TestAnalyze_SubsetShadowTypedAcceptance(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-general", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}},
		{ID: "T-specific", Action: "act.a", Given: []string{"cond.x", "cond.y"}, Then: []string{"eff.b"}},
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)
	snap.Decisions = []model.Decision{
		{ID: "01SHADOWACK", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-specific"},
			Why: "この多重発火は意図的（specific 優先の想定）", At: "2026-01-01T00:00:00Z",
			Acknowledges: []string{"subset-shadow"}},
	}
	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 1 || r.SubsetShadows[0].AcknowledgedBy != "01SHADOWACK" {
		t.Fatalf("ペアの transition 宛て subset-shadow 容認が畳まれていない: %+v", r.SubsetShadows)
	}
}

func TestAnalyze_NonTotalAxisNeverReportsGap(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: false},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"),
	}
	txs := []model.Transition{
		{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.TotalGaps) != 0 {
		t.Fatalf("non-total axis must never report a gap, got %+v", r.TotalGaps)
	}
}

// TestAnalyze_FlagshipUpdateReconstruction reproduces design-options §7.3's
// worked proof: act.user.update's 5 real transitions, unmodified given sets,
// with 4 axes declared (install/platform/mode/status) purely via axis tags
// on the *existing* condition vocab — no re-authoring of any transition.
// This is the fixture-based dogfooding the #39 implementation handoff calls
// for: the tool must surface, on its own, the guide-windows vs
// already-latest multi-coverage the design review discovered by hand.
func TestAnalyze_FlagshipUpdateReconstructionSurfacesKnownOverlap(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.install", Name: "install 経路", Kind: "axis", Total: true},
		{ID: "axis.platform", Name: "platform", Kind: "axis", Total: true},
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true},
		{ID: "axis.status", Name: "status", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.update-check-flag", "axis.mode"),
		condVocab("cond.update-apply", "axis.mode"),
		condVocab("cond.install-release-binary", "axis.install"),
		condVocab("cond.install-source-goinstall", "axis.install"),
		condVocab("cond.platform-windows", "axis.platform"),
		condVocab("cond.platform-unix-self-replaceable", "axis.platform"),
		condVocab("cond.update-up-to-date", "axis.status"),
		condVocab("cond.update-available", "axis.status"),
	}
	// Given sets copied verbatim from .scholia/transitions/T-update-*.json —
	// deliberately under-qualified (not the design doc's fully-qualified
	// sub-cube rewrite), to prove the tool needs no re-authoring to catch it.
	txs := []model.Transition{
		{ID: "tx.update.check", Action: "act.user.update", Given: []string{"cond.update-check-flag"}, Then: []string{"eff.log.update-check-report"}},
		{ID: "tx.update.guide-source", Action: "act.user.update", Given: []string{"cond.install-source-goinstall"}, Then: []string{"eff.log.update-guide-goinstall"}},
		{ID: "tx.update.guide-windows", Action: "act.user.update", Given: []string{"cond.platform-windows"}, Then: []string{"eff.log.update-guide-manual"}},
		{ID: "tx.update.already-latest", Action: "act.user.update", Given: []string{"cond.install-release-binary", "cond.update-up-to-date"}, Then: []string{"eff.log.update-already-latest"}},
		{ID: "tx.update.self-replace", Action: "act.user.update", Given: []string{"cond.install-release-binary", "cond.platform-unix-self-replaceable", "cond.update-available"}, Then: []string{"eff.http.download-release"}},
	}
	snap := &store.Snapshot{
		Config: model.DefaultConfig(),
		Vocab: append([]model.VocabEntry{
			{ID: "act.user.update", Category: model.CategoryAction, Label: "update"},
		}, vocab...),
		Tags:        tags,
		Transitions: txs,
	}
	ix := index.Build(snap)

	r := Analyze(snap, ix, "act.user.update")

	if len(r.Axes) != 4 {
		t.Fatalf("expected all 4 axes relevant, got %+v", r.Axes)
	}

	foundFlagshipOverlap := false
	for _, o := range r.Overlaps {
		set := map[string]bool{}
		for _, id := range o.Transitions {
			set[id] = true
		}
		if set["tx.update.guide-windows"] && set["tx.update.already-latest"] {
			foundFlagshipOverlap = true
			if o.Cell["axis.platform"] != "cond.platform-windows" {
				t.Fatalf("overlap cell should pin platform=windows, got %+v", o.Cell)
			}
			if o.Cell["axis.install"] != "cond.install-release-binary" {
				t.Fatalf("overlap cell should pin install=release-binary, got %+v", o.Cell)
			}
			if o.Cell["axis.status"] != "cond.update-up-to-date" {
				t.Fatalf("overlap cell should pin status=up-to-date, got %+v", o.Cell)
			}
		}
	}
	if !foundFlagshipOverlap {
		t.Fatalf("expected tool to surface the known guide-windows/already-latest multi-coverage, got overlaps=%+v", r.Overlaps)
	}

	// This fixture materializes cond.update-apply as a vocab entry (unlike
	// the real .scholia, where it does not exist at all yet — see
	// axis.update.mode.json's description). With it materialized, the flow
	// engine's L-total signal correctly fires: mode is total=true and no
	// real update transition's given ever names cond.update-apply — the
	// exact authoring gap design-options identified by hand (§0/§6.3-1's
	// "破綻点2"). install/platform/status each have both real values used
	// somewhere across the 5 transitions, so they report no gap.
	if len(r.TotalGaps) != 1 || r.TotalGaps[0].AxisID != "axis.mode" || r.TotalGaps[0].Value != "cond.update-apply" {
		t.Fatalf("expected exactly the known cond.update-apply total-gap, got %+v", r.TotalGaps)
	}
}

func TestAnalyze_AcknowledgedRemainderExcludedFromCoverageAndReportedSeparately(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"),
	}
	txs := []model.Transition{
		{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}},
		{ID: "T-fallback", Action: "act.a", Given: nil, Then: []string{"eff.z"}, Tags: []string{RemainderTagID}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.Remainder) != 1 || r.Remainder[0].TransitionID != "T-fallback" {
		t.Fatalf("expected T-fallback reported as remainder, got %+v", r.Remainder)
	}
	if !r.Scope.HasRemainder {
		t.Fatalf("scope must acknowledge a remainder is present")
	}
	// cond.apply is still a real, unresolved total-gap: the remainder must
	// NOT silently absorb it and turn this into a false "no gaps".
	if len(r.TotalGaps) != 1 || r.TotalGaps[0].Value != "cond.apply" {
		t.Fatalf("acknowledged-remainder must not suppress the real total-gap, got %+v", r.TotalGaps)
	}
	for _, o := range r.Overlaps {
		for _, id := range o.Transitions {
			if id == "T-fallback" {
				t.Fatalf("remainder transition must never appear in coverage/overlap accounting, got %+v", r.Overlaps)
			}
		}
	}
}

func TestAnalyze_OverlapExcludesPairsAlreadyExplainedBySubsetShadow(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"),
	}
	txs := []model.Transition{
		{ID: "T-general", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}},
		{ID: "T-specific", Action: "act.a", Given: []string{"cond.check", "cond.apply"}, Then: []string{"eff.b"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 1 {
		t.Fatalf("expected the pair to be reported as subset-shadow, got %+v", r.SubsetShadows)
	}
	for _, o := range r.Overlaps {
		if len(o.Transitions) == 2 {
			t.Fatalf("pair already explained by subset-shadow must not also be reported as overlap: %+v", o)
		}
	}
}

// TestAnalyze_OverlapSurvivesWhenOnlyOnePairIsSubsetShadow reproduces the
// #39 follow-up major fix: a cell covered by 3+ transitions where exactly
// one pair (A⊊B) is a subset-shadow and a third transition C is
// incomparable to both must still report the real, unexplained A↔C and
// B↔C ambiguity. Before the fix, dropping a transition whenever *any* one
// of its pairs was a shadow erased both A and B from the cell, leaving only
// C — fewer than 2 transitions — so the cell's overlap silently vanished
// even though world(a1, cond.x, cond.y) fires A, B, and C together.
func TestAnalyze_OverlapSurvivesWhenOnlyOnePairIsSubsetShadow(t *testing.T) {
	tags := []model.Tag{
		{ID: "axis.a", Name: "a", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.a1", "axis.a"), condVocab("cond.a2", "axis.a"),
		condVocab("cond.x"),
		condVocab("cond.y"),
	}
	txs := []model.Transition{
		{ID: "T-A", Action: "act.a", Given: []string{"cond.a1"}, Then: []string{"eff.a"}},
		{ID: "T-B", Action: "act.a", Given: []string{"cond.a1", "cond.x"}, Then: []string{"eff.b"}},
		{ID: "T-C", Action: "act.a", Given: []string{"cond.y"}, Then: []string{"eff.c"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")

	if len(r.SubsetShadows) != 1 || r.SubsetShadows[0].Subset != "T-A" || r.SubsetShadows[0].Superset != "T-B" {
		t.Fatalf("expected T-A ⊊ T-B subset-shadow, got %+v", r.SubsetShadows)
	}

	var cellOverlap *Overlap
	for i, o := range r.Overlaps {
		if o.Cell["axis.a"] == "cond.a1" {
			cellOverlap = &r.Overlaps[i]
		}
	}
	if cellOverlap == nil {
		t.Fatalf("expected overlap for cell axis.a=cond.a1 (A↔C and B↔C unexplained by the A⊊B shadow), got %+v", r.Overlaps)
	}
	set := map[string]bool{}
	for _, id := range cellOverlap.Transitions {
		set[id] = true
	}
	if !set["T-A"] || !set["T-B"] || !set["T-C"] {
		t.Fatalf("expected overlap to include T-A, T-B, T-C, got %+v", cellOverlap.Transitions)
	}
}

// TestAnalyze_SubsetShadowDetectsEmptyGivenAsProperSubset reproduces the
// #39 follow-up minor fix: isProperSubset used to short-circuit to false
// whenever the candidate subset's given was empty, missing that the empty
// set fires in every world and is therefore a proper subset of any
// non-empty given — a remainder-less transition with no given shadows every
// other transition of the action. Equal (both empty) given sets must still
// be excluded as not proper.
func TestAnalyze_SubsetShadowDetectsEmptyGivenAsProperSubset(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-always", Action: "act.a", Given: nil, Then: []string{"eff.a"}},
		{ID: "T-x", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.b"}},
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 1 {
		t.Fatalf("SubsetShadows = %+v, want exactly 1", r.SubsetShadows)
	}
	got := r.SubsetShadows[0]
	if got.Subset != "T-always" || got.Superset != "T-x" {
		t.Fatalf("SubsetShadows[0] = %+v, want Subset=T-always Superset=T-x", got)
	}
}

func TestAnalyze_SubsetShadowIgnoresTwoEmptyGivenSets(t *testing.T) {
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: nil, Then: []string{"eff.a"}},
		{ID: "T-2", Action: "act.a", Given: nil, Then: []string{"eff.b"}},
	}
	snap, ix := buildAnalyzeFixture(txs, nil, nil)

	r := Analyze(snap, ix, "act.a")
	if len(r.SubsetShadows) != 0 {
		t.Fatalf("two empty-given transitions are equal, not proper-subset: got %+v", r.SubsetShadows)
	}
}

func TestAnalyze_ProductCellsAreBoundedNotTwoToTheN(t *testing.T) {
	// 3 axes with 2 values each => 2*2*2 = 8 cells, never 2^(number of
	// individual conditions used, which here is 6) and never counting
	// same-axis co-occurrence as an extra dimension.
	tags := []model.Tag{
		{ID: "axis.a", Name: "a", Kind: "axis", Total: true},
		{ID: "axis.b", Name: "b", Kind: "axis", Total: true},
		{ID: "axis.c", Name: "c", Kind: "axis", Total: true},
	}
	vocab := []model.VocabEntry{
		condVocab("cond.a1", "axis.a"), condVocab("cond.a2", "axis.a"),
		condVocab("cond.b1", "axis.b"), condVocab("cond.b2", "axis.b"),
		condVocab("cond.c1", "axis.c"), condVocab("cond.c2", "axis.c"),
	}
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.a1", "cond.b1", "cond.c1"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if len(r.Cells) != 8 {
		t.Fatalf("Cells = %d, want 8 (2^3 axes, not 2^6 conditions)", len(r.Cells))
	}
}

func hasCondition(list []string, want string) bool {
	i := sort.SearchStrings(list, want)
	return i < len(list) && list[i] == want
}

func intp(n int) *int { return &n }

// #45 D8: an overlap whose involved transitions all carry distinct declared
// priorities is folded to Resolved (evaluation order settles the winner);
// any undeclared or duplicated priority leaves it unresolved. A third
// higher-priority transition (T-tail) keeps the action from being a
// 2-transition full-declaration action (which would fold one of the pair into
// the declarative remainder) — here it never covers cond.a1, so the
// contending pair on cond.a1 is what we assert on.
func TestAnalyze_OverlapResolvedOnlyWhenAllPrioritiesDistinct(t *testing.T) {
	tags := []model.Tag{{ID: "axis.a", Name: "a", Kind: "axis", Total: true}}
	vocab := []model.VocabEntry{
		condVocab("cond.a1", "axis.a"), condVocab("cond.a2", "axis.a"),
		condVocab("cond.x"), condVocab("cond.y"),
	}
	// T-1 and T-2 both cover cell axis.a=cond.a1, contending — an overlap.
	// T-tail pins axis.a=cond.a2, so it never enters the cond.a1 cell but its
	// undeclared/declared priority is irrelevant to that cell's resolution.
	base := func(p1, p2 *int) []model.Transition {
		return []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.a1", "cond.x"}, Then: []string{"eff.a"}, Priority: p1},
			{ID: "T-2", Action: "act.a", Given: []string{"cond.a1", "cond.y"}, Then: []string{"eff.b"}, Priority: p2},
			{ID: "T-tail", Action: "act.a", Given: []string{"cond.a2"}, Then: []string{"eff.c"}, Priority: intp(9)},
		}
	}
	cond := func(r Report) *Overlap {
		for i, o := range r.Overlaps {
			if o.Cell["axis.a"] == "cond.a1" && len(o.Transitions) >= 2 {
				return &r.Overlaps[i]
			}
		}
		return nil
	}

	// distinct priorities → resolved, winner = smaller priority.
	snap, ix := buildAnalyzeFixture(base(intp(1), intp(2)), vocab, tags)
	c := cond(Analyze(snap, ix, "act.a"))
	if c == nil {
		t.Fatalf("expected overlap on cell axis.a=cond.a1")
	}
	if !c.Resolved {
		t.Fatalf("distinct priorities must resolve the overlap, got %+v", c)
	}

	// same priority → NOT resolved (a real, still-undefined hole).
	snap2, ix2 := buildAnalyzeFixture(base(intp(1), intp(1)), vocab, tags)
	if c := cond(Analyze(snap2, ix2, "act.a")); c == nil || c.Resolved {
		t.Fatalf("same priority must NOT resolve the overlap: %+v", c)
	}

	// one undeclared → NOT resolved.
	snap3, ix3 := buildAnalyzeFixture(base(intp(1), nil), vocab, tags)
	if c := cond(Analyze(snap3, ix3, "act.a")); c == nil || c.Resolved {
		t.Fatalf("an undeclared priority must NOT resolve the overlap: %+v", c)
	}
}

// #45 D8: a resolved overlap carries the derived complement — each
// transition's effective given excludes the union of smaller-priority
// transitions' givens (the else derived from priority).
func TestAnalyze_ResolvedOverlapCarriesDerivedComplement(t *testing.T) {
	tags := []model.Tag{{ID: "axis.a", Name: "a", Kind: "axis", Total: true}}
	vocab := []model.VocabEntry{
		condVocab("cond.a1", "axis.a"), condVocab("cond.a2", "axis.a"),
		condVocab("cond.x"), condVocab("cond.y"),
	}
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.a1", "cond.x"}, Then: []string{"eff.a"}, Priority: intp(1)},
		{ID: "T-2", Action: "act.a", Given: []string{"cond.a1", "cond.y"}, Then: []string{"eff.b"}, Priority: intp(2)},
		{ID: "T-tail", Action: "act.a", Given: []string{"cond.a2"}, Then: []string{"eff.c"}, Priority: intp(9)},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)
	r := Analyze(snap, ix, "act.a")

	var contended *Overlap
	for i, o := range r.Overlaps {
		if o.Cell["axis.a"] == "cond.a1" && len(o.Transitions) >= 2 {
			contended = &r.Overlaps[i]
		}
	}
	if contended == nil || !contended.Resolved {
		t.Fatalf("expected a resolved overlap on cond.a1, got %+v", r.Overlaps)
	}
	if len(contended.EffectiveGiven) != 2 {
		t.Fatalf("expected 2 effective-given entries, got %+v", contended.EffectiveGiven)
	}
	// ordered by ascending priority: first entry (p1) excludes nothing.
	if contended.EffectiveGiven[0].TransitionID != "T-1" || len(contended.EffectiveGiven[0].Excludes) != 0 {
		t.Fatalf("first (p1) entry should be T-1 with no excludes, got %+v", contended.EffectiveGiven[0])
	}
	// second entry (p2) excludes T-1's given (cond.a1, cond.x).
	got := contended.EffectiveGiven[1]
	if got.TransitionID != "T-2" || !reflect.DeepEqual(got.Excludes, []string{"cond.a1", "cond.x"}) {
		t.Fatalf("second (p2) entry should be T-2 excluding [cond.a1 cond.x], got %+v", got)
	}
}

// #45 D8: a subset-shadow pair with distinct declared priorities folds to
// Resolved (the winner is well-defined); an undeclared/same-priority pair
// stays unconditionally reported.
func TestAnalyze_SubsetShadowResolvedOnDistinctPriorities(t *testing.T) {
	// A third transition (T-tail, unrelated disjoint given, highest priority)
	// keeps this from being a 2-transition full-declaration action whose tail
	// would fold into the declarative remainder and vanish from the pair.
	mk := func(p1, p2 *int) []model.Transition {
		return []model.Transition{
			{ID: "T-general", Action: "act.a", Given: []string{"cond.x"}, Then: []string{"eff.a"}, Priority: p1},
			{ID: "T-specific", Action: "act.a", Given: []string{"cond.x", "cond.y"}, Then: []string{"eff.b"}, Priority: p2},
			{ID: "T-tail", Action: "act.a", Given: []string{"cond.z"}, Then: []string{"eff.c"}, Priority: intp(9)},
		}
	}
	shadow := func(r Report) *SubsetShadow {
		for i := range r.SubsetShadows {
			s := &r.SubsetShadows[i]
			if s.Subset == "T-general" && s.Superset == "T-specific" {
				return s
			}
		}
		return nil
	}
	// distinct → resolved, winner is the smaller-priority transition.
	snap, ix := buildAnalyzeFixture(mk(intp(2), intp(1)), nil, nil)
	if s := shadow(Analyze(snap, ix, "act.a")); s == nil || !s.Resolved || s.Winner != "T-specific" {
		t.Fatalf("distinct priorities must resolve subset-shadow with winner=T-specific (p1), got %+v", s)
	}
	// undeclared → not resolved.
	snap2, ix2 := buildAnalyzeFixture(mk(nil, nil), nil, nil)
	if s := shadow(Analyze(snap2, ix2, "act.a")); s == nil || s.Resolved {
		t.Fatalf("undeclared priorities must NOT resolve subset-shadow, got %+v", s)
	}
}

// #45 D8/amend②: an action whose transitions ALL declare a priority exempts
// L-total — the last-evaluated transition is the declarative remainder that
// receives the otherwise-missing total axis value. Partial declaration must
// NOT exempt (the gap stays visible; a false "no gaps" is forbidden).
func TestAnalyze_DeclarativeRemainderExemptsLTotalOnlyOnFullDeclaration(t *testing.T) {
	tags := []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"), // never given → would be an L-total
	}
	// full declaration: both transitions have a priority → tail is declarative
	// remainder, exempting the cond.apply L-total.
	full := []model.Transition{
		{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}, Priority: intp(1)},
		{ID: "T-tail", Action: "act.a", Given: nil, Then: []string{"eff.z"}, Priority: intp(2)},
	}
	snap, ix := buildAnalyzeFixture(full, vocab, tags)
	r := Analyze(snap, ix, "act.a")
	if len(r.TotalGaps) != 0 {
		t.Fatalf("full-declaration action must exempt L-total via declarative remainder, got %+v", r.TotalGaps)
	}
	if len(r.Remainder) != 1 || r.Remainder[0].TransitionID != "T-tail" {
		t.Fatalf("T-tail (max priority) must be the declarative remainder, got %+v", r.Remainder)
	}

	// partial declaration: one transition has no priority → NO declarative
	// remainder, L-total stays visible.
	partial := []model.Transition{
		{ID: "T-check", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.a"}, Priority: intp(1)},
		{ID: "T-tail", Action: "act.a", Given: nil, Then: []string{"eff.z"}}, // no priority
	}
	snap2, ix2 := buildAnalyzeFixture(partial, vocab, tags)
	r2 := Analyze(snap2, ix2, "act.a")
	if len(r2.TotalGaps) != 1 || r2.TotalGaps[0].Value != "cond.apply" {
		t.Fatalf("partial declaration must NOT exempt L-total (gap stays visible), got %+v", r2.TotalGaps)
	}
}

func TestAnalyze_ScopeDisclosureListsDeclaredAxesAndDontCareConditions(t *testing.T) {
	tags := []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}}
	vocab := []model.VocabEntry{
		condVocab("cond.check", "axis.mode"),
		condVocab("cond.apply", "axis.mode"),
		condVocab("cond.verbose"), // no axis tag: a "free" flag, per design §6.3-3 §7.3
	}
	txs := []model.Transition{
		{ID: "T-1", Action: "act.a", Given: []string{"cond.check", "cond.verbose"}, Then: []string{"eff.a"}},
	}
	snap, ix := buildAnalyzeFixture(txs, vocab, tags)

	r := Analyze(snap, ix, "act.a")
	if !hasCondition(r.Scope.DeclaredAxes, "axis.mode") {
		t.Fatalf("Scope.DeclaredAxes = %v, want axis.mode", r.Scope.DeclaredAxes)
	}
	if !hasCondition(r.Scope.UndeclaredGiven, "cond.verbose") {
		t.Fatalf("Scope.UndeclaredGiven = %v, want cond.verbose (free flag outside any axis)", r.Scope.UndeclaredGiven)
	}
	if hasCondition(r.Scope.UndeclaredGiven, "cond.check") {
		t.Fatalf("cond.check carries an axis tag and must not be listed as undeclared: %v", r.Scope.UndeclaredGiven)
	}
}
