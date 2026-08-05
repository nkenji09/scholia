// jsonio.go — package cli が `encoding/json` に触れる**唯一の場所**
// （01KZ7V637RNMPXJMVACYV6V1AS 条項2「`--json` を書き出す経路を1つの関数に寄せる」）。
//
// # なぜ file を1つに閉じるのか
//
// 条項2 の前は `enc.SetIndent("", "  ")` が package cli の非テスト file に **44 箇所**
// 複製されていた。**新しい面はどれかを複製して増える**——複製元がある限り、
// 既定を変える判断は次の面に届かない。
//
// ⚠️ 正本は「45 箇所」と書いている。**45 番目は `internal/viewer/httpjson.go` で、
// 同じ正本が「viewer の HTTP 応答は変わらない（別経路）」として射程から外している。**
// 同様に、正本が数えた `json.MarshalIndent` 3 箇所のうち `--json` の経路にあるものは
// **0 箇所**である——1 つは下の renderIndentedJSON（テキストの面の断片）、
// 残る 2 つは `.scholia/` のレコードファイルを書く経路（`internal/store`・
// `internal/review`）で、いずれも整形したまま 1 バイトも変えていない。
// 「これからは全部の `--json` を絞る」と文で書く案が採れないことは
// `01KXS68HCNQ0H9QKNYFQ869J19` が実測で否定している（明文化後に作られた 62 件が
// 62 件とも旧様式のままだった）。だから置き場所は出力口そのものになる。
//
// # 塞ぎ方と、塞げていない範囲（CLAUDE.md「配線ガードの書き方」6）
//
// Go では「この package では `encoding/json` を import できない」を**コンパイラには
// 言わせられない**——import は file ごとの宣言で、字句スコープから取り除けない。
// 単位AY が `snap` を描画側のスコープから消して変異をコンパイルエラーにした形は、
// ここでは**使えない**。代わりに 2 つの歯止めを組で置く（jsonio_test.go）:
//
//   - **境界**: package cli の非テスト file のうち `encoding/json` を import して
//     よいのはこの file だけ。`go/parser` で AST の import を見るので、別名・
//     dot import・blank import でも同じに落ちる（TestCLIPackageTouchesEncodingJSONOnlyHere）。
//   - **バイトの突き合わせ**: cobra の木から数え上げた `--json` の面を実際に走らせ、
//     **標準出力に出たバイト列が、この file が書いたバイト列と 1 バイトも違わない**
//     ことを見る（TestEveryJSONFaceGoesThroughTheSingleExit）。`fmt.Fprintf` で
//     JSON を手書きする経路は import を増やさないので境界を素通りするが、
//     こちらで落ちる。
//
// **落ちない（射程の外・正直に名乗る）:** package cli の**外**に新しい出力口を
// 作り、そこから `--json` を書く面。境界はこの package の file しか見ず、
// バイトの突き合わせも「cobra の木にぶら下がった面」しか回さない。
package cli

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/spf13/cobra"
)

// emitJSON は `--json` の出力が通る唯一の口。
//
// **整形しない**（条項1）。欄も、順序も、値も変えない——空白だけが無い。
func emitJSON(cmd *cobra.Command, v any) error {
	return emitJSONTo(cmd.OutOrStdout(), v)
}

// emitJSONTo は書き先を直に渡す形。`tag list` のように cobra.Command ではなく
// io.Writer を持ち回る経路のためにある（同じ 1 つの口を通る）。
func emitJSONTo(w io.Writer, v any) error {
	if spy := jsonEmitSpy; spy != nil {
		var seen bytes.Buffer
		defer func() { spy(seen.Bytes()) }()
		w = io.MultiWriter(w, &seen)
	}
	return json.NewEncoder(w).Encode(v)
}

// jsonEmitSpy は「この口が何バイト書いたか」をテストへ渡す唯一の穴（既定 nil・
// 本番では 1 度も呼ばれない）。
//
// ⚠️ **これが無いと、バイトの突き合わせが書けない。** 出力が compact であること
// だけを見る検査は、**共有の口を通さずに compact な JSON を手書きした面**を
// 素通りさせる——同じ意味を別の綴りで書かれれば捕まらない（CLAUDE.md 2）。
// 「この口を通ったか」は出力の見た目からは決まらないので、通った事実そのものを
// 観測できる点をここに 1 つだけ開けてある。
var jsonEmitSpy func([]byte)

// renderJSONLine は「`--json` が渡すバイト列」を決める純関数（CLAUDE.md 1）。
// emitJSONTo が書くのはこれと同じバイト列である。
func renderJSONLine(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// renderIndentedJSON は**テキストの面**が本文に埋め込む JSON 断片を作る
// （`config infer-id-policy` の「宣言案」）。
//
// ⚠️ **`--json` ではない。** 条項1 の射程は `--json` の出力で、テキスト出力は
// 1 バイトも変わらない。ここが整形を残す唯一の場所である。
func renderIndentedJSON(v any) ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// decodeJSON は外から来た JSON を読む（`update` が GitHub の応答を読む経路）。
// 出力ではないが、`encoding/json` に触れる以上この file に置く。
func decodeJSON(r io.Reader, v any) error {
	return json.NewDecoder(r).Decode(v)
}
