// usage_delivery_test.go — 計測の「本文が渡った記録」（deliveredIds）の歯止め
// （01M09FHFG4PVTGN4CA10N7BQZK）。
//
// # ここの歯止めが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// ⚠️ **この節は 2 度書き直した。** どちらも「**最も安く緑にできる書き方が、そのまま抜け道**」という
// 同じ形で、名乗りが実態より広かった。
//
//  1. 1 回目: 宣言を `textDeliversNothing`（当時は iota のゼロ値・理由も不要）にするだけで、
//     **本文を 614,333 バイト渡していても全部緑**で通った。
//  2. 2 回目: 面の宣言に **`unrunnable: "…"` を 1 つ足すだけ**で、走る面が検査から丸ごと外れた。
//     **ゼロ値より手数が少ない**（正しい enum を選ぶ必要すら無く、任意の文字列を書けばよい）。
//
// 🔴 **2 回とも「1 つのゲートの内側にいないこと」しか見ていなかった**（CLAUDE.md 3）。
// いまは**抜け道になりうる書き方の側を消して**ある——
// ゼロ値を無効値にし、理由をどの宣言にも必須にし、**「走らせられない」という欄そのものを面の宣言から外し**、
// **成り立たない引き方を素通りさせず**、出力のバイト列を**両向き**（出していない本文を数えない／
// 渡したと記録した本文は無加工で出ている）に確かめる。
// ⚠️ **それでも残る抜け道は、下の「落ちない」に全部書いてある。**
//
// **落ちる:**
//   - **読み取りの面を新しく足して、この項目を配線しなかったとき。**
//     面は cobra の木から数え上げるので、宣言の無い面があれば落ちる
//     （TestUsage_EveryRunnableSurfaceDeclaresItsDelivery）。**列挙を足したのではない。**
//   - **宣言を書かずに済ませようとしたとき。** `textDelivery` のゼロ値は**無効値**で、
//     理由（why）も**どの宣言にも**必須である。
//   - **「走らせられない」と名乗って検査から外れようとしたとき。** その欄は面の宣言に無く、
//     閉じた集合（unrunnableSurfaces）にしか書けない。**`--json` を持つ面はそこにも書けない**
//     （TestUsage_UnrunnableSurfacesAreClosed）。
//   - **引き方を壊して素通りしようとしたとき。** 人が読む面の起動が失敗したら落ちる。
//     ⚠️ **これは実際に 1 件見つかった**——`skills show` の引き方が成り立っておらず、
//     この面は**一度も検査されていなかった**（直した）。
//   - **本文を飾って出す面が「渡した」と宣言したとき。** 渡したと記録した記録の本文が
//     **無加工で丸ごと**出ていることも見る（assertDeliveredBodiesAppearInText）。
//   - **配線したが中身が食い違うとき。** `--json` の面を**宣言された bool フラグの
//     全部分集合**で実際に走らせ、**出たバイト列（機械可読出力）の構造から導いた
//     「本文つきの記録」の集合**と、記録された deliveredIds が**一致する**ことを値で見る
//     （TestUsage_DeliveredIDsMatchTheMachineReadableOutput）。
//     導出は出力の欄の形だけを見て、実装の型・関数名を 1 つも参照しない——
//     **同じ意味を別の綴りで書き直しても答えは変わらない**（CLAUDE.md 2）。
//   - **畳んだ出力を「渡った」と数える変異。** `tag list --json`（既定は description を
//     空にして渡す）と `tag list --all --json` は上の照合で別々の答えになる。
//     標本は全タグ・全語彙に本文を持たせてあるので、畳み忘れは必ず差になる。
//   - 🔴 **出力に出ていない本文を「渡った」と数える変異。** 標本には
//     **本文が 1 文字も違わない decision が 2 件**（片方は取り下げ）あり、
//     `spec --json` は在効の側の本文しか出さない。**出ていない側まで数えると照合が落ちる。**
//     （差し戻し 1 回目 FAIL-2 の型。直す前のコードにこの標本を当てて赤を実見した。）
//   - 🔴 **人が読む面が本文を出しているのに記録しないとき。**
//     宣言と記録の突き合わせだけでなく、**人が読む出力のバイト列を実際に探す**
//     ——記録した集合の外にある本文が丸ごと出ていたら落ちる（assertNoUnexpectedBodyInText）。
//     `textDeliversNothing` と宣言した面にも同じ検査が当たる。
//   - **人が読む面で数えすぎるとき／落とす種類の宣言と実物がずれるとき。**
//     期待値は「`--json` の集合から宣言した種類を落としたもの」で、**値そのもの**を照合する。
//   - **段の表から外れる変異。** 4 段 × 全項目の既存の検査に自動的に載る
//     （internal/usage/fields_test.go）。
//   - **入れ物が起動をまたいで混ざる変異。** 入れ物は 1 起動 1 つで、
//     並行に積んでも壊れないこと（`-race`）を見る（TestDeliveryLog_*）。
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 🔴 **機械可読出力を持たない面。** `scholia export` は静的な画面を書き出す面で、
//     `--json` を持たないので上の照合が届かない。**この面は「この項目を持たない」と
//     宣言してある**（画面経由の閲覧を数えないという正本 条項 5 の帰結・deliverySpecs）。
//     宣言どおり 1 件も積まないことと、**標準出力に本文が出ていない**ことは走らせて見ているが、
//     **書き出した HTML の中身は見ていない**——レビュアの実測では `export --html` は 7.5 MB を書き、
//     標本 60 件のうち 43 件の decision 本文が丸ごと入っていた。**穴の大きさはこれである。**
//   - 🔴 **短い本文が、要約の面から丸ごと出ること。** 出力のバイト列を探す検査は
//     **101 字以上の本文しか探さない**（要約の面は 100 字で切り詰めるので、
//     それ以下は「偶然そのまま出た」と区別できない）。
//     ⚠️ したがって、**100 字以下の本文しか持たない記録については、この検査は何も言わない。**
//   - 🔴 **本文を 1 件も渡さないと宣言した面が、本文を「飾って」出すこと。**
//     探すのは `strings.Contains` なので、**1 行ずつ前置きを付ける等の加工をされると一致しない。**
//     ⚠️ **渡すと宣言した面については上の逆向きの検査が落とす**が、
//     「渡さない」と宣言した面が加工して出す場合は、いまも探せない。
//     ⚠️ **この repo の既存の面は本文を無加工で出している**（レビュアの実測: `rules` 9 件・
//     `spec` 10 件・`show decision` 1 件が丸ごと現れる）ので、**検査には現に歯がある**
//     ——これは慣行から外れた面の話である。
//   - 🔴 **「走らせられない」という宣言そのもの。** 閉じた集合に 2 件（`view` / `update`）あり、
//     留め金で固定してあるが、**本当に走らせられないかを確かめる仕組みは無い**
//     （走らせてみるまで分からない。常駐するものと網の外へ出るものを単体テストで起こさない、
//     という判断で置いている）。**2 か所を直せば、走る面でもここへ入れられる**
//     ——ただし `--json` を持つ面は入れられない。
//   - 🔴 **本文が 1 文字も違わない記録が複数あるとき、どれが出たかは言えない。**
//     バイト列から言えるのは「この本文が出た」までなので、
//     **持ち主のうち 1 件でも記録されていれば通す**（過小に落とさないため）。
//   - 🔴 **「本文が渡った」の判定そのものが間違っているとき。** 照合は
//     「機械可読出力に本文の欄が載っているか」を正としている。出力の構造の側で
//     本文つきと存在だけを取り違えていれば、照合も一緒に間違える。
//   - 🔴 **画面（`scholia view`）経由の閲覧。** 数えないと決めた面なので歯止めも無い。
//     配線としては、入れ物が cobra の context にしか無いので HTTP ハンドラから届かない
//     （レビュアが実バイナリで確認済み——画面から本文 239KB を返させても行は空）。
//   - **位置引数・文字列フラグの値で分かれる枝。** 走らせる引き方は面ごとに 1 つ＋
//     bool フラグの全部分集合で、`jsonio_test.go` が名乗っているのと同じ穴がここにもある。
//   - **Hidden なコマンド。** 面の数え上げ（usageRunnableSurfaces）が飛ばすので、
//     宣言も照合も無しに通る。いま Hidden な面は 0 件。
//   - **`scholia update` / `scholia view`。** 前者は網の外へ出て、後者は常駐する。
//     走らせないので、宣言だけがある（unrunnableSurfaces）。
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/usage"
)

// ---------------------------------------------------------------------------
// 面ごとの宣言
// ---------------------------------------------------------------------------

// textDelivery は「**人が読む面**が本文を渡す記録の集合」を、`--json` の面との
// 関係で宣言したもの。
//
// ⚠️ 機械可読出力の側は宣言しない——あちらは出たバイト列から導いて照合する。
type textDelivery int

const (
	// textDeliveryUnset は**無効値**である。
	//
	// 🔴 **ここに「1 件も渡さない」を置いてはいけない。** 差し戻し 1 回目でレビュアが実測した:
	// 新しい読み取り面を足し、宣言を `textDeliversNothing` にするだけで、
	// **本文を 614,333 バイト渡していても歯止めは全部緑で通った。**
	// `textDeliversNothing` が iota のゼロ値で、理由の記述も要らなかったからである
	// ——**宣言を書けと迫られた人が、最も少ない手数で緑にできる書き方が、そのまま穴だった。**
	// ゼロ値を無効値にして、書かなければ落ちるようにしてある。
	textDeliveryUnset textDelivery = iota
	// textDeliversNothing: 人が読む面は本文を 1 件も渡さない（索引・断片・書き込みの面）。
	textDeliversNothing
	// textDeliversSameAsJSON: `--json` と同じ集合を渡す。
	textDeliversSameAsJSON
	// textWithholds: `--json` が渡す集合から、**宣言した種類の記録だけ**を落として渡す。
	// 落とす種類は withholds に書く（例: `spec` の人が読む面は遷移と語彙を渡さない）。
	textWithholds
	// textNotCounted: この面はこの項目を持たないと宣言した面。
	// 🔴 **黙って穴にしないための宣言である**（正本の歯止めの節）。
	textNotCounted
)

// deliverySpec は 1 つの面の宣言。
type deliverySpec struct {
	text textDelivery
	// withholds は textWithholds のときに落とす記録の種類（tag/transition/vocab/decision）。
	withholds []string
	// why は**どの宣言でも必須**（なぜその関係になるのか）。
	//
	// 🔴 **「1 件も渡さない」にも理由を書かせる。** 理由の要らない宣言があると、
	// そこが「考えずに緑にできる出口」になる（差し戻し 1 回目の穴）。
	// 同じ理由が並ぶ面は下の定数を使う——**書き写しではなく参照にする**ことで、
	// 理由を変えたときに全部に届く。
	why string
	// args は `--json` を持たない面の引き方（持つ面は jsonFaceInvocations から取る）。
	args []string
}

// unrunnableSurfaces は「テストから走らせられない面」の**閉じた集合**。
//
// 🔴 **面ごとの宣言から欄を外して、ここへ移してある。** 差し戻し 1 回目の直しのあと、
// レビュアが**面の宣言に `unrunnable: "…"` を 1 つ足すだけで、走る面が検査から丸ごと外れる**
// ことを実測した（E2/E5）。**任意の文字列を書けば通る欄は、最も安い抜け道になる。**
// 欄そのものを無くしたので、**面の側から「走らせられない」と名乗ることはできない。**
//
// ⚠️ **加えて、ここに書けるのは `--json` を持たない面だけである**
// （TestUsage_UnrunnableSurfacesAreClosed）。`--json` を持つ面は
// `--json` の照合で必ず走るので、「走らせられない」は成り立たない。
// **E5（`--json` を持つ面が名乗る）はこの条件で塞がる。**
//
// ⚠️ **この集合そのものは、誰も検査していない宣言である**（何が本当に走らせられないかは、
// 走らせてみるまで分からない）。だから**2 件しかない**ことと、その理由を目に見える形で置く。
var unrunnableSurfaces = map[string]string{
	"scholia view":   "常駐して待ち受けるので、テストから走らせられない",
	"scholia update": "網の外（GitHub）へ出るので、テストから走らせない",
}

// unrunnableSurfacesPinned は上の集合の**留め金**。黙って増えないように、
// 中身そのものをここに固定する。
//
// 🔴 **増やすには 2 か所を直すことになる。** 検査に頼らず「1 欄で外れる」形を無くすのが目的で、
// **これでも「2 か所を直せば外れる」ことは変わらない**（下の名乗り）。
// 増やす前に確かめること: (1) その面は `--json` を持たないか（持つなら名乗れない）、
// (2) **本当に走らせられないのか**（走らせられるなら、引き方を書いて走らせる）、
// (3) 外した面は**この歯止めの外に出る**——それを承知しているか。
var unrunnableSurfacesPinned = []string{"scholia update", "scholia view"}

// 面のまとまりごとに共有する理由（書き写さずに参照する）。
const (
	whyWriteFace  = "書き込みの面。人が読む形で出るのは要約と allow / advisory の行だけで、レコードの本文は出ない（`--json` は保存したレコードを返すので、そちらは本文を渡す）"
	whyToolFace   = "道具・設定の面。`.scholia` の記録を 1 件も出力に組み立てない"
	whyIndexFace  = "索引・要約の面。人が読む形では id と名前（と切り詰めた要約）しか出さない"
	whyNotARecord = "出しているのは `.scholia` の記録（tag/transition/vocab/decision）ではない"
	whyFullInBoth = "人が読む面も `--json` も、同じレコードの本文を全文で出す"
)

// deliverySpecs は**実行できる全ての面**の宣言。
//
// 🔴 **これは「面の一覧」ではない。** 面は cobra の木から数え上げる
// （usageRunnableSurfaces）。ここに無い面があれば
// TestUsage_EveryRunnableSurfaceDeclaresItsDelivery が落ちる——
// **新しい面を足した人は、ここに宣言を書くまで緑にできない。**
//
// 🎯 **実際に捕まえた**: 別の単位が同時期に `scholia decision applied` を新設し、この項目の
// 配線をしないまま着地した。**どちらのブランチも単独では緑で、main へ合流した瞬間に落ちた。**
var deliverySpecs = map[string]deliverySpec{
	// --- 読み取りの面 ---
	"scholia rules": {text: textDeliversSameAsJSON,
		why: "人が読む面も `--json` も、本文を渡すのは foldRules が本文側へ分けた群だけ"},
	"scholia spec": {text: textWithholds, withholds: []string{recordKindTransition, recordKindVocab},
		why: "人が読む面はタグの description と decision の本文まで。遷移は label へ解決した 1 行、語彙は書かない"},
	"scholia show tag": {text: textDeliversSameAsJSON,
		why: whyFullInBoth + "（description）"},
	"scholia show tx": {text: textDeliversSameAsJSON,
		why: "遷移は自由文の欄を持たず、どちらもレコードの全部を出す"},
	"scholia show vocab": {text: textWithholds, withholds: []string{recordKindDecision},
		why: "人が読む面は語彙の description まで。decision は切り詰めるので数えない"},
	"scholia show decision": {text: textDeliversSameAsJSON,
		why: whyFullInBoth + "（why）"},
	"scholia decision show": {text: textDeliversSameAsJSON,
		why: whyFullInBoth + "（why）"},
	"scholia decision list": {text: textDeliversNothing,
		why: whyIndexFace + "。why は 100 字で切り詰める＝断片（`--json` は全件の本文を渡す）"},
	"scholia decision relink-commits": {text: textDeliversNothing,
		why: "出すのは decision の id・commit hash・git の見出しだけで、`--json` も同じ——decision の本文はどちらにも渡さない"},
	"scholia tag list": {text: textDeliversNothing,
		why: whyIndexFace + "。`--json --all` だけが本文を渡す"},
	"scholia list": {text: textDeliversNothing,
		why: whyIndexFace + "。`--json` は遷移レコードを丸ごと渡す"},
	"scholia search": {text: textDeliversNothing,
		why: "抜粋は断片。`--json` も抜粋しか渡さない"},
	"scholia flow": {text: textDeliversNothing,
		why: "解析結果は id と数だけで、レコードの本文を渡さない"},
	"scholia gaps": {text: textDeliversNothing,
		why: "同上"},
	"scholia diff": {text: textDeliversNothing,
		why: "人が読む面は id と欄名だけ（`--json` は差分のレコードを丸ごと渡す）"},
	"scholia refs scan":    {text: textDeliversNothing, why: "ソース中の id の出現箇所を出す面。記録の本文は出さない"},
	"scholia refs rewrite": {text: textDeliversNothing, why: "id の張り替えの面。記録の本文は出さない"},
	"scholia review list": {text: textDeliversNothing,
		why: "レビューは揮発層のコメントで、`.scholia` の記録（tag/transition/vocab/decision）ではない"},
	"scholia export": {text: textNotCounted,
		why:  "静的な画面の書き出し。画面経由の閲覧を数えないという正本 条項 5 の帰結で、この面はこの項目を持たない",
		args: []string{"--html", "_export"}}, // 標本の複製の中へ書く（面を実際に走らせるため）
	"scholia view": {text: textNotCounted,
		why: "画面そのもの（正本 条項 5）。入れ物は cobra の context にしかないので HTTP ハンドラからは届かない"},

	// --- 書き込み・設定・道具の面（記録の本文を人が読む形では渡さない） ---
	"scholia activity":               {text: textDeliversNothing, why: "git から導いた数と日付だけを出す面。記録の本文は出さない"},
	"scholia config get":             {text: textDeliversNothing, why: whyToolFace},
	"scholia config infer-id-policy": {text: textDeliversNothing, why: whyToolFace},
	"scholia config set":             {text: textDeliversNothing, why: whyToolFace},
	"scholia decide":                 {text: textDeliversNothing, why: "保存後の表示は allow/advisory だけ（`--json` は保存したレコードを返す）"},
	"scholia decision add-commit":    {text: textDeliversNothing, why: whyWriteFace},
	"scholia decision add-ref":       {text: textDeliversNothing, why: whyWriteFace},
	// ⚠️ **`decision applied` は `decision add-commit` と同じ形である**（実測: 人が読む面は
	// 要約 1 行だけ・`--json` は更新後のレコードを封筒で返す）。同じ理由の定数を参照して揃える。
	"scholia decision applied":     {text: textDeliversNothing, why: whyWriteFace},
	"scholia decision link":        {text: textDeliversNothing, why: whyWriteFace},
	"scholia init":                 {text: textDeliversNothing, why: whyToolFace},
	"scholia kind get":             {text: textDeliversNothing, why: whyToolFace},
	"scholia kind list":            {text: textDeliversNothing, why: whyToolFace},
	"scholia kind set":             {text: textDeliversNothing, why: whyToolFace},
	"scholia lint":                 {text: textDeliversNothing, why: "検査の所見（規則名・id・短い説明）だけを出す面。記録の本文は出さない"},
	"scholia lint baseline update": {text: textDeliversNothing, why: whyToolFace},
	"scholia retrofit":             {text: textDeliversNothing, why: "棚卸しの面。修正候補の断片は出すが、記録の本文は出さない"},
	"scholia review add":           {text: textDeliversNothing, why: whyWriteFace},
	"scholia review adopt":         {text: textDeliversNothing, why: "昇格した decision を返すのは `--json` だけ"},
	"scholia review reject":        {text: textDeliversNothing, why: whyWriteFace},
	"scholia review rm":            {text: textDeliversNothing, why: whyWriteFace},
	"scholia skills install":       {text: textDeliversNothing, why: whyToolFace},
	"scholia skills ls":            {text: textDeliversNothing, why: whyNotARecord + "（配布スキルの一覧）", args: []string{}},
	"scholia skills show":          {text: textDeliversNothing, why: whyNotARecord + "（配布スキルの本文）", args: []string{"evaluating-changes"}},
	"scholia tag create":           {text: textDeliversNothing, why: whyWriteFace},
	"scholia tag edit":             {text: textDeliversNothing, why: whyWriteFace},
	"scholia tag rename":           {text: textDeliversNothing, why: whyWriteFace},
	"scholia tag rm":               {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx add":               {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx edit":              {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx merge":             {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx rename":            {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx rm":                {text: textDeliversNothing, why: whyWriteFace},
	"scholia tx tag":               {text: textDeliversNothing, why: whyWriteFace},
	"scholia update": {text: textDeliversNothing,
		why: "自分自身の版を取り替える面で、記録を 1 件も読まない"},
	"scholia version":             {text: textDeliversNothing, why: whyToolFace},
	"scholia vocab add":           {text: textDeliversNothing, why: whyWriteFace},
	"scholia vocab edit":          {text: textDeliversNothing, why: whyWriteFace},
	"scholia vocab owner-migrate": {text: textDeliversNothing, why: whyWriteFace},
	"scholia vocab rename":        {text: textDeliversNothing, why: whyWriteFace},
	"scholia vocab rm":            {text: textDeliversNothing, why: whyWriteFace},
	"scholia vocab tag":           {text: textDeliversNothing, why: whyWriteFace},
}

// TestUsage_EveryRunnableSurfaceDeclaresItsDelivery は、宣言の無い面が無いこと。
//
// ⚠️ CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」。
// 宣言が無いまま面を足すと、その面が本文を渡していても記録が黙って欠ける
// ——ログを読む側から見れば「引かれていない」と読める。
func TestUsage_EveryRunnableSurfaceDeclaresItsDelivery(t *testing.T) {
	surfaces := usageRunnableSurfaces()
	if len(surfaces) == 0 {
		t.Fatal("面を 1 つも数え上げられていない（この検査は何も見ていない）")
	}
	var undeclared []string
	present := map[string]bool{}
	for _, s := range surfaces {
		present[s] = true
		if _, ok := deliverySpecs[s]; !ok {
			undeclared = append(undeclared, s)
		}
	}
	if len(undeclared) > 0 {
		sort.Strings(undeclared)
		t.Errorf(`「本文が渡った記録」の宣言が無い面がある: %v

usage_delivery_test.go の deliverySpecs に足すこと。
人が読む面が本文を 1 件も渡さないなら textDeliversNothing、
`+"`--json`"+` と同じ集合を渡すなら textDeliversSameAsJSON、
部分集合なら textDeliversSubset（理由を書く）。
この項目を持たない面だと決めたなら textNotCounted と理由（黙って穴にしない）。`, undeclared)
	}
	for s, spec := range deliverySpecs {
		if !present[s] {
			t.Errorf("deliverySpecs に載っている %q は実在しない（改名・削除したなら宣言も直す）", s)
		}
		if spec.text == textDeliveryUnset {
			t.Errorf("%q の宣言が未設定（textDelivery のゼロ値）。"+
				"何を渡す面なのかを宣言すること——**ゼロ値は無効値である**", s)
		}
		if spec.why == "" {
			t.Errorf("%q の宣言に理由が無い。**どの宣言にも理由が要る**"+
				"（「1 件も渡さない」を理由なしで書けると、そこが考えずに緑にできる出口になる）", s)
		}
	}
	t.Logf("宣言を突き合わせた面: %d 個", len(surfaces))
}

// TestUsage_UnrunnableSurfacesAreClosed は、「走らせられない」と名乗れる面が
// **閉じた集合**であり、そこに `--json` を持つ面が入っていないことを見る。
//
// 🔴 **差し戻し 1 回目の直しのあと、レビュアがここを抜けた**——面ごとの宣言に
// `unrunnable: "…"` を 1 つ足すだけで、走る面が検査から丸ごと外れた（E2/E5）。
// 欄そのものを無くしたうえで、残った集合にも 2 つの条件を課す:
//
//   - 実在する面であること（消えた面の宣言が残らない）
//   - **`--json` を持たないこと**——`--json` を持つ面は `--json` の照合で必ず走るので、
//     「走らせられない」は成り立たない。**E5 はこの条件で塞がる。**
func TestUsage_UnrunnableSurfacesAreClosed(t *testing.T) {
	surfaces := map[string]bool{}
	for _, s := range usageRunnableSurfaces() {
		surfaces[s] = true
	}
	jsonFaces := map[string]bool{}
	for _, f := range discoverJSONFaces(t) {
		jsonFaces["scholia "+f] = true
	}
	if len(jsonFaces) == 0 {
		t.Fatal("`--json` の面を 1 つも拾えていない（この検査は何も見ていない）")
	}
	if got := sortedKeys(unrunnableSurfaces); !equalIDs(got, unrunnableSurfacesPinned) {
		t.Errorf(`「走らせられない」と名乗る面が留め金と違う
 いま: %v
 留め金: %v
——増やすなら、その面が `+"`--json`"+` を持たないこと・本当に走らせられないこと・
外した面がこの歯止めの外に出ることを承知していることを確かめて、留め金も直すこと。`, got, unrunnableSurfacesPinned)
	}
	for face, why := range unrunnableSurfaces {
		if !surfaces[face] {
			t.Errorf("unrunnableSurfaces に載っている %q は実在しない", face)
		}
		if why == "" {
			t.Errorf("%q に理由が無い（何が走らせられないのかを書くこと）", face)
		}
		if jsonFaces[face] {
			t.Errorf(`%q は `+"`--json`"+` を持つ。**`+"`--json`"+` を持つ面は「走らせられない」と名乗れない**
——その面は `+"`--json`"+` の照合で必ず走っているので、人が読む面だけ走らせられない理由が無い。`, face)
		}
	}
	t.Logf("走らせない面: %d 個（%v）", len(unrunnableSurfaces), sortedKeys(unrunnableSurfaces))
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// 走らせて、記録された deliveredIds を取る
// ---------------------------------------------------------------------------

// runWithDelivery は**計測を有効にした本物の入口**（execute）で 1 回走らせ、
// 標準出力と、その起動で記録された deliveredIds を返す。
//
// ⚠️ executeForTest ではなく execute を通すのは、**入れ物を context に載せる配線ごと**
// 検査するためである。executeForTest は newRootCmd を直に走らせるので、
// 入れ物が載らず deliveredIds は必ず空になる。
func runWithDelivery(t *testing.T, dir string, args ...string) (stdout string, delivered []string, err error) {
	t.Helper()
	var out, errb bytes.Buffer
	var got usage.Observation
	seen := 0
	lookup := func(name string) (string, bool) {
		if name == usage.EnvVar {
			return "normal", true
		}
		return os.LookupEnv(name)
	}
	sink := func(_ usage.Level, o usage.Observation) {
		seen++
		got = o
	}
	err = execute(lookup, sink, append([]string{"--dir", dir}, args...), &out, &errb)
	if seen != 1 {
		t.Fatalf("1 起動 1 行のはずが sink が %d 回呼ばれた: %v", seen, args)
	}
	return out.String(), got.DeliveredIDs, err
}

// ---------------------------------------------------------------------------
// 機械可読出力の構造から「本文つきの記録」を導く
// ---------------------------------------------------------------------------

// deliveredFromJSON は、出たバイト列の**構造だけ**から本文つきの記録の id を導く。
//
// 🔴 **実装の型も関数名も参照しない。** 見るのは「レコードとして完全な形の object が
// 出ているか」だけである:
//
//	decision  … id・target・at を持ち、why が空でない
//	tag       … id・name を持ち、description が空でない
//	vocab     … id・category・label を持ち、description が空でない
//	transition… id・action・given・then を持つ（自由文の欄が無いので、載れば全部が渡っている）
//
// 本文の欄を落とした出力形（経由・取り下げ・畳んだタグ）はこの形にならないので、
// 導出にも入らない。**同じ意味を別の綴りで書き直しても答えは変わらない**（CLAUDE.md 2）。
//
// ⚠️ 似た形の object（config の kind 宣言は id・label・description を持つ）を拾わない
// ように、**レコードごとに必須の欄まで見る**（kind 宣言には name も category も無い）。
func deliveredFromJSON(t *testing.T, out string) []string {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(out), &decoded); err != nil {
		t.Fatalf("`--json` の出力が JSON として読めない: %v\n%s", err, out)
	}
	found := map[string]bool{}
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			if id, ok := recordIDFromJSONObject(x); ok {
				found[id] = true
			}
			for _, sub := range x {
				walk(sub)
			}
		case []any:
			for _, sub := range x {
				walk(sub)
			}
		}
	}
	walk(decoded)
	ids := make([]string, 0, len(found))
	for id := range found {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func recordIDFromJSONObject(o map[string]any) (string, bool) {
	id, ok := o["id"].(string)
	if !ok || id == "" {
		return "", false
	}
	nonEmptyString := func(key string) bool {
		s, ok := o[key].(string)
		return ok && s != ""
	}
	has := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := o[k]; !ok {
				return false
			}
		}
		return true
	}
	switch {
	case has("target", "at") && nonEmptyString("why"): // decision
		return id, true
	case has("name") && nonEmptyString("description"): // tag
		return id, true
	case has("category", "label") && nonEmptyString("description"): // vocab
		return id, true
	case has("action", "given", "then"): // transition
		return id, true
	}
	return "", false
}

// ---------------------------------------------------------------------------
// 照合（機械可読出力）
// ---------------------------------------------------------------------------

// TestUsage_DeliveredIDsMatchTheMachineReadableOutput は、**数え上げた全ての
// `--json` の面**を、**その面が宣言している bool フラグの全部分集合**で走らせ、
// 記録された deliveredIds が「出たバイト列から導いた集合」と一致することを見る。
//
// 🔴 **これが正本の言う「配線したが中身が食い違うとき」の歯止めである。**
// 面を列挙しない（cobra の木から数え上げる）ので、`--json` の面を新しく足せば
// 自動的に回る。枝も列挙しない（bool フラグの宣言から数え上げる）ので、
// `--all` のような「畳んだものを開く」枝も自動的に回る。
func TestUsage_DeliveredIDsMatchTheMachineReadableOutput(t *testing.T) {
	template, ids := seedJSONFaceFixture(t)
	t.Setenv("EDITOR", "true")
	t.Setenv("HOME", t.TempDir())

	ran, rejected := 0, 0
	for _, face := range discoverJSONFaces(t) {
		extra, ok := jsonFaceInvocations[face]
		if !ok {
			continue // TestEveryJSONFaceIsExercised が別途落とす
		}
		t.Run(face, func(t *testing.T) {
			okRuns := 0
			for _, combo := range boolFlagSubsets(t, face) {
				name := "既定"
				if len(combo) > 0 {
					name = strings.Join(combo, " ")
				}
				t.Run(name, func(t *testing.T) {
					dir := copyFixture(t, template)
					t.Chdir(dir)

					args := append(strings.Fields(face), ids.resolve(extra)...)
					args = append(args, "--json")
					args = append(args, combo...)

					stdout, delivered, err := runWithDelivery(t, dir, args...)
					if err != nil || stdout == "" {
						rejected++
						t.Logf("この引き方は出力を持たない（検査対象外）: %v (%v)", args, err)
						return
					}
					okRuns++
					ran++
					want := deliveredFromJSON(t, stdout)
					if !equalIDs(delivered, want) {
						t.Errorf(`記録された deliveredIds が、機械可読出力から導いた集合と違う: %v
 記録: %v
 出力: %v
出力に本文が載っているのに記録されていない id は配線漏れ、
記録されているのに出力に本文が無い id は数えすぎ（畳んだ側を数えている）。`,
							args, delivered, want)
					}
				})
			}
			if okRuns == 0 {
				t.Errorf("この面はどの引き方でも出力を得られなかった（1 度も照合されていない）")
			}
		})
	}
	t.Logf("照合した起動: %d 通り（成り立たなかった引き方 %d 通り）", ran, rejected)
	if ran == 0 {
		t.Fatal("1 つも走っていない（この検査は何も見ていない）")
	}
}

// ---------------------------------------------------------------------------
// 人が読む面
// ---------------------------------------------------------------------------

// TestUsage_TextFacesDeliverWhatTheyDeclare は、人が読む面が宣言どおりの集合を
// 渡すことを、実際に走らせて値で見る。
//
// ⚠️ **人が読む面は機械可読出力を持たないので、集合そのものを導けない。**
// だから宣言は「`--json` と同じ／その部分集合／1 件も渡さない」の 3 つに限り、
// **部分集合の面についてどの記録が欠けるべきかは検査していない**（file 冒頭の射程）。
func TestUsage_TextFacesDeliverWhatTheyDeclare(t *testing.T) {
	template, ids := seedJSONFaceFixture(t)
	t.Setenv("EDITOR", "true")
	t.Setenv("HOME", t.TempDir())

	checked, skipped := 0, 0
	for _, face := range usageRunnableSurfaces() {
		spec := deliverySpecs[face]
		if why, ok := unrunnableSurfaces[face]; ok {
			skipped++
			t.Logf("%s は走らせない: %s", face, why)
			continue
		}
		short := strings.TrimPrefix(face, "scholia ")
		extra, hasJSON := jsonFaceInvocations[short]
		if !hasJSON {
			extra = spec.args
		}
		t.Run(face, func(t *testing.T) {
			dir := copyFixture(t, template)
			t.Chdir(dir)
			base := append(strings.Fields(short), ids.resolve(extra)...)

			stdoutText, gotText, err := runWithDelivery(t, dir, base...)
			// 🔴 **成り立たない引き方を素通りさせない。** 失敗した起動は出力が空になるので、
			// 「本文を出していない」検査も「渡した集合」の照合も**素通りする**
			// ——引き方を壊すだけで緑にできる、もう 1 つの安い抜け道になる。
			if err != nil {
				t.Fatalf("人が読む面が走らない: %v\n引き方: %v\n"+
					"（走らせられない面は unrunnableSurfaces に置く。ただし `--json` を持つ面は置けない）", err, base)
			}
			checked++

			if spec.text == textDeliversNothing || spec.text == textNotCounted {
				if len(gotText) != 0 {
					t.Fatalf("この面は本文を 1 件も渡さないと宣言しているのに記録されている: %v", gotText)
				}
				// 🔴 **記録が空であることは、渡していないことの証明ではない。**
				// 出したのに記録しなければ、この宣言はいくらでも緑にできる（差し戻し 1 回目の穴）。
				assertNoUnexpectedBodyInText(t, stdoutText, nil, recordBodies(t, dir))
				return
			}

			if !hasJSON {
				t.Fatalf("`--json` を持たない面は textDeliversNothing か textNotCounted しか宣言できない")
			}
			dir2 := copyFixture(t, template)
			t.Chdir(dir2)
			_, gotJSON, err := runWithDelivery(t, dir2, append(append([]string{}, base...), "--json")...)
			if err != nil {
				t.Fatalf("`--json` の面が走らない: %v", err)
			}

			// 期待値は「`--json` が渡した集合から、宣言した種類を落としたもの」——
			// **値そのもの**で照合する。部分集合であることだけを見る形は、
			// 人が読む面の配線を 1 本外す変異を通した（実見）。
			want := withoutKinds(t, gotJSON, spec.withholds, recordKinds(t, dir2))
			if len(want) == 0 {
				t.Fatalf("この面の期待値が空になった（標本が宣言に噛み合っていない）。`--json` 側は %v", gotJSON)
			}
			if spec.text == textWithholds && equalIDs(want, gotJSON) {
				t.Fatalf("落とすと宣言した種類が `--json` の集合に 1 件も無い（宣言か標本が古い）: %v", gotJSON)
			}
			if !equalIDs(gotText, want) {
				t.Errorf(`人が読む面が渡した記録が宣言と違う
 記録: %v
 期待: %v（`+"`--json`"+` の %v から種類 %v を落としたもの）`, gotText, want, gotJSON, spec.withholds)
			}
			// 記録した集合の外にある記録の本文が、人が読む出力に丸ごと出ていないこと。
			bodies := recordBodies(t, dir2)
			assertNoUnexpectedBodyInText(t, stdoutText, want, bodies)
			// 🔴 **逆向きも見る**——渡したと記録した記録の本文は、**無加工で丸ごと**出ていること。
			// これが無いと、本文を飾って出す面（1 行ずつ前置きを付ける等）が
			// 「出していない」側の検査を素通りし、上の検査に歯が無くなる。
			assertDeliveredBodiesAppearInText(t, stdoutText, want, bodies)
		})
	}
	t.Logf("人が読む面を走らせた: %d 個（走らせなかった %d 個）", checked, skipped)
	if checked == 0 {
		t.Fatal("1 つも走っていない（この検査は何も見ていない）")
	}
}

// deliveredBodyProbeMinRunes は「人が読む出力に本文が丸ごと出ていないか」を探すときの、
// 本文の最短の長さ。
//
// ⚠️ **要約の面が切り詰める長さ（`decision list` の 100 字）より長いものだけを探す。**
// これより短い本文は、要約の面が**偶然そのまま全文を出しうる**ので、
// 「丸ごと出た＝本文を渡した」と読めない。**射程を名乗ってこの線を引いている。**
const deliveredBodyProbeMinRunes = 101

// recordBodies は標本の `.scholia/` を読んで、**本文 → その本文を持つ記録の id** を返す
// （`deliveredBodyProbeMinRunes` 以上のものだけ）。
//
// ⚠️ **鍵が本文で、値が id の並びなのは、本文が 1 文字も違わない記録が複数ありうるため**である。
// バイト列から言えるのは「この本文が出た」までで、**どの記録の本文かは言えない**
// ——初版の `--json` 側の確かめはここを取り違えていた（差し戻し 1 回目 FAIL-2）。
func recordBodies(t *testing.T, dir string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, sub := range []string{"tags", "transitions", "vocab", "decisions"} {
		entries, err := os.ReadDir(filepath.Join(dir, ".scholia", sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			name := strings.TrimSuffix(e.Name(), ".json")
			if name == e.Name() {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, ".scholia", sub, e.Name()))
			if err != nil {
				continue
			}
			var rec map[string]any
			if err := json.Unmarshal(raw, &rec); err != nil {
				continue
			}
			for _, key := range []string{"why", "description"} {
				body, _ := rec[key].(string)
				if len([]rune(body)) >= deliveredBodyProbeMinRunes {
					out[body] = append(out[body], name)
				}
			}
		}
	}
	return out
}

// assertNoUnexpectedBodyInText は、人が読む出力に**丸ごと**現れた本文について、
// その本文を持つ記録が 1 件でも `delivered` に入っていることを確かめる。
//
// 🔴 **これが差し戻し 1 回目（FAIL-1）の直しである。** 宣言と記録だけを突き合わせる形では、
// 「本文を 614,333 バイト出しているのに 1 件も記録しない面」を
// `textDeliversNothing` と宣言するだけで緑にできた。**出したかどうかを出力の値で見る。**
//
// ⚠️ **「その本文が出た」までしか言えない。** 本文が 1 文字も違わない記録が複数あるとき、
// どれが出たかはバイト列からは決まらないので、**1 件でも記録されていれば通す**（過小に落とさない）。
func assertNoUnexpectedBodyInText(t *testing.T, stdout string, delivered []string, bodies map[string][]string) {
	t.Helper()
	if len(bodies) == 0 {
		t.Fatal("標本に、丸ごと出たかを探せる長さの本文が 1 つも無い（この検査は何も見ていない）")
	}
	in := make(map[string]bool, len(delivered))
	for _, id := range delivered {
		in[id] = true
	}
	for body, owners := range bodies {
		if !strings.Contains(stdout, body) {
			continue
		}
		hit := false
		for _, id := range owners {
			if in[id] {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf(`人が読む出力に、記録していない本文が丸ごと出ている（%d 字・持ち主 %v）
記録した集合: %v
——出しているのに記録しないなら、この面の宣言が実物と違う。`, len([]rune(body)), owners, delivered)
		}
	}
}

// assertDeliveredBodiesAppearInText は、**渡したと記録した記録の本文が、無加工で丸ごと
// 出ていること**を確かめる（探す長さの下限は上と同じ）。
//
// 🔴 **上の検査（出していない本文を数えない）と対で意味を持つ。**
// 出力のバイト列を `strings.Contains` で探す形は、**本文を飾って出す面**
// （1 行ずつ前置きを付ける等）を素通りさせる。片側だけ置くと、
// 「飾って出す面を作れば何を出しても緑」という抜け道が残る。
// **両側を置くと、飾った面は「渡した」と宣言した時点でこちらが落ちる。**
//
// ⚠️ **落ちない範囲**: 本文を 1 件も渡さないと宣言した面が、本文を飾って出す場合。
// そこは今も探せない（file 冒頭に名乗ってある）。
func assertDeliveredBodiesAppearInText(t *testing.T, stdout string, delivered []string, bodies map[string][]string) {
	t.Helper()
	owned := map[string][]string{}
	for body, owners := range bodies {
		for _, id := range owners {
			owned[id] = append(owned[id], body)
		}
	}
	checked := 0
	for _, id := range delivered {
		for _, body := range owned[id] {
			checked++
			if !strings.Contains(stdout, body) {
				t.Errorf(`「渡した」と記録した %s の本文（%d 字）が、人が読む出力に丸ごと現れない。
本文を加工して出しているなら、出していない本文を探す検査に歯が無くなる。`, id, len([]rune(body)))
			}
		}
	}
	if checked == 0 {
		t.Logf("⚠️ この面が渡した記録には、探せる長さ（%d 字以上）の本文が無い", deliveredBodyProbeMinRunes)
	}
}

// 記録の種類（`.scholia/` の置き場所と 1 対 1）。
const (
	recordKindTag        = "tag"
	recordKindTransition = "transition"
	recordKindVocab      = "vocab"
	recordKindDecision   = "decision"
)

// recordKinds は標本の `.scholia/` を読んで id → 種類の対応を作る。
//
// ⚠️ **実装のどの関数も通さない。** 置き場所（ディレクトリ）だけを見るので、
// 分類の実装を書き換えてもこの対応は変わらない。
func recordKinds(t *testing.T, dir string) map[string]string {
	t.Helper()
	kinds := map[string]string{}
	for sub, kind := range map[string]string{
		"tags": recordKindTag, "transitions": recordKindTransition,
		"vocab": recordKindVocab, "decisions": recordKindDecision,
	} {
		entries, err := os.ReadDir(filepath.Join(dir, ".scholia", sub))
		if err != nil {
			continue // その種類がまだ 1 件も無い
		}
		for _, e := range entries {
			if name := strings.TrimSuffix(e.Name(), ".json"); name != e.Name() {
				kinds[name] = kind
			}
		}
	}
	if len(kinds) == 0 {
		t.Fatalf("標本の `.scholia` から 1 件も読めていない: %s", dir)
	}
	return kinds
}

// withoutKinds は id の並びから、宣言された種類のものを落とす。
// 種類の分からない id が来たら落とさずに残す（黙って消さない）。
func withoutKinds(t *testing.T, ids, withholds []string, kinds map[string]string) []string {
	t.Helper()
	drop := make(map[string]bool, len(withholds))
	for _, k := range withholds {
		drop[k] = true
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		kind, known := kinds[id]
		if !known {
			t.Logf("⚠️ id %q の種類が標本から分からない（落とさずに残す）", id)
		}
		if drop[kind] {
			continue
		}
		out = append(out, id)
	}
	return out
}

func equalIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------------------
// 判定そのもの（純関数・入力と出力の対・CLAUDE.md 1）
// ---------------------------------------------------------------------------

// sampleDecision / sampleTag / sampleTransition は判定の検査用の最小レコード。
func sampleDecision(id, why string) model.Decision {
	return model.Decision{ID: id, Target: model.DecisionTarget{Type: "tag", ID: "req.a"}, Why: why, At: "2026-01-01T00:00:00Z"}
}

func sampleTag(id, desc string) model.Tag {
	return model.Tag{ID: id, Name: "名前", Description: desc}
}

func sampleTransition(id string) model.Transition {
	return model.Transition{ID: id, Action: "act.submit", Given: []string{"cond.valid"}, Then: []string{"eff.token"}}
}

func TestDeliveredRecords_InputOutputPairs(t *testing.T) {
	withBody := sampleTag("req.a", "本文。")
	folded := withBody
	folded.Description = "" // tag list の既定が渡す形

	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"nil", nil, nil},
		{"本文つきのタグ", withBody, []string{"req.a"}},
		{"畳んだタグ（本文の欄を空にした形）は数えない", folded, nil},
		{"本文つきの decision", sampleDecision("D1", "本文。"), []string{"D1"}},
		{"本文の無い decision は数えない", sampleDecision("D1", ""), nil},
		{"存在だけの出力形（inheritedOut）は数えない",
			inheritedOut{ID: "D1", Heading: "見出し"}, nil},
		{"取り下げの出力形（withdrawnOut）は数えない",
			withdrawnOut{ID: "D1", ReplacedBy: []string{"D2"}}, nil},
		{"埋め込みの中のレコードも拾う",
			decisionOut{Decision: sampleDecision("D1", "本文。"), Effect: EffectInForce}, []string{"D1"}},
		{"スライス・入れ子・重複",
			[]any{
				[]decisionOut{{Decision: sampleDecision("D2", "本文。")}, {Decision: sampleDecision("D1", "本文。")}},
				map[string]any{"x": sampleDecision("D1", "本文。")},
			}, []string{"D1", "D2"}},
		{"遷移は本文の欄が無いので、載れば渡ったと数える",
			sampleTransition("T-a"), []string{"T-a"}},
		{"nil ポインタ", []*model.Decision{nil}, nil},
		{"ポインタの先も歩く", &struct {
			D model.Decision `json:"d"`
		}{sampleDecision("D1", "本文。")}, []string{"D1"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, r := range deliveredRecords(c.in) {
				got = append(got, r.id)
			}
			if !equalIDs(got, c.want) {
				t.Errorf("deliveredRecords\n got: %v\nwant: %v", got, c.want)
			}
		})
	}
}

// shadowInner / shadowOuter は `specOutput` と同じ形——**外側の欄が、埋め込みの
// 同名の欄を覆う**。覆われた側は Go の値には居るが、JSON には 1 バイトも出ない。
type shadowInner struct {
	Decisions []model.Decision `json:"decisions"`
}

type shadowOuter struct {
	shadowInner
	Decisions []withdrawnOut `json:"decisions"`
}

// bodyShadowOuter は**レコードの本文の欄そのもの**を外側が覆う形。
type bodyShadowOuter struct {
	model.Decision
	Why string `json:"why"`
}

// TestDeliveredRecords_DoesNotWalkShadowedFields は、外側の欄に覆われた埋め込みの欄を
// 歩かないことを、入力と出力の対で見る（CLAUDE.md 1）。
//
// 🔴 **差し戻し 1 回目（FAIL-2）で落ちたのはここである。** 覆われた側を歩いてしまい、
// **出力に 1 バイトも出ていない本文を「渡った」と書いた。**
// 初版はそれを「書くバイト列に本文が現れるか」で確かめていたが、
// **本文が 1 文字も違わない 2 件があると区別できなかった**（下の 3 つ目の場合）。
func TestDeliveredRecords_DoesNotWalkShadowedFields(t *testing.T) {
	same := "この 2 件は本文が 1 文字も違わない。"
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"覆われた埋め込みの欄は歩かない",
			shadowOuter{
				shadowInner: shadowInner{Decisions: []model.Decision{sampleDecision("D-hidden", "本文。")}},
				Decisions:   []withdrawnOut{{ID: "D-hidden"}},
			}, nil},
		{"覆っている側は歩く",
			struct {
				shadowInner
				Kept []model.Decision `json:"kept"`
			}{
				shadowInner: shadowInner{Decisions: []model.Decision{sampleDecision("D-hidden", "本文。")}},
				Kept:        []model.Decision{sampleDecision("D-kept", "本文。")},
			}, []string{"D-hidden", "D-kept"}},
		{"本文が同じでも、覆われた側は数えない",
			shadowOuter{
				shadowInner: shadowInner{Decisions: []model.Decision{
					sampleDecision("D-old", same), sampleDecision("D-new", same),
				}},
				Decisions: []withdrawnOut{{ID: "D-old"}},
			}, nil},
		{"本文の欄そのものが覆われていたら数えない",
			bodyShadowOuter{Decision: sampleDecision("D1", "本文。"), Why: "外側の値"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for _, r := range deliveredRecords(c.in) {
				got = append(got, r.id)
			}
			if !equalIDs(got, c.want) {
				t.Errorf("deliveredRecords\n got: %v\nwant: %v", got, c.want)
			}
		})
	}
}

// TestDeliveredRecords_SkipsFieldsThatNeverReachTheOutput は、JSON に出ない欄
// （`json:"-"`）を歩かないことを見る。出ない本文を「渡った」と書かないため。
func TestDeliveredRecords_SkipsFieldsThatNeverReachTheOutput(t *testing.T) {
	type wrapper struct {
		Shown  model.Decision `json:"shown"`
		Hidden model.Decision `json:"-"`
	}
	v := wrapper{Shown: sampleDecision("D1", "本文。"), Hidden: sampleDecision("D2", "本文。")}
	var got []string
	for _, r := range deliveredRecords(v) {
		got = append(got, r.id)
	}
	if !equalIDs(got, []string{"D1"}) {
		t.Errorf("JSON に出ない欄まで数えている: %v", got)
	}
}

// ---------------------------------------------------------------------------
// 入れ物
// ---------------------------------------------------------------------------

// TestDeliveryLog_NilIsANoop は、計測がオフのとき（入れ物が無いとき）に
// 積む側が何もしないことを見る。**面の側にオフの分岐を書かせないための性質**である。
func TestDeliveryLog_NilIsANoop(t *testing.T) {
	var d *deliveryLog
	d.note("a", "b")
	if got := d.ids(); len(got) != 0 {
		t.Errorf("nil の入れ物が値を返した: %v", got)
	}
	// 計測を通さない実行経路（オフ）では、cobra の context に入れ物が載らない。
	root := newPlainRoot([]string{"version"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err := root.Execute(); err != nil {
		t.Fatalf("version が走らない: %v", err)
	}
	if got := deliveryLogFrom(root); got != nil {
		t.Errorf("オフの実行経路に入れ物が載っている: %+v", got)
	}
}

// TestDeliveryLog_ConcurrentNotes は、同じ起動の中で並行に積んでも壊れないことを見る。
//
// 🔴 **`-race` と対で意味を持つ**（CLAUDE.md「検証」）。入れ物をパッケージ変数に置く
// 変異を入れると、この検査は緑のままでも viewer の共有と組み合わさったときに壊れる
// ——だから入れ物は 1 起動 1 つで、置き場所そのものを配線で決めてある。
func TestDeliveryLog_ConcurrentNotes(t *testing.T) {
	d := &deliveryLog{}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < 32; j++ {
				d.note(fmt.Sprintf("id-%02d", (i*32+j)%16))
			}
		}(i)
	}
	wg.Wait()
	got := d.ids()
	if len(got) != 16 {
		t.Fatalf("並行に積んだ結果が %d 件（want 16）: %v", len(got), got)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1] >= got[i] {
			t.Fatalf("昇順・重複なしになっていない: %v", got)
		}
	}
}

// TestDeliveryLog_IsPerInvocation は、入れ物が起動ごとに別であることを見る
// （前の起動で渡した記録が次の行に混ざらない）。
func TestDeliveryLog_IsPerInvocation(t *testing.T) {
	template, ids := seedJSONFaceFixture(t)
	dir := copyFixture(t, template)
	t.Chdir(dir)

	_, first, err := runWithDelivery(t, dir, "show", "decision", ids.decision)
	if err != nil {
		t.Fatalf("1 回目が走らない: %v", err)
	}
	if len(first) == 0 {
		t.Fatal("1 回目が何も記録していない（この検査は何も見ていない）")
	}
	_, second, err := runWithDelivery(t, dir, "version")
	if err != nil {
		t.Fatalf("2 回目が走らない: %v", err)
	}
	if len(second) != 0 {
		t.Errorf("前の起動で渡した記録が次の行に混ざっている: %v", second)
	}
}
