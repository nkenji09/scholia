package model

import (
	"strings"
	"testing"
)

// 見出しの判定は入力と出力の対で検査する（CLAUDE.md「配線ガードの書き方」1）。
// 画面もコマンドも起こさない——起こす検査は、同じ判定を別の綴りで書き直された
// ときに黙って通る。
func TestCheckDecisionHeading(t *testing.T) {
	long := strings.Repeat("あ", DecisionHeadingMaxRunes)
	tooLong := strings.Repeat("あ", DecisionHeadingMaxRunes+1)

	cases := []struct {
		name    string
		why     string
		ok      bool
		heading string
	}{
		{"3 条件を満たす", "# 見出し\n\n本文", true, "見出し"},
		{"複数の #（節見出し）も見出し", "## 見出し\n\n本文", true, "見出し"},
		{"空行なしで本文が続く", "# 見出し\n本文", true, "見出し"},
		{"上限ちょうど", "# " + long + "\n\n本文", true, long},
		{"前後の空白は本体に含めない", "#   見出し  \n\n本文", true, "見出し"},
		{"CRLF", "# 見出し\r\n\r\n本文", true, "見出し"},

		{"1 行目が # で始まらない", "見出しではない\n\n本文", false, ""},
		{"空の why", "", false, ""},
		{"# だけ", "#\n\n本文", false, ""},
		{"# と空白だけ", "#   \n\n本文", false, ""},
		{"上限を 1 字超える", "# " + tooLong + "\n\n本文", false, ""},
		{"見出しだけで本文が無い", "# 見出し", false, ""},
		{"2 行目以降が空白だけ", "# 見出し\n\n   \n\t\n", false, ""},
		{"先頭に空行がある（1 行目は空行）", "\n# 見出し\n\n本文", false, ""},

		// ⚠️ この 1 件が閾値を置いた理由そのもの。「1 行目が # で始まる」だけを
		// 条件にすると、`# ` に続けて 431 字を書いた既存の書き方が素通りする
		// （実測で見出しの無い 1 行目は中央値 431 字・最大 2,042 字）。
		{"# に続けて長文を 1 行で書く", "# " + strings.Repeat("長", 431) + "\n\n本文", false, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckDecisionHeading(c.why)
			if got.OK != c.ok {
				t.Fatalf("OK = %v, want %v（理由: %s）", got.OK, c.ok, got.Reason)
			}
			if got.Heading != c.heading {
				t.Fatalf("Heading = %q, want %q", got.Heading, c.heading)
			}
			if !got.OK && got.Reason == "" {
				t.Fatalf("満たさないときは理由を返すべき（書き手が何を直せばよいか分からなくなる）")
			}
			// 表示側の入口が同じ答えを返すこと。別々に判定すると、保存時に
			// 通った why の 1 行目が一覧に出ない（あるいはその逆）が起きる。
			h, ok := DecisionHeadingOf(c.why)
			if ok != got.OK || h != got.Heading {
				t.Fatalf("DecisionHeadingOf = (%q, %v), CheckDecisionHeading = (%q, %v) — 2 つの面が別の基準を持っている",
					h, ok, got.Heading, got.OK)
			}
		})
	}
}
