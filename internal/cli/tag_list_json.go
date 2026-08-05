// tag_list_json.go — `tag list --json` が 1 タグについて何を渡すかの判断
// （01KZ5ACN6P279S96D5M3AHY9HZ）。
//
// **なぜ純関数にするか。** CLAUDE.md「配線ガードの書き方」1 の適用である。
// 渡す値を決めるのは「タグ」と「`--all` か否か」の 2 つだけなので、出力の体裁から
// 切り離して**入力と出力の対**で検査できる形にしておく——書式の照合を検査にすると、
// 同じ判断を別の綴りで書き直された瞬間に捕まらなくなる（同 2）。
//
// ⚠️ **`--json` には平坦と入れ子（`--tree`）の 2 つの面があり、判断はその両方を
// 通る位置に 1 つだけ置く。** 面ごとに分岐を書くと、片方だけ直す変異が通る——
// 面を足すたびにガードを置き忘れる型（同 5）は、置き忘れを数え上げるのではなく、
// 分かれる前に判断を済ませる形で塞ぐ。
package cli

import "github.com/nkenji09/scholia/internal/model"

// tagListJSONItems は `tag list --json` が渡すタグ列を返す。
//
// showAll のときは入力と同じ値をそのまま返す——`--all` の意味は「畳んでいるものを
// 全部開く」であり、**出力は畳む前と 1 バイトも変わらない**（正本の条件）。
// 既定では description だけを落とす。**落とすのは description ただ 1 つで、
// 他のフィールドには触れない**（同「他のフィールドは1つも変わらない」）。
//
// ⚠️ **落とし方は「別の構造体へ写す」ではなく「複製してその 1 フィールドを空にする」。**
// 写す形にすると、model.Tag に足したフィールドが宣言し忘れで黙って落ち、
// 「他のフィールドは1つも変わらない」が将来破れる。この形なら破れない。
//
// 入力のスライスと要素は変更しない。呼び出し側が握っている値をその場で書き換える
// 実装は、同じスナップショットを共有する面から見ると別の要求の応答を壊す
// （01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項 2 と同じ型の事故）。
func tagListJSONItems(tags []model.Tag, showAll bool) []model.Tag {
	out := make([]model.Tag, len(tags))
	copy(out, tags)
	if showAll {
		return out
	}
	for i := range out {
		out[i].Description = ""
	}
	return out
}
