package lint

import (
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

	var out []Finding
	for _, t := range snap.Tags {
		if !traceKinds[t.Kind] || covered[t.ID] {
			continue
		}
		out = append(out, finding("requirement-gap", SeverityWarn, t.ID,
			"tag %s: traceability kind %q ですが、実効タグとしてこれを持つ遷移が 0 件です（未充足要件）", t.ID, t.Kind))
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

// --- ref-freshness: decision.ref が file:line 形式（腐りやすい）なら警告 ---

func checkRefFreshness(snap store.Snapshot) []Finding {
	var out []Finding
	for _, d := range snap.Decisions {
		if d.Ref == "" || !isFileLineRef(d.Ref) {
			continue
		}
		out = append(out, finding("ref-freshness", SeverityWarn, d.ID,
			"decision %s: ref %q は file:line 形式です（コード変更で腐る。URL/commit hash を推奨）", d.ID, d.Ref))
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

// --- decision-coverage: decision が 1 件も付いていない遷移を列挙 ---

func checkDecisionCoverage(snap store.Snapshot) []Finding {
	covered := make(map[string]bool, len(snap.Decisions))
	for _, d := range snap.Decisions {
		if d.Target.Type == model.DecisionTargetTransition {
			covered[d.Target.ID] = true
		}
	}
	var out []Finding
	for _, t := range snap.Transitions {
		if covered[t.ID] {
			continue
		}
		out = append(out, finding("decision-coverage", SeverityInfo, t.ID,
			"transition %s: decision が 1 件も付いていません（why が未記録）", t.ID))
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
		for _, axisID := range axisIDs {
			vals := hits[axisID]
			if len(vals) < 2 {
				continue
			}
			sort.Strings(vals)
			out = append(out, finding("exclusive-violation", SeverityWarn, t.ID,
				"transition %s: given が軸 %s の複数値を同時に持っています（%s）＝軸排他の不変条件が破れています", t.ID, axisID, strings.Join(vals, ", ")))
		}
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
	var out []Finding
	for _, v := range snap.Vocab {
		if referenced[v.ID] {
			continue
		}
		out = append(out, finding("unused-vocab", SeverityInfo, v.ID,
			"vocab %s: どの遷移からも参照されていません（vocab rm の候補）", v.ID))
	}
	return out
}
