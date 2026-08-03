package cli

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
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
	if err := s.CreateDecision(model.Decision{
		ID:     "01AAAAAAAAAAAAAAAAAAAAAAAA",
		Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-d1"},
		Why:    "# テスト用の見出し\n\ninternal/a.go:12 を見て T-gone を廃止した",
		At:     "2026-07-17T00:00:00Z",
	}, store.DecisionCreateOptions{}); err != nil {
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

// dogfoodKnownAckOnly は「レコード由来で、是正が原理的に不能」と既に容認した
// advisory の集合。キーは rule+target（lint --ci の baseline entry と同じキー取り・
// 01KXS4BB6KKX02XMDCS9EHNE6X）で、message や引用位置には依存しない。
// 値は「何を容認したのか」の一言——新しい entry を足す人が、diff だけで説明を
// 済ませられるようにするため。
//
// ここに載るのは decision の判断欄位（why/changed/ref）に書かれてしまった引用
// だけである。decision は append-only なので後から書き換えられない——だから
// 「是正しろ」ではなく「既知として容認する」しかない（#45 U2）。
var dogfoodKnownAckOnly = map[string]string{
	// why が実装のファイル:行番号を引用している（行番号は実装が動けば腐る）。
	"why-file-line|01KXNGQYRM718XH18RGSNSQCW1": "why: internal/flow/text.go:58",
	"why-file-line|01KXNGQYRV3ZQSMSWXNA4BGTF7": "why: internal/flow/analyze.go:323",
	"why-file-line|01KXNGQYS14G2XPDW19Y8JGX4B": "why: internal/flow/analyze.go:192 ほか3箇所",
	"why-file-line|01KXNGQYS8997QET0KWJA29B38": "why: internal/flow/analyze.go:449",

	// 判断欄位が、store に存在しない id を引用している。
	"dangling-id|01KXFEXG01RS00RHAVS3TMP25Y": "changed: 廃止済み tx.action",
	"dangling-id|01KY1VDJWZF7M23K4X1J62QYXV": "why: 説明のための架空例 req.foo.1-1",

	// 判断欄位が、repo に無い文書を引用している（作業台帳・gitignore 対象）。
	"dead-doc-ref|01KXFEXG08YT8TB04BR7RA400Q": "why: tweaks3 §",
	"dead-doc-ref|01KXJ3JEKNGHAF4XHGM8WV9N90": "ref: .concierge/decision.md",
	"dead-doc-ref|01KXJ7GESNX3JCQ1FCEXTMSGDK": "why: .concierge/decision.md",
	"dead-doc-ref|01KXMGGD6DS88CHGRJ9GPRBRVX": "why: design-options §",
	"dead-doc-ref|01KXMRBB3PYJZMEXS7JTQQPP8D": "ref: design-options §",
	"dead-doc-ref|01KXMRBB3XJYTQ4WM3MZGZZY7C": "ref: design-options §",
	"dead-doc-ref|01KXMRBB447FDSRPH6ZAWVC7W2": "ref: design-options §",
	"dead-doc-ref|01KXMRBNXTN8742KDJVV4HW15V": "why・ref: design-options §",
}

// TestRetrofitDogfoodAdvisories は、この repo 自身の `.scholia` を retrofit で
// 走査し「自分の記録が自分の規則に従っているか」を見る dogfood ガード。
//
// # このガードが落ちる範囲（射程）
//
//  1. 是正可能な advisory が1件でも残ったとき（fixable != 0/0）。
//  2. レコード由来の acknowledge-only に、dogfoodKnownAckOnly（rule+target キー）の外が出たとき。
//  3. commit を対象に取る advisory が decision-stale 以外に現れたとき。
//  4. 走査対象の store が空だったとき（1・2 が空振りで緑になるのを塞ぐ）。
//
// # 落ちない範囲（原理を1つ）
//
// **このガードは「まだ知らない種類が出たか」だけを見る。既に知っている種類が
// 何件に増減したかは見ない。** よって decision-stale が窓の出入りで増えても
// 減っても、このガードは何も言わない。これは漏れではなく設計である。
//
// decision-stale は直近 decisionStaleScanLimit(=200) commit という**移動窓**から
// 導出される（internal/lint/rules_decision_stale.go）。窓は commit を積むだけで
// 動くので、件数を固定すると**このガードが検査していない作業が commit を積んだだけ
// で期待値が動く**。実際に動いた——期待値の移動 11 回・そのための専用 commit 8 本・
// 説明コメント 75 行・マージ衝突 1 回。しかも正当な追随と「実装に合わせて検査を
// 曲げた」が外形上まったく同じに見えるため、毎回レビューの注意力を食っていた。
// 正本 01KXWPQDGMDB01V86KZ91M0BPQ（D7）は decision-stale について
// 「機械マイグレーション型 commit の偽陽性が残るため容認可能とする」と定めている。
// **容認可能と決めたものを、容認するたびに検査の編集で払わせない。**
// 窓の境界そのものの挙動は、生の履歴ではなく合成した履歴で決定的に検査する
// （internal/lint の TestDecisionStaleWindowBoundary）。
//
// 既知集合の向きは lint --ci の baseline ratchet（01KXS4BB6KKX02XMDCS9EHNE6X）に
// 揃えた——**新しいキーは落とし、消えたキーでは落とさない**（記録が良くなる方向で
// 赤くしない）。台帳ファイル .scholia/lint-baseline.json 自体は使わない: あれは
// severity=warn の歯止めで advisory(info) を意図的に対象外にしており、そこへ広げる
// のは製品の振る舞いの変更になる。考え方だけを借りて機構は増やさない。
func TestRetrofitDogfoodAdvisories(t *testing.T) {
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store not found: %v", err)
	}
	root := filepath.Dir(s.Dir)

	// 走査対象が空でないことの錨（射程 4）。fixable==0 と「既知集合の外なし」は
	// どちらも空の store で真になるので、これが無いと store を見失っても緑になる。
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("dogfood LoadAll: %v", err)
	}
	if len(snap.Decisions) == 0 || len(snap.Transitions) == 0 {
		t.Fatalf("dogfood store が空に見える（decisions %d / transitions %d）——走査が成立していない",
			len(snap.Decisions), len(snap.Transitions))
	}

	out, err := run(t, root, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json on dogfood: %v", err)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}

	// 射程 1: 是正可能な advisory は残っていない（記録を編集すれば直せるものは直してある）。
	if resp.Fixable.FindingCount != 0 || resp.Fixable.RecordCount != 0 {
		t.Fatalf("dogfood fixable = %d findings / %d records, want 0/0（byRule: %v）",
			resp.Fixable.FindingCount, resp.Fixable.RecordCount, resp.Fixable.ByRule)
	}

	// 射程 2・3: acknowledge-only を出自で分ける。
	//   git 由来（TargetType == "commit"）  = 移動窓——件数を見ない。
	//   レコード由来                        = 既知集合の外が出たら落とす。
	seen := make(map[string]bool, len(dogfoodKnownAckOnly))
	var unknown []string
	for _, f := range resp.Findings {
		if !f.AcknowledgeOnly {
			continue // fixable は射程 1 で 0 件と確認済み
		}
		if f.TargetType == "commit" {
			if f.Rule != "decision-stale" {
				t.Fatalf("commit を対象に取る advisory が decision-stale 以外に増えた: %+v\n"+
					"（移動窓由来の finding をこのガードは数えない。新しい窓依存の規則を足したなら射程を書き直すこと）", f)
			}
			continue
		}
		key := f.Rule + "|" + f.Target
		seen[key] = true
		if _, ok := dogfoodKnownAckOnly[key]; !ok {
			unknown = append(unknown, fmt.Sprintf("  %s（%s）: %s", key, f.Field, f.Quote))
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		t.Fatalf("既知集合に無い acknowledge-only advisory が出た（%d 件）:\n%s\n\n"+
			"decision の判断欄位は append-only で是正できない。内容を見直せないなら\n"+
			"dogfoodKnownAckOnly に「何を容認したのか」を添えて追加すること。",
			len(unknown), strings.Join(unknown, "\n"))
	}
	for key, note := range dogfoodKnownAckOnly {
		if !seen[key] {
			// 落とさない（記録が良くなる方向で赤くしない・lint --ci の stale entry と同じ扱い）。
			t.Logf("既知集合の entry が今は出ていない（解消済み・次に触る人が消してよい）: %s（%s）", key, note)
		}
	}
}
