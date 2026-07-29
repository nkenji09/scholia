package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// retrofitJSON は `scholia retrofit --json` の応答形。
type retrofitJSON struct {
	Rules    []string       `json:"rules"`
	Findings []lint.Finding `json:"findings"`
	Fixable  struct {
		FindingCount int            `json:"findingCount"`
		RecordCount  int            `json:"recordCount"`
		ByRule       map[string]int `json:"byRule"`
	} `json:"fixable"`
	AcknowledgeOnly struct {
		FindingCount int            `json:"findingCount"`
		RecordCount  int            `json:"recordCount"`
		ByRule       map[string]int `json:"byRule"`
	} `json:"acknowledgeOnly"`
}

// setupRetrofitStore は advisory 規則が広く発火する店構えを store API で組む
// （CLI の書き込みゲートを介さず、既存レコードの「先例汚染」を再現する）。
func setupRetrofitStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.TagKinds = append(cfg.TagKinds, model.KindDecl{ID: "axis"})
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// derived-value＋stale-tense＋prose-ref＋dead-doc-ref が同時ヒットする axis desc
	// （axis-without-decision も：own decision なし）
	if err := s.SaveTag(model.Tag{ID: "axis.a", Name: "軸A", Kind: "axis", Total: true,
		Description: "値＝{cond.v1}。total=true。現状は #12 の新設。missing-doc.md を参照。"}); err != nil {
		t.Fatal(err)
	}
	for _, v := range []model.VocabEntry{
		{ID: "cond.v1", Category: model.CategoryCondition, Label: "v1", Tags: []string{"axis.a"}},
		{ID: "act.a", Category: model.CategoryAction, Label: "a"},
		{ID: "eff.a", Category: model.CategoryEffect, Label: "e"},
	} {
		if err := s.SaveVocab(v); err != nil {
			t.Fatal(err)
		}
	}
	// duplicate-atom（同一原子 2 本）
	for _, id := range []string{"T-d1", "T-d2"} {
		if err := s.SaveTransition(model.Transition{ID: id, Action: "act.a", Then: []string{"eff.a"}}); err != nil {
			t.Fatal(err)
		}
	}
	// why-file-line＋dangling-id（判断欄位＝acknowledge-only）
	if err := s.SaveDecision(model.Decision{
		ID:     "01AAAAAAAAAAAAAAAAAAAAAAAA",
		Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-d1"},
		Why:    "internal/a.go:12 を見て T-gone を廃止した",
		At:     "2026-07-17T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRetrofitTextOutputSeparatesFixableAndAcknowledgeOnly(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit")
	if err != nil {
		t.Fatalf("retrofit must exit 0 even with findings: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"fixable（是正可能）:",
		"[derived-value-in-desc] tag axis.a（description）",
		"[stale-tense] tag axis.a（description）",
		"[prose-ref] tag axis.a（description）",
		"[axis-without-decision] tag axis.a",
		"[duplicate-atom] transition T-d1: T-d1・T-d2",
		"[dead-doc-ref] tag axis.a（description）: missing-doc.md",
		"acknowledge-only（decision 判断欄位・append-only により是正不能・容認で畳む対象）:",
		"[why-file-line] decision 01AAAAAAAAAAAAAAAAAAAAAAAA（why）: internal/a.go:12",
		"[dangling-id] decision 01AAAAAAAAAAAAAAAAAAAAAAAA（why）: T-gone",
		"→ 修正候補:",
		"集計: fixable 6 findings / 2 レコード・acknowledge-only 2 findings / 1 レコード",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("retrofit output missing %q:\n%s", want, out)
		}
	}
}

func TestRetrofitJSONCarriesCountsAndTier(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json: %v\noutput:\n%s", err, out)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v\noutput:\n%s", err, out)
	}
	// #45 D6 で dangling-acknowledges・D7 で decision-stale（両 TierAdvisory）を
	// 追加したため 8→10。retrofit は TierAdvisory 規則を動的に拾うので、新 advisory
	// 規則が正しく走査対象に入っていることの確認でもある。
	if len(resp.Rules) != 10 {
		t.Fatalf("advisory 10 規則のはず: %v", resp.Rules)
	}
	for _, f := range resp.Findings {
		if f.Tier != lint.TierAdvisory || f.Severity != lint.SeverityInfo {
			t.Fatalf("finding must be tier=advisory severity=info: %+v", f)
		}
	}
	if resp.Fixable.FindingCount != 6 || resp.Fixable.RecordCount != 2 {
		t.Fatalf("fixable counts wrong: %+v", resp.Fixable)
	}
	if resp.AcknowledgeOnly.FindingCount != 2 || resp.AcknowledgeOnly.RecordCount != 1 {
		t.Fatalf("acknowledgeOnly counts wrong: %+v", resp.AcknowledgeOnly)
	}
	wantFixByRule := map[string]int{
		"derived-value-in-desc": 1, "stale-tense": 1, "prose-ref": 1, "why-file-line": 0,
		"axis-without-decision": 1, "duplicate-atom": 1, "dangling-id": 0, "dead-doc-ref": 1,
	}
	for rule, n := range wantFixByRule {
		if resp.Fixable.ByRule[rule] != n {
			t.Fatalf("fixable byRule[%s] = %d, want %d", rule, resp.Fixable.ByRule[rule], n)
		}
	}
	if resp.AcknowledgeOnly.ByRule["why-file-line"] != 1 || resp.AcknowledgeOnly.ByRule["dangling-id"] != 1 {
		t.Fatalf("acknowledgeOnly byRule wrong: %+v", resp.AcknowledgeOnly.ByRule)
	}
}

func TestRetrofitRuleFilterAndUnknownRule(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit", "--rule", "dangling-id", "--json")
	if err != nil {
		t.Fatalf("retrofit --rule: %v\noutput:\n%s", err, out)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(resp.Rules) != 1 || resp.Rules[0] != "dangling-id" {
		t.Fatalf("rules should be filtered: %v", resp.Rules)
	}
	for _, f := range resp.Findings {
		if f.Rule != "dangling-id" {
			t.Fatalf("filtered run must not include %+v", f)
		}
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("expected 1 dangling-id finding, got %+v", resp.Findings)
	}

	if _, err := run(t, dir, "retrofit", "--rule", "no-such-rule"); err == nil {
		t.Fatalf("unknown --rule must error")
	}
}

// dogfood 統合: 実 store の件数（desc-length は U3 のスコープなので 8 規則ベース）。
// フェーズ2 増分2.3a の retrofit fixable の desc 浄化（D4-d）で、fixable の
// derived-value-in-desc 4／stale-tense 7／dead-doc-ref 11 を 13 レコードの desc から
// 除いて fixable は 22/13 → 0/0 へ追随（ack-only 13/13 は不変）。dead-doc-ref の
// 総数は 19 → 8（残り 8 は全て decision 判断欄位の design-options/tweaks3/.concierge
// 参照＝append-only ゆえ acknowledge-only）。
func TestRetrofitDogfoodCounts(t *testing.T) {
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store not found: %v", err)
	}
	root := filepath.Dir(s.Dir)

	out, err := run(t, root, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json on dogfood: %v", err)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if resp.Fixable.FindingCount != 0 || resp.Fixable.RecordCount != 0 {
		t.Fatalf("dogfood fixable = %d findings / %d records, want 0/0", resp.Fixable.FindingCount, resp.Fixable.RecordCount)
	}
	// #45 D7 で decision-stale（git 導出・commit 対象・AcknowledgeOnly=true）を
	// 追加したため 13/13 → 14/14。増分は「レコード変更 commit に decision 非同伴」の
	// 1 commit（record 編集では是正できず acknowledges でのみ解消＝ack-only）。
	// さらに #45 Step3 の U3③(B) で write-gate/authoring-advisory を実装遷移へタグ付け
	// した data-work commit（遷移2件のタグ変更・scholia decision 非同伴）が decision-stale
	// の2件目として真ヒットし 14/14 → 15/15。
	// #45 束5（D8）で update 5遷移への priority 宣言＋tx.flow effect vocab の意味論改訂を
	// 束ねた data-work commit（正本 decision は前もって base commit で記録済・当該 commit
	// 自体には decision 非同伴）が decision-stale の3件目として真ヒットし 15/15 → 16/16。
	// #45 束6（D9）の data-work commit（owner 60・condition kind 47・subject 2枚・
	// ownerKind 宣言＝107 vocab＋config を編集・正本 decision 01KXY6PXRR… は前もって
	// base commit で記録済ゆえ当該 commit 自体には decision 非同伴）が decision-stale の
	// 4件目として真ヒットし 16/16 → 17/17。D7 が「機械マイグレーション/データ作業型
	// commit は偽陽性として残り acknowledges で容認可」と規定した挙動どおりの新実測
	// （info・lint --ci は warn を数えるため緑・regression ではない）。
	// #45 束7（D10b-5）の flow 入口昇格 data-work commit（既存 tx.viewer.action-flow-link
	// に subject.viewer.vocab を追加＝vocab 導線を正規形『1原子＋複数 subject タグ』で
	// 記録・正本 decision 01KXYED63YFK… は前もって base commit 326007a で記録済ゆえ当該
	// commit 自体には decision 非同伴）が decision-stale の5件目として真ヒットし
	// 17/17 → 18/18。D7 規定どおりの新実測（info・lint --ci 緑・regression ではない）。
	// decision 01KY1VDJWZF7M23K4X1J62QYXV（BrowseRail サジェスト順位付け・
	// req.comfortable-viewer.faceted-nav amend）の why 例示 `req.foo.1-1` が
	// dangling-id の新規真ヒットとして加わり 18/18 → 19/19（判断欄位＝append-only
	// ゆえ是正不能・acknowledge-only）。
	// #5（集約2面の廃止）の実装コミットで commit 95679d2b が decisionStaleScanLimit
	// (=200) の窓から外れ、その decision-stale が1件落ちて 19/19 → 18/18。是正でも
	// 回帰でもなく**窓の外に出ただけ**——decision-stale は「最近レコードを触ったのに
	// decision を結ばなかった」を見る規則なので、古い commit が落ちるのは設計どおり。
	// この期待値は commit を積むだけで動くので、ズレたらまず窓の境界を疑うこと
	// （`git rev-list --count <commit>..HEAD` が 200 を超えていないか）。
	// ——実際にまた動いた。しかも**2つの単位が独立に同じ境界を踏んだ**: 「概要タブが
	// 成立する条件」の単位と「端末で取り下げた規則を渡さない」の単位が、それぞれ
	// commit fc81ff6f を窓の外へ押し出して 18/18 → 17/17 に更新していた（マージで
	// 両方の説明が衝突した）。
	// **そしてマージそのものが、さらにもう1件を窓の外へ出した。** 両単位の commit が
	// 合流して履歴が伸び、commit 0b3a04bb が 195 → 209 になり 17/17 → 16/16。
	// マージ直後の実測（`git rev-list --count <commit>..HEAD`・窓は 200）:
	//   e1d44d18: 183 ／ 9df25e5b: 193 ／ 0b3a04bb: 209 ／ fc81ff6f: 221
	// `lint` が挙げる decision-stale も e1d44d18・9df25e5b の2件に減っている。
	// 是正でも回帰でもなく**窓の外に出ただけ**である。
	//
	// ⚠️⚠️ **`go test` の結果キャッシュに騙されないこと。** この検査は git の履歴を
	// 入力に取るが、**Go のテストキャッシュはそれを入力として見ない**。マージした直後に
	// `go test ./...` を打つと、ソースが変わっていないパッケージは**キャッシュ済みの
	// 緑をそのまま返す**——実際にこの単位で、16/16 になっているのに `ok (cached)` と
	// 出た。**この検査を信じる前に `-count=1` を付けて走らせること。**
	//
	// ——**予告どおりに動いた。** 直前の注記が「あと7コミットで 9df25e5b が窓を出る」と
	// 書いていたところへ、「構造ツリーを役割まで絞る」単位が3コミット積み、9df25e5b が
	// ちょうど 200 に達して窓の外へ出た。16/16 → 15/15。
	// その単位の着地時点の実測（窓は 200・`git rev-list --count <commit>..HEAD`）:
	//   e1d44d18: 190 ／ 9df25e5b: 200 ／ 0b3a04bb: 216 ／ fc81ff6f: 228
	// main（7551cb0）時点では 9df25e5b が 197 だったので、**3コミット積んだぶんだけ動いた**
	// ＝是正でも回帰でもなく窓の外に出ただけである。`lint` が挙げる decision-stale も
	// e1d44d18 の1件だけに減っている（実測）。
	// なお同単位が足した decision 01KYPFJV04R347HWHQKQ2TW275 は advisory を1件も出さない
	// （`decide --dry-run` で確認済み）ので、この差分に寄与していない。
	//
	// ⚠️ 次に触る人へ: **e1d44d18 が 190 で、境界まであと10コミット。**
	// そこを越えると 15/15 → 14/14 になり、**decision-stale は0件になる**（設計どおり）。
	if resp.AcknowledgeOnly.FindingCount != 15 || resp.AcknowledgeOnly.RecordCount != 15 {
		t.Fatalf("dogfood acknowledgeOnly = %d findings / %d records, want 15/15", resp.AcknowledgeOnly.FindingCount, resp.AcknowledgeOnly.RecordCount)
	}
	if total := resp.Fixable.ByRule["dead-doc-ref"] + resp.AcknowledgeOnly.ByRule["dead-doc-ref"]; total != 8 {
		t.Fatalf("dogfood dead-doc-ref total = %d, want 8", total)
	}
}
