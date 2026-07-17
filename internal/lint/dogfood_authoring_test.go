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

// dangling-id: 真ヒット 1 件（偽陽性ゼロ）。素朴実装は 8 件中 7 件が偽陽性に
// なる実データ（族 glob `T-comment-*`・プレースホルダ `T-xxx`/`req.foobar`・
// kind 族 `eff.log`）を、除外3種 (E1)(E2)(E3) が全て畳むことを固定する。
func TestDogfoodDanglingIDHasZeroFalsePositives(t *testing.T) {
	snap := dogfoodSnapshot(t)
	findings := checkDanglingID(snap)

	// 素朴実装の偽陽性 7 件（kit-bundle2-retrofit-findings.md §7）が
	// 1 件も finding にならないこと。
	falsePositiveRecords := []string{
		"01KXM9VN3FPGE5C2APBTNRWGHA",        // why: T-comment-*（E1）
		"req.evaluate-change.adopt-cleanup", // desc: `T-comment-*`（E1）
		"01KXEVDGYNB32K3WXKMV8Z4RVW",        // changed: T-skills-install-*（E1）
		"01KXFEXG01RS00RHAVS3TMP25Y",        // why: T-xxx（E2）
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

	if len(findings) != 1 {
		t.Fatalf("真ヒット 1 件（偽陽性ゼロ）のはずが %d 件: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Target != "01KXM9X0E21T5C6W1HKKDZWM91" || f.TargetType != "decision" ||
		f.Field != "changed" || f.Quote != "T-viewer-adopt-comment-removed" || !f.AcknowledgeOnly {
		t.Fatalf("真ヒットの内容が想定と違う: %+v", f)
	}
}

// dead-doc-ref: design-options 参照型（散逸文書 16 レコード）＋.concierge 系＋
// tweaks3 系を実 store で検出できること。versioned に解決する参照
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

	wantFix := []string{
		"axis.update.install", "axis.update.mode", "axis.update.platform", "axis.update.status",
		"cond.update-apply",
		"req.action-flow", "req.action-flow.acknowledged-remainder", "req.action-flow.axis-gaps",
		"req.action-flow.scope-honesty", "req.action-flow.subset-shadow", "req.action-flow.visualize",
	}
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
		"derived-value-in-desc": 4,
		"stale-tense":           7,
		"prose-ref":             0,
		"why-file-line":         4,
		"axis-without-decision": 0,
		"duplicate-atom":        5,
		"dangling-id":           1,
		"dead-doc-ref":          19, // design-options 系 16＋.concierge 系 2＋tweaks3 系 1
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
