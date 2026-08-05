package lint

// dogfood 実 store（この repo の .scholia/・331 レコード）に対する advisory
// 規則の精度固定テスト（#45 U2）。数値・対象は kit-bundle2-retrofit-findings.md
// の read-only 実走リストに一致させる（dead-doc-ref の decision
// 01KXFEXG08YT8TB04BR7RA400Q〔tweaks3 §2〕だけは本実装の追加発見＝真ヒット）。
// データ後段の retrofit で store が是正されたら、このテストは新しい実測値に
// 更新する（現状を固定するのが目的であって不変条件ではない）。

import (
	"sort"
	"testing"

	"github.com/nkenji09/scholia/internal/store"
)

func dogfoodSnapshot(t *testing.T) store.Snapshot {
	t.Helper()
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store が見つかりません（repo checkout が壊れている）: %v", err)
	}
	snap, err := s.LoadAll()
	if err != nil {
		t.Fatalf("dogfood store の読み込みに失敗: %v", err)
	}
	return snap
}

// dangling-id: 真ヒット 2 件（偽陽性ゼロ）。素朴実装は偽陽性が多発する実データ
// （族 glob `T-comment-*`・プレースホルダ `req.foobar`・kind 族 `eff.log`）を、
// 除外3種 (E1)(E2)(E3) が全て畳むことを固定する。
//
// decision 01KY1VDJWZF7M23K4X1J62QYXV（req.comfortable-viewer.faceted-nav
// amend・BrowseRail サジェスト順位付け）の why が例示に使った `req.foo.1-1`
// は E2（`req.foobar`／`req.foo-bar` 形）に一致しないハイフン+数字の別形なの
// で新たに真ヒットし 1→2。判断欄位（why）は append-only ゆえ是正不能・
// acknowledge-only（コメント冒頭の「retrofit で実測が変わったら追随する」
// 方針どおりの実測値更新）。
//
// フェーズ2 の retrofit（#45・決定①）で store が全面改名され、prefix 候補集合が
// 入れ替わった。旧真ヒット（decision 01KXM9X0… の changed 内 `T-viewer-adopt-
// comment-removed`）は、既存レコードから `T-` prefix が推定されなくなったため
// id 様トークンとして認識されなくなった。代わって config.idPolicy が `tx.` を
// 宣言したことで、decision 01KXFEXG… の changed 内のコード式 `tx.action`
// （`vocabLabelById.get(tx.action)`）が新たに tx. prefix の id 様トークンとして
// 引っかかる。これは append-only の判断欄位に元から書かれたコード断片で是正
// 不能ゆえ acknowledge-only の真ヒット扱い（コメント冒頭の「retrofit で実測が
// 変わったら追随する」方針どおり更新した実測値）。
func TestDogfoodDanglingIDHasZeroFalsePositives(t *testing.T) {
	snap := dogfoodSnapshot(t)
	findings := checkDanglingID(snap)

	// 素朴実装の偽陽性（kit-bundle2-retrofit-findings.md §7）が 1 件も finding に
	// ならないこと。改名後は同 decision の why の T- 系トークンは prefix 候補から
	// 落ちて消えるが、E1/E2/E3 で畳むべきパターンは引き続き finding にしない。
	falsePositiveRecords := []string{
		"01KXM9VN3FPGE5C2APBTNRWGHA",        // why: T-comment-*（E1）
		"req.evaluate-change.adopt-cleanup", // desc: `T-comment-*`（E1）
		"01KXEVDGYNB32K3WXKMV8Z4RVW",        // changed: T-skills-install-*（E1）
		"01KXJ3JEKNGHAF4XHGM8WV9N90",        // why: req.foobar・req.foo-bar（E2）
		"01KXFK6V81TEDF3340AFGA08WG",        // why: eff.log（E3）
	}
	for _, f := range findings {
		for _, fp := range falsePositiveRecords {
			if f.Target == fp {
				t.Fatalf("既知の偽陽性パターンを誤検出: %+v", f)
			}
		}
	}

	if len(findings) != 2 {
		t.Fatalf("真ヒット 2 件（偽陽性ゼロ）のはずが %d 件: %+v", len(findings), findings)
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Target < findings[j].Target })
	f := findings[0]
	if f.Target != "01KXFEXG01RS00RHAVS3TMP25Y" || f.TargetType != "decision" ||
		f.Field != "changed" || f.Quote != "tx.action" || !f.AcknowledgeOnly {
		t.Fatalf("真ヒット1件目の内容が想定と違う: %+v", f)
	}
	f2 := findings[1]
	if f2.Target != "01KY1VDJWZF7M23K4X1J62QYXV" || f2.TargetType != "decision" ||
		f2.Field != "why" || f2.Quote != "req.foo.1-1" || !f2.AcknowledgeOnly {
		t.Fatalf("真ヒット2件目の内容が想定と違う: %+v", f2)
	}
}

// dead-doc-ref: フェーズ2 増分2.3a の retrofit 適用（D4-d）で、tag/vocab desc の
// design-options 参照（fixable 11 レコード）は全て desc から除かれた。残る dead-doc-ref
// は全て decision 判断欄位（why/changed/ref）＝append-only ゆえ acknowledge-only の
// .concierge 系＋tweaks3 系＋design-options 系 8 件。versioned に解決する参照
// （DESIGN §N・RELEASING.md 等 20 件超）は誤検出しないこと。
func TestDogfoodDeadDocRefDetectsDesignOptionsType(t *testing.T) {
	snap := dogfoodSnapshot(t)
	findings := checkDeadDocRef(snap)

	var fixTargets, ackTargets []string
	for _, f := range findings {
		if f.AcknowledgeOnly {
			ackTargets = append(ackTargets, f.Target)
		} else {
			fixTargets = append(fixTargets, f.Target)
		}
	}
	sort.Strings(fixTargets)
	sort.Strings(ackTargets)

	// desc 浄化後は fixable な dead-doc-ref はゼロ（tag/vocab の design-options 参照が
	// 全て除かれた）。
	wantFix := []string{}
	sort.Strings(wantFix)
	wantAck := []string{
		"01KXFEXG08YT8TB04BR7RA400Q", // tweaks3 §2（本実装の追加発見・真ヒット）
		"01KXJ3JEKNGHAF4XHGM8WV9N90", // ref: .concierge/decision.md
		"01KXJ7GESNX3JCQ1FCEXTMSGDK", // why: .concierge/decision.md（是正 decision 自身の引用）
		"01KXMGGD6DS88CHGRJ9GPRBRVX",
		"01KXMRBB3PYJZMEXS7JTQQPP8D",
		"01KXMRBB3XJYTQ4WM3MZGZZY7C",
		"01KXMRBB447FDSRPH6ZAWVC7W2",
		"01KXMRBNXTN8742KDJVV4HW15V",
	}
	if !equalStrings(fixTargets, wantFix) {
		t.Fatalf("fixable 対象が想定と違う:\n got %v\nwant %v", fixTargets, wantFix)
	}
	if !equalStrings(ackTargets, wantAck) {
		t.Fatalf("acknowledge-only 対象が想定と違う:\n got %v\nwant %v", ackTargets, wantAck)
	}
}

// 全 advisory 規則の実 store 件数（record×rule 単位）を固定する。
func TestDogfoodAdvisoryRuleCounts(t *testing.T) {
	snap := dogfoodSnapshot(t)

	counts := make(map[string]int)
	for _, r := range Rules {
		if r.Tier != TierAdvisory {
			continue
		}
		counts[r.Name] = len(r.Check(snap))
	}
	want := map[string]int{
		"derived-value-in-desc": 0, // 増分2.3a の desc 浄化（D4-d）で axis.update.* の派生値列挙を除去し 4→0
		"stale-tense":           0, // 同上で #39/現状/新設/Level1/rev3/#40 等を除去し 7→0
		"prose-ref":             0,
		"why-file-line":         5, // 4→5: decision 01KZ7V637RNMPXJMVACYV6V1AS の why が、証拠として web/src の2箇所を file:line で引用した（append-only ゆえ是正不能・容認の理由は internal/cli の dogfoodKnownAckOnly）
		"axis-without-decision": 0,
		"duplicate-atom":        0, // フェーズ2 の duplicate merge（決定⑩）で 5グループ13遷移→5 に統合済み
		"dangling-id":           2, // decision 01KY1VDJWZF7M23K4X1J62QYXV の why 例示 `req.foo.1-1` が新規真ヒット
		"dead-doc-ref":          8, // 増分2.3a で tag/vocab desc の design-options 参照 11 を除去し 19→8（残 8 は decision 判断欄位＝append-only）
	}
	for rule, n := range want {
		if counts[rule] != n {
			t.Errorf("%s: %d 件（want %d）", rule, counts[rule], n)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
