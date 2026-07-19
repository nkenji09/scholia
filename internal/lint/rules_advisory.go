package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// --- requirement-gap: traceabilityKinds のタグで、実効タグとして充足する遷移が 0 件 ---

func checkRequirementGap(snap store.Snapshot) []Finding {
	traceKinds := make(map[string]bool, len(snap.Config.TraceabilityKinds))
	for _, k := range snap.Config.TraceabilityKinds {
		traceKinds[k] = true
	}
	if len(traceKinds) == 0 {
		return nil
	}

	covered := make(map[string]bool)
	for i := range snap.Transitions {
		for _, tagID := range index.EffectiveTags(&snap, &snap.Transitions[i]) {
			covered[tagID] = true
		}
	}

	directDecisions := tagDecisionCounts(snap.Decisions)
	var out []Finding
	for _, t := range snap.Tags {
		if !traceKinds[t.Kind] || covered[t.ID] {
			continue
		}
		out = append(out, finding("requirement-gap", SeverityWarn, t.ID,
			"tag %s: traceability kind %q ですが、実効タグとしてこれを持つ遷移が 0 件です（未充足要件・direct decision %d 件）",
			t.ID, t.Kind, directDecisions[t.ID]))
	}
	return out
}

// tagDecisionCounts は tag id -> その tag を直接 target とする decision 件数
// （requirement-gap の件数併記と decision-coverage の via-tag 判定の共有ヘルパ）。
func tagDecisionCounts(decisions []model.Decision) map[string]int {
	out := make(map[string]int)
	for _, d := range decisions {
		if d.Target.Type == model.DecisionTargetTag {
			out[d.Target.ID]++
		}
	}
	return out
}

// --- kind-missing: kind 未設定のタグを列挙（facet/traceability から漏れる） ---

func checkKindMissing(snap store.Snapshot) []Finding {
	var out []Finding
	for _, t := range snap.Tags {
		if t.Kind != "" {
			continue
		}
		out = append(out, finding("kind-missing", SeverityWarn, t.ID,
			"tag %s: kind が未設定です（どの facet/traceability にも属さないため階層・要件追跡から外れます）", t.ID))
	}
	return out
}

// --- ref-freshness: decision.ref / vocab.ref が file:line 形式（腐りやすい）なら警告 ---
//
// vocab.ref は #45 D5 で新設した外部契約アンカー。decision.ref と同型に file:line
// を warn する（腐る file:line を新スロットに書けてしまう穴を塞ぐ）。

func checkRefFreshness(snap store.Snapshot) []Finding {
	var out []Finding
	for _, d := range snap.Decisions {
		if d.Ref == "" || !isFileLineRef(d.Ref) {
			continue
		}
		out = append(out, finding("ref-freshness", SeverityWarn, d.ID,
			"decision %s: ref %q は file:line 形式です（コード変更で腐る。URL/commit hash を推奨）", d.ID, d.Ref))
	}
	for _, v := range snap.Vocab {
		if v.Ref == "" || !isFileLineRef(v.Ref) {
			continue
		}
		out = append(out, finding("ref-freshness", SeverityWarn, v.ID,
			"vocab %s: ref %q は file:line 形式です（コード変更で腐る。URL/commit hash・versioned 文書の § 参照を推奨）", v.ID, v.Ref))
	}
	return out
}

// isFileLineRef は "foo.go:42" のような file:line 参照を検出する。
// URL（"://" を含む）や末尾が数字でない参照（PR#42・commit hash 等）は対象外。
// 実装判断: DESIGN は「file:line」の例のみ示すため、この判定は本実装で定めるヒューリスティック。
func isFileLineRef(ref string) bool {
	if strings.Contains(ref, "://") {
		return false
	}
	idx := strings.LastIndex(ref, ":")
	if idx <= 0 || idx == len(ref)-1 {
		return false
	}
	linePart := ref[idx+1:]
	for _, c := range linePart {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// --- decision-coverage: 全遷移の why 到達性を3段（direct/via-tag/none）で判定 ---
//
// own decision の有無だけを見る旧判定は、tag 宛 decision へ実効タグ
// （own∪参照 vocab∪祖先閉包・§3.7）経由で到達できる遷移まで「why 未記録」と
// 報告していた（dogfood 実測で info 34 件中 32 件＝偽陽性 94%）。既にある宣言
// （tag 宛 decision・タグ祖先）を消費し、finding は全遷移分を coverage 付きで
// 返す（--json 全件）。none だけを列挙しサマリ行を出すのは CLI 側の表示規約。

func checkDecisionCoverage(snap store.Snapshot) []Finding {
	directCounts := make(map[string]int, len(snap.Decisions))
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition {
			directCounts[d.Target.ID]++
		}
	}
	tagCounts := tagDecisionCounts(snap.Decisions)

	var out []Finding
	for i := range snap.Transitions {
		t := &snap.Transitions[i]
		if n := directCounts[t.ID]; n > 0 {
			out = append(out, Finding{Rule: "decision-coverage", Severity: SeverityInfo, Target: t.ID,
				Coverage: CoverageDirect,
				Message:  fmt.Sprintf("transition %s: own decision %d 件（direct）", t.ID, n)})
			continue
		}
		// via-tag: 実効タグ全経路（EffectiveTagsWithProvenance＝own∪vocab∪祖先閉包・
		// 循環セーフ）のうち tag 宛 decision を持つものを出自として列挙する。
		var parts []string
		for _, et := range index.EffectiveTagsWithProvenance(&snap, t) {
			if n := tagCounts[et.ID]; n > 0 {
				parts = append(parts, fmt.Sprintf("%s (%d)", et.ID, n))
			}
		}
		if len(parts) > 0 {
			detail := "via " + strings.Join(parts, " / ")
			out = append(out, Finding{Rule: "decision-coverage", Severity: SeverityInfo, Target: t.ID,
				Coverage: CoverageViaTag, Detail: detail,
				Message: fmt.Sprintf("transition %s: own decision はありませんが、実効タグ経由で decision に到達できます（%s）", t.ID, detail)})
			continue
		}
		out = append(out, Finding{Rule: "decision-coverage", Severity: SeverityInfo, Target: t.ID,
			Coverage: CoverageNone,
			Message:  fmt.Sprintf("transition %s: own にも実効タグにも decision が 1 件もありません（none・why 未記録）", t.ID)})
	}
	return out
}

// --- exclusive-violation: 同一 given が同一 axis タグの複数値を同時に持つ ---
//
// #39 action-flow の gap 検出（internal/flow）の健全性は「軸の値は互いに排他」
// という不変条件に依存するが、model はこれを表現も検査もできない
// （req.action-flow.axis-gaps）。このルールは唯一 lint で機械検査できる部分＝
// 「1つの given が同一軸の2値を*名指し*していないか」を守る（軸が現実に排他
// かどうかまでは検査できない＝design-options §6.3 の重要注記1）。

func checkExclusiveViolation(snap store.Snapshot) []Finding {
	axisValues := axisValueTags(snap)
	if len(axisValues) == 0 {
		return nil
	}

	var out []Finding
	for _, t := range snap.Transitions {
		out = append(out, transitionExclusiveViolations(axisValues, t)...)
	}
	return out
}

// transitionExclusiveViolations は 1 transition 分の exclusive-violation 検査
// （lint 全量走査と write ゲート reject (a) が共有する検査コア・#45 U3。
// write ゲートだけ total 限定にすると lint と判定が食い違うため、どちらも
// 全 axis kind を対象にする）。
func transitionExclusiveViolations(axisValues map[string][]string, t model.Transition) []Finding {
	hits := make(map[string][]string) // axisID -> given 条件ids
	for _, g := range t.Given {
		for _, axisID := range axisValues[g] {
			hits[axisID] = append(hits[axisID], g)
		}
	}
	axisIDs := make([]string, 0, len(hits))
	for axisID := range hits {
		axisIDs = append(axisIDs, axisID)
	}
	sort.Strings(axisIDs)
	var out []Finding
	for _, axisID := range axisIDs {
		vals := hits[axisID]
		if len(vals) < 2 {
			continue
		}
		sort.Strings(vals)
		out = append(out, finding("exclusive-violation", SeverityWarn, t.ID,
			"transition %s: given が軸 %s の複数値を同時に持っています（%s）＝軸排他の不変条件が破れています", t.ID, axisID, strings.Join(vals, ", ")))
	}
	return out
}

// --- complement-missing: total=true 軸で materialize された値が2件未満 ---
//
// total=true は「軸の値のうち必ず1つが真」という宣言なので、値が1件しか
// materialize されていなければ相補条件が欠落している（design-options が指摘
// した cond.update-apply 不在の類）。internal/flow の L-total 抜け検出は
// 「materialize 済みの値がどの given にも現れない」ことしか見ないため、値自体
// が存在しない欠落はこの lint だけが拾う（gap 検出の健全性の前提条件）。

func checkComplementMissing(snap store.Snapshot) []Finding {
	valueCount := make(map[string]int)
	for _, v := range snap.Vocab {
		if v.Category != model.CategoryCondition {
			continue
		}
		for _, tagID := range v.Tags {
			valueCount[tagID]++
		}
	}

	var out []Finding
	for _, t := range snap.Tags {
		if t.Kind != "axis" || !t.Total {
			continue
		}
		if n := valueCount[t.ID]; n < 2 {
			out = append(out, finding("complement-missing", SeverityWarn, t.ID,
				"tag %s: total=true の軸だが materialize された値(condition)が %d 件しかありません（相補条件の欠落の疑い）", t.ID, n))
		}
	}
	return out
}

// axisValueTags は condition id -> それが貼られた axis タグ id 群、を返す
// （exclusive-violation の共有ヘルパ）。
func axisValueTags(snap store.Snapshot) map[string][]string {
	axisKind := make(map[string]bool)
	for _, t := range snap.Tags {
		if t.Kind == "axis" {
			axisKind[t.ID] = true
		}
	}
	if len(axisKind) == 0 {
		return nil
	}
	out := make(map[string][]string)
	for _, v := range snap.Vocab {
		if v.Category != model.CategoryCondition {
			continue
		}
		for _, tagID := range v.Tags {
			if axisKind[tagID] {
				out[v.ID] = append(out[v.ID], tagID)
			}
		}
	}
	return out
}

// --- unused-vocab: どの遷移からも参照されない語彙を列挙 ---
//
// axis kind タグ付き condition（＝軸の値）には「vocab rm の候補」を出さない。
// 軸の値が given に未出現なのは placeholder/remainder として想定内でありうる
// （正本 decision「rm しない」と真逆の削除助言を lint が配っていた是正・U1）。
// 代わりに軸 id・given 未出現の事実・当該軸の decision 件数と直近 id を文脈表示する。

func checkUnusedVocab(snap store.Snapshot) []Finding {
	referenced := make(map[string]bool)
	for _, t := range snap.Transitions {
		referenced[t.Action] = true
		for _, g := range t.Given {
			referenced[g] = true
		}
		for _, e := range t.Then {
			referenced[e] = true
		}
	}
	axisKind := make(map[string]bool)
	for _, t := range snap.Tags {
		if t.Kind == "axis" {
			axisKind[t.ID] = true
		}
	}
	var out []Finding
	for _, v := range snap.Vocab {
		if referenced[v.ID] {
			continue
		}
		var axes []string
		if v.Category == model.CategoryCondition {
			for _, tagID := range v.Tags {
				if axisKind[tagID] {
					axes = append(axes, tagID)
				}
			}
		}
		if len(axes) == 0 {
			out = append(out, finding("unused-vocab", SeverityInfo, v.ID,
				"vocab %s: どの遷移からも参照されていません（vocab rm の候補）", v.ID))
			continue
		}
		sort.Strings(axes)
		out = append(out, finding("unused-vocab", SeverityInfo, v.ID,
			"vocab %s: 軸 %s の値です（どの遷移の given にも未出現・placeholder/remainder 候補）。軸の decision: %s",
			v.ID, strings.Join(axes, "／"), axisDecisionSummary(snap.Decisions, axes)))
	}
	return out
}

// axisDecisionSummary は軸タグ群への direct decision の件数（＋直近 decision id）
// を「2 件（直近 01KX…）」形式で返す。複数軸に貼られた値は軸ごとに併記する。
func axisDecisionSummary(decisions []model.Decision, axes []string) string {
	latest := make(map[string]model.Decision)
	counts := make(map[string]int)
	for _, d := range decisions {
		if d.Target.Type != model.DecisionTargetTag {
			continue
		}
		counts[d.Target.ID]++
		cur, ok := latest[d.Target.ID]
		if !ok || d.At > cur.At || (d.At == cur.At && d.ID > cur.ID) {
			latest[d.Target.ID] = d
		}
	}
	segs := make([]string, 0, len(axes))
	for _, a := range axes {
		seg := fmt.Sprintf("%d 件", counts[a])
		if d, ok := latest[a]; ok {
			seg += fmt.Sprintf("（直近 %s）", d.ID)
		}
		if len(axes) > 1 {
			seg = a + " " + seg
		}
		segs = append(segs, seg)
	}
	return strings.Join(segs, "・")
}
