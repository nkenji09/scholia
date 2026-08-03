package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/nkenji09/scholia/internal/usage"
)

// runnableSurfaces は走査時点で「実行できるコマンド」すべて。
// 子を持ちつつ自分でも走るコマンド（scholia lint など）も 1 つの面である。
func usageRunnableSurfaces() []string {
	var leaves []string
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		name := strings.TrimSpace(path + " " + c.Name())
		if c.Runnable() {
			leaves = append(leaves, name)
		}
		for _, k := range c.Commands() {
			if k.Name() == "help" || k.Name() == "completion" || k.Hidden {
				continue
			}
			walk(k, name)
		}
	}
	walk(newRootCmd(), "")
	sort.Strings(leaves)
	return leaves
}

// --- 分類の宣言に対する歯止め ---
//
// ⚠️ **射程の名乗り**（CLAUDE.md 6）。下の「落ちる」はすべて、実際に変異を当てて赤を実見している。
//
// ⚠️ **「1 面で赤を実見した」は「どの面でも落ちる」ではない。**
// 初版はここを取り違えた——`scholia spec` を名指しした検査で赤を見て「面全体の性質を守っている」と
// 名乗ったが、`scholia show tag` へ移すと緑になり、実バイナリで自由文のパスが通常の段の recordIds に出た。
// だから面に関わる項目は、**全面を回す検査**（EveryStringFlagIsClassified /
// ExtraPositionIsNotRecordedOnAnyFace / PositionalDeclarationCoversEveryAcceptedPosition /
// ClosedSetValuesOnly）で守り、変異も**別々の面**に当てて確かめてある。
//
// 落ちる:
//   - 新しいコマンドが、**既に別のコマンドで使われている名前**の文字列フラグを宣言せずに足す
//     （`scholia export --to` / `scholia version --id`）。**この単位の本体。**
//   - 分類表のキーをフラグ名だけに戻す。
//   - 継承した永続フラグの宣言元の解決を壊す（flagDeclarationPath）。
//   - 未宣言の既定を classFreeText 以外にする。
//   - 組で外れたら名前で引き直す形で継承を復活させる。**綴りを変えても捕まる**
//     ——見ているのは書き方ではなく観測された値だからである（CLAUDE.md 2）。
//   - 分類表から 1 行消す。
//   - 新しい面を足して位置引数の宣言をしない。
//   - 宣言を超えた位置に最後の分類を延ばす。**全 34 面で落ちる**（1 面の名指しではない）。
//   - **既存の面が受け取る引数の個数を増やして、宣言を足さない。**
//     `variadic: true` を名乗った面でも、`Args` が引数の中身を検査する面でも落ちる
//     （初版はこの 2 つで落ちなかった。前者は実バイナリで漏れた）。
//   - **`variadic: true` を、上限のある面が名乗る**（宣言を足さずに緑にする逃げ道を塞ぐ）。
//   - **`variadic` が classRecordID を未宣言の位置へ延ばす**（延ばす先は安全側の分類に限る）。
//   - 受け取る位置の検査が空振りに退化する。**2 通りとも**——1 面も見ない形と、
//     面は見るが**受理個数の答えが 1 つも返らない**形（後者は初版で緑だった）。
//   - 閉じた集合の外の値を語彙として書く。**全 12 宣言で落ちる**（1 つの名指しではない）。
//
// ⚠️ 落ちない（＝ここは守っていない）:
//   - **宣言が「正しい」こと。** 自由文を classRecordID と宣言すれば、その値は通常の段に出る。
//     ここが見るのは「宣言があるか」だけである。フラグの説明文に `id` が含まれるか、といった
//     弱い一致検査なら書けるが、**同じ意味を別の綴りで書かれれば捕まらない**（CLAUDE.md 2）ので置いていない。
//   - **cobra を通らない入力。** 環境変数・標準入力・`$EDITOR` に書かせた本文は表を通らない
//     （`$EDITOR` 経由は正本が自ら射程の外に置いている）。
//   - **真偽・数値のフラグ。** 型で扱うので表に載らない（structuralFlagValue）。
//   - **`positionalSpecs` の at が空の面**は、受け取る位置の検査から外してある。
//     そこに位置が増えても記録されない（漏れない）が、**黙って欠けることは落ちない。**
//   - **本当に上限の無い面で、`variadic` が最後の宣言を延ばし続けること自体。**
//     延ばす先が classFreeText / classToolVocab であることは検査するが、
//     **その宣言自体が正しいか**（その位置が本当に自由文か）は上の 1 つ目と同じく見ていない。
//   - **受理個数の問い合わせは 16 個までで打ち切る。** 17 個目からしか受理しない `Args` は
//     「1 個も受理しない」と読まれる。⚠️ ただし**黙って素通りはしない**——宣言のある面が
//     1 個も受理しないという答えは gap として報告される（＝落ちる）。読み違えても気づける。

// TestUsage_EveryRunnableSurfaceIsClassified は、位置引数の分類に載っていない面が無いこと。
//
// ⚠️ CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」。
// 未分類の面は値を記録しない（安全側に倒れる）ので**壊れはしない**が、
// 黙って記録が欠ける状態は、ログを読む側から見れば「引かれていない」と読める。
func TestUsage_EveryRunnableSurfaceIsClassified(t *testing.T) {
	var unclassified []string
	for _, s := range usageRunnableSurfaces() {
		if _, ok := positionalSpecs[s]; !ok {
			unclassified = append(unclassified, s)
		}
	}
	if len(unclassified) > 0 {
		t.Fatalf(`位置引数の分類が無い面がある: %v

位置引数を取らない（値を記録しない）なら {} を、取るなら {at: []argSpec{…}} を
usage_args.go の positionalSpecs に足すこと。
レコード id を取る位置なら classRecordID と選択子の種類を、自由文なら classFreeText を宣言する。
⚠️ 個数が決まらない（可変長の）コマンドは variadic: true も名乗る。
名乗らないと、宣言を超えた位置は「記録しない」へ倒れる。`, unclassified)
	}

	present := map[string]bool{}
	for _, s := range usageRunnableSurfaces() {
		present[s] = true
	}
	for s := range positionalSpecs {
		if !present[s] {
			t.Errorf("positionalSpecs に載っている %q は実在しない（改名・削除したなら分類も直す）", s)
		}
	}
}

// TestPositionalSpecAt は、**宣言を超えた位置**の扱いを入力と出力の対で見る。
//
// フラグの表が「名前だけで引く」ことで既存の分類を継承していたのと同じ形の穴が、
// 位置引数側では「最後の宣言を暗黙に残り全部へ延ばす」として出る
// ——既存のコマンドの位置が 1 つ増えたときに、誰も何も宣言しないまま最後の分類が付く。
// だから延ばすのは **variadic と名乗ったコマンドだけ**である。
func TestPositionalSpecAt(t *testing.T) {
	recTag := argSpec{class: classRecordID, selector: selTag}
	free := argSpec{class: classFreeText}
	cases := []struct {
		name    string
		spec    positionalSpec
		i       int
		want    argSpec
		wantRec bool
	}{
		{"宣言のある位置", positionalSpec{at: []argSpec{recTag}}, 0, recTag, true},
		{"可変長と名乗っていないコマンドの、宣言を超えた位置は記録しない",
			positionalSpec{at: []argSpec{recTag}}, 1, argSpec{}, false},
		{"可変長なら最後の宣言が延びる", positionalSpec{at: []argSpec{recTag, free}, variadic: true}, 5, free, true},
		{"可変長でも位置ごとの宣言が優先", positionalSpec{at: []argSpec{recTag, free}, variadic: true}, 0, recTag, true},
		{"宣言が無い面は記録しない", positionalSpec{}, 0, argSpec{}, false},
		{"可変長と名乗っても延ばす元が無ければ記録しない", positionalSpec{variadic: true}, 0, argSpec{}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, rec := positionalSpecAt(c.spec, c.i)
			if rec != c.wantRec {
				t.Fatalf("記録するか = %v（want %v）", rec, c.wantRec)
			}
			if rec && (got.class != c.want.class || got.selector != c.want.selector) {
				t.Errorf("分類が %+v（want %+v）", got, c.want)
			}
		})
	}
}

// usageSyntheticSurface は、与えたコマンドパスと同じ CommandPath を持つ空のコマンド木を作って
// 末端を返す。分類表は CommandPath で引くので、これで**任意の面**の観測を組み立てられる。
func usageSyntheticSurface(t *testing.T, path string) *cobra.Command {
	t.Helper()
	names := strings.Fields(path)
	if len(names) == 0 {
		t.Fatalf("面の名前が空")
	}
	cur := &cobra.Command{Use: names[0]}
	for _, n := range names[1:] {
		child := &cobra.Command{Use: n}
		cur.AddCommand(child)
		cur = child
	}
	cur.Run = func(*cobra.Command, []string) {}
	if cur.CommandPath() != path {
		t.Fatalf("組み立てた面の名前が %q（want %q）", cur.CommandPath(), path)
	}
	return cur
}

// TestUsage_ExtraPositionIsNotRecordedOnAnyFace は、宣言を超えた位置が記録されないことを
// **分類表にあるすべての面について**、実際の観測で見る。
//
// ⚠️ **1 つの面を名指しした検査を、面全体の性質の歯止めと数えてはいけない。**
// この検査の前身は `scholia spec` を名指しで固定していた。だから
// 「`scholia show tag` に variadic を名乗らせて受理個数を増やす」変異は緑で通り、
// 実バイナリで自由文のパスが通常の段の recordIds に出た。**面が違えば捕まらなかった。**
// この repo はこの型（1 面の実例を性質の証明と読み違える）を繰り返している。
func TestUsage_ExtraPositionIsNotRecordedOnAnyFace(t *testing.T) {
	const marker = "/Users/someone/acme-confidential-roadmap-q4"
	checked := 0
	for path, spec := range positionalSpecs {
		if len(spec.at) == 0 || spec.variadic {
			continue // 記録しないと宣言した面／延ばすと宣言した面は、別の検査が見る
		}
		t.Run(path, func(t *testing.T) {
			checked++
			cmd := usageSyntheticSurface(t, path)
			// 宣言のある位置は宣言どおりの値を、その次（＝宣言していない位置）に目印を置く。
			args := make([]string, 0, len(spec.at)+1)
			for i, a := range spec.at {
				if a.class == classToolVocab && len(a.values) > 0 {
					args = append(args, a.values[0])
				} else {
					args = append(args, fmt.Sprintf("declared-%d", i))
				}
			}
			args = append(args, marker)
			if err := cmd.Flags().Parse(args); err != nil {
				t.Fatalf("parse: %v", err)
			}

			shape := observeInvocation(cmd)

			for _, id := range shape.recordIDs {
				if id == marker {
					t.Errorf("宣言していない位置が recordIds に入った: %v", shape.recordIDs)
				}
			}
			for k, v := range shape.flagValues {
				if s, ok := v.(string); ok && s == marker {
					t.Errorf("宣言していない位置の値が %q として書かれた", k)
				}
			}
			if _, ok := shape.freeTextLens["arg"+strconv.Itoa(len(spec.at))]; ok {
				t.Errorf("宣言していない位置の長さが書かれた: %v", shape.freeTextLens)
			}
		})
	}
	if checked == 0 {
		t.Fatal("この検査が 1 面も見ていない。空振りで緑になっている")
	}
}

// usageMaxProbeArgs は、cobra の Args 検証に問い合わせる引数の個数の上限。
// ここまで全部受理されたら「個数が決まらない（可変長）」と読む。
const usageMaxProbeArgs = 16

// usageProbeArgs は問い合わせに使う長さ k の引数列。
//
// ⚠️ **空文字で問い合わせてはいけない。** `Args` が引数の中身を検査する面では空文字が拒否され、
// 「1 個も受理しない」と読まれる。すると下の検査はその面を**黙って素通り**する
// ——答えが出ていないのに緑になる形である。
func usageProbeArgs(k int) []string {
	a := make([]string, k)
	for i := range a {
		a[i] = "x"
	}
	return a
}

// usageAcceptedArgCounts は、その面が受理する引数の**個数**を実際に問い合わせて返す。
//
// cobra の Args は関数なので「いくつ取るか」を宣言として読むことはできないが、
// **呼べば分かる**——長さ k の引数列を渡して検証が通るかを k=0..usageMaxProbeArgs で見る。
func usageAcceptedArgCounts(c *cobra.Command) (max int, unbounded bool) {
	for k := 0; k <= usageMaxProbeArgs; k++ {
		if c.ValidateArgs(usageProbeArgs(k)) == nil && k > max {
			max = k
		}
	}
	return max, c.ValidateArgs(usageProbeArgs(usageMaxProbeArgs)) == nil
}

// TestUsage_PositionalDeclarationCoversEveryAcceptedPosition は、
// **そのコマンドが受け取れる位置すべてに宣言が届いている**こと。
//
// ⚠️ これが無いと、既存の面の位置が 1 つ増えたときに何も落ちない。
// `scholia spec` を ExactArgs(1) → ExactArgs(2) にすれば、2 つ目の位置は
// 宣言に届いていないのに、面の宣言（positionalSpecs にキーがある）は通ったままである。
//
// ⚠️ **`variadic` を名乗った面を検査から外してはいけない。** 外すと、
// 「受理個数を増やす → 赤 → 言われたとおり variadic を名乗る → 緑」という経路ができ、
// **最後の宣言が未宣言の位置へ延びて、この単位が潰したはずの穴が復活する。**
// だから variadic の面も見て、(a) 本当に個数が決まらないか (b) 延ばす先が安全側か、の 2 つを確かめる。
//
// 値を記録しないと宣言した面（at が空）は対象外——増えた位置も記録されないので害が無い。
func TestUsage_PositionalDeclarationCoversEveryAcceptedPosition(t *testing.T) {
	var gaps []string
	checked, answered := 0, 0
	usageWalkCommands(func(c *cobra.Command) {
		if !c.Runnable() {
			return
		}
		spec := positionalSpecs[c.CommandPath()]
		if len(spec.at) == 0 {
			return
		}
		checked++
		max, unbounded := usageAcceptedArgCounts(c)

		// ⚠️ **問い合わせから答えが返ってきたことを見る。** 「面をいくつ見たか」だけを数える検査は、
		// 問い合わせが黙っても（＝受理個数が常に 0 と読まれても）緑になる
		// ——不在の主張しか積んでいないからである。
		// at に宣言がある＝位置引数を取る面なので、「1 個も受理しない」という答えはありえない。
		if !unbounded && max == 0 {
			gaps = append(gaps, fmt.Sprintf("%s: 位置引数の宣言があるのに 1 個も受理しないと読めた。"+
				"問い合わせが答えを返していないか（Args が \"x\" を拒む・上限 %d を超える）、宣言が古い",
				c.CommandPath(), usageMaxProbeArgs))
			return
		}
		answered++

		if spec.variadic {
			if !unbounded {
				gaps = append(gaps, fmt.Sprintf("%s: variadic を名乗っているのに受理個数が %d で止まる"+
					"（最後の宣言が、届かない位置ではなく**宣言していない位置**へ延びる）", c.CommandPath(), max))
			}
			// 延ばす先は、未宣言と同じ**安全側**に倒れる分類でなければならない。
			// classRecordID を延ばすと、宣言していない位置の自由文がそのまま recordIds に出る。
			if last := spec.at[len(spec.at)-1]; last.class == classRecordID {
				gaps = append(gaps, fmt.Sprintf("%s: variadic が classRecordID を未宣言の位置へ延ばしている"+
					"（自由文が recordIds に出うる）", c.CommandPath()))
			}
			return
		}
		if max > len(spec.at) {
			gaps = append(gaps, fmt.Sprintf("%s: 引数を %d 個まで受け取るのに宣言は %d 位置しかない",
				c.CommandPath(), max, len(spec.at)))
		}
	})
	// ⚠️ 除外の条件が広がると、この検査は**1 面も見ずに緑**になる。空振りを緑と読まない。
	if checked == 0 {
		t.Fatal("この検査が 1 面も見ていない（at に宣言のある面が 1 つも無い）。空振りで緑になっている")
	}
	if answered == 0 {
		t.Fatalf("%d 面を見たが、受理個数の答えが 1 つも返っていない。問い合わせが死んでいる", checked)
	}
	sort.Strings(gaps)
	if len(gaps) > 0 {
		t.Errorf(`受け取れる位置に宣言が届いていない面がある: %v

usage_args.go の positionalSpecs で、その位置の argSpec を at に足すこと。
⚠️ **variadic: true は「宣言を足さずに緑にする逃げ道」ではない。**
名乗ってよいのは**受け取る個数に上限が無い面だけ**で、名乗ると最後の宣言が
**宣言していない位置すべて**へ延びる。だから延ばす先は classFreeText か classToolVocab
（＝未宣言と同じ安全側に倒れる分類）に限る。
上限があるなら、その位置の argSpec を 1 つずつ at に書くこと。`, gaps)
	}
}

// usageWalkCommands はコマンド木を歩く（help / completion / 隠しコマンドは除く）。
func usageWalkCommands(visit func(*cobra.Command)) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		visit(c)
		for _, k := range c.Commands() {
			if k.Name() == "help" || k.Name() == "completion" || k.Hidden {
				continue
			}
			walk(k)
		}
	}
	walk(newRootCmd())
}

// usageStringFlagDeclarations は、コマンド木にある**文字列フラグの宣言**すべてを
// 分類表のキー（(コマンド, フラグ名) の組）で返す。
//
// LocalFlags はそのコマンド自身の宣言（局所・永続とも）だけで、**親から継承した永続フラグを含まない**。
// だから `--dir` の宣言は root の 1 つだけ数えられる。
func usageStringFlagDeclarations() map[string]bool {
	keys := map[string]bool{}
	usageWalkCommands(func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *flag.Flag) {
			if isStringLikeFlag(f) {
				keys[flagSpecKey(c.CommandPath(), f.Name)] = true
			}
		})
	})
	return keys
}

// TestUsage_EveryStringFlagIsClassified は、文字列を取るフラグで分類の無いものが無いこと。
//
// 真偽・数値のフラグは型から扱えるので宣言が要らない（structuralFlagValue）。
// 宣言が要るのは、値がプロジェクトを指しうる文字列フラグだけである。
//
// ⚠️ **見るのは (コマンド, フラグ名) の組であって、フラグ名ではない。**
// 名前だけを見る検査は、**新しいコマンドが既存の名前を再利用したときに素通りする**
// ——`scholia export` に自由文のパスを取る `--to` を足すだけで、誰も何も宣言しないまま
// classRecordID を継承し、通常の段の recordIds に自由文のパスが出た（正本 条項 3 違反）。
// **「分類し忘れ」は名前でも捕まるが、「既存の分類の継承」は組でしか捕まらない。**
//
// 3 方向を見る:
//  1. **宣言側** — 木にある宣言すべてが表にあること。
//  2. **実行側** — 面から見えるフラグすべてが、実行時と同じ引き当て（lookupStringFlagSpec）で
//     表に届くこと。検査だけが通って実行時は未宣言、という食い違いを塞ぐ。
//  3. **逆向き** — 表にあるキーが実在すること。
func TestUsage_EveryStringFlagIsClassified(t *testing.T) {
	declared := usageStringFlagDeclarations()

	// 1) 宣言側。
	var unclassified []string
	for key := range declared {
		if _, ok := stringFlagSpecs[key]; !ok {
			unclassified = append(unclassified, key)
		}
	}
	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf(`分類の無い文字列フラグがある: %v

usage_args.go の stringFlagSpecs に、**"<コマンドパス> --<フラグ名>" のキー**で足すこと。
⚠️ 同じ名前のフラグが別のコマンドに既にあっても、それは別の宣言である。分類は引き継がれない。
・レコード id を取る → classRecordID ＋ 選択子の種類
・道具の側の閉じた集合 → classToolVocab ＋ values（集合の外の値は自由文へ倒れる）
・それ以外（自由文・config が宣言する値・パス） → classFreeText`, unclassified)
	}

	// 2) 実行側。面から見えるフラグ（局所 ∪ 継承）を、実行時と同じ経路で引き当てる。
	var unreachable []string
	usageWalkCommands(func(c *cobra.Command) {
		if !c.Runnable() {
			return
		}
		seen := map[string]bool{}
		check := func(f *flag.Flag) {
			if !isStringLikeFlag(f) || seen[f.Name] {
				return
			}
			seen[f.Name] = true
			if _, ok := lookupStringFlagSpec(c, f.Name); !ok {
				unreachable = append(unreachable,
					c.CommandPath()+" --"+f.Name+" → "+flagSpecKey(flagDeclarationPath(c, f.Name), f.Name))
			}
		}
		c.LocalFlags().VisitAll(check)
		c.InheritedFlags().VisitAll(check)
	})
	sort.Strings(unreachable)
	if len(unreachable) > 0 {
		t.Errorf(`面から見えるのに実行時の引き当てが表に届かないフラグがある
（宣言側は通っているので、届かないのは flagDeclarationPath の解決がずれている）: %v`, unreachable)
	}

	// 3) 逆向き。
	for key := range stringFlagSpecs {
		if !declared[key] {
			t.Errorf("stringFlagSpecs に載っている %q は実在しない（改名・削除したなら分類も直す）", key)
		}
	}
}

// TestUsage_FlagNameAloneDoesNotClassify は、この単位の本体を**値**で見る。
//
// 既存の表にある名前（`--to`＝`scholia tx rename` ではレコード id）を、
// **表に無いコマンドが**使っても、レコード id として扱われないこと。
// 自由文のパスが recordIds に出た前回の実漏洩（レビュア変異 R-2）そのものである。
func TestUsage_FlagNameAloneDoesNotClassify(t *testing.T) {
	const freeTextPath = "/Users/someone/acme-confidential-roadmap-q4"
	for _, name := range []string{"to", "id", "set", "add", "rm", "on", "tag"} {
		t.Run("--"+name, func(t *testing.T) {
			if _, ok := stringFlagSpecs[name]; ok {
				t.Fatalf("表がフラグ名 %q だけで引ける形に戻っている", name)
			}
			cmd := &cobra.Command{Use: "brand-new", Run: func(*cobra.Command, []string) {}}
			var v string
			cmd.Flags().StringVar(&v, name, "", "既存の名前を再利用した新しいコマンドのフラグ")
			if err := cmd.Flags().Parse([]string{"--" + name, freeTextPath}); err != nil {
				t.Fatalf("parse: %v", err)
			}

			shape := observeInvocation(cmd)

			if len(shape.recordIDs) != 0 {
				t.Errorf("未宣言の (コマンド, フラグ) が recordIds に入った: %v", shape.recordIDs)
			}
			if got, ok := shape.flagValues[name]; ok {
				t.Errorf("未宣言の (コマンド, フラグ) の値が書かれた: %v", got)
			}
			if got := shape.freeTextLens[name]; got != utf8.RuneCountInString(freeTextPath) {
				t.Errorf("長さへ倒れていない: %v", shape.freeTextLens)
			}
			if shape.selectorKind != "" {
				t.Errorf("未宣言なのに選択子の種類を名乗った: %q", shape.selectorKind)
			}
		})
	}
}

// TestFlagDeclarationPath は「どのコマンドの宣言か」の解決を、入力と出力の対で見る。
//
// 分類表のキーはここが返す名前で組まれるので、ここがずれると
// 「検査は通るのに実行時は未宣言」あるいはその逆になる。
func TestFlagDeclarationPath(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	var s string
	root.PersistentFlags().StringVar(&s, "inherited", "", "")
	root.PersistentFlags().StringVar(&s, "shadowed", "", "")

	mid := &cobra.Command{Use: "mid"}
	root.AddCommand(mid)

	leaf := &cobra.Command{Use: "leaf", Run: func(*cobra.Command, []string) {}}
	leaf.Flags().StringVar(&s, "own", "", "")
	leaf.Flags().StringVar(&s, "shadowed", "", "") // 親の同名を隠す
	mid.AddCommand(leaf)

	sibling := &cobra.Command{Use: "sibling", Run: func(*cobra.Command, []string) {}}
	sibling.Flags().StringVar(&s, "own", "", "") // 同じ名前・別の宣言
	root.AddCommand(sibling)

	cases := []struct {
		cmd  *cobra.Command
		flag string
		want string
	}{
		{leaf, "own", "root mid leaf"},
		{sibling, "own", "root sibling"},    // 同名でも宣言が違えば別のキーになる
		{leaf, "inherited", "root"},         // 継承した永続フラグは宣言元で引く
		{leaf, "shadowed", "root mid leaf"}, // 隠したほうが勝つ
		{sibling, "shadowed", "root"},       // 隠していない兄弟は宣言元のまま
		{leaf, "nowhere", "root mid leaf"},  // どこにも無い＝実行されたコマンドの名前（＝未宣言へ倒れる）
	}
	for _, c := range cases {
		if got := flagDeclarationPath(c.cmd, c.flag); got != c.want {
			t.Errorf("flagDeclarationPath(%s, %q) = %q（want %q）", c.cmd.CommandPath(), c.flag, got, c.want)
		}
	}
}

// --- 配線した経路を、実際に走らせて値で見る ---

// usageTestEnv はログの置き場所をテスト用に閉じ込め、パスを返す。
func usageTestEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	// ⚠️ 実環境の $XDG_STATE_HOME が漏れ込むと、設定している人の手元だけ別の場所を見る。
	unsetUsageEnvVar(t, usage.StateHomeEnv)
	// 呼び出し元の名乗りは環境に左右されるので、テストでは固定する。
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "test-harness")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "session-under-test")
	t.Setenv("AI_AGENT", "")
	usageProjectRoot = ""
	path, err := usage.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	return path
}

// runMeasured は段を立てて 1 回実行し、書かれた行を返す（1 行だけのはず）。
func runMeasured(t *testing.T, level usage.Level, args ...string) (out string, line map[string]any) {
	t.Helper()
	path := usageTestEnv(t)
	var stdout, stderr bytes.Buffer
	_ = executeWithUsage(level, usage.Record, args, &stdout, &stderr)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("計測ログを読めない（%s）: %v\nstdout:\n%s\nstderr:\n%s", path, err, stdout.String(), stderr.String())
	}
	rows := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(rows) != 1 {
		t.Fatalf("1 起動なのに %d 行書かれた:\n%s", len(rows), data)
	}
	if err := json.Unmarshal([]byte(rows[0]), &line); err != nil {
		t.Fatalf("行が JSON として読めない: %v\n%s", err, rows[0])
	}
	return stdout.String(), line
}

// seedStore は検査用の .scholia を作り、プロジェクトが名付けたレコードを 1 件置く。
func seedStore(t *testing.T, tagID string) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", tagID, "--name", "検査用", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	return dir
}

// projectNamedArg はプロジェクトが名付けた文字列。マスクの行に現れてはいけない。
const projectNamedArg = "req.acme-confidential-billing"

// TestUsage_MaskedLineDoesNotContainProjectNamedArguments は、正本の歯止め 3 を
// **配線した経路を実際に走らせて**値で検査する。
//
// ⚠️ **この検査の射程**（正本の歯止め 4・CLAUDE.md 6）:
// 落ちるのは「**引数の値がそのままログに出ること**」だけである。
// 値から**導いたもの**——長さ・先頭数文字・ダイジェスト——が出ても、この検査は落ちない。
// 本 repo のタグは 78 件しかなく長さはほぼ一意なので、長さが出れば名前は指せてしまう。
// マスクの境界は性質として usage.Records / Field.NamesProject の側に書いてあり、
// **ここはその一部しか担保しない。ここを埋めたつもりになってはいけない。**
//
// 導いたものの側は TestUsage_MaskedLineIsNonInterferingWithProjectNames が差分で見ている
// （2 つの入力についてバイト同一であること）。**両方合わせても「すべての入力の証明」にはならない。**
func TestUsage_MaskedLineDoesNotContainProjectNamedArguments(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	_, line := runMeasured(t, usage.Masked, "--dir", dir, "rules", "--tag", projectNamedArg)

	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("再符号化できない: %v", err)
	}
	for _, s := range []string{projectNamedArg, dir} {
		if strings.Contains(string(raw), s) {
			t.Errorf("マスクの行にプロジェクトが名付けた文字列 %q が現れた:\n%s", s, raw)
		}
	}
	// 漏らさないだけなら空行を書けば済むので、「残るべきものが残る」も同時に見る。
	if got := line["command"]; got != "scholia rules" {
		t.Errorf("command が %v", got)
	}
	if got := line["selectorKind"]; got != "tag" {
		t.Errorf("選択子の種類（道具の側の語彙）が残っていない: %v", got)
	}
	if got := line["level"]; got != "masked" {
		t.Errorf("段の名前が行に無い: %v", got)
	}
	if got := line["sessionId"]; got != "session-under-test" {
		t.Errorf("セッション識別子はマスクでも残るはず: %v", got)
	}
	if got := line["caller"]; got != "test-harness" {
		t.Errorf("呼び出し元の名乗りはマスクでも残るはず: %v", got)
	}
	for _, key := range []string{"recordIds", "projectRoot", "flagValues", "freeTextLens"} {
		if line[key] != nil {
			t.Errorf("マスクの行で %q が null でない: %v", key, line[key])
		}
	}
}

// usageNonInterferenceHoldOut は「プロジェクトが名付けたものと無関係に、あるいは量そのものとして
// 実行ごとに変わってよい」と**明示的に宣言した**項目。
//
// ⚠️ **ここに無い項目は、2 回の実行でバイト同一でなければならない。**
// Field.NamesProject の手宣言（項目ごと・18 個）と違い、**新しい項目はここに足さない限り
// 自動的に検査対象になる**——閉じ方がこちらのほうが強い。
//
// ⚠️ **必要だと確かめていない項目をここへ足さないこと。** この検査が「捕まえない項目の列挙」へ
// 退化するとしたら、**必要な hold-out からではなく余分な hold-out から始まる。**
// 実例がある——`stdoutBytes` を「長さ違いの対でだけ」hold-out していた版は、
// 外しても緑だった（この検査が回す `rules --tag <id>` の出力は id を含まないので、
// 出力長は id の長さに依存しない）。**その余分な 1 つが、ちょうど 1 本の漏洩経路を通していた**
// ——マスクのときだけ id の長さを `stdoutBytes` に足す変異（N-2b）が全部緑で通り、条項 4 を破った。
var usageNonInterferenceHoldOut = map[string]bool{
	"ts":         true, // 時刻。実行ごとに進む
	"durationUs": true, // 所要。実行ごとに揺れる
}

// TestUsage_MaskedLineIsNonInterferingWithProjectNames は、マスクの行が
// **プロジェクトが名付けた入力の関数になっていない**ことを差分で見る。
//
// 正本の歯止め 3 の値検査（TestUsage_MaskedLineDoesNotContainProjectNamedArguments /
// TestLine_MaskedDoesNotContainProjectNamedValues）が捕まえるのは「値がそのまま出ること」だけで、
// 値から**導いたもの**——長さ・先頭数文字・ダイジェスト——は捕まえない。
// こちらはプロジェクトが名付けたものだけを変えて 2 回走らせ、行がバイト同一であることを見るので、
// 導いたものが 1 ビットでも行に漏れれば落ちる。
//
// **これは「捕まえられない綴りの列挙」ではなく 1 つの性質である**（CLAUDE.md 2）。
//
// ⚠️ **この検査の射程**（CLAUDE.md 6）:
//   - 言えるのは「**この 2 つの入力について行が同じ**」までで、すべての入力についての証明ではない。
//   - hold-out（`ts` / `durationUs` の 2 つ）に載せた項目は見ていない。
//   - **マスクの段だけを見る。** 通常・詳細はプロジェクトが名付けたものを載せると決めた段なので、
//     この性質は成り立たないし、成り立ってはいけない。
func TestUsage_MaskedLineIsNonInterferingWithProjectNames(t *testing.T) {
	const (
		shortID = "req.a"
		longID  = "req.bbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		sameA   = "req.aaaaaaaaaaaaaaaa"
		sameB   = "req.zzzzzzzzzzzzzzzz"
	)
	cases := []struct {
		name string
		a, b string
	}{
		{"同じ長さ・違う中身（値・先頭・ダイジェストの漏れを捕まえる）", sameA, sameB},
		// ⚠️ **stdoutBytes も検査対象のままにする。** ここで回す `rules --tag <id>` の出力は
		// id を含まないので、id の長さが違っても出力長は変わらない。
		{"違う長さ（長さの漏れを捕まえる）", shortID, longID},
		// レコード id を同じにして、違うのはプロジェクトのパスだけ。
		// 正本の「マスクでは複数プロジェクトを区別できない」が実装で成立していること。
		{"プロジェクトのパスだけ違う", sameA, sameA},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			la := maskedLineFor(t, c.a)
			lb := maskedLineFor(t, c.b)
			for _, m := range []map[string]any{la, lb} {
				for k := range usageNonInterferenceHoldOut {
					delete(m, k)
				}
			}
			ja, err := json.Marshal(la)
			if err != nil {
				t.Fatalf("再符号化できない: %v", err)
			}
			jb, err := json.Marshal(lb)
			if err != nil {
				t.Fatalf("再符号化できない: %v", err)
			}
			if string(ja) != string(jb) {
				t.Errorf(`マスクの行がプロジェクトが名付けたものに依存している（＝導いたものが漏れている）
  %q の行: %s
  %q の行: %s`, c.a, ja, c.b, jb)
			}
			// 空の行どうしを比べても同一になるので、「残るべきものが残る」も同時に見る。
			if la["level"] != "masked" || la["command"] != "scholia rules" {
				t.Errorf("比べた行が空になっている: %s", ja)
			}
		})
	}
}

// maskedLineFor は、プロジェクトが名付けたレコード 1 件だけを置いた新しい store に対して
// マスクで 1 回走らせ、その行を返す。**store は呼び出しごとに別の場所に作られる**ので、
// レコード id とプロジェクトのパスの両方が呼び出しごとに変わる。
func maskedLineFor(t *testing.T, tagID string) map[string]any {
	t.Helper()
	dir := seedStore(t, tagID)
	_, line := runMeasured(t, usage.Masked, "--dir", dir, "rules", "--tag", tagID)
	return line
}

// TestUsage_NormalLineNamesTheRecordAndTheProject は、通常が
// 「どのレコードが太いか」に答えられること。
func TestUsage_NormalLineNamesTheRecordAndTheProject(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	out, line := runMeasured(t, usage.Normal, "--dir", dir, "rules", "--tag", projectNamedArg)

	ids, _ := line["recordIds"].([]any)
	if len(ids) != 1 || ids[0] != projectNamedArg {
		t.Errorf("recordIds が %v", line["recordIds"])
	}
	root, _ := line["projectRoot"].(string)
	if root == "" || !strings.HasSuffix(dir, filepath.Base(root)) {
		t.Errorf("projectRoot が %q（--dir は %q）", root, dir)
	}
	if got, want := int(line["stdoutBytes"].(float64)), len(out); got != want {
		t.Errorf("stdoutBytes=%d だが実際に渡したのは %d バイト", got, want)
	}
	// 詳細だけの項目は通常では null のまま。
	for _, key := range []string{"flagValues", "freeTextLens", "stderrBytes", "durationPartsUs"} {
		if line[key] != nil {
			t.Errorf("通常の行で %q が null でない: %v", key, line[key])
		}
	}
}

// TestUsage_DetailedRecordsFreeTextLengthNotValue は、貫く原理
// 「ログは .scholia の中身を写さない」を最上段で検査する。
// 詳細は**呼び出しの形と量の解像度**を上げる段であって、自由文の値を書く段ではない。
func TestUsage_DetailedRecordsFreeTextLengthNotValue(t *testing.T) {
	dir := seedStore(t, projectNamedArg)
	body := strings.Repeat("秘", 137)
	// 見出しは保存時ゲート（01KZ06SYR3APGF3JD4NQRFTEEN）を通すための足場。
	// 自由文の本体は body のままなので、「値が写っていないか」の検査は変わらない。
	why := "# テスト用の見出し\n\n" + body

	_, line := runMeasured(t, usage.Detailed,
		"--dir", dir, "decide", "--on", "tag:"+projectNamedArg, "--why", why, "--json")

	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("再符号化できない: %v", err)
	}
	if strings.Contains(string(raw), body) {
		t.Errorf("詳細の行に自由文の値が写っている:\n%s", raw)
	}
	lens, _ := line["freeTextLens"].(map[string]any)
	got, _ := lens["why"].(float64)
	if want := len([]rune(why)); int(got) != want {
		t.Errorf("自由文の長さが %v（%d 文字のはず）: %v", lens["why"], want, lens)
	}
	values, _ := line["flagValues"].(map[string]any)
	if values["json"] != true {
		t.Errorf("真偽のフラグ値が残っていない: %v", values)
	}
	if values["on"] != "tag:"+projectNamedArg {
		t.Errorf("レコードを指す値が残っていない: %v", values)
	}
	if got := line["selectorKind"]; got != "tag" {
		t.Errorf("--on の前置から選択子の種類を取れていない: %v", got)
	}
}

// TestUsage_UnknownSubcommandStillRecordsALine は、本業が失敗した起動も 1 行残ること。
func TestUsage_UnknownSubcommandStillRecordsALine(t *testing.T) {
	_, line := runMeasured(t, usage.Detailed, "no-such-command")
	if got := line["exitCode"]; got != float64(1) {
		t.Errorf("exitCode が %v", got)
	}
	if got, _ := line["stderrBytes"].(float64); got <= 0 {
		t.Errorf("標準エラーへ渡した量が数えられていない: %v", line["stderrBytes"])
	}
}

// --- 段の解決 ---

func TestResolveUsageLevel(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		set      bool
		want     usage.Level
		wantNote bool
	}{
		{"未設定は注記も出さない", "", false, usage.Off, false},
		{"空文字は設定されているので注記する", "", true, usage.Off, true},
		{"未知の値は注記してオフへ倒す", "verbose", true, usage.Off, true},
		{"明示のオフは注記しない", "off", true, usage.Off, false},
		{"マスク", "masked", true, usage.Masked, false},
		{"通常", "normal", true, usage.Normal, false},
		{"詳細", "detailed", true, usage.Detailed, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			level, note := resolveUsageLevel(func(string) (string, bool) { return c.raw, c.set })
			if level != c.want {
				t.Errorf("段が %s（want %s）", level, c.want)
			}
			if (note != "") != c.wantNote {
				t.Errorf("注記が %q（出すべきか: %v）", note, c.wantNote)
			}
			if note != "" && strings.Count(note, "\n") != 1 {
				t.Errorf("注記は 1 行であること: %q", note)
			}
		})
	}
}

// --- 既定 off の不変（正本 条項 10: オフのときは何もしない）---

// unsetUsageEnvVar は環境変数を**未設定**にする。
//
// t.Setenv を一度通してから消すのは、テスト終了時に元の値へ戻す後始末を testing に登録するため
// （t.Unsetenv は無い）。段の検査では os.LookupEnv を本番と同じまま使いたいので、
// lookup を偽装せずに環境そのものを未設定にする。
func unsetUsageEnvVar(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "この値は Unsetenv で消える（後始末の登録のためだけに一度置く）")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%s): %v", key, err)
	}
}

// usageOffCase は「既定 off で何も変わらない」を確かめる 1 つの起動。
type usageOffCase struct {
	name string
	args func(dir string) []string
}

// usageOffCases は起動の並び。
//
// ⚠️ **読み取りだけを並べない。** 単位AI の手測り 16 通りは全部読み取り系で、
// 書き込み系が 1 つも無かった（レビュアが 22 通りへ広げて埋めた軸）。
// 書き込み・失敗する起動・引数不足・`--help`・store を開かない起動を入れてある。
var usageOffCases = []usageOffCase{
	{"読み取り（出力がある）", func(dir string) []string {
		return []string{"--dir", dir, "rules", "--tag", projectNamedArg}
	}},
	{"読み取り（一覧）", func(dir string) []string { return []string{"--dir", dir, "list"} }},
	{"書き込み", func(dir string) []string {
		return []string{"--dir", dir, "tag", "create", "req.written-while-off", "--name", "オフで書く", "--kind", "requirement"}
	}},
	{"失敗する起動", func(dir string) []string { return []string{"--dir", dir, "no-such-command"} }},
	{"引数が足りない起動", func(dir string) []string { return []string{"--dir", dir, "show"} }},
	{"--help", func(string) []string { return []string{"--help"} }},
	{"store を開かない起動", func(string) []string { return []string{"version"} }},
}

// TestUsage_DefaultOffDoesNotEnterTheMeasuredPath は、正本で**最も重い不変**
// （条項 10: 環境変数が未設定なら出力・exit code・生成物のいずれも変わらない）を自動で守る。
//
// 2 つを**対で**見る。
//
//  1. **計測経路に入らないこと**——sink が 1 度も呼ばれない。
//     ⚠️ 「ログのファイルが作られない」だけでは足りない。usage.Record 自身がオフを弾くので、
//     オフで計測経路を通ってしまう変異（レビュアの R-1: off 分岐を丸ごと消す）でもファイルは増えない。
//  2. **出力が計測層を通さない経路とバイト同一であること**——注記の混入・writer の非素通しを捕まえる。
//
// ⚠️ **この検査の射程**（CLAUDE.md 6）:
// 落ちるのは「オフなのに観測を組み立てて sink へ渡すこと」と「オフなのに出力・エラー・生成物が変わること」である。
// **sink を呼ばずに writer を包むだけの変異は落ちない**——ただしそれは出力・exit code・生成物の
// どれも変えないので、条項 10 が名指しする観測可能な差にはならない（変わるのは所要だけである）。
// 「包んでいないこと」自体は TestUsage_PlainRootHandsCobraTheWritersUnwrapped が同一性で見ている。
//
// ⚠️ **この検査が呼ぶのは execute であって、本番の入口（Execute）ではない。**
// `Execute()` が「os.LookupEnv と usage.Record と os.Stdout/os.Stderr を渡して execute を呼ぶだけ」
// であることは、ここではなく usage_entrypoint_test.go が別プロセスで見ている
// （段を決め打ちする・別の sink を渡す・別の writer を渡す変異は、そちらで落ちる）。
func TestUsage_DefaultOffDoesNotEnterTheMeasuredPath(t *testing.T) {
	for _, c := range usageOffCases {
		t.Run(c.name, func(t *testing.T) {
			logPath := usageTestEnv(t)
			unsetUsageEnvVar(t, usage.EnvVar)

			// 1) 計測層をまったく通さない経路（＝計測を入れる前の Execute と同じこと）。
			baseDir := seedStore(t, projectNamedArg)
			var baseOut bytes.Buffer
			baseRoot := newRootCmd()
			baseRoot.SetArgs(c.args(baseDir))
			baseRoot.SetOut(&baseOut)
			baseRoot.SetErr(&baseOut)
			baseErr := baseRoot.Execute()

			// 2) 環境変数が未設定のまま、本番の入口（Execute の中身）を通す。
			offDir := seedStore(t, projectNamedArg)
			var offOut bytes.Buffer
			sinkCalls := 0
			sink := func(l usage.Level, _ usage.Observation) {
				sinkCalls++
				t.Errorf("環境変数が未設定なのに計測経路を通り、sink が段 %s で呼ばれた", l)
			}
			offErr := execute(os.LookupEnv, sink, c.args(offDir), &offOut, &offOut)

			if sinkCalls != 0 {
				t.Errorf("sink が %d 回呼ばれた（オフでは 0 回）", sinkCalls)
			}
			if _, err := os.Stat(logPath); !os.IsNotExist(err) {
				t.Errorf("オフなのに計測ログが作られた（%s・err=%v）", logPath, err)
			}
			if _, err := os.Stat(filepath.Dir(logPath)); !os.IsNotExist(err) {
				t.Errorf("オフなのに計測ログのディレクトリが作られた: %s", filepath.Dir(logPath))
			}

			// プロジェクトルートが違うので、そこだけ正規化してからバイト比較する。
			want := strings.ReplaceAll(baseOut.String(), baseDir, "<DIR>")
			got := strings.ReplaceAll(offOut.String(), offDir, "<DIR>")
			if got != want {
				t.Errorf("オフで出力が変わった\n計測層なし: %q\nオフ経路  : %q", want, got)
			}
			if (baseErr == nil) != (offErr == nil) {
				t.Errorf("オフでエラーの有無が変わった: 計測層なし=%v / オフ経路=%v", baseErr, offErr)
			}
			if baseErr != nil && offErr != nil {
				wantErr := strings.ReplaceAll(baseErr.Error(), baseDir, "<DIR>")
				gotErr := strings.ReplaceAll(offErr.Error(), offDir, "<DIR>")
				if gotErr != wantErr {
					t.Errorf("オフでエラーが変わった\n計測層なし: %q\nオフ経路  : %q", wantErr, gotErr)
				}
			}
		})
	}
}

// TestUsage_PlainRootHandsCobraTheWritersUnwrapped は、オフ経路が cobra へ渡す writer が
// **渡されたものそのもの**であること——包んでいない＝観測していないこと——を同一性で見る。
func TestUsage_PlainRootHandsCobraTheWritersUnwrapped(t *testing.T) {
	var out, errw bytes.Buffer
	root := newPlainRoot(nil, &out, &errw)
	if got := root.OutOrStdout(); got != io.Writer(&out) {
		t.Errorf("オフ経路が標準出力を包んでいる: %T", got)
	}
	if got := root.ErrOrStderr(); got != io.Writer(&errw) {
		t.Errorf("オフ経路が標準エラーを包んでいる: %T", got)
	}
}

// TestUsage_RootSilencesUsageOnError は、オフ経路で SetOut していることが振る舞いを変えない
// **前提**を固定する。
//
// cobra の `cmd.Print*` と既定の UsageFunc は `OutOrStderr()` へ書く——SetOut していなければ
// 標準エラー、していれば標準出力である。計測を入れる前のオフ経路は SetOut していなかったので、
// **失敗した起動で usage が出ないこと**が「オフで出力が変わらない」の前提になっている。
// SilenceUsage が false に戻ると、この前提が崩れて宛先が stderr → stdout に動く。
func TestUsage_RootSilencesUsageOnError(t *testing.T) {
	if root := newRootCmd(); !root.SilenceUsage {
		t.Errorf("SilenceUsage が false。オフ経路は SetOut しているので、" +
			"失敗した起動の usage が標準エラーではなく標準出力へ出るようになる（計測前との差になる）")
	}
}

// TestUsage_CountingDoesNotChangeOutput は、計測を入れた経路と入れない経路で
// 標準出力のバイト列が同一であること（計数 writer が素通しであること）。
func TestUsage_CountingDoesNotChangeOutput(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	plain, err := run(t, dir, "rules", "--tag", projectNamedArg)
	if err != nil {
		t.Fatalf("計測なしの実行が失敗: %v", err)
	}

	usageTestEnv(t)
	var stdout, stderr bytes.Buffer
	if err := executeWithUsage(usage.Detailed, usage.Record, []string{"--dir", dir, "rules", "--tag", projectNamedArg}, &stdout, &stderr); err != nil {
		t.Fatalf("計測ありの実行が失敗: %v", err)
	}
	if got := stdout.String() + stderr.String(); got != plain {
		t.Errorf("計測を入れると出力が変わった\n計測なし: %q\n計測あり: %q", plain, got)
	}
}

// --- 分類の性質 ---

// TestUsage_ClosedSetValuesOnly は、閉じた集合の外の値が「道具の側の語彙」として
// 書かれないこと。**分類を間違えても、書けるのは宣言した語彙だけ**という性質を見る。
//
// ⚠️ **表にある classToolVocab の宣言をすべて見る。** 前の版は `scholia rules --sort` 1 つを
// 名指ししていた——1 面の実例を性質の歯止めと数えない（F3 を招いたのと同じ型）。
func TestUsage_ClosedSetValuesOnly(t *testing.T) {
	const outsider = "プロジェクトが名付けた何か"
	checked := 0
	check := func(t *testing.T, where string, spec argSpec) {
		if spec.class != classToolVocab {
			return
		}
		checked++
		t.Run(where, func(t *testing.T) {
			if len(spec.values) == 0 {
				t.Fatalf("classToolVocab なのに閉じた集合が空（値が書かれることは無いが、宣言として誤り）")
			}
			shape := invocationShape{flagValues: map[string]any{}, freeTextLens: map[string]int{}}
			shape.apply("v", spec.values[0], spec, map[string]bool{})
			shape.apply("v", outsider, spec, map[string]bool{})

			if got := shape.flagValues["v"]; got != spec.values[0] {
				t.Errorf("閉じた集合の中の値は残るはず: %v", got)
			}
			if shape.freeTextLens["v"] == 0 {
				t.Errorf("閉じた集合の外の値は長さへ倒れるはず: %v", shape.freeTextLens)
			}
			for _, v := range shape.flagValues {
				if s, ok := v.(string); ok && s == outsider {
					t.Errorf("閉じた集合の外の値が書かれた: %v", v)
				}
			}
			if len(shape.recordIDs) != 0 {
				t.Errorf("道具の側の語彙が recordIds に入った: %v", shape.recordIDs)
			}
		})
	}
	for key, spec := range stringFlagSpecs {
		check(t, key, spec)
	}
	for path, ps := range positionalSpecs {
		for i, spec := range ps.at {
			check(t, fmt.Sprintf("%s arg%d", path, i), spec)
		}
	}
	if checked == 0 {
		t.Fatal("この検査が 1 つの宣言も見ていない（classToolVocab の宣言が表から消えた）。空振りで緑になっている")
	}
}

// TestUsage_UnclassifiedStringFlagFallsBackToLength は、未分類の文字列フラグが
// 値ではなく長さへ倒れること（安全側の既定）。
func TestUsage_UnclassifiedStringFlagFallsBackToLength(t *testing.T) {
	cmd := &cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {}}
	var v string
	cmd.Flags().StringVar(&v, "brand-new-flag", "", "分類表に無いフラグ")
	if err := cmd.Flags().Parse([]string{"--brand-new-flag", "req.secret"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	shape := observeInvocation(cmd)
	if _, ok := shape.flagValues["brand-new-flag"]; ok {
		t.Errorf("未分類のフラグの値が書かれた: %v", shape.flagValues)
	}
	if got := shape.freeTextLens["brand-new-flag"]; got != utf8.RuneCountInString("req.secret") {
		t.Errorf("長さへ倒れていない: %v", shape.freeTextLens)
	}
	if len(shape.recordIDs) != 0 {
		t.Errorf("未分類のフラグが recordIds に入った: %v", shape.recordIDs)
	}
}
