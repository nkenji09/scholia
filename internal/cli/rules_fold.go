// rules_fold.go — 「本文を渡すか、存在と引き方だけ渡すか」の判断
// （01KZ06SYP12ZFDG1WPNYM529D8）。
//
// **なぜ純関数にするか。** CLAUDE.md「配線ガードの書き方」1 の適用である。
// どの decision の本文を出すかは、出自（own / effective-tag / parent）と効力
// （効いている / 取り下げ済み）と `--all` の 3 つだけで決まる。出力の体裁から
// 切り離して値で検査できる形にしておく——書式の照合を検査にすると、同じ意味を
// 別の綴りで書かれた瞬間に捕まらなくなる（CLAUDE.md 2）。
package cli

import (
	"sort"

	"github.com/nkenji09/scholia/internal/index"
	"github.com/nkenji09/scholia/internal/model"
)

// foldedRules は rules の出力を 3 群に分けた結果。
//
// 3 群の合計は入力の全件と等しい（1 件も落とさない・受け入れ基準）。
type foldedRules struct {
	// Bodies は本文を渡す群＝**その記録自身への decision で、効いているもの**。
	// --all のときは全件がここへ来る。
	Bodies []index.GovernsEntry
	// Inherited は経由で届く群（effective-tag / parent）。本文を渡さない。
	// 取り下げ済みのものもここに含む（下記「取り下げとの交差」）。
	Inherited []index.GovernsEntry
	// Withdrawn は自身への decision のうち取り下げられたもの。
	Withdrawn []index.GovernsEntry
}

// foldRules は本文を渡す群と、存在と引き方だけ渡す群に分ける。
//
// showAll のときは何も畳まない（--all の意味＝「畳んでいるものを全部開く」・
// 01KZ06SYP12ZFDG1WPNYM529D8 変更2）。
//
// **取り下げとの交差**（正本が未定義だった箇所・実装で一方に寄せる）:
// 「経由で届き、かつ取り下げられている」decision は **Inherited へ寄せ、行に
// 失効の印を付ける**。取り下げ群（Withdrawn）は「この記録自身に書かれた規則が
// 何に置き換わったか」を示す欄で、置き換えた側が同じ出力の本文側に必ず載って
// いることを前提に書かれている（currency.go writeWithdrawn）。経由分の置き換え
// 先は本文側に載らないので、同じ欄へ混ぜるとその前提が崩れる。加えて、経由分を
// Withdrawn へ移すと Inherited の件数が「--all で本文が返る経由分の集合」と
// 一致しなくなり、受け入れ基準（件数と経由タグが一致する）を満たせない。
func foldRules(entries []index.GovernsEntry, replaced func(id string) bool, showAll bool) foldedRules {
	if showAll {
		return foldedRules{Bodies: append([]index.GovernsEntry(nil), entries...)}
	}
	var out foldedRules
	for _, e := range entries {
		switch {
		case e.Provenance != index.GovernsOwn:
			out.Inherited = append(out.Inherited, e)
		case replaced(e.Decision.ID):
			out.Withdrawn = append(out.Withdrawn, e)
		default:
			out.Bodies = append(out.Bodies, e)
		}
	}
	return out
}

// provenanceLabel は経由の種別の表示語（変更3 ⚠️「経由の種別を区別して出す」）。
// 「この遷移が属する要件の規則」と「その要件の上位領域の規則」は読み手にとって
// 別の重みを持つので、同じ語で束ねない。
func provenanceLabel(p index.GovernsProvenance) string {
	switch p {
	case index.GovernsEffectiveTag:
		return "直接持つタグ"
	case index.GovernsViaParent:
		return "祖先タグ"
	}
	return ""
}

// governsDecisions は出自を落として decision だけ取り出す（本文側の描画用）。
func governsDecisions(entries []index.GovernsEntry) []model.Decision {
	out := make([]model.Decision, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Decision)
	}
	return out
}

// ownEntries は出自の概念が無い選択子（--facet・選択子なし）の結果を
// GovernsEntry へ包む。経由が無い以上すべて own で、畳み込みは当たらない
// （01KZ06SYP12ZFDG1WPNYM529D8 変更1 ⚠️）。
func ownEntries(decisions []model.Decision) []index.GovernsEntry {
	out := make([]index.GovernsEntry, 0, len(decisions))
	for _, d := range decisions {
		out = append(out, index.GovernsEntry{Decision: d, Provenance: index.GovernsOwn})
	}
	return out
}

// sortGovernsEntries は本文側と畳んだ側に同じ並び順を当てる。
func sortGovernsEntries(entries []index.GovernsEntry, sortBy string) {
	if sortBy == "target" {
		sort.SliceStable(entries, func(i, j int) bool {
			ti, tj := entries[i].Decision.Target, entries[j].Decision.Target
			if ti.Type != tj.Type {
				return ti.Type < tj.Type
			}
			if ti.ID != tj.ID {
				return ti.ID < tj.ID
			}
			return entries[i].Decision.At < entries[j].Decision.At
		})
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].Decision.At < entries[j].Decision.At
	})
}
