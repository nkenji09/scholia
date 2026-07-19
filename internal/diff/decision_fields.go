// decision_fields.go — DecisionViolation の欄位単位再定義（#45 U4／P2）。
//
// append-only とは「decision ファイル完全不変」ではなく「判断欄位の不変＋来歴
// 欄位の単調追記」である:
//   - 判断欄位（why / changed / ref / at・target.type）は凍結（不可侵）。
//   - commits[]・acknowledges[]（#45 D6）・supersedes[]（#45 D7）は追記のみ
//     許容（既存要素の削除・改変・並べ替えは違反）。supersedes は {id,mode} 単位で
//     順序保存包含を要求する（mode 改変も既存 link の改変＝違反）。
//   - target.id は正本レコード側の rename／merge の機械追随でのみ張替わる——
//     同一 diff 内のペア照合（旧 id 消滅＋新 id 出現＝rename、旧 transition
//     消滅＋現存 transition 宛＝merge・決定⑩）が取れる場合のみ許容。
//   - 本実装がまだ知らない将来 additive フィールドの追記は violation にしない
//     （前方互換）。model.Decision decode が未知フィールドを無視するため、
//     そのような変更はそもそも Changed に現れない（diffDecisions のコメント参照）。
//
// これにより `decision add-commit`・`tag/tx/vocab rename`・`tx merge` の正規操作が
// 偽陽性で撃墜される問題（実証: commit 29e817c の add-commit）を解消しつつ、
// #42 型の判断欄位改変（実証: commit 65cb5a4）は違反のまま検出する。
package diff

import (
	"fmt"
	"reflect"

	"github.com/nkenji09/scholia/internal/model"
)

// pairContext は rename/merge ペア照合に使う before/after のレコード索引。
type pairContext struct {
	txBefore, txAfter       map[string]model.Transition
	tagBefore, tagAfter     map[string]model.Tag
	vocabBefore, vocabAfter map[string]model.VocabEntry
	// tagRenames は removed/added タグ集合から内容照合で復元した rename ペア
	// （旧 id → 新 id・cascade はサブツリー単位で照合済み）。
	tagRenames map[string]string
}

func newPairContext(before, after refSnapshot) *pairContext {
	ctx := &pairContext{
		txBefore:    indexByID(before.Transitions, model.Transition.GetID),
		txAfter:     indexByID(after.Transitions, model.Transition.GetID),
		tagBefore:   indexByID(before.Tags, model.Tag.GetID),
		tagAfter:    indexByID(after.Tags, model.Tag.GetID),
		vocabBefore: indexByID(before.Vocab, model.VocabEntry.GetID),
		vocabAfter:  indexByID(after.Vocab, model.VocabEntry.GetID),
	}
	removedTags := map[string]model.Tag{}
	for id, t := range ctx.tagBefore {
		if _, ok := ctx.tagAfter[id]; !ok {
			removedTags[id] = t
		}
	}
	addedTags := map[string]model.Tag{}
	for id, t := range ctx.tagAfter {
		if _, ok := ctx.tagBefore[id]; !ok {
			addedTags[id] = t
		}
	}
	ctx.tagRenames = tagRenameMap(removedTags, addedTags)
	return ctx
}

// classifyDecisionChange は同一 id の decision の before/after を欄位単位で
// 「許容（allowed）」と「違反（violated）」に分類する。
func classifyDecisionChange(b, a model.Decision, ctx *pairContext) (allowed, violated []string) {
	if b.Why != a.Why {
		violated = append(violated, "why")
	}
	if b.Changed != a.Changed {
		violated = append(violated, "changed")
	}
	if b.Ref != a.Ref {
		violated = append(violated, "ref")
	}
	if b.At != a.At {
		violated = append(violated, "at")
	}
	if b.Target.Type != a.Target.Type {
		violated = append(violated, "target.type")
		if b.Target.ID != a.Target.ID {
			// type ごと変わる張替えに rename/merge の正規形は無い。
			violated = append(violated, "target.id")
		}
	} else if b.Target.ID != a.Target.ID {
		if desc, ok := ctx.classifyRepoint(b.Target.Type, b.Target.ID, a.Target.ID); ok {
			allowed = append(allowed, desc)
		} else {
			violated = append(violated, "target.id")
		}
	}
	if !reflect.DeepEqual(b.Commits, a.Commits) {
		if commitsAppendOnly(b.Commits, a.Commits) {
			allowed = append(allowed, fmt.Sprintf("commits(+%d)", len(a.Commits)-len(b.Commits)))
		} else {
			violated = append(violated, "commits")
		}
	}
	// Supersedes（#45 D7）は commits と同型の追記専用: 既存 link（{id,mode}
	// 単位）の順序保存包含のみ許容（追記のみ）。既存 link の削除・改変（mode
	// 変更を含む）・並べ替えは違反。model に Supersedes を足すと未知フィールド
	// でなくなり reflect.DeepEqual が変更を検出する——ここで分類しないと既存 link
	// の改変が allowed にも violated にも入らず黙認される穴（append-only 破れ）に
	// なるため、必ず分類する。
	if !reflect.DeepEqual(b.Supersedes, a.Supersedes) {
		if supersedesAppendOnly(b.Supersedes, a.Supersedes) {
			allowed = append(allowed, fmt.Sprintf("supersedes(+%d)", len(a.Supersedes)-len(b.Supersedes)))
		} else {
			violated = append(violated, "supersedes")
		}
	}
	// Acknowledges（#45 D6）も追記専用: 既存要素を削除すると、過去に畳んで
	// いた finding が retroactively 蘇る（容認の取り消し＝判断の書き換え）。
	// commits と同型に、既存⊆新の順序保存包含のみ許容する。
	if !reflect.DeepEqual(b.Acknowledges, a.Acknowledges) {
		if commitsAppendOnly(b.Acknowledges, a.Acknowledges) {
			allowed = append(allowed, fmt.Sprintf("acknowledges(+%d)", len(a.Acknowledges)-len(b.Acknowledges)))
		} else {
			violated = append(violated, "acknowledges")
		}
	}
	return allowed, violated
}

// supersedesAppendOnly は before の各 link（{id,mode} 完全一致）が after に
// 順序保存で包含される（既存 link の削除・mode 改変・並べ替えなし＝追記のみ）
// かを返す。commitsAppendOnly の {id,mode} 版。
func supersedesAppendOnly(before, after []model.SupersedeLink) bool {
	i := 0
	for _, l := range after {
		if i < len(before) && before[i] == l {
			i++
		}
	}
	return i == len(before)
}

// commitsAppendOnly は before が after に順序保存で包含される（既存要素の削除・
// 改変・並べ替えなし＝追記のみ）かを返す（許容① Commits 追記・既存⊆新）。
func commitsAppendOnly(before, after []string) bool {
	i := 0
	for _, c := range after {
		if i < len(before) && before[i] == c {
			i++
		}
	}
	return i == len(before)
}

// classifyRepoint は target.id の張替え（old→new）が rename／merge の機械追随と
// して許容できるかを判定する（許容②③）。共通の必要条件は「旧 id のレコードが
// 同一 diff 内で消滅していること」——旧レコードが残ったままの張替えに正規操作は
// 無い。悪用対策: 判断欄位はいかなるペアの存在下でも不可侵（呼び出し側で独立に
// 判定される）。
func (ctx *pairContext) classifyRepoint(targetType, oldID, newID string) (string, bool) {
	switch targetType {
	case model.DecisionTargetTransition:
		if !removedFrom(ctx.txBefore, ctx.txAfter, oldID) {
			return "", false
		}
		newTx, inAfter := ctx.txAfter[newID]
		if !inAfter {
			return "", false
		}
		if _, existedBefore := ctx.txBefore[newID]; existedBefore {
			// merge ペア照合（決定⑩）: 旧 transition 消滅＋現存 transition 宛。
			return fmt.Sprintf("target.id(merge %s→%s)", oldID, newID), true
		}
		if transitionsSemanticallyEqual(ctx.txBefore[oldID], newTx) {
			return fmt.Sprintf("target.id(rename %s→%s)", oldID, newID), true
		}
		// rename 後に同一 PR 内でレコード自体がさらに編集された形（内容同一
		// 照合は破れるが、旧 id 消滅＋新 id 出現のペアは取れている）。
		return fmt.Sprintf("target.id(rename+edit %s→%s)", oldID, newID), true
	case model.DecisionTargetTag:
		if !removedFrom(ctx.tagBefore, ctx.tagAfter, oldID) {
			return "", false
		}
		if _, inAfter := ctx.tagAfter[newID]; !inAfter {
			return "", false
		}
		if _, existedBefore := ctx.tagBefore[newID]; existedBefore {
			// 既存タグへの張替え（tag の merge）に正規操作は無い。
			return "", false
		}
		if ctx.tagRenames[oldID] == newID {
			return fmt.Sprintf("target.id(rename %s→%s)", oldID, newID), true
		}
		return fmt.Sprintf("target.id(rename+edit %s→%s)", oldID, newID), true
	case "vocab":
		// P5 前方互換: vocab-target decision の導入（P5）前に判定意味論のみ
		// 定義しておく（vocab rename も rename ペア照合の対象・#45 U4）。
		if !removedFrom(ctx.vocabBefore, ctx.vocabAfter, oldID) {
			return "", false
		}
		newV, inAfter := ctx.vocabAfter[newID]
		if !inAfter {
			return "", false
		}
		if _, existedBefore := ctx.vocabBefore[newID]; existedBefore {
			return "", false
		}
		if vocabEqualExceptID(ctx.vocabBefore[oldID], newV) {
			return fmt.Sprintf("target.id(rename %s→%s)", oldID, newID), true
		}
		return fmt.Sprintf("target.id(rename+edit %s→%s)", oldID, newID), true
	}
	return "", false
}

// removedFrom は id が before に存在し after で消滅していることを返す。
func removedFrom[T any](before, after map[string]T, id string) bool {
	if _, had := before[id]; !had {
		return false
	}
	_, still := after[id]
	return !still
}

// vocabEqualExceptID は id 以外の内容が同一か（tags は集合として比較）。
func vocabEqualExceptID(a, b model.VocabEntry) bool {
	if !reflect.DeepEqual(sortedCopy(a.Tags), sortedCopy(b.Tags)) {
		return false
	}
	a.ID, b.ID = "", ""
	a.Tags, b.Tags = nil, nil
	return reflect.DeepEqual(a, b)
}

// tagRenameMap は removed/added タグ集合から rename ペア（旧 id → 新 id）を
// 内容照合で復元する。cascade（サブツリー改名）は「親の対応が確定すると子の
// parentIds 置換（親 id の張替え）が検証可能になる」構造なので、対応が増え
// なくなるまで反復する＝サブツリー単位の照合。内容が同一で対応先が曖昧な
// ペアは対応付けない（呼び出し側が rename+edit として扱う——保守的でも許容
// 側に落ちるため偽陽性は生まない）。
func tagRenameMap(removed, added map[string]model.Tag) map[string]string {
	m := map[string]string{}
	used := map[string]bool{}
	if len(removed) == 0 || len(added) == 0 {
		return m
	}
	removedIDs := sortedKeys(removed)
	addedIDs := sortedKeys(added)
	for {
		progress := false
		for _, oldID := range removedIDs {
			if _, done := m[oldID]; done {
				continue
			}
			var matches []string
			for _, newID := range addedIDs {
				if used[newID] {
					continue
				}
				if tagsEqualExceptIDWithParentMap(removed[oldID], added[newID], m) {
					matches = append(matches, newID)
				}
			}
			if len(matches) != 1 {
				continue
			}
			newID := matches[0]
			// 逆向きの一意性: 同じ added に対応し得る未対応の removed が他にも
			// あれば曖昧（内容が同一の別タグ）なので対応付けない。
			ambiguous := false
			for _, otherOld := range removedIDs {
				if otherOld == oldID {
					continue
				}
				if _, done := m[otherOld]; done {
					continue
				}
				if tagsEqualExceptIDWithParentMap(removed[otherOld], added[newID], m) {
					ambiguous = true
					break
				}
			}
			if ambiguous {
				continue
			}
			m[oldID] = newID
			used[newID] = true
			progress = true
		}
		if !progress {
			return m
		}
	}
}

// tagsEqualExceptIDWithParentMap は id 以外の内容が同一かを、r（旧）の parentIds
// に rename 対応 m を適用したうえで比較する（cascade の子孫は「親 id の置換」
// だけが変わるため、それを追随扱いにする・#45 U4）。
func tagsEqualExceptIDWithParentMap(r, a model.Tag, m map[string]string) bool {
	if r.Name != a.Name || r.Kind != a.Kind || r.Description != a.Description ||
		r.Color != a.Color || r.Ref != a.Ref || r.Total != a.Total {
		return false
	}
	mapped := make([]string, len(r.ParentIDs))
	for i, p := range r.ParentIDs {
		if n, ok := m[p]; ok {
			mapped[i] = n
		} else {
			mapped[i] = p
		}
	}
	return reflect.DeepEqual(sortedCopy(mapped), sortedCopy(a.ParentIDs))
}
