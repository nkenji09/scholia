package textcheck

import (
	"fmt"
	"strings"
)

// DefaultQuestion は、記録を書いた側の経緯を知らない読み手に、意味が複数考えられる言葉や文を挙げさせる既定の問い（telos と同じ文）。
// 記録の種類を問わず、同じ文で使う。
const DefaultQuestion = `あなたは、このプロジェクトのことも、ほかの記録のことも何も知らない読み手です。渡された文だけを読み、初めて読んだ人が「これはどういう意味だろう」と迷う言葉や文を挙げてください。
- 字のとおりの意味と違う物を指していそうな言葉（例：「検査を撃つ」「読む口」）
- 読む人によって指す物が変わりそうな言葉（例：「新しい方」）
- 何と何を比べるのか、どの状態の物かが書かれていない条件（例：「古ければ拒む」）
- 二通りに読める文、同じ句が重なって崩れた文
知らない用語でも、意味が一通りに決まりそうなら挙げなくてよいです。迷ったら挙げてください。誤りでも構いません。
直し方は書かないでください。reason は 15 字以内で書いてください。`

// Lines は、結果を人向けの行にする。候補は 1 件 1 行、行われなかった部分は 1 部分 1 行、
// どちらも無い確かめた結果は 1 行。読み手の指定が無かった結果は何も書かない。
func Lines(r Result) []string {
	if r.NoReader {
		return nil
	}
	var lines []string
	for _, c := range r.Candidates {
		lines = append(lines, fmt.Sprintf("確かめ: %s 「%s」 %s", c.Key, c.Phrase, c.Reason))
	}
	for _, n := range r.NotChecked {
		lines = append(lines, fmt.Sprintf("確かめ: 行われなかった（%s） %s", n.Reason, strings.Join(n.IDs, "、")))
	}
	if len(lines) == 0 {
		lines = append(lines, "確かめ: わかりにくい表現の候補は無い")
	}
	return lines
}
