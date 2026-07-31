package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/usage"
)

// --- 本番の入口（Execute）を、別プロセスで実際に走らせる ---
//
// ⚠️ **なぜ別プロセスなのか。** `Execute()` は
// 「`os.LookupEnv` / `usage.Record` / `nil` / `os.Stdout` / `os.Stderr` を execute へ渡す」1 行で、
// **渡すものを書き換える変異は、同じプロセスの中からは値で見えない**——`os.Stdout` を別の writer に
// 差し替えられても、その差し替えを観測する立場がプロセスの中に無いからである。
// 別プロセスにすると `os.Stdout` は親が握るパイプになり、**入口が実際に何へ何バイト渡したか**を
// 記録された行と突き合わせられる。
//
// ⚠️ **なぜ go build / go run ではなくテストバイナリの再実行なのか。**
// 走らせたいのは `Execute()` であって、そこへ届く形はいくつかある。
// ビルドを伴う形（`go build` したバイナリ・`go run ./cmd/scholia`）は、
// Go ツールチェイン・ビルドキャッシュ・buildvcs スタンプ（linked worktree では既定で落ちる）に
// 依存する検査になる。**この repo はビルド依存・時間依存の検査で flaky 事故の前例がある。**
// テストバイナリの再実行なら依存が増えず、スキップする理由も無いので、
// **CI（ubuntu）でも手元でも同じ 1 本が必ず走る。** その代わり `cmd/scholia/main.go` は通らない
// ——下の「落ちない」で名乗る。
//
// ⚠️ **なぜ環境変数ではなく argv の先頭で合図するのか。**
// 初版は環境変数（`SCHOLIA_TEST_REAL_ENTRYPOINT`）を目印にしていた。それだと
// **その名前が実環境に立っているだけで、このパッケージは 0 本のテストで緑になる**
// ——`go test` がテストバイナリへ渡す `-test.timeout=10m0s` の形（`-` 1 個 ＋ `=`）を
// cobra は help を出して **nil** で受けるので、入口分岐が exit 0 で終わり `m.Run()` に到達しない。
// 下の非空振りの歯止めも、そのとき**原理的に発火できない。**
// 初版はここに「黙って緑になることはない」と書いていた——**レビュアの実測で反証された**
// （`-v` なのにテスト名が 1 つも出ずに 0.255 秒で ok。手元でも再現した）。
// 合図を argv の先頭へ移すと、この条件は**検出ではなく構造で消える**——`go test` は
// テストバイナリの argv を自分で組み立て、先頭は `-test.*` で、`-args` で足した値は後ろに付く
// （実測済み）。**しかも入口分岐は 2 つの合図が揃ったときだけ通る**ので、
// **環境に何が立っていても、`go test` からこの分岐へ入る道は無い。**
// ⚠️ **「名前を衝突しにくくする」形は採らなかった。** それは確率を下げるだけで、
// **唯一の無効化条件は残る**。残さない形を選んだ。
// ⚠️ 残余: テストバイナリを**直に**叩き、合図を先頭に置き、かつ環境変数も立てれば入口分岐は通る。
// それは意図してやらないと起きない（`go test` からは組み立てられない）。
//
// ⚠️ **射程の名乗り**（CLAUDE.md 6）。下の「落ちる」はすべて、実際に変異を当てて赤を実見している。
//
// 落ちる:
//   - `os.LookupEnv` を**常に未設定を返す**関数に差し替える（計測が永久にオフになる）。
//   - `os.LookupEnv` を**常に別の段を返す**関数に差し替える（段が環境ではなく実装で決まる）。
//     ⚠️ 「ログが 1 行できたか」だけを見る検査はこれを通す。**段の名前まで見る。**
//   - `usage.Record` を**何もしない sink** に差し替える。
//   - `os.Stdout` を別の writer（`io.Discard`・`os.Stderr`）に差し替える。
//   - `os.Stderr` を別の writer に差し替える。
//   - 引数の `nil` を `[]string{}` に差し替える（cobra が `os.Args[1:]` を読まなくなる）。
//   - 入口が**返り値のエラーを握り潰す**（失敗した起動の exit code が 0 になる）。
//   - 段が立っていないのに計測経路へ入る（＝既定で計測ログが生える。条項 10）。
//   - **この仕組み自身の空振り 5 通り**——
//     (a) TestMain の入口分岐が効かない（子が入口ではなくテストを走らせる）。
//     (b) 入口の検査を**1 本でも**消す・スキップする。⚠️ 初版はパッケージ全体で「1 回以上走ったか」しか
//     見ていなかったので、**3 本のうち 1 本を消しても残り 2 本が数を埋めて沈黙した**
//     （消えたのは条項 10 を本番の入口で見る唯一の検査だった）。いまは下の表と名前で突き合わせる。
//     (c) 1 本の中の**再実行を 1 回でも**減らす（表は回数まで宣言している）。
//     ⚠️ **検査が緑のままでも落ちる**——`StaysOffByDefault` の対照（段を立てた 1 回）を
//     まるごと消すと、その検査自体は緑で通るが回数が 1 になって落ちる。
//     (d) 入口の検査を足して表に載せない（表に無い名前が走ったら落ちる）。
//     (e) 申告（`usageEntrypointRan` への記録）を消す・合図の受け渡しを壊す。
//
// ⚠️ 落ちない（＝ここは守っていない）:
//   - **`cmd/scholia/main.go` の 3 行。** 走らせているのは `Execute()` であって `main()` ではない。
//     下の入口分岐が main と同じこと（err なら exit 1）をしているのは**写しであって検査ではない**。
//     main が `Execute()` を呼ばなくなる変異・err を捨てる変異は、ここでは捕まらない。
//   - **配布されるバイナリのビルド。** goreleaser・ldflags・埋め込みが壊れても落ちない。
//   - **回した起動の外。** ここが通すのは `rules`（成功）と未知のコマンド（失敗）の 2 通りだけで、
//     全コマンドの配線ではない。⚠️ **1 面で赤を見たことを面全体の性質と読まないこと**
//     ——ただしここで見ているのは面ごとの分類ではなく**入口が渡す 5 つ**なので、面を増やしても
//     捕まえるものは増えない（面の側の性質は usage_test.go の全面を回す検査が持つ）。
//   - **`-run` / `-skip` で絞ったときに、宣言した検査が走らないこと。** 絞ったのは人の意図なので見ない。
//     絞り込みがあっても (d)（表に無い名前が走る）は見る。CI は絞らないので両向き効く。
//   - ⚠️ **表そのものを空にし、かつ入口の検査を全部消すこと。** そこまでやると要求も申告も無くなるので
//     沈黙する。**片方だけでは沈黙しない**——表を空にすれば (d) が、検査を消せば (b) が落ちる。
//     これは表を持つ歯止めに共通の残余で、この形では閉じられない。

// usageEntrypointArg / usageEntrypointDepthEnv は、テストバイナリを
// **「本番の入口を 1 回走らせるだけのプロセス」**にする 2 つの合図。
//
// ⚠️ **立てるのは AND、止めるのは OR。**
// 入口分岐へ入るのは**両方揃ったときだけ**（＝乗っ取りに対して安全側）。
// 再実行を断るのは**どちらか 1 つでもあるとき**（＝暴走に対して安全側）。
// 2 つを別の経路（argv と環境変数）に置いたのは、片方を落とす変異でも
// **入れ子の上限が消えない**ようにするためである。
//
// ⚠️ argv 側は**先頭に置いたときだけ**効く。`go test` はテストバイナリの argv を自分で組み立て、
// 先頭は `-test.*`（`-test.paniconexit0` は `-timeout 0` でも付く）で、
// **`-args` で足した値は後ろに付く**——実測で確かめた。
// だから `go test` から argv 側の合図が先頭に来る道が無く、そのうえ環境変数側も要る。
const (
	usageEntrypointArg      = "--scholia-real-entrypoint"
	usageEntrypointDepthEnv = "SCHOLIA_TEST_ENTRYPOINT_DEPTH"
)

// usageEntrypointSignals は、いまのプロセスに立っている合図を返す。
func usageEntrypointSignals() (arg bool, depth bool) {
	arg = len(os.Args) > 1 && os.Args[1] == usageEntrypointArg
	_, depth = os.LookupEnv(usageEntrypointDepthEnv)
	return arg, depth
}

// usageEntrypointRequired は、**本番の入口を走らせる検査と、その再実行の回数**の表。
//
// ⚠️ **「1 回以上走ったか」では足りない。** 初版はパッケージ全体の回数だけを見ていたので、
// 3 本のうち 1 本を消しても残りが数を埋めて沈黙した。この repo が繰り返している
// 「表から 1 行消すと黙って検査が消える」型（CLAUDE.md 5）が、**ガードを主題にしたこの単位で
// 新設した面にも開いていた。** だから名前と回数の対で突き合わせる。
var usageEntrypointRequired = map[string]int{
	"TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStdout": 1,
	"TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStderr": 1,
	// ⚠️ 既定（オフ）と段を立てた対で見るので 2 回。**対照の 1 回を落とすと
	// 「ログが無い」が空振りかどうか分からなくなる**ので、回数まで宣言する。
	"TestUsage_RealEntrypointStaysOffByDefault": 2,
}

// usageEntrypointRan は、実際に本番の入口を走らせた検査と、その回数。
var usageEntrypointRan = map[string]int{}

// TestMain は 2 つの顔を持つ。
//
//  1. 合図が**2 つとも**立っていれば、**本番の入口を 1 回走らせて終わる**。
//     ここは `cmd/scholia/main.go` と同じことをする——⚠️ **写しであって、main の検査ではない。**
//  2. そうでなければ通常どおりテストを走らせ、**入口の検査が表のとおり走ったか**を突き合わせる。
func TestMain(m *testing.M) {
	if arg, depth := usageEntrypointSignals(); arg && depth {
		// 合図を取り除いてから渡す。Execute は cobra の既定で os.Args[1:] を読む。
		os.Args = append(os.Args[:1], os.Args[2:]...)
		// cmd/scholia/main.go と同じ: cobra がエラーを標準エラーへ出すので、ここは exit code だけ。
		if err := Execute(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	code := m.Run()
	if code == 0 {
		if problems := usageEntrypointShortfall(); len(problems) > 0 {
			fmt.Fprintf(os.Stderr, `本番の入口（Execute）を走らせる検査が、宣言（usageEntrypointRequired）と合わない:
  %s
検査が消された・スキップされた・回数が変わった・表に載せずに足したのいずれかである。
usage_entrypoint_test.go を参照。
`, strings.Join(problems, "\n  "))
			code = 1
		}
	}
	os.Exit(code)
}

// usageEntrypointShortfall は、走った検査を表と突き合わせて食い違いを返す。
func usageEntrypointShortfall() []string {
	var problems []string

	// 1) 表に無い名前が走った——入口の検査を足して表に載せていない。
	//    ⚠️ こちらは**絞り込みの有無に関わらず**見る（絞っても、走ったものは走ったので）。
	for name, n := range usageEntrypointRan {
		if _, ok := usageEntrypointRequired[name]; !ok {
			problems = append(problems,
				fmt.Sprintf("%s: 本番の入口を %d 回走らせたが、表に無い", name, n))
		}
	}

	// 2) 表にある検査が宣言どおり走っていない——消された・スキップされた・回数が減った。
	//    ⚠️ `-run` / `-skip` で絞ったときは人が意図して減らしているので見ない。CI は絞らない。
	if !usageEntrypointNarrowed() {
		for name, want := range usageEntrypointRequired {
			if got := usageEntrypointRan[name]; got != want {
				problems = append(problems,
					fmt.Sprintf("%s: 本番の入口を %d 回走らせた（表の宣言は %d 回）", name, got, want))
			}
		}
	}

	sort.Strings(problems)
	return problems
}

// usageEntrypointNarrowed は、走らせる検査が絞り込まれているかを返す。
func usageEntrypointNarrowed() bool {
	for _, name := range []string{"test.run", "test.skip"} {
		if f := flag.Lookup(name); f != nil && f.Value.String() != "" {
			return true
		}
	}
	return false
}

// usageEntrypointResult は本番の入口を 1 回走らせた結果。
type usageEntrypointResult struct {
	stdout   string
	stderr   string
	exitCode int
}

// runRealEntrypoint はテストバイナリを再実行し、**本番の入口（Execute）**を 1 回走らせる。
//
// 環境は親のものを引き継ぐ（HOME・XDG_STATE_HOME・呼び出し元の名乗りは usageTestEnv が固定する）。
// extra はその上書きで、段を立てるときだけ使う。
// 作業ディレクトリは空の一時ディレクトリにする——cwd の `.scholia` を偶然拾わせないため。
func runRealEntrypoint(t *testing.T, extra map[string]string, args ...string) usageEntrypointResult {
	t.Helper()

	// ⚠️ **入口を走らせるはずのプロセスがテストを走らせている**なら、ここで止める。
	// 止めないと、この関数が孫プロセスを産み続ける。
	//
	// 実測した上限は 2 段構えである。
	//  1. **合図が `--` で始まること自体が 1 段目**——入口分岐を外す変異では、子の argv 先頭に
	//     合図が残るので testing のフラグ解析が拒み、**子は 0.01 秒で exit 2**（テストを走らせない）。
	//  2. **合図を argv から落とす変異では 1 段目が効かない**ので、深さの合図がここで止める。
	//     実測: 子はテスト一式を 1 回だけ走らせ、その中のこの関数が断って**1 世代で止まる**。
	//
	// ⚠️ **止めるのは OR**——合図がどちらか 1 つでも立っていたら断る。
	// 片方を落とす変異（合図を argv から外す・環境から外す）でも上限が消えないようにするため。
	if arg, depth := usageEntrypointSignals(); arg || depth {
		t.Fatalf("再実行の合図（argv=%v・%s=%v）が立ったプロセスがテストを走らせている。"+
			"TestMain の入口分岐か、合図の受け渡しが壊れている", arg, usageEntrypointDepthEnv, depth)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("テストバイナリのパスを取れない: %v", err)
	}

	env := map[string]string{usageEntrypointDepthEnv: "1"}
	for k, v := range extra {
		env[k] = v
	}
	cmd := exec.Command(exe, append([]string{usageEntrypointArg}, args...)...)
	cmd.Dir = t.TempDir()
	cmd.Env = usageEntrypointEnviron(env)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	code := 0
	switch e := cmd.Run().(type) {
	case nil:
	case *exec.ExitError:
		code = e.ExitCode()
	default:
		t.Fatalf("本番の入口を走らせられない: %v\nstderr:\n%s", e, stderr.String())
	}
	// どの検査が何回走らせたかを申告する（TestMain が表と突き合わせる）。
	// 部分検査（t.Run）から呼ばれても、申告するのは**その検査の名前**である。
	top, _, _ := strings.Cut(t.Name(), "/")
	usageEntrypointRan[top]++
	return usageEntrypointResult{stdout: stdout.String(), stderr: stderr.String(), exitCode: code}
}

// usageEntrypointEnviron は、親の環境から overrides のキーを取り除いてから overrides を足す。
// 同じキーを 2 度並べたときの扱いに依存しないため、重複させずに組む。
func usageEntrypointEnviron(overrides map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(overrides))
	for _, kv := range os.Environ() {
		k, _, ok := strings.Cut(kv, "=")
		if ok {
			if _, replaced := overrides[k]; replaced {
				continue
			}
		}
		env = append(env, kv)
	}
	for k, v := range overrides {
		env = append(env, k+"="+v)
	}
	return env
}

// usageSingleLine は計測ログを読み、1 行だけであることを確かめて返す。
func usageSingleLine(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("計測ログを読めない（%s）: %v", path, err)
	}
	rows := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(rows) != 1 {
		t.Fatalf("1 起動なのに %d 行書かれた:\n%s", len(rows), data)
	}
	var line map[string]any
	if err := json.Unmarshal([]byte(rows[0]), &line); err != nil {
		t.Fatalf("行が JSON として読めない: %v\n%s", err, rows[0])
	}
	return line
}

// usageLineNumber は行の数値項目を返す（null や別の型なら落とす）。
func usageLineNumber(t *testing.T, line map[string]any, key string) int {
	t.Helper()
	v, ok := line[key].(float64)
	if !ok {
		t.Fatalf("行の %q が数ではない: %v", key, line[key])
	}
	return int(v)
}

// TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStdout は、**本番の入口が渡した 5 つ**を
// 別プロセスの実行結果と突き合わせる。
//
// ここが同時に見ているもの:
//   - 段が**環境から**決まる（`os.LookupEnv` を渡している）——level が detailed であること。
//   - 記録が**本物の sink** へ行く（`usage.Record` を渡している）——ログが 1 行できること。
//   - 数えている writer が**本物の標準出力**である（`os.Stdout` を渡している）——
//     記録された stdoutBytes が、親が受け取ったバイト数と一致すること。
//   - 引数が cobra の既定（`os.Args[1:]`）で読まれる（`nil` を渡している）——
//     command が `scholia rules` になること。
func TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStdout(t *testing.T) {
	logPath := usageTestEnv(t)
	unsetUsageEnvVar(t, usage.EnvVar)
	dir := seedStore(t, projectNamedArg)

	res := runRealEntrypoint(t, map[string]string{usage.EnvVar: "detailed"},
		"--dir", dir, "rules", "--tag", projectNamedArg)

	if res.exitCode != 0 {
		t.Fatalf("入口が exit %d で終わった\nstdout:\n%s\nstderr:\n%s", res.exitCode, res.stdout, res.stderr)
	}
	// ⚠️ 出力が空だと「一致した」が無意味になる（0 == 0）。まず**渡っていること**を見る。
	if res.stdout == "" {
		t.Fatalf("入口が本物の標準出力へ何も渡していない\nstderr:\n%s", res.stderr)
	}
	// 本業の出力がそのまま渡っていること（プロセス内で同じコマンドを回したものと突き合わせる）。
	plain, err := run(t, dir, "rules", "--tag", projectNamedArg)
	if err != nil {
		t.Fatalf("プロセス内の実行が失敗: %v\n%s", err, plain)
	}
	if res.stdout != plain {
		t.Errorf("入口が渡した標準出力が本業の出力と違う\nプロセス内: %q\n本番の入口: %q", plain, res.stdout)
	}

	line := usageSingleLine(t, logPath)
	if got := line["level"]; got != "detailed" {
		t.Errorf("段が環境から決まっていない: level=%v（入口が os.LookupEnv を渡していない）", got)
	}
	if got := line["command"]; got != "scholia rules" {
		t.Errorf("command が %v（入口が cobra へ os.Args[1:] を読ませていない）", got)
	}
	if got, want := usageLineNumber(t, line, "stdoutBytes"), len(res.stdout); got != want {
		t.Errorf("stdoutBytes=%d だが、本物の標準出力が受け取ったのは %d バイト"+
			"（入口が数えているのは os.Stdout ではない）", got, want)
	}
	if got, want := usageLineNumber(t, line, "stderrBytes"), len(res.stderr); got != want {
		t.Errorf("stderrBytes=%d だが、本物の標準エラーが受け取ったのは %d バイト", got, want)
	}
	if got := usageLineNumber(t, line, "exitCode"); got != 0 {
		t.Errorf("exitCode が %d", got)
	}
}

// TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStderr は、**標準エラー側**を同じ形で見る。
//
// ⚠️ **成功する起動だけでは `os.Stderr` の差し替えを捕まえられない**——標準エラーが空なら
// 「記録 0 バイト == 受け取り 0 バイト」で一致してしまう。だから量が出る起動（失敗）を別に回す。
// ついでに、入口から**エラーが返ること**（exit code 1 になること）も見る。
func TestUsage_RealEntrypointRecordsWhatItHandedToTheRealStderr(t *testing.T) {
	logPath := usageTestEnv(t)
	unsetUsageEnvVar(t, usage.EnvVar)

	// --dir を渡すのは他の 2 本と揃えるため。渡さないと、$TMPDIR が scholia プロジェクトの
	// 内側に置かれた端末で、cwd からの上方探索が偶然どこかの .scholia を拾いうる。
	res := runRealEntrypoint(t, map[string]string{usage.EnvVar: "detailed"},
		"--dir", t.TempDir(), "no-such-command")

	if res.exitCode != 1 {
		t.Fatalf("失敗する起動が exit %d で終わった（入口が返したエラーが exit code になっていない）"+
			"\nstdout:\n%s\nstderr:\n%s", res.exitCode, res.stdout, res.stderr)
	}
	if res.stderr == "" {
		t.Fatalf("入口が本物の標準エラーへ何も渡していない\nstdout:\n%s", res.stdout)
	}

	line := usageSingleLine(t, logPath)
	if got, want := usageLineNumber(t, line, "stderrBytes"), len(res.stderr); got != want {
		t.Errorf("stderrBytes=%d だが、本物の標準エラーが受け取ったのは %d バイト"+
			"（入口が数えているのは os.Stderr ではない）", got, want)
	}
	if got, want := usageLineNumber(t, line, "stdoutBytes"), len(res.stdout); got != want {
		t.Errorf("stdoutBytes=%d だが、本物の標準出力が受け取ったのは %d バイト", got, want)
	}
	if got := usageLineNumber(t, line, "exitCode"); got != 1 {
		t.Errorf("失敗した起動なのに exitCode が %d", got)
	}
}

// TestUsage_RealEntrypointStaysOffByDefault は、正本で**最も重い不変**（条項 10）を
// **本番の入口で**見る。
//
// ⚠️ **「ログが無い」だけを積む検査にしない。** 同じ起動を段を立てて回し、**同じ仕組みで
// ログが生えること**を続けて確かめる——生えないなら、上の「無かった」は空振りである。
// 併せて、段を立てても出力・exit code が 1 バイトも変わらないことを見る（条項 10 の観測可能な差）。
func TestUsage_RealEntrypointStaysOffByDefault(t *testing.T) {
	logPath := usageTestEnv(t)
	unsetUsageEnvVar(t, usage.EnvVar)
	dir := seedStore(t, projectNamedArg)
	args := []string{"--dir", dir, "rules", "--tag", projectNamedArg}

	off := runRealEntrypoint(t, nil, args...)

	if off.exitCode != 0 {
		t.Fatalf("既定の起動が exit %d で終わった\nstdout:\n%s\nstderr:\n%s", off.exitCode, off.stdout, off.stderr)
	}
	if off.stdout == "" {
		t.Fatalf("既定の起動が標準出力へ何も渡していない\nstderr:\n%s", off.stderr)
	}
	if off.stderr != "" {
		t.Errorf("既定なのに標準エラーへ何か出た: %q", off.stderr)
	}
	if _, err := os.Stat(logPath); !os.IsNotExist(err) {
		t.Errorf("既定なのに計測ログが作られた（%s・err=%v）", logPath, err)
	}
	if _, err := os.Stat(filepath.Dir(logPath)); !os.IsNotExist(err) {
		t.Errorf("既定なのに計測ログのディレクトリが作られた: %s", filepath.Dir(logPath))
	}

	// ⚠️ ここから下が「無かった」を空振りにしないための対照である。
	on := runRealEntrypoint(t, map[string]string{usage.EnvVar: "detailed"}, args...)

	if on.stdout != off.stdout {
		t.Errorf("段を立てると標準出力が変わった\n既定: %q\n詳細: %q", off.stdout, on.stdout)
	}
	if on.stderr != off.stderr {
		t.Errorf("段を立てると標準エラーが変わった\n既定: %q\n詳細: %q", off.stderr, on.stderr)
	}
	if on.exitCode != off.exitCode {
		t.Errorf("段を立てると exit code が変わった: 既定=%d / 詳細=%d", off.exitCode, on.exitCode)
	}
	if line := usageSingleLine(t, logPath); line["level"] != "detailed" {
		t.Fatalf("段を立てても記録されない。上の「既定でログが無い」は空振りである: %v", line["level"])
	}
}
