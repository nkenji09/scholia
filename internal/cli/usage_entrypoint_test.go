package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
//   - 段が立っていないのに計測経路へ入る（＝既定で計測ログが生える。条項 10）。
//   - **この仕組み自身の空振り 2 通り**——TestMain の入口分岐が効かない（子がテストを走らせる）／
//     入口を 1 度も走らせないまま全体が緑になる。
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
//   - **`-run` で絞ったときの空振り。** 下の非空振りの歯止めは、全体実行のときだけ効く。

// usageEntrypointEnv は、テストバイナリを**「本番の入口を 1 回走らせるだけのプロセス」**として
// 再実行するための目印。
//
// ⚠️ この名前が実環境に置かれていると `go test ./internal/cli` がテストではなく scholia を走らせる。
// そのときは cobra が `-test.timeout` 等を未知のフラグとして拒み **exit 1** になるので、
// 黙って全テストが緑になることはない。
const usageEntrypointEnv = "SCHOLIA_TEST_REAL_ENTRYPOINT"

// usageEntrypointRuns は、本番の入口を実際に走らせた回数。
//
// ⚠️ **不在の主張だけを積む検査は、規則が死んでいても緑になる。** この repo が繰り返している型で、
// この仕組みも例外ではない——入口を 1 度も走らせなくても、他が緑なら全体は緑になりうる。
// だから TestMain が走った回数を見て、0 回なら落とす。
var usageEntrypointRuns int

// TestMain は 2 つの顔を持つ。
//
//  1. usageEntrypointEnv が置かれていれば、**本番の入口を 1 回走らせて終わる**。
//     ここは `cmd/scholia/main.go` と同じことをする——⚠️ **写しであって、main の検査ではない。**
//  2. そうでなければ通常どおりテストを走らせ、**入口を 1 度も走らせずに緑になること**を拒む。
func TestMain(m *testing.M) {
	if _, ok := os.LookupEnv(usageEntrypointEnv); ok {
		// cmd/scholia/main.go と同じ: cobra がエラーを標準エラーへ出すので、ここは exit code だけ。
		if err := Execute(); err != nil {
			os.Exit(1)
		}
		os.Exit(0)
	}

	code := m.Run()
	if code == 0 && usageEntrypointRuns == 0 && usageEntrypointGuardRequired() {
		fmt.Fprintf(os.Stderr, `本番の入口（Execute）を 1 度も走らせないまま緑になった。
検査が消された・スキップされた・再実行が届いていないのいずれかである。
%s を参照。
`, "usage_entrypoint_test.go")
		code = 1
	}
	os.Exit(code)
}

// usageEntrypointGuardRequired は、非空振りの歯止めを効かせる実行かを返す。
//
// `-run` で 1 本だけ走らせたときにまで落とすと、開発中に単体で走らせられなくなる。
// CI は `go test ./...`（＝絞り込み無し）なので、そこでは必ず効く。
func usageEntrypointGuardRequired() bool {
	f := flag.Lookup("test.run")
	return f == nil || f.Value.String() == ""
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
	// 止めないと、この関数が孫プロセスを産み続ける（TestMain の入口分岐を外す変異で実際に起きる）。
	if _, ok := os.LookupEnv(usageEntrypointEnv); ok {
		t.Fatalf("%s が置かれているのにテストが走っている。TestMain の入口分岐が効いていない", usageEntrypointEnv)
	}

	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("テストバイナリのパスを取れない: %v", err)
	}

	env := map[string]string{usageEntrypointEnv: "1"}
	for k, v := range extra {
		env[k] = v
	}
	cmd := exec.Command(exe, args...)
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
	usageEntrypointRuns++
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

	res := runRealEntrypoint(t, map[string]string{usage.EnvVar: "detailed"}, "no-such-command")

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
