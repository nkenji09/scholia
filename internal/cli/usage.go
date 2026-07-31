package cli

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/nkenji09/scholia/internal/usage"
)

// usageCallerEnvVars / usageSessionEnvVars は、呼び出し元の名乗りとセッション識別子を読む環境変数。
//
// ⚠️ **写すだけで、AI か人かの判定はしない**（正本 条項 13）。
// これらの名前を置くのは特定のエージェントの実装であって、別のエージェントは別の名乗りをするか
// 何も置かない。名乗りが無ければ空のままにする——「無い」を「人」と読み替えない。
// 同じセッションから人が手で打った実行も同じ名乗りになる。
//
// どちらもプロジェクトを指さない（実行環境の名乗りである）ので、マスクでも残る。
var (
	usageCallerEnvVars  = []string{"CLAUDE_CODE_ENTRYPOINT", "AI_AGENT"}
	usageSessionEnvVars = []string{"CLAUDE_CODE_SESSION_ID"}
)

// usageProjectRoot は openStore が解決したプロジェクトルート。
// 「この起動がどのプロジェクトに対して動いたか」の唯一の出どころで、
// store を開かなかったコマンド（version 等）では空のままになる。
var usageProjectRoot string

// Execute is the CLI entrypoint called from cmd/scholia/main.go.
//
// ⚠️ **この 1 行の委譲そのものには検査が届いていない。** 中身（execute）は
// TestUsage_DefaultOffDoesNotEnterTheMeasuredPath 以下が値で見ているが、
// ここで渡す 5 つを書き換える変異は全部緑のまま通る。塞ぐには本物のバイナリを走らせる検査が要る。
func Execute() error {
	return execute(os.LookupEnv, usage.Record, nil, os.Stdout, os.Stderr)
}

// execute は Execute の中身を、**環境の読み方・記録の宛先・引数・出力先を渡せる形**に割ったもの。
//
// 収集レベルが立っているときだけ、標準出力・標準エラーを数える writer で包み、
// 起動ごとに 1 行を記録する（正本 01KYSKM4T0RWRY1N7407KZSZ17）。
//
// ⚠️ **オフ（既定）のときは計測経路に入らない**——writer も差し替えず、時刻も測らず、
// 観測も組み立てず、sink も呼ばない（条項 10）。
//
// この 4 つの引数を外から渡せるようにしたのは、**その不変を自動で検査するため**である。
// 割る前は `Execute()` が os の値を直に読んでいたので、off 分岐を丸ごと消す変異
// （レビュアの R-1）を当てても全部緑のままだった——「オフのときは何もしない」という
// 正本で最も重い不変に、自動ガードが 1 本も無かった。
// 検査は TestUsage_DefaultOffDoesNotEnterTheMeasuredPath（射程はそこに書いてある）。
func execute(lookup func(string) (string, bool), sink usage.Sink, args []string, stdout, stderr io.Writer) error {
	level, note := resolveUsageLevel(lookup)
	if note != "" {
		fmt.Fprint(stderr, note)
	}
	if level <= usage.Off {
		return newPlainRoot(args, stdout, stderr).Execute()
	}
	return executeWithUsage(level, sink, args, stdout, stderr)
}

// newPlainRoot は**計測をまったく入れない**実行経路の root コマンドを組み立てる。
//
// ⚠️ **渡された writer をそのまま cobra へ渡す**（包まない）。ここが条項 10 の実体で、
// 「包んでいないこと」は TestUsage_PlainRootHandsCobraTheWritersUnwrapped が値（同一性）で見ている。
//
// args が nil のときは cobra の既定（os.Args[1:]）を使う。テストだけが値を渡す。
func newPlainRoot(args []string, stdout, stderr io.Writer) *cobra.Command {
	root := newRootCmd()
	if args != nil {
		root.SetArgs(args)
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	return root
}

// resolveUsageLevel は環境変数から段を決め、必要なら標準エラーへ出す注記を返す。
//
// 未設定・空文字・未知の値はいずれもオフ（安全側）。ただし**設定されているのに解釈できない**
// ときだけ注記を返す——未設定のときに注記を出すと、既定の振る舞いが変わってしまう。
func resolveUsageLevel(lookup func(string) (string, bool)) (usage.Level, string) {
	raw, set := lookup(usage.EnvVar)
	if !set {
		return usage.Off, ""
	}
	level, ok := usage.ParseLevel(raw)
	if !ok {
		return usage.Off, usage.UnparsableNote(raw)
	}
	return level, ""
}

// executeWithUsage は段が立っているときの実行経路。
//
// 出力を数える点が 1 か所で足りるのは、全コマンドが cmd.OutOrStdout() に書き、
// cobra の getOut が自分の writer を持たないコマンドを親へ委ねるためである
// （root に差せば全コマンドを覆える。⚠️ 子コマンドが自前で SetOut したらこの前提は崩れる）。
//
// args が nil のときは cobra の既定（os.Args[1:]）を使う。テストだけが値を渡す。
func executeWithUsage(level usage.Level, sink usage.Sink, args []string, stdout, stderr io.Writer) error {
	start := time.Now()

	outCounter := usage.NewCountingWriter(stdout)
	errCounter := usage.NewCountingWriter(stderr)

	root := newRootCmd()
	if args != nil {
		root.SetArgs(args)
	}
	root.SetOut(outCounter)
	root.SetErr(errCounter)

	executed, err := root.ExecuteC()
	elapsed := time.Since(start)

	// ⚠️ ここから先で何が起きても、返すのは cobra の err だけである（条項 11）。
	sink(level, buildObservation(level, executed, err, elapsed, outCounter, errCounter))
	return err
}

// buildObservation は 1 起動分の観測を組み立てる。段による取捨はしない（usage.Records の仕事）。
func buildObservation(level usage.Level, executed *cobra.Command, err error, elapsed time.Duration, out, errw *usage.CountingWriter) usage.Observation {
	exit := 0
	if err != nil {
		exit = 1
	}
	shape := observeInvocation(executed)

	writeUs := out.Spent().Microseconds() + errw.Spent().Microseconds()
	restUs := elapsed.Microseconds() - writeUs
	if restUs < 0 {
		restUs = 0
	}

	return usage.Observation{
		Timestamp:     time.Now(),
		Command:       shape.command,
		FlagNames:     shape.flagNames,
		SelectorKind:  shape.selectorKind,
		ArgCount:      shape.argCount,
		ExitCode:      exit,
		StdoutBytes:   out.Bytes(),
		Duration:      elapsed,
		Caller:        firstEnv(usageCallerEnvVars),
		SessionID:     firstEnv(usageSessionEnvVars),
		ToolVersion:   resolveVersionInfo().Version,
		RecordIDs:     shape.recordIDs,
		ProjectRoot:   usageProjectRoot,
		FlagValues:    shape.flagValues,
		FreeTextLens:  shape.freeTextLens,
		StderrBytes:   errw.Bytes(),
		DurationParts: map[string]int64{"write": writeUs, "rest": restUs},
	}
}

// firstEnv は最初に見つかった非空の環境変数の値を返す。無ければ空文字。
func firstEnv(names []string) string {
	for _, n := range names {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}
	return ""
}
