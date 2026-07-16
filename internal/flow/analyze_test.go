package flow

import (
	"reflect"
	"sort"
	"testing"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
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
	// Given sets copied verbatim from .pmem/transitions/T-update-*.json —
	// deliberately under-qualified (not the design doc's fully-qualified
	// sub-cube rewrite), to prove the tool needs no re-authoring to catch it.
	txs := []model.Transition{
		{ID: "T-update-check", Action: "act.user.update", Given: []string{"cond.update-check-flag"}, Then: []string{"eff.log.update-check-report"}},
		{ID: "T-update-guide-source", Action: "act.user.update", Given: []string{"cond.install-source-goinstall"}, Then: []string{"eff.log.update-guide-goinstall"}},
		{ID: "T-update-guide-windows", Action: "act.user.update", Given: []string{"cond.platform-windows"}, Then: []string{"eff.log.update-guide-manual"}},
		{ID: "T-update-already-latest", Action: "act.user.update", Given: []string{"cond.install-release-binary", "cond.update-up-to-date"}, Then: []string{"eff.log.update-already-latest"}},
		{ID: "T-update-self-replace", Action: "act.user.update", Given: []string{"cond.install-release-binary", "cond.platform-unix-self-replaceable", "cond.update-available"}, Then: []string{"eff.http.download-release"}},
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
		if set["T-update-guide-windows"] && set["T-update-already-latest"] {
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
	// the real .pmem, where it does not exist at all yet — see
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
