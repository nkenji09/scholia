// decision_heading.go — decision の `why` 1 行目が「見出し」かの機械判定
// （01KZ06SYR3APGF3JD4NQRFTEEN 変更1・2）。
//
// **この 1 つの述語が、2 つの面の唯一の根拠である。**
//
//	保存時の拒否（store.CreateDecision / lint.CheckWrite）
//	  — 見出しの無い why を新規保存させない
//	畳んだ規則一覧の見出し表示（cli の rules 深掘り）
//	  — 「著者が見出しとして書いた行」だけを出す
//
// 判定を model に置くのは、store が lint を import できない（lint → store の
// 一方向依存）ためであり、かつ**2 面が別々に「見出しとは何か」を決めない**ため
// である。保存時に通った why の 1 行目は必ず見出しとして表示でき、表示に出た行は
// 必ず保存時に通った行である——この対応が崩れると、保存時ゲートが要求した形と
// 一覧が出す形がずれる。
//
// ⚠️ **「良い見出しかどうか」は判定しない**（01KYKS4RH2MX3KQWB56FPD0MKG:
// 機械判定できない書き方の規律は正本に置かない）。ここが決めるのは
// 「著者が見出しとして書いた行が在るか」だけである。
package model

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// DecisionHeadingMaxRunes は見出し本体の上限（rune 数）。
//
// 80 の根拠は実測（01KZ06SYR3APGF3JD4NQRFTEEN 変更1）——見出し付き 37 件のうち
// 34 件（92%）が 80 字以内・中央値 44 字。60 字だと 86%・100 字だと 97%。
// ⚠️ 既存の書き方から決めた数字であり、新しい書き手に妥当かは測れていない。
const DecisionHeadingMaxRunes = 80

// DecisionHeadingResult は 1 件の判定結果。
type DecisionHeadingResult struct {
	// Heading は見出し本体（`#` と続く空白を除いた部分）。OK のときだけ非空。
	Heading string
	// OK は 3 条件をすべて満たすか。
	OK bool
	// Reason は満たさないときの理由（人向け・OK のときは空）。
	Reason string
}

// CheckDecisionHeading は why の 1 行目が見出しかを判定する。
//
// 3 条件（01KZ06SYR3APGF3JD4NQRFTEEN 変更1）:
//
//	(1) 1 行目が `#` で始まる
//	(2) `#` と続く空白を除いた本体が 1 字以上 DecisionHeadingMaxRunes 字以内（rune）
//	(3) 2 行目以降に本文がある（見出しだけの why を通さない）
//
// (2) の上限が要る理由: (1) だけでは `# ` に続けて 431 字を書けば素通りする
// （実測で見出しの無い 1 行目は中央値 431 字・最大 2,042 字）。
func CheckDecisionHeading(why string) DecisionHeadingResult {
	lines := strings.Split(strings.ReplaceAll(why, "\r\n", "\n"), "\n")
	first := strings.TrimRight(lines[0], "\r")

	if !strings.HasPrefix(first, "#") {
		return DecisionHeadingResult{Reason: "1 行目が `#` で始まっていません"}
	}
	heading := strings.TrimSpace(strings.TrimLeft(first, "#"))
	if heading == "" {
		return DecisionHeadingResult{Reason: "見出しの本体が空です（`#` と空白しかありません）"}
	}
	if n := utf8.RuneCountInString(heading); n > DecisionHeadingMaxRunes {
		return DecisionHeadingResult{Reason: fmt.Sprintf(
			"見出しの本体が %d 字あります（上限 %d 字）", n, DecisionHeadingMaxRunes)}
	}
	if !hasBodyAfterFirstLine(lines) {
		return DecisionHeadingResult{Reason: "2 行目以降に本文がありません（見出しだけの why は通しません）"}
	}
	return DecisionHeadingResult{Heading: heading, OK: true}
}

// DecisionHeadingOf は表示側の入口——見出しがあればその本体を返す。
// 判定は CheckDecisionHeading と同じもので、別の基準を持たない。
func DecisionHeadingOf(why string) (string, bool) {
	r := CheckDecisionHeading(why)
	return r.Heading, r.OK
}

func hasBodyAfterFirstLine(lines []string) bool {
	for _, l := range lines[1:] {
		if strings.TrimSpace(l) != "" {
			return true
		}
	}
	return false
}
