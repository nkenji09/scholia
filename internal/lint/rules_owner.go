// rules_owner.go — multiple-owner-action lint（#45 D9・info 級）。
//
// #40③（01KXNGQYS14G2XPDW19Y8JGX4B）が「owner 導出設計を詰めてからの別提案」と
// して保留した lint の、その別提案。owner を effect に構造化（VocabEntry.Owner）
// したうえで、宣言軸が張られた action の owner 集合が単一 subject に定まるかを
// info で開示する。error にはしない——保存拒否は自己矛盾不変条件のみ、の原則の枠内。
//
// 導出規則（D9 why）:
//   - transition の owner 集合＝その then 効果の owner の和集合。
//   - action の owner 集合＝その action に属する全 transition の owner 集合の和集合。
//   - 検査対象は「宣言軸が張られた action」のみ（flow の relevantAxes 非空を再利用）。
//
// 同一性は owner id の厳密一致（祖先畳み込みなし）。owner 無指定 effect の混在も
// 開示に含める。OwnerKind 未宣言時も走るが「自由文字列の完全一致ゆえ表記揺れで
// 偽の複数判定が出うる」旨を finding 文面に開示する。
package lint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nkenji09/scholia/internal/flow"
	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

func checkMultipleOwnerAction(snap store.Snapshot) []Finding {
	ix := index.Build(&snap)
	vocabByID := indexVocab(snap.Vocab)

	// action id -> その action に属する transition 群。
	txByAction := make(map[string][]model.Transition)
	for _, t := range snap.Transitions {
		txByAction[t.Action] = append(txByAction[t.Action], t)
	}

	actionIDs := make([]string, 0, len(txByAction))
	for id := range txByAction {
		actionIDs = append(actionIDs, id)
	}
	sort.Strings(actionIDs)

	var out []Finding
	for _, actionID := range actionIDs {
		// 宣言軸が張られた action のみ検査対象（relevantAxes 非空を再利用）。
		// flow.Analyze は cfg を snap.Config から読むので axis 述語は D9 と一貫する。
		report := flow.Analyze(&snap, ix, actionID)
		if len(report.Axes) == 0 {
			continue
		}

		// owner 集合の導出（then 効果の owner の和集合・厳密 id 一致）。
		owners := make(map[string]bool)
		hasUnowned := false
		for _, t := range txByAction[actionID] {
			for _, effID := range t.Then {
				e, ok := vocabByID[effID]
				if !ok || e.Category != model.CategoryEffect {
					continue
				}
				if e.Owner == "" {
					hasUnowned = true
					continue
				}
				owners[e.Owner] = true
			}
		}

		distinct := len(owners)
		// 単一 owner に定まる（無指定 effect も無い）なら沈黙。
		if distinct <= 1 && !hasUnowned {
			continue
		}
		// owner が全く無い（全 effect が無指定）action は「owner 導出の材料が無い」
		// だけで複数混在ではない——開示対象外（沈黙）。
		if distinct == 0 {
			continue
		}
		// ここに来るのは distinct>=2、または distinct==1 だが無指定 effect 混在。
		ownerList := make([]string, 0, len(owners))
		for o := range owners {
			ownerList = append(ownerList, o)
		}
		sort.Strings(ownerList)
		display := strings.Join(ownerList, ", ")
		if hasUnowned {
			if display == "" {
				display = "(owner 無指定のみ)"
			} else {
				display += ", (owner 無指定 effect あり)"
			}
		}

		msg := fmt.Sprintf(
			"action %s: 宣言軸が張られていますが owner 集合が単一 subject に定まりません（%s）＝偽 overlap の可能性・軸分析の単一 owner 前提を満たしません",
			actionID, display)
		if snap.Config.OwnerKind == "" {
			msg += "（ownerKind 未宣言のため owner は自由文字列の完全一致で判定——表記揺れで偽の複数判定が出うる）"
		}
		out = append(out, Finding{
			Rule:       "multiple-owner-action",
			Severity:   SeverityInfo,
			Target:     actionID,
			TargetType: targetVocab,
			Message:    msg,
		})
	}
	return out
}
