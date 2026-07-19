package lint

import (
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// gateSnap は書き込みゲート検査用の最小 snapshot（手組み・Root 空なので
// dead-doc-ref はスキップされる）。axis.mode（total=false）と axis.other の
// 2 軸・各値 condition・action/effect を持つ。
func gateSnap() store.Snapshot {
	return store.Snapshot{
		Config: model.Config{
			TagKinds: []string{"requirement", "subject", "axis"},
		},
		Tags: []model.Tag{
			{ID: "axis.mode", Name: "モード", Kind: "axis"}, // total=false（全軸対象の証）
			{ID: "axis.other", Name: "別軸", Kind: "axis"},
			{ID: "subject.x", Name: "主題", Kind: "subject"},
		},
		Vocab: []model.VocabEntry{
			{ID: "cond.mode-a", Category: model.CategoryCondition, Label: "モードA", Tags: []string{"axis.mode"}},
			{ID: "cond.mode-b", Category: model.CategoryCondition, Label: "モードB", Tags: []string{"axis.mode"}},
			{ID: "cond.other-c", Category: model.CategoryCondition, Label: "別軸C", Tags: []string{"axis.other"}},
			{ID: "act.x", Category: model.CategoryAction, Label: "実行"},
			{ID: "eff.x", Category: model.CategoryEffect, Label: "効果"},
		},
		Transitions: []model.Transition{
			{ID: "T-existing", Action: "act.x", Given: []string{"cond.mode-a"}, Then: []string{"eff.x"}},
		},
	}
}

func rejectionRules(res GateResult) []string {
	var out []string
	for _, f := range res.Rejections {
		out = append(out, f.Rule)
	}
	return out
}

func advisoryRulesOf(res GateResult) []string {
	var out []string
	for _, f := range res.Advisories {
		out = append(out, f.Rule)
	}
	return out
}

// reject (a): 同一 axis kind タグの2値を同時 given ——total=false の軸でも
// 拒否する（全軸対象・決定③。total 限定にすると lint と判定が食い違う）。
func TestCheckWriteRejectsSameAxisTwoValueGiven(t *testing.T) {
	snap := gateSnap()
	tx := model.Transition{ID: "T-new", Action: "act.x", Given: []string{"cond.mode-a", "cond.mode-b"}, Then: []string{"eff.x"}}
	res := CheckWrite(snap, WriteOp{Transition: &tx, IsNew: true})

	if len(res.Rejections) != 1 || res.Rejections[0].Rule != GateExclusiveViolation {
		t.Fatalf("同一軸2値 given は exclusive-violation 1 件で reject のはず: %+v", res.Rejections)
	}
	if res.Rejections[0].Severity != SeverityError {
		t.Fatalf("gate の reject は severity=error のはず: %+v", res.Rejections[0])
	}
	if res.Rejections[0].Target != "T-new" {
		t.Fatalf("reject は保存対象レコードを指すはず: %+v", res.Rejections[0])
	}

	// 別軸の値どうしなら矛盾ではない。
	ok := model.Transition{ID: "T-ok", Action: "act.x", Given: []string{"cond.mode-a", "cond.other-c"}, Then: []string{"eff.x"}}
	if res := CheckWrite(snap, WriteOp{Transition: &ok, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("別軸の2値は reject しないはず: %+v", res.Rejections)
	}

	// 既存 id の edit でも exclusive-violation は検査する（id-policy と違い
	// 「新規のみ」の限定は無い）。
	edit := model.Transition{ID: "T-existing", Action: "act.x", Given: []string{"cond.mode-a", "cond.mode-b"}, Then: []string{"eff.x"}}
	if res := CheckWrite(snap, WriteOp{Transition: &edit, IsNew: false}); len(res.Rejections) != 1 {
		t.Fatalf("edit 経路でも同一軸2値は reject のはず: %+v", res.Rejections)
	}
}

// reject (b): axis 挙動を持たない kind への total=true。
func TestCheckWriteRejectsTotalOnNonAxisKind(t *testing.T) {
	snap := gateSnap()

	bad := model.Tag{ID: "req.total", Name: "要件", Kind: "requirement", Total: true}
	res := CheckWrite(snap, WriteOp{Tag: &bad, IsNew: true})
	if got := rejectionRules(res); len(got) != 1 || got[0] != GateTotalKindMismatch {
		t.Fatalf("非 axis kind への total=true は total-kind-mismatch で reject のはず: %v", got)
	}

	// kind=axis なら total=true は正当。
	okTag := model.Tag{ID: "axis.new", Name: "新軸", Kind: "axis", Total: true}
	if res := CheckWrite(snap, WriteOp{Tag: &okTag, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("kind=axis の total=true は reject しないはず: %+v", res.Rejections)
	}

	// 既存タグの edit で total を立てる場合も同様に reject。
	editBad := model.Tag{ID: "subject.x", Name: "主題", Kind: "subject", Total: true}
	if res := CheckWrite(snap, WriteOp{Tag: &editBad, IsNew: false}); len(res.Rejections) != 1 {
		t.Fatalf("edit 経路でも total×非 axis kind は reject のはず: %+v", res.Rejections)
	}
}

// reject (c): idPolicy 違反は新規 id のみ（既存 id の edit は対象外）。
func TestCheckWriteIDPolicyRejectsNewIDsOnly(t *testing.T) {
	snap := gateSnap()
	snap.Config.IDPolicy = &model.IDPolicy{
		Transition: "tx.",
		Vocab:      map[string]string{"condition": "cond."},
		TagByKind:  map[string]string{"axis": "axis."},
	}

	// 新規 transition の prefix 違反 → reject。
	bad := model.Transition{ID: "T-bad", Action: "act.x", Then: []string{"eff.x"}}
	res := CheckWrite(snap, WriteOp{Transition: &bad, IsNew: true})
	if got := rejectionRules(res); len(got) != 1 || got[0] != GateIDPolicy {
		t.Fatalf("idPolicy 違反の新規 transition id は reject のはず: %v", got)
	}

	// 宣言準拠なら通る。
	ok := model.Transition{ID: "tx.Comp.run", Action: "act.x", Then: []string{"eff.x"}}
	if res := CheckWrite(snap, WriteOp{Transition: &ok, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("宣言準拠の新規 id は reject しないはず: %+v", res.Rejections)
	}

	// 既存 id の edit は対象外（T-existing は宣言前からある id）。
	edit := model.Transition{ID: "T-existing", Action: "act.x", Given: []string{"cond.mode-a"}, Then: []string{"eff.x"}}
	if res := CheckWrite(snap, WriteOp{Transition: &edit, IsNew: false}); len(res.Rejections) != 0 {
		t.Fatalf("既存 id の edit は idPolicy 対象外のはず: %+v", res.Rejections)
	}

	// vocab カテゴリ別宣言。
	badVocab := model.VocabEntry{ID: "x.bad", Category: model.CategoryCondition, Label: "x"}
	if res := CheckWrite(snap, WriteOp{Vocab: &badVocab, IsNew: true}); len(rejectionRules(res)) != 1 {
		t.Fatalf("idPolicy.vocab.condition 違反は reject のはず: %+v", res.Rejections)
	}
	// 宣言の無いカテゴリ（effect）は強制しない。
	freeVocab := model.VocabEntry{ID: "anything.effect", Category: model.CategoryEffect, Label: "x"}
	if res := CheckWrite(snap, WriteOp{Vocab: &freeVocab, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("宣言の無いカテゴリは reject しないはず: %+v", res.Rejections)
	}

	// tag kind 別宣言。
	badTag := model.Tag{ID: "mode-axis", Name: "軸", Kind: "axis"}
	if res := CheckWrite(snap, WriteOp{Tag: &badTag, IsNew: true}); len(rejectionRules(res)) != 1 {
		t.Fatalf("idPolicy.tagByKind.axis 違反は reject のはず: %+v", res.Rejections)
	}
	// 宣言の無い kind は強制しない。
	freeTag := model.Tag{ID: "whatever", Name: "主題", Kind: "subject"}
	if res := CheckWrite(snap, WriteOp{Tag: &freeTag, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("宣言の無い kind は reject しないはず: %+v", res.Rejections)
	}

	// idPolicy 未宣言（nil）なら何も強制しない。
	snap.Config.IDPolicy = nil
	if res := CheckWrite(snap, WriteOp{Transition: &bad, IsNew: true}); len(res.Rejections) != 0 {
		t.Fatalf("idPolicy nil は reject しないはず: %+v", res.Rejections)
	}
}

// advisory は保存対象レコード限定スコープ——store に既存の違反があっても、
// 触れていないレコードの findings は返さない。
func TestCheckWriteAdvisoriesScopedToWrittenRecord(t *testing.T) {
	snap := gateSnap()
	// 既存レコードに stale-tense 違反を仕込む。
	snap.Vocab[0].Description = "現状は暫定のモードA"

	clean := model.VocabEntry{ID: "cond.clean", Category: model.CategoryCondition, Label: "clean", Description: "常に成り立つ前提"}
	res := CheckWrite(snap, WriteOp{Vocab: &clean, IsNew: true})
	if len(res.Advisories) != 0 {
		t.Fatalf("触れていないレコードの違反は advisory に含めないはず: %+v", res.Advisories)
	}

	dirty := model.VocabEntry{ID: "cond.dirty", Category: model.CategoryCondition, Label: "dirty", Description: "現状は未実装の前提"}
	res = CheckWrite(snap, WriteOp{Vocab: &dirty, IsNew: true})
	if got := advisoryRulesOf(res); len(got) != 1 || got[0] != "stale-tense" {
		t.Fatalf("保存対象レコードの stale-tense だけが advisory のはず: %+v", res.Advisories)
	}
	if res.Advisories[0].Target != "cond.dirty" {
		t.Fatalf("advisory は保存対象を指すはず: %+v", res.Advisories[0])
	}
}

// duplicate-atom はグループ代表（辞書順先頭）が target になるため、保存対象が
// 代表でない一員でも advisory に含める（Quote のグループ構成で照合）。
func TestCheckWriteAdvisoryDuplicateAtomMembership(t *testing.T) {
	snap := gateSnap()

	dup := model.Transition{ID: "T-zzz-dup", Action: "act.x", Given: []string{"cond.mode-a"}, Then: []string{"eff.x"}}
	res := CheckWrite(snap, WriteOp{Transition: &dup, IsNew: true})
	if got := advisoryRulesOf(res); len(got) != 1 || got[0] != "duplicate-atom" {
		t.Fatalf("同一原子の追加は duplicate-atom advisory のはず: %+v", res.Advisories)
	}
	if !strings.Contains(res.Advisories[0].Quote, "T-zzz-dup") || !strings.Contains(res.Advisories[0].Quote, "T-existing") {
		t.Fatalf("グループ構成に保存対象と既存の両方が入るはず: %+v", res.Advisories[0])
	}

	// 別原子（given が違う）なら duplicate ではない。
	other := model.Transition{ID: "T-other", Action: "act.x", Given: []string{"cond.other-c"}, Then: []string{"eff.x"}}
	if res := CheckWrite(snap, WriteOp{Transition: &other, IsNew: true}); len(res.Advisories) != 0 {
		t.Fatalf("別原子は duplicate-atom を出さないはず: %+v", res.Advisories)
	}
}

// desc 長（curated set の＋1・vocab.description のみ・閾値は定数）。
func TestCheckWriteAdvisoryDescLength(t *testing.T) {
	snap := gateSnap()

	long := model.VocabEntry{ID: "eff.long", Category: model.CategoryEffect, Label: "長い",
		Description: strings.Repeat("あ", DescLengthThreshold+1)}
	res := CheckWrite(snap, WriteOp{Vocab: &long, IsNew: true})
	if got := advisoryRulesOf(res); len(got) != 1 || got[0] != "desc-length" {
		t.Fatalf("閾値超の desc は desc-length advisory のはず: %+v", res.Advisories)
	}

	exact := model.VocabEntry{ID: "eff.exact", Category: model.CategoryEffect, Label: "境界",
		Description: strings.Repeat("あ", DescLengthThreshold)}
	if res := CheckWrite(snap, WriteOp{Vocab: &exact, IsNew: true}); len(res.Advisories) != 0 {
		t.Fatalf("閾値ちょうどは advisory を出さないはず: %+v", res.Advisories)
	}

	// tag.description は対象外（ユーザーストーリー形式が正当）。
	longTag := model.Tag{ID: "req.long", Name: "長い", Kind: "requirement",
		Description: strings.Repeat("い", DescLengthThreshold+1)}
	res = CheckWrite(snap, WriteOp{Tag: &longTag, IsNew: true})
	for _, f := range res.Advisories {
		if f.Rule == "desc-length" {
			t.Fatalf("tag desc は desc-length の対象外のはず: %+v", f)
		}
	}
}

// curated set の選別: 正規フロー上つねに未充足になる規則（axis-without-
// decision）と dangling-id は write-time advisory に含めない。
func TestCheckWriteCuratedSetExcludesAlwaysFiringRules(t *testing.T) {
	snap := gateSnap()

	// 新設の axis タグは decision 0 件が正規状態——advisory を出さない。
	newAxis := model.Tag{ID: "axis.fresh", Name: "新軸", Kind: "axis"}
	res := CheckWrite(snap, WriteOp{Tag: &newAxis, IsNew: true})
	for _, f := range res.Advisories {
		if f.Rule == "axis-without-decision" {
			t.Fatalf("axis-without-decision は write-time advisory に含めないはず: %+v", f)
		}
	}

	// dangling-id も write-time では通知しない（lint/retrofit の領分）。
	dangling := model.VocabEntry{ID: "eff.dangler", Category: model.CategoryEffect, Label: "x",
		Description: "cond.does-not-exist とペアで使う"}
	res = CheckWrite(snap, WriteOp{Vocab: &dangling, IsNew: true})
	for _, f := range res.Advisories {
		if f.Rule == "dangling-id" {
			t.Fatalf("dangling-id は write-time advisory に含めないはず: %+v", f)
		}
	}
}

// decide 経路: decision の why/changed への advisory（why-file-line）が保存前
// 検査（--dry-run）と同じコアで返る。
func TestCheckWriteDecisionWhyFileLineAdvisory(t *testing.T) {
	snap := gateSnap()

	d := model.Decision{ID: "01TESTULID0000000000000000",
		Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-existing"},
		Why:    "internal/foo.go:12 の分岐に合わせた", At: "2026-01-01T00:00:00Z"}
	res := CheckWrite(snap, WriteOp{Decision: &d, IsNew: true})
	if got := advisoryRulesOf(res); len(got) != 1 || got[0] != "why-file-line" {
		t.Fatalf("why の file:line は why-file-line advisory のはず: %+v", res.Advisories)
	}
	if !res.Advisories[0].AcknowledgeOnly {
		t.Fatalf("decision 判断欄位由来は acknowledge-only 区分のはず: %+v", res.Advisories[0])
	}
	if len(res.Rejections) != 0 {
		t.Fatalf("decision に reject 規則は無いはず: %+v", res.Rejections)
	}
}

// decide 経路 (d) unknown-acknowledges（#45 D6）: 新規 decision の acknowledges
// が有効な rule id に解決しないなら保存前に弾く（typo・候補提示）。
func TestCheckWriteRejectsUnknownAcknowledges(t *testing.T) {
	snap := gateSnap()
	d := model.Decision{ID: "01TESTULID0000000000000000",
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why:    "意図的に容認する", At: "2026-01-01T00:00:00Z",
		Acknowledges: []string{"requirement-gap", "not-a-real-rule"}}
	res := CheckWrite(snap, WriteOp{Decision: &d, IsNew: true})
	if len(res.Rejections) != 1 || res.Rejections[0].Rule != GateUnknownAcknowledges {
		t.Fatalf("未知 rule id の acknowledges は unknown-acknowledges reject のはず: %+v", res.Rejections)
	}
	if res.Rejections[0].Quote != "not-a-real-rule" {
		t.Fatalf("reject の Quote は未知 rule 名のはず: %+v", res.Rejections[0])
	}
	if !strings.Contains(res.Rejections[0].Message, "有効") {
		t.Fatalf("reject メッセージに有効 rule 候補が含まれるべき: %q", res.Rejections[0].Message)
	}
}

// 実在 rule id（lint.Rules 名・flow rule 名の両方）の acknowledges は通す。
func TestCheckWriteAllowsKnownAcknowledges(t *testing.T) {
	snap := gateSnap()
	d := model.Decision{ID: "01TESTULID0000000000000001",
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why:    "意図的に容認する", At: "2026-01-01T00:00:00Z",
		Acknowledges: []string{"requirement-gap", "total-gap", "overlap"}}
	res := CheckWrite(snap, WriteOp{Decision: &d, IsNew: true})
	for _, f := range res.Rejections {
		if f.Rule == GateUnknownAcknowledges {
			t.Fatalf("実在 rule id の acknowledges が弾かれた: %+v", f)
		}
	}
}

// add-commit（IsNew=false）や既存 decision の再検査はしない（新規のみ検査）。
func TestCheckWriteAcknowledgesOnlyCheckedForNew(t *testing.T) {
	snap := gateSnap()
	d := model.Decision{ID: "01TESTULID0000000000000002",
		Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
		Why:    "既存", At: "2026-01-01T00:00:00Z",
		Acknowledges: []string{"legacy-renamed-rule"}}
	res := CheckWrite(snap, WriteOp{Decision: &d, IsNew: false})
	for _, f := range res.Rejections {
		if f.Rule == GateUnknownAcknowledges {
			t.Fatalf("IsNew=false の再検査で unknown-acknowledges が出た（新規のみのはず）: %+v", f)
		}
	}
}

// dangling-acknowledges lint: 既存 decision の解決しない acknowledges を info で警告。
func TestDanglingAcknowledgesLint(t *testing.T) {
	snap := gateSnap()
	snap.Decisions = []model.Decision{
		{ID: "01D0", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
			Why: "ok", At: "2026-01-01T00:00:00Z", Acknowledges: []string{"requirement-gap"}},
		{ID: "01D1", Target: model.DecisionTarget{Type: model.DecisionTargetTag, ID: "subject.x"},
			Why: "dangling", At: "2026-01-01T00:00:00Z", Acknowledges: []string{"gone-rule"}},
	}
	out := checkDanglingAcknowledges(snap)
	if len(out) != 1 || out[0].Target != "01D1" || !out[0].AcknowledgeOnly {
		t.Fatalf("宙吊り acknowledges 1件を acknowledge-only info で返すはず: %+v", out)
	}
}
