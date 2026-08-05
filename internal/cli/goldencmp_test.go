// goldencmp_test.go — golden を採り直したとき「変わったのは空白だけか」を機械で見る。
//
// # なぜ repo の中に置くのか
//
// `01KZ7V637RNMPXJMVACYV6V1AS`（`--json` の整形をやめる）で golden を採り直したとき、
// **「採り直したものが空白だけの差であることを確かめる」は手順書の 1 行だった。**
// 手順書の 1 行は飛ばせる。飛ばせば、**欄が消えても気づけないまま golden が正典になる。**
//
// さらに、その手順が最初に指していた比べ方は `jq -S .` だった。⚠️ **あれは使えない**
// ——`-S` は**キー順を正規化する**ので、順序が変わっても一致してしまう。
// 「欄も**順序も**内容も同じで空白だけが消える」を確かめる道具としては弱い。
//
// そこで比べ方そのものを repo の中へ入れ、**採り直しの手順に組み込んだ**
// （`SCHOLIA_GOLDEN_UPDATE=1` で走らせると自動で当たる。tag_list_bytes_test.go）。
// 外部スクリプトも `jq` も要らない。
//
// # この比べ方が落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:** 値が変わった／欄が増えた・減った／**キーの順序が変わった**／
// 同じキーの重複が増減した／**数値リテラルの綴りが変わった**（`1` と `1.0` を
// 同じと見ない）／文字列の中身が変わった（**文字列の中の空白も 1 つの違い**）。
//
// **落ちない（射程の外）:** JSON として読めないもの同士の比較——テキストの
// golden は JSON ではないので、そちらは**生バイトで**比べる（呼び出し側の役目）。
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"
)

// jsonSameIgnoringWhitespace は 2 つの JSON 文書が「空白の置き方を除いて同一か」を返す。
//
// 比べるのは `json.Decoder` の**トークン列**である。トークン列は文書順に出るので
// **キーの順序が保たれ**、`UseNumber` を立てているので**数値リテラルの綴りも保たれる**。
// 空白は文法上の区切りなのでトークンには現れない——だから「空白だけを無視する」が
// 素朴な文字列処理なしに成り立つ。
func jsonSameIgnoringWhitespace(a, b []byte) (bool, string) {
	da, db := json.NewDecoder(bytes.NewReader(a)), json.NewDecoder(bytes.NewReader(b))
	da.UseNumber()
	db.UseNumber()
	for i := 0; ; i++ {
		ta, ea := da.Token()
		tb, eb := db.Token()
		if ea == io.EOF && eb == io.EOF {
			return true, ""
		}
		if ea != nil || eb != nil {
			if ea == io.EOF {
				return false, fmt.Sprintf("旧のほうが %d トークンで終わっている（新はまだ %v が続く）", i, tb)
			}
			if eb == io.EOF {
				return false, fmt.Sprintf("新のほうが %d トークンで終わっている（旧はまだ %v が続く）", i, ta)
			}
			return false, fmt.Sprintf("%d トークン目で読めなくなった: 旧 %v / 新 %v", i, ea, eb)
		}
		if fmt.Sprintf("%T:%v", ta, ta) != fmt.Sprintf("%T:%v", tb, tb) {
			return false, fmt.Sprintf("%d トークン目が違う: 旧 %#v / 新 %#v", i, ta, tb)
		}
	}
}

// TestJSONSameIgnoringWhitespace は比べ方そのものを入力と出力の対で検査する
// （CLAUDE.md 1）。⚠️ **敵対的な入力を含める**——「空白を無視する」実装は、
// 文字列の中の空白まで消したり、キー順や数値の綴りを取りこぼしたりしやすい。
func TestJSONSameIgnoringWhitespace(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		same bool
	}{
		{"整形の有無だけ", "{\n  \"k\": 1\n}", `{"k":1}`, true},
		{"末尾の改行だけ", "{\"k\":1}\n", `{"k":1}`, true},
		{"入れ子の整形だけ", "{\n  \"a\": {\n    \"b\": [1, 2]\n  }\n}", `{"a":{"b":[1,2]}}`, true},

		{"文字列の中の空白は違い", `{"k":"a b"}`, `{"k":"ab"}`, false},
		{"キーの中の空白は違い", `{"a b":1}`, `{"ab":1}`, false},
		{"キーの順序が違えば違い", `{"a":1,"b":2}`, `{"b":2,"a":1}`, false},
		{"値が違えば違い", `{"k":"a"}`, `{"k":"b"}`, false},
		{"欄が増えれば違い", `{"a":1}`, `{"a":1,"b":2}`, false},
		{"欄が減れば違い", `{"a":1,"b":2}`, `{"a":1}`, false},
		{"数値リテラルの綴りが違えば違い", `{"k":1}`, `{"k":1.0}`, false},
		{"重複キーの数が違えば違い", `{"k":1,"k":2}`, `{"k":1}`, false},
		{"null と 空文字は違い", `{"k":null}`, `{"k":""}`, false},
		{"true と " + `"true"` + " は違い", `{"k":true}`, `{"k":"true"}`, false},
		{"配列の順序が違えば違い", `[1,2]`, `[2,1]`, false},

		// escape の綴りは値の一部ではない（同じ文字列を指す）。
		// 左は json.Encoder が既定で書く形、右は生の `<`。どちらも同じ文字列。
		{"HTML escape の綴りが違っても同じ文字列なら同じ", `{"k":"\u003ca\u003e"}`, `{"k":"<a>"}`, true},
		// 一方、escape を解いた中身が違えば落ちる。
		{"escape を解いた中身が違えば違い", `{"k":"<a>"}`, `{"k":"<b>"}`, false},
		{"escape された引用符を跨ぐ", "{\n  \"k\": \"a\\\"b c\"\n}", `{"k":"a\"b c"}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, why := jsonSameIgnoringWhitespace([]byte(tc.a), []byte(tc.b))
			if got != tc.same {
				t.Errorf("jsonSameIgnoringWhitespace(%q, %q) = %v（%s）, want %v", tc.a, tc.b, got, why, tc.same)
			}
		})
	}
}
