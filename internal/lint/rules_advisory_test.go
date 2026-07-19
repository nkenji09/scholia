package lint

import (
	"strings"
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
		if !strings.Contains(f.Message, "direct decision 0 件") {
			t.Fatalf("requirement-gap message must carry the direct decision count, got %q", f.Message)
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

// 未充足のままでも direct decision を持つタグは、その件数が warn 行に併記される
// （併記は判断材料の提示であり、decision があっても warn は沈黙しない）。
func TestRequirementGapDirectDecisionCountAnnotated(t *testing.T) {
	cfg := model.DefaultConfig()
	snap := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.auth", Name: "auth", Kind: "requirement"}},
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.auth"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	got := checkRequirementGap(snap)
	if len(got) != 1 {
		t.Fatalf("expected requirement-gap to still warn (no untyped silencing), got %+v", got)
	}
	if !strings.Contains(got[0].Message, "direct decision 1 件") {
		t.Fatalf("expected direct decision count 1 in message, got %q", got[0].Message)
	}
}

// #45 D6: fulfillment=property のタグは遷移充足検査から外すが、
// acknowledges:[requirement-gap] を含む direct decision が無ければ warn のまま
// （怠慢な property 宣言を許さない・偽陰性ガード）。
func TestRequirementGapPropertyNeedsAcknowledgingDecision(t *testing.T) {
	cfg := model.DefaultConfig()
	// property 宣言のみ・decision 無し → warn（畳まない）。
	declOnly := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.standalone", Name: "standalone", Kind: "requirement", Fulfillment: model.FulfillmentProperty}},
	}
	got := checkRequirementGap(declOnly)
	if len(got) != 1 || got[0].AcknowledgedBy != "" {
		t.Fatalf("property 宣言のみ（decision 無し）は warn のまま・未容認のはず: %+v", got)
	}
	if !strings.Contains(got[0].Message, "fulfillment=property") {
		t.Fatalf("property の warn メッセージが専用文言でない: %q", got[0].Message)
	}

	// property + acknowledges:[requirement-gap] を含む direct decision → 容認済み。
	folded := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.standalone", Name: "standalone", Kind: "requirement", Fulfillment: model.FulfillmentProperty}},
		Decisions: []model.Decision{
			{ID: "01ACK", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.standalone"},
				Why: "単一バイナリは遷移で充足されない性質", At: "2026-01-01T00:00:00Z",
				Acknowledges: []string{"requirement-gap"}},
		},
	}
	got = checkRequirementGap(folded)
	if len(got) != 1 || got[0].AcknowledgedBy != "01ACK" {
		t.Fatalf("property + 容認 decision は AcknowledgedBy 付きで畳むはず: %+v", got)
	}
}

// #45 D6: 無関係な（requirement-gap を acknowledge しない）decision では畳まない
// ——untyped 容認の偽陰性を再導入しない。
func TestRequirementGapUnrelatedDecisionDoesNotFold(t *testing.T) {
	cfg := model.DefaultConfig()
	snap := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.standalone", Name: "standalone", Kind: "requirement", Fulfillment: model.FulfillmentProperty}},
		Decisions: []model.Decision{
			// 別 rule を acknowledge している decision（requirement-gap ではない）。
			{ID: "01OTHER", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.standalone"},
				Why: "別の話", At: "2026-01-01T00:00:00Z", Acknowledges: []string{"overlap"}},
		},
	}
	got := checkRequirementGap(snap)
	if len(got) != 1 || got[0].AcknowledgedBy != "" {
		t.Fatalf("requirement-gap を acknowledge しない decision では畳んではいけない: %+v", got)
	}
}

// #45 D6: transitions 型（fulfillment 未設定）のタグも、requirement-gap を
// acknowledge する direct decision があれば畳む（性質型でなくても typed 容認は効く）。
func TestRequirementGapTransitionsKindFoldsWithAcknowledge(t *testing.T) {
	cfg := model.DefaultConfig()
	snap := store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "req.auth", Name: "auth", Kind: "requirement"}}, // fulfillment 未設定
		Decisions: []model.Decision{
			{ID: "01ACK2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.auth"},
				Why: "意図的に未充足", At: "2026-01-01T00:00:00Z", Acknowledges: []string{"requirement-gap"}},
		},
	}
	got := checkRequirementGap(snap)
	if len(got) != 1 || got[0].AcknowledgedBy != "01ACK2" {
		t.Fatalf("transitions 型でも acknowledge があれば容認済みに畳むはず: %+v", got)
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

func TestDecisionCoverageThreeTiers(t *testing.T) {
	snap := store.Snapshot{
		Tags: []model.Tag{
			{ID: "req.parent", Name: "parent", Kind: "requirement"},
			{ID: "req.child", Name: "child", Kind: "requirement", ParentIDs: []string{"req.parent"}},
			{ID: "req.vocab", Name: "vocab", Kind: "requirement"},
		},
		Vocab: []model.VocabEntry{
			{ID: "act.plain", Category: model.CategoryAction, Label: "plain"},
			{ID: "act.tagged", Category: model.CategoryAction, Label: "tagged", Tags: []string{"req.vocab"}},
		},
		Transitions: []model.Transition{
			// direct: own decision を持つ。
			{ID: "T-direct", Action: "act.plain", Then: []string{"eff.a"}},
			// via-tag（祖先経由）: 子タグしか持たないが、親タグ宛 decision に祖先閉包で到達する。
			{ID: "T-via-ancestor", Action: "act.plain", Then: []string{"eff.a"}, Tags: []string{"req.child"}},
			// via-tag（vocab 経由）: own タグは無いが、参照する語彙のタグ宛 decision に到達する。
			{ID: "T-via-vocab", Action: "act.tagged", Then: []string{"eff.a"}},
			// none: own にも実効タグにも decision が無い。
			{ID: "T-none", Action: "act.plain", Then: []string{"eff.a"}},
		},
		Decisions: []model.Decision{
			{ID: "d-tx", Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-direct"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d-parent", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.parent"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d-vocab", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "req.vocab"}, Why: "w", At: "2026-01-01T00:00:00Z"},
		},
	}
	got := checkDecisionCoverage(snap)
	if len(got) != len(snap.Transitions) {
		t.Fatalf("expected one finding per transition (%d), got %d: %+v", len(snap.Transitions), len(got), got)
	}
	byTarget := make(map[string]Finding, len(got))
	for _, f := range got {
		if f.Severity != SeverityInfo {
			t.Fatalf("decision-coverage must be info severity, got %s for %s", f.Severity, f.Target)
		}
		byTarget[f.Target] = f
	}

	if f := byTarget["T-direct"]; f.Coverage != CoverageDirect {
		t.Fatalf("T-direct coverage = %q, want direct: %+v", f.Coverage, f)
	}
	if f := byTarget["T-via-ancestor"]; f.Coverage != CoverageViaTag || f.Detail != "via req.parent (1)" {
		t.Fatalf("T-via-ancestor coverage/detail = %q/%q, want via-tag / via req.parent (1): %+v", f.Coverage, f.Detail, f)
	}
	if f := byTarget["T-via-vocab"]; f.Coverage != CoverageViaTag || f.Detail != "via req.vocab (1)" {
		t.Fatalf("T-via-vocab coverage/detail = %q/%q, want via-tag / via req.vocab (1): %+v", f.Coverage, f.Detail, f)
	}
	if f := byTarget["T-none"]; f.Coverage != CoverageNone {
		t.Fatalf("T-none coverage = %q, want none: %+v", f.Coverage, f)
	}

	direct, viaTag, none := CoverageCounts(got)
	if direct != 1 || viaTag != 2 || none != 1 {
		t.Fatalf("CoverageCounts = %d/%d/%d, want 1/2/1", direct, viaTag, none)
	}
}

// 実効タグのうち decision を持つタグが複数あるときは、出自を id 順にすべて列挙する。
func TestDecisionCoverageViaTagDetailListsAllSources(t *testing.T) {
	snap := store.Snapshot{
		Tags: []model.Tag{
			{ID: "concern.a", Name: "a", Kind: "concern"},
			{ID: "concern.b", Name: "b", Kind: "concern"},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Then: []string{"eff.a"}, Tags: []string{"concern.b", "concern.a"}},
		},
		Decisions: []model.Decision{
			{ID: "d1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "concern.a"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d2", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "concern.b"}, Why: "w", At: "2026-01-02T00:00:00Z"},
			{ID: "d3", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "concern.b"}, Why: "w", At: "2026-01-03T00:00:00Z"},
		},
	}
	got := checkDecisionCoverage(snap)
	if len(got) != 1 || got[0].Coverage != CoverageViaTag {
		t.Fatalf("expected single via-tag finding, got %+v", got)
	}
	if want := "via concern.a (1) / concern.b (2)"; got[0].Detail != want {
		t.Fatalf("Detail = %q, want %q", got[0].Detail, want)
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
	if !strings.Contains(got[0].Message, "vocab rm の候補") {
		t.Fatalf("non-axis unused vocab must keep the rm-candidate advice, got %q", got[0].Message)
	}
}

// axis kind タグ付き condition には削除助言を出さず、軸の値（given 未出現）の
// 文脈＋軸 decision の件数・直近 id を表示する（U1）。
func TestUnusedVocabAxisValueContext(t *testing.T) {
	snap := store.Snapshot{
		Tags: []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}},
		Vocab: []model.VocabEntry{
			{ID: "cond.apply", Category: model.CategoryCondition, Label: "apply", Tags: []string{"axis.mode"}},
			// axis タグの無い未使用 condition は従来どおり rm 候補のまま。
			{ID: "cond.plain", Category: model.CategoryCondition, Label: "plain"},
		},
		Decisions: []model.Decision{
			{ID: "d-old", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "axis.mode"}, Why: "w", At: "2026-01-01T00:00:00Z"},
			{ID: "d-new", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "axis.mode"}, Why: "w", At: "2026-02-01T00:00:00Z"},
		},
	}
	got := checkUnusedVocab(snap)
	if len(got) != 2 {
		t.Fatalf("expected 2 unused-vocab findings, got %+v", got)
	}
	byTarget := make(map[string]Finding, len(got))
	for _, f := range got {
		byTarget[f.Target] = f
	}

	axisMsg := byTarget["cond.apply"].Message
	for _, want := range []string{"軸 axis.mode の値です", "given にも未出現", "placeholder/remainder 候補", "軸の decision: 2 件（直近 d-new）"} {
		if !strings.Contains(axisMsg, want) {
			t.Fatalf("axis-value context message missing %q: %q", want, axisMsg)
		}
	}
	if strings.Contains(axisMsg, "rm の候補") {
		t.Fatalf("axis-value condition must not get deletion advice, got %q", axisMsg)
	}
	if !strings.Contains(byTarget["cond.plain"].Message, "vocab rm の候補") {
		t.Fatalf("non-axis condition must keep the rm-candidate advice, got %q", byTarget["cond.plain"].Message)
	}
}

// 軸 decision が 0 件でも文脈表示は出る（件数 0 として明示）。
func TestUnusedVocabAxisValueContextWithoutDecisions(t *testing.T) {
	snap := store.Snapshot{
		Tags: []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis"}},
		Vocab: []model.VocabEntry{
			{ID: "cond.apply", Category: model.CategoryCondition, Label: "apply", Tags: []string{"axis.mode"}},
		},
	}
	got := checkUnusedVocab(snap)
	if len(got) != 1 {
		t.Fatalf("expected 1 finding, got %+v", got)
	}
	if !strings.Contains(got[0].Message, "軸の decision: 0 件") {
		t.Fatalf("expected explicit 0-decision context, got %q", got[0].Message)
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
