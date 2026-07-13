package lint

import (
	"strings"

	"github.com/nkenji09/product-memory/internal/index"
	"github.com/nkenji09/product-memory/internal/model"
	"github.com/nkenji09/product-memory/internal/store"
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
