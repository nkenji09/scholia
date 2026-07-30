// Package usage は「道具が渡した量」を起動ごとに 1 行記録する計測を持つ。
//
// 正本は decision 01KYSKM4T0RWRY1N7407KZSZ17（tag: req.usage-measurement）。
// 貫く原理は 1 つだけである——**ログは .scholia の中身を写さない。**
// 「どの記録を、どう引いて、どれだけ渡したか」は持ち、「その記録に何が書いてあるか」は持たない。
// 記録の中身は .scholia にあり、id から引ける。
//
// 収集レベルは環境変数 1 つで 4 段（オフ／マスク／通常／詳細）。既定（未設定）はオフで、
// オフのときは観測も記録もしない——出力・exit code・所要・生成物のいずれも変わらない。
package usage

import "strings"

// EnvVar は収集レベルを切り替える唯一の環境変数。
//
// ⚠️ 口をここ以外に増やさない（正本「段の切り替え」——段は互いに排他で組み合わせが無いので、
// 複数変数にすると存在しない状態を表現できてしまう）。
const EnvVar = "SCHOLIA_USAGE_LEVEL"

// Level は収集レベル（段）。値が大きいほど記録する項目が多い。
//
// 段は包含関係にある（マスク ⊂ 通常 ⊂ 詳細）。Records はこの順序に依存するので、
// 定数の順序を入れ替えてはいけない。
type Level int

const (
	// Off は既定。観測しない・書かない。
	Off Level = iota
	// Masked は道具の側の語彙と、数と時刻と、実行環境の名乗りだけを残す段。
	// プロジェクトが名付けたものは、値としても、そこから導いた形（長さ・先頭・ダイジェスト）としても残さない。
	Masked
	// Normal はマスクに加えて、プロジェクトが名付けたものを指す値（レコード id・プロジェクトルート）を残す段。
	Normal
	// Detailed は通常に加えて、その 1 回の呼び出しの形と量の内訳を残す段。
	// ⚠️ 「自由文の値も書く段」ではない——自由文は長さだけである（貫く原理）。
	Detailed
)

// levelNames は段の名前。環境変数の値としても、行の level 欄の値としても、この綴りを使う。
var levelNames = [...]string{
	Off:      "off",
	Masked:   "masked",
	Normal:   "normal",
	Detailed: "detailed",
}

// String は段の名前を返す。範囲外は "off"（安全側）。
func (l Level) String() string {
	if l < 0 || int(l) >= len(levelNames) {
		return levelNames[Off]
	}
	return levelNames[l]
}

// AllLevels は 4 段すべてを段の順に返す。表の検査はこれを回す。
func AllLevels() []Level {
	return []Level{Off, Masked, Normal, Detailed}
}

// LevelNames は 4 段の名前を段の順に返す（注記の文面で使う）。
func LevelNames() []string {
	return append([]string(nil), levelNames[:]...)
}

// ParseLevel は環境変数の値を段に解釈する。
//
// ok=false は「解釈できなかった」を意味し、段は Off に倒れる（安全側）。
// 前後の空白は落とし、大小は畳む——"OFF" と書いた人を黙ってオフにしないため
// （解釈できたものは解釈する。倒すのは本当に読めない値だけ）。
func ParseLevel(raw string) (Level, bool) {
	s := strings.ToLower(strings.TrimSpace(raw))
	for i, name := range levelNames {
		if s == name {
			return Level(i), true
		}
	}
	return Off, false
}
