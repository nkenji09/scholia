// jsonio_test.go — `--json` の出力口が 1 つであることの歯止め
// （01KZ7V637RNMPXJMVACYV6V1AS 条項1・条項2）。
//
// # ここの歯止めが落とす範囲（CLAUDE.md「配線ガードの書き方」6）
//
// **落ちる:**
//   - **共有の口（jsonio.go）を通さずに `--json` を書いた**——整形して書いても、
//     compact に書いても、`fmt.Fprintf` で JSON を手書きしても落ちる。
//     🔴 **新しい面だけでなく、既存の面に足した「あるフラグの組み合わせのときだけ
//     通さない」枝でも落ちる**——突き合わせは `executeForTest`（cli_test.go）に
//     置いてあり、**テスト一式がこの package で叩く起動は全部そのまま対象になる**。
//   - `--json` の面を新しく足して、**この歯止めに引き方を書かなかった**
//     ——面は cobra の木から数え上げるので、表に無い面があれば落ちる
//     （TestEveryJSONFaceIsExercised）。⚠️ **列挙を足したのではない。**
//     列挙が**足りないこと自体**が赤になる形にしてある。
//   - package cli の非テスト file が `jsonio.go` 以外で `encoding/json` に触れた
//     ——AST の import を見るので別名・dot import・blank import でも落ちる
//     （TestCLIPackageTouchesEncodingJSONOnlyHere）。
//   - 共有の口が整形して書くようになった（TestRenderJSONLineIsCompact・
//     入力と出力の対で見る純関数の検査）。
//   - `--json` の面が 2 行以上出す・出力が JSON として読めない
//     （TestEveryJSONFaceGoesThroughTheSingleExit。**面ごとに引き方 1 つ**）。
//
// **落ちない（射程の外・正直に名乗る）:**
//   - 🔴 **この package のテストが 1 度も叩かない引き方。** 突き合わせが見るのは
//     **走った起動だけ**で、フラグの組み合わせを網羅してはいない。
//     ⚠️ **これは直しても残る穴である**——`executeForTest` へ移す前は
//     「面ごとに 1 つの引き方」しか走らせておらず、`rules --all` の枝に置いた
//     素通りが緑で通った（実見）。移した後はテスト一式の引き方が全部対象になるが、
//     **どのテストも叩かない引き方**（例: `config get --local <key> --json`）は
//     依然として通らない。面の枝を増やすなら、その枝を叩くテストも要る。
//   - 🔴 **`executeForTest` を通さずに root コマンドを走らせるテスト。**
//     `usage_test.go` は入口の配線そのものを見るために意図的に直に走らせている。
//     そこから出る `--json` は突き合わせに掛からない。
//   - 🔴 **package cli の外**に出力口を作り、そこから `--json` を書く面。
//     import の境界はこの package の file しか見ない。**バイトの突き合わせは
//     cobra の木にぶら下がった面なら拾う**ので、そちらでは落ちる——
//     ただし「cobra の木に載せない出力経路」（viewer の HTTP 等）は初めから射程外である。
//   - `--json` の面が**標準エラー**へ書くもの（`diff` の注記など）。ここで見るのは
//     標準出力だけ。
//   - repo の外の消費者。
//
// # なぜ「共有の口を1つ作った」だけでは足りないのか
//
// 単位AY は「面ごとに分岐を書かない形にした」と名乗ったのに、**畳む前のデータが
// 描画側のスコープに残っていた**ため、新しい面がそれを直に読めば判断を素通りできた。
// 最後に効いたのは検査を足すことではなく、**素通りできる材料をスコープから消して
// コンパイルを通らなくする**ことだった。
//
// ⚠️ **その形は、ここでは使えない。** 素通りの材料は `encoding/json`（package の
// import）と `cmd.OutOrStdout()`（テキストの面が同じ関数の中で使う）で、
// **どちらも字句スコープから取り除けない。** だから「書きようがない」形には
// できない。代わりに、**書いたら落ちる**を 2 通りの独立な見方で置いてある
// （import の境界と、出たバイトの突き合わせ）。
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	pflag "github.com/spf13/pflag"
)

// jsonExitFile は package cli の中で `encoding/json` に触れてよい唯一の非テスト file。
const jsonExitFile = "jsonio.go"

// ---------------------------------------------------------------------------
// 面の数え上げ
// ---------------------------------------------------------------------------

// discoverJSONFaces は cobra の木を歩いて、`--json` フラグを持つ面のコマンド列
// （"show decision" のような空白区切り）を返す。**面をここで列挙しない。**
func discoverJSONFaces(t *testing.T) []string {
	t.Helper()
	var faces []string
	var walk func(c *cobra.Command, prefix []string)
	walk = func(c *cobra.Command, prefix []string) {
		path := prefix
		if c.Parent() != nil { // root（"scholia"）自身は名前に入れない
			path = append(append([]string{}, prefix...), c.Name())
		}
		hasJSON := false
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if f.Name == "json" {
				hasJSON = true
			}
		})
		if hasJSON && c.Runnable() {
			faces = append(faces, strings.Join(path, " "))
		}
		for _, sub := range c.Commands() {
			walk(sub, path)
		}
	}
	walk(newRootCmd(), nil)
	sort.Strings(faces)
	if len(faces) == 0 {
		t.Fatal("`--json` の面を 1 つも拾えていない（この検査は何も見ていない）")
	}
	return faces
}

// 標本の中で作られる id の置き換え札。引き方の表に直接 ULID は書けない
// （毎回変わる）ので、走らせる直前に差し替える。
const (
	placeholderDecisionID    = "<decision-id>"
	placeholderOldDecisionID = "<old-decision-id>"
	placeholderReviewID      = "<review-id>"
)

// jsonFaceInvocations は面ごとの引き方（コマンド列と `--json` は含まない）。
//
// 🔴 **これは「面の一覧」ではない。** 面は cobra の木から数え上げる
// （discoverJSONFaces）。この表は**数え上げた面を実際に走らせるための引数**で、
// 表に無い面があれば TestEveryJSONFaceIsExercised が落ちる——
// **新しい面を足した人は、ここに引き方を書くまで緑にできない。**
var jsonFaceInvocations = map[string][]string{
	"config get":             {},
	"config infer-id-policy": {},
	"config set":             {"tagKinds", "requirement,concern,subject,axis"},
	"decide":                 {"--on", "tag:req.b", "--why", "# 見出し\n\n本文。"},
	"decision add-commit":    {placeholderDecisionID, "0123456789abcdef0123456789abcdef01234567"},
	"decision link":          {placeholderDecisionID, "--supersedes", placeholderOldDecisionID},
	"decision list":          {},
	"decision show":          {placeholderDecisionID},
	"diff":                   {},
	"flow":                   {"act.submit"},
	"gaps":                   {"act.submit"},
	"init":                   {},
	"kind get":               {"action"},
	"kind list":              {},
	"kind set":               {"action", "user,system"},
	"lint":                   {},
	"lint baseline update":   {},
	"list":                   {},
	"refs rewrite":           {"req.a", "req.a2"},
	"refs scan":              {},
	"retrofit":               {},
	"review add":             {"--on", "tag:req.b", "--body", "# 見出し\n\n本文。"},
	"review adopt":           {placeholderReviewID},
	"review list":            {},
	"review reject":          {placeholderReviewID},
	"review rm":              {placeholderReviewID},
	"rules":                  {},
	"search":                 {"要件"},
	"show decision":          {placeholderDecisionID},
	"show tag":               {"req.a"},
	"show tx":                {"T-a"},
	"show vocab":             {"act.submit"},
	"skills install":         {},
	"spec":                   {"subject.core"},
	"tag create":             {"req.new", "--name", "新要件", "--kind", "requirement"},
	"tag edit":               {"req.a", "--name", "要件A改"},
	"tag list":               {},
	"tag rename":             {"req.a", "req.a2"},
	"tag rm":                 {"concern.unused"},
	"tx add":                 {"T-new", "--action", "act.submit", "--then", "eff.token"},
	"tx edit":                {"T-a", "--priority", "1"},
	"tx merge":               {"T-dup", "--into", "T-a"},
	"tx rename":              {"T-a", "--to", "T-a2"},
	"tx rm":                  {"T-b", "--force", "--why", "歯止めの標本で消す"},
	"tx tag":                 {"T-b", "--add", "req.b"},
	"version":                {},
	"vocab add":              {"condition", "cond.new", "--label", "新しい条件"},
	"vocab edit":             {"cond.valid", "--label", "前提が成り立つ（改）"},
	"vocab owner-migrate":    {},
	"vocab rename":           {"cond.valid", "--to", "cond.valid2"},
	"vocab rm":               {"cond.unused"},
	"vocab tag":              {"cond.valid", "--add", "req.a"},
}

// TestEveryJSONFaceIsExercised は「数え上げた面」と「引き方の表」がちょうど
// 一致することを見る。**片方だけ増えたら落ちる。**
func TestEveryJSONFaceIsExercised(t *testing.T) {
	faces := discoverJSONFaces(t)
	declared := make(map[string]bool, len(jsonFaceInvocations))
	for k := range jsonFaceInvocations {
		declared[k] = true
	}

	var missing []string
	for _, f := range faces {
		if !declared[f] {
			missing = append(missing, f)
		}
		delete(declared, f)
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("`--json` を持つのに引き方が書かれていない面がある（jsonFaceInvocations に足すこと）: %v", missing)
	}
	var stale []string
	for k := range declared {
		stale = append(stale, k)
	}
	if len(stale) > 0 {
		sort.Strings(stale)
		t.Errorf("引き方だけあって面が無い（消えた面の引き方が残っている）: %v", stale)
	}
	t.Logf("cobra の木から数え上げた `--json` の面: %d 個", len(faces))
}

// ---------------------------------------------------------------------------
// バイトの突き合わせ
// ---------------------------------------------------------------------------

// assertJSONWentThroughTheSingleExit は「標準出力に出たバイト列が、共有の口
// （jsonio.go）が書いたバイト列そのものか」を見る。**条項2 の本体はこれである。**
//
// 🔴 **呼び出し元は `executeForTest`（cli_test.go）1 つだけで、テストが
// root コマンドを走らせるときは必ずここを通る。** 面ごとの表に引き方を並べる
// 形をやめたのは、**表に無い引き方に置いた素通りが緑のまま通った**ため——
// `rules` に「`--all` のときだけ共有の口を通さず整形して書く」枝を入れると、
// 面の表は `"rules": {}`（引き方 1 つ）なのでその枝を 1 度も通らず、
// **123 行の整形済み JSON が出ているのに歯止めは 1 本も落ちなかった。**
// CLAUDE.md 3（1 つのゲートの内側にいないことだけを見る実装は、別のゲートで
// 包む変異を通す）の実例である。
//
// 「compact であること」だけを見る検査では足りない理由は別にある——
// **共有の口を通さずに compact な JSON を書いた面**を素通りさせるからで、
// 同じ意味を別の綴りで書かれれば捕まらない（CLAUDE.md 2）。
// ここでは「通ったかどうか」そのものを観測している。
//
// ⚠️ **見送る場合**（下の 2 つ。射程として file 冒頭にも書いてある）:
//   - 引数に `--json` が無く、共有の口も 1 バイトも書かなかった起動
//     （＝そもそも JSON を出していない）
//   - 標準出力が空の起動（比べる相手が無い）
func assertJSONWentThroughTheSingleExit(t *testing.T, args []string, stdout, emitted string) {
	t.Helper()
	if !argsRequestJSON(args) && emitted == "" {
		return
	}
	if stdout == "" {
		return
	}
	if stdout != emitted {
		t.Errorf("`--json` の出力が共有の口（%s）の書いたバイト列と違う。"+
			"書く経路がもう1つある: %v\n--- 標準出力 (%d B) ---\n%s\n--- 共有の口 (%d B) ---\n%s",
			jsonExitFile, args, len(stdout), stdout, len(emitted), emitted)
	}
}

// argsRequestJSON は引数列に `--json` があるかを見る（`--json=true` も拾う）。
func argsRequestJSON(args []string) bool {
	for _, a := range args {
		if a == "--json" || strings.HasPrefix(a, "--json=") {
			return true
		}
	}
	return false
}

// TestEveryJSONFaceGoesThroughTheSingleExit は、**数え上げた全ての面が少なくとも
// 1 度は走ること**を担う。バイトの突き合わせ自体は `executeForTest` が行うので、
// ここが見るのは「どの面も検査されないまま残らない」ことと、整形されていない
// こと・JSON として読めることの 2 つである。
//
// ⚠️ **引き方は面ごとに 1 つしか書いていない。** それでよいのは、**表に無い
// 引き方も `executeForTest` が拾う**ようになったため——この表の役目は
// 「全ての面が最低 1 回は通る」を保つことに縮んでいる。
func TestEveryJSONFaceGoesThroughTheSingleExit(t *testing.T) {
	template, ids := seedJSONFaceFixture(t)

	for _, face := range discoverJSONFaces(t) {
		extra, ok := jsonFaceInvocations[face]
		if !ok {
			continue // TestEveryJSONFaceIsExercised が別途落とす
		}
		t.Run(face, func(t *testing.T) {
			dir := copyFixture(t, template)
			t.Chdir(dir) // `skills install --project` は cwd に書く

			args := append(strings.Fields(face), ids.resolve(extra)...)
			args = append(args, "--json")

			// バイトの突き合わせは runSplit の中（executeForTest）で当たる。
			stdout, stderr, err := runSplit(t, dir, args...)
			if err != nil {
				t.Fatalf("%v が失敗した（引き方が古い可能性がある）: %v\n--- stdout ---\n%s\n--- stderr ---\n%s",
					args, err, stdout, stderr)
			}
			if stdout == "" {
				t.Fatalf("%v が標準出力に何も出さなかった（この面は検査されていない）", args)
			}
			// 整形されていないこと（改行は末尾の 1 つだけ）。
			if n := strings.Count(stdout, "\n"); n != 1 || !strings.HasSuffix(stdout, "\n") {
				t.Errorf("`--json` の出力が 1 行でない（改行 %d 個）:\n%s", n, stdout)
			}
			// JSON として読めること。
			var decoded any
			if err := json.Unmarshal([]byte(stdout), &decoded); err != nil {
				t.Errorf("`--json` の出力が JSON として読めない: %v\n%s", err, stdout)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 標本
// ---------------------------------------------------------------------------

// fixtureIDs は標本の中で生成された id（毎回変わるので走らせる直前に差し替える）。
type fixtureIDs struct {
	decision    string
	oldDecision string
	review      string
}

func (ids fixtureIDs) resolve(args []string) []string {
	out := make([]string, len(args))
	for i, a := range args {
		switch a {
		case placeholderDecisionID:
			out[i] = ids.decision
		case placeholderOldDecisionID:
			out[i] = ids.oldDecision
		case placeholderReviewID:
			out[i] = ids.review
		default:
			out[i] = a
		}
	}
	return out
}

// seedJSONFaceFixture は全ての面が走れる標本を 1 つ作る（面ごとに複製して使う）。
//
// ⚠️ **本番の `.scholia` は 4 カテゴリ（vocab・tags・transitions・decisions）を持つ。**
// git 履歴も作る——`diff` は作業ツリーと HEAD を比べるので、git 無しでは走れない。
func seedJSONFaceFixture(t *testing.T) (string, fixtureIDs) {
	t.Helper()
	dir := t.TempDir()
	must := func(args ...string) string {
		t.Helper()
		out, err := run(t, dir, args...)
		if err != nil {
			t.Fatalf("標本づくり %v が失敗: %v\n%s", args, err, out)
		}
		return out
	}

	must("init")
	must("config", "set", "tagKinds", "requirement,concern,subject,axis")

	must("vocab", "add", "condition", "cond.valid", "--label", "前提が成り立つ")
	must("vocab", "add", "condition", "cond.other", "--label", "別の前提")
	must("vocab", "add", "condition", "cond.unused", "--label", "どこからも参照されない前提")
	must("vocab", "add", "action", "act.submit", "--label", "送信する", "--kind", "user")
	must("vocab", "add", "effect", "eff.token", "--label", "トークンを発行する", "--kind", "state", "--owner", "server")

	must("tag", "create", "subject.core", "--name", "中核", "--kind", "subject", "--desc", "説明を持つ親タグ。")
	must("tag", "create", "req.a", "--name", "要件A", "--kind", "requirement", "--parent", "subject.core",
		"--desc", "引用符 \" と < > & を含む説明。")
	must("tag", "create", "req.b", "--name", "要件B", "--kind", "requirement", "--parent", "subject.core")
	must("tag", "create", "concern.unused", "--name", "どこからも参照されない関心", "--kind", "concern")

	must("tx", "add", "T-a", "--action", "act.submit", "--given", "cond.valid", "--then", "eff.token", "--tags", "req.a")
	must("tx", "add", "T-b", "--action", "act.submit", "--given", "cond.other", "--then", "eff.token")
	must("tx", "add", "T-dup", "--action", "act.submit", "--given", "cond.valid", "--then", "eff.token")

	var ids fixtureIDs
	ids.oldDecision = extractJSONID(t, must("decide", "--on", "tag:req.a",
		"--why", "# 標本用の見出し\n\n置き換えられる側の判断。", "--json"))
	ids.decision = extractJSONID(t, must("decide", "--on", "transition:T-a",
		"--why", "# 標本用の見出し 2\n\n置き換える側の判断。", "--json"))
	ids.review = extractJSONID(t, must("review", "add", "--on", "tag:req.a",
		"--body", "# 提案の見出し\n\n提案の本文。", "--json"))

	// `refs scan` / `refs rewrite` が拾うソース側の引用。
	src := "// req.a を参照するコメント\npackage x\n"
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	seedGitHistory(t, dir)
	return dir, ids
}

// seedGitHistory は `diff` が比べる HEAD を作る。
//
// ⚠️ **git が無ければ skip ではなく fail させる。** skip は「素通り」と見分けが
// つかない（CLAUDE.md の趣旨）。`diff --json` はこの repo の CI ゲートでもあるので、
// 検査しないまま緑にはしない。
func seedGitHistory(t *testing.T, dir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Fatalf("この歯止めは git を要る（`diff --json` を走らせるため）: %v", err)
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "guard@example.invalid"},
		{"config", "user.name", "guard"},
		{"add", "-A"},
		{"commit", "-q", "-m", "seed"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// copyFixture は標本を丸ごと複製する（面ごとに書き込みが混ざらないように）。
func copyFixture(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("標本の複製に失敗: %v", err)
	}
	return dst
}

// ---------------------------------------------------------------------------
// import の境界
// ---------------------------------------------------------------------------

// TestCLIPackageTouchesEncodingJSONOnlyHere は、package cli の非テスト file の
// うち `encoding/json` を import してよいのが jsonio.go だけであることを見る。
//
// 見るのは **AST の import 宣言**なので、別名（`j "encoding/json"`）・
// dot import・blank import のいずれでも同じに落ちる。
//
// ⚠️ **これは「JSON を書いたか」を見ていない。** `fmt.Fprintf` で JSON を手書き
// する経路は import を増やさないので、ここは素通りする——そちらは
// TestEveryJSONFaceGoesThroughTheSingleExit が落とす。2 つで組である。
func TestCLIPackageTouchesEncodingJSONOnlyHere(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var offenders []string
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		checked++
		f, err := parser.ParseFile(fset, name, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s を parse できない: %v", name, err)
		}
		for _, imp := range f.Imports {
			path, err := parseImportPath(imp)
			if err != nil {
				t.Fatalf("%s の import を読めない: %v", name, err)
			}
			if path == "encoding/json" && name != jsonExitFile {
				offenders = append(offenders, name)
			}
		}
	}
	if checked == 0 {
		t.Fatal("非テスト file を 1 つも見ていない（この検査は何も見ていない）")
	}
	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("package cli で `encoding/json` に触れてよいのは %s だけ。触っている file: %v\n"+
			"（`--json` を書くなら emitJSON / emitJSONTo を通すこと・01KZ7V637RNMPXJMVACYV6V1AS 条項2）",
			jsonExitFile, offenders)
	}
	t.Logf("非テスト file %d 本を見た", checked)
}

func parseImportPath(imp *ast.ImportSpec) (string, error) {
	if imp.Path == nil {
		return "", fmt.Errorf("import path が無い")
	}
	return strings.Trim(imp.Path.Value, `"`), nil
}

// ---------------------------------------------------------------------------
// 整形しないこと（純関数・入力と出力の対）
// ---------------------------------------------------------------------------

// TestRenderJSONLineIsCompact は「何を渡すと何が出るか」を対で見る（CLAUDE.md 1）。
// 画面を起こさずに、整形の判断そのものを検査している。
func TestRenderJSONLineIsCompact(t *testing.T) {
	type inner struct {
		B []int  `json:"b"`
		C string `json:"c"`
	}
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"空の object", struct{}{}, "{}\n"},
		{"空の配列", []int{}, "[]\n"},
		{"nil スライス", []int(nil), "null\n"},
		{"入れ子", struct {
			A inner `json:"a"`
		}{inner{B: []int{1, 2}, C: "x"}}, `{"a":{"b":[1,2],"c":"x"}}` + "\n"},
		{"配列の要素が object", []inner{{B: []int{1}, C: "y"}, {B: nil, C: ""}},
			`[{"b":[1],"c":"y"},{"b":null,"c":""}]` + "\n"},
		// 値そのものは 1 つも変えない。HTML escape も改行の escape も従来どおり
		// （json.Encoder は既定で `<` `>` `&` を \uXXXX へ escape する。整形をやめても
		// その既定は動かない——変わるのは空白だけ、を欄そのもので固定する）。
		{"HTML escape は従来どおり", map[string]string{"k": "<a> & </a>"},
			`{"k":"\u003ca\u003e \u0026 \u003c/a\u003e"}` + "\n"},
		{"文字列の中の改行は escape のまま", map[string]string{"k": "1 行目\n2 行目"},
			`{"k":"1 行目\n2 行目"}` + "\n"},
		{"日本語はそのまま", map[string]string{"k": "要件"}, `{"k":"要件"}` + "\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := renderJSONLine(tc.in)
			if err != nil {
				t.Fatalf("renderJSONLine: %v", err)
			}
			if string(got) != tc.want {
				t.Errorf("renderJSONLine(%v)\n got: %q\nwant: %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestEmitJSONToWritesRenderJSONLine は、書き出す口と純関数が同じバイト列を
// 出すことを固定する（純関数だけ compact にして口が整形を続ける、を落とす）。
func TestEmitJSONToWritesRenderJSONLine(t *testing.T) {
	v := map[string]any{"a": []int{1, 2}, "b": "x"}
	want, err := renderJSONLine(v)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := emitJSONTo(&buf, v); err != nil {
		t.Fatal(err)
	}
	if buf.String() != string(want) {
		t.Errorf("emitJSONTo と renderJSONLine が違う\n got: %q\nwant: %q", buf.String(), want)
	}
}

// TestRenderIndentedJSONStaysIndented は、テキストの面が埋め込む JSON 断片が
// **整形されたまま**であることを固定する（条項1 の射程は `--json` の出力で、
// テキスト出力は 1 バイトも変わらない）。
func TestRenderIndentedJSONStaysIndented(t *testing.T) {
	got, err := renderIndentedJSON(map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"a\": 1\n}"; string(got) != want {
		t.Errorf("テキストの面の JSON 断片が整形されていない\n got: %q\nwant: %q", got, want)
	}
}
