package lint

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// ownerFixture は「宣言軸が張られた1 action」を最小構成で組む。axis タグ
// axis.mode を2値（cond.check/cond.apply）に貼り、2遷移がそれぞれ異なる owner の
// effect を then に持つ。
func ownerFixture(t *testing.T, ownerA, ownerB string, ownerKind string) store.Snapshot {
	t.Helper()
	cfg := baseConfig()
	cfg.OwnerKind = ownerKind
	// axis.mode を軸として認識させる（compat: kind=="axis"）。
	return store.Snapshot{
		Config: cfg,
		Tags:   []model.Tag{{ID: "axis.mode", Name: "mode", Kind: "axis", Total: true}},
		Vocab: []model.VocabEntry{
			{ID: "act.a", Category: model.CategoryAction, Label: "a"},
			{ID: "cond.check", Category: model.CategoryCondition, Label: "check", Tags: []string{"axis.mode"}},
			{ID: "cond.apply", Category: model.CategoryCondition, Label: "apply", Tags: []string{"axis.mode"}},
			{ID: "eff.x", Category: model.CategoryEffect, Label: "x", Owner: ownerA},
			{ID: "eff.y", Category: model.CategoryEffect, Label: "y", Owner: ownerB},
		},
		Transitions: []model.Transition{
			{ID: "T-1", Action: "act.a", Given: []string{"cond.check"}, Then: []string{"eff.x"}},
			{ID: "T-2", Action: "act.a", Given: []string{"cond.apply"}, Then: []string{"eff.y"}},
		},
	}
}

// 単一 owner に定まる action は沈黙する（検算: act.user.update 相当）。
func TestMultipleOwnerAction_SingleOwnerSilent(t *testing.T) {
	snap := ownerFixture(t, "subject.cli", "subject.cli", "subject")
	findings := checkMultipleOwnerAction(snap)
	if len(findings) != 0 {
		t.Fatalf("single-owner action must be silent, got %+v", findings)
	}
}

// 複数 owner 混在は info で開示する（error にしない）。
func TestMultipleOwnerAction_MixedOwnersDisclosed(t *testing.T) {
	snap := ownerFixture(t, "subject.cli", "subject.store", "subject")
	findings := checkMultipleOwnerAction(snap)
	if len(findings) != 1 {
		t.Fatalf("mixed owners must yield 1 finding, got %+v", findings)
	}
	f := findings[0]
	if f.Severity != SeverityInfo {
		t.Fatalf("severity = %q, want info (disclosure only)", f.Severity)
	}
	if f.Target != "act.a" {
		t.Fatalf("target = %q, want act.a", f.Target)
	}
	if !strings.Contains(f.Message, "subject.cli") || !strings.Contains(f.Message, "subject.store") {
		t.Fatalf("message should enumerate owners, got %q", f.Message)
	}
}

// owner 無指定 effect の混在も開示する（distinct==1 でも無指定混在は finding）。
func TestMultipleOwnerAction_UnownedMixDisclosed(t *testing.T) {
	snap := ownerFixture(t, "subject.cli", "", "subject")
	findings := checkMultipleOwnerAction(snap)
	if len(findings) != 1 {
		t.Fatalf("unowned mix must yield 1 finding, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "無指定") {
		t.Fatalf("message should disclose unowned effect, got %q", findings[0].Message)
	}
}

// 宣言軸が無い action は検査対象外で沈黙する（検算: act.user.flow 相当）。
func TestMultipleOwnerAction_NoAxisNotChecked(t *testing.T) {
	snap := ownerFixture(t, "subject.cli", "subject.store", "subject")
	// axis タグを外す＝宣言軸が無くなる。owner は依然混在だが検査対象外。
	snap.Tags = nil
	findings := checkMultipleOwnerAction(snap)
	if len(findings) != 0 {
		t.Fatalf("action without a declared axis must not be checked, got %+v", findings)
	}
}

// ownerKind 未宣言時も走るが、finding 文面に「自由文字列の完全一致」の開示を含む。
func TestMultipleOwnerAction_OwnerKindUnsetDiscloses(t *testing.T) {
	snap := ownerFixture(t, "cli", "store", "")
	findings := checkMultipleOwnerAction(snap)
	if len(findings) != 1 {
		t.Fatalf("mixed free-string owners must yield 1 finding, got %+v", findings)
	}
	if !strings.Contains(findings[0].Message, "ownerKind 未宣言") {
		t.Fatalf("message should disclose free-string caveat, got %q", findings[0].Message)
	}
}
