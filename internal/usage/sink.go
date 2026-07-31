package usage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	// StateHomeEnv は置き場所の基点を上書きする環境変数（XDG の state ディレクトリ）。
	//
	// ⚠️ **これは計測を有効にする口ではない。** 段の指定は SCHOLIA_USAGE_LEVEL ただ 1 つで
	// （正本 条項 2）、こちらは OS 側に既にある口をそのまま尊重するだけである
	// ——scholia 固有の環境変数は増やさない（正本「本 decision では口を増やさない」）。
	StateHomeEnv = "XDG_STATE_HOME"

	// stateHomeFallback は StateHomeEnv が使えないときの基点（ホームからの相対）。
	//
	// ⚠️ **OS で分岐しない。** os.UserConfigDir / os.UserCacheDir は darwin と linux で
	// 別の場所を返すので、置き場所を検査するガードも OS で分岐することになる。
	// 規則を 1 つにするほうが、ガードを 1 本にできる。
	stateHomeFallbackA = ".local"
	stateHomeFallbackB = "state"

	// dirName / fileName は計測ログの置き場所。
	//
	// ⚠️ リポジトリの外の 1 ファイルであること（正本 条項 8）。
	// state（＝再起動をまたいで残すが、設定でもキャッシュでもないデータ）を選んだのは、
	// このログが**数か月かけて貯めて判断材料にする**前提だからである。
	// キャッシュ配下は OS が掃除しうるので、その前提と噛み合わない。
	// 一方このログは真実の源ではない——git-as-DB（DESIGN §1）は不変で、ログは捨てても
	// `.scholia/` は損なわれない。
	dirName  = "scholia"
	fileName = "usage.jsonl"
)

// DefaultPath は計測ログのパスを返す。
//
// 置き場所の規則は 1 つ:
//
//	<state>/scholia/usage.jsonl        <state> = $XDG_STATE_HOME もしくは $HOME/.local/state
//
// ⚠️ **`.scholia` という名前のディレクトリを作らない。** store.Discover は名前が `.scholia` の
// ディレクトリを**中身を見ずに**ストアとして拾うので、ホーム直下に作ると
// 「$HOME 配下の非プロジェクトから走らせた scholia が、それをストアとして開こうとする」
// 状態を**着地後もずっと**残すことになる（実測で確認した）。
//
// ⚠️ **オフのときはここへ来ない。** 呼ぶのは Record だけで、Record はオフを先に弾く（条項 10）。
// 環境が壊れていて基点が決まらないときはエラーを返し、Record がそれを飲み込んで何もしない
// ——**記録の失敗が本業を落とさない**（条項 11）。
func DefaultPath() (string, error) {
	base, err := stateHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName, fileName), nil
}

// stateHome は置き場所の基点を返す。
//
// ⚠️ **$XDG_STATE_HOME が相対パスのときは無効として無視する**（XDG の規定どおり）。
// これは作法の問題ではなく条項 8 の問題である——相対パスをそのまま使うと、
// 基点が cwd になり、**ログがプロジェクトの中に落ちる**。
func stateHome() (string, error) {
	if v := os.Getenv(StateHomeEnv); filepath.IsAbs(v) {
		return v, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("計測ログの置き場所を決められません（%s も HOME も使えません）: %w", StateHomeEnv, err)
	}
	return filepath.Join(home, stateHomeFallbackA, stateHomeFallbackB), nil
}

// AppendLine は 1 行を追記する。
//
// 追記専用で開き、行全体を 1 回の Write で渡す——並行して走る scholia どうしで行が混ざらないため。
func AppendLine(path string, line []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Sink は 1 起動分の観測を記録する宛先。既定は Record ただ 1 つで、テストだけが差し替える。
//
// ⚠️ **この型を置いた理由は、「オフのときは計測経路に入らない」を自動で検査できるようにするため**である
// （正本 条項 10）。Record 自身がオフを弾くので、オフで Record を呼んでもファイルは増えない
// ——つまり「ファイルが作られないこと」だけでは、オフで計測経路を通ってしまう変異を捕まえられない。
// 呼び出し側が sink を差し替えられれば、**呼ばれたかどうか**を値で見られる。
type Sink func(Level, Observation)

// Record は 1 起動分の観測を記録する。
//
// ⚠️ **記録の失敗は本業を落とさない**（正本 条項 11）。書けなくてもコマンドは成功し、
// exit code は変わらない。行の組み立てが壊れても、置き場所が決まらなくても、
// 書き込みが失敗しても、panic が起きても、ここで止める。
//
// オフのときは何もしない——観測しない・書かない（条項 10）。
func Record(l Level, obs Observation) {
	defer func() {
		// 計測の欠陥で本業を落とすことだけは避ける。飲み込んだことは誰にも伝えない
		// ——標準エラーに何か書けば、それ自体が「既定で何も変わらない」を壊す方向へ効く。
		_ = recover()
	}()
	if l <= Off {
		return
	}
	line, err := Line(l, obs)
	if err != nil {
		return
	}
	path, err := DefaultPath()
	if err != nil {
		return
	}
	_ = AppendLine(path, line)
}

// UnparsableNote は、環境変数が設定されているのに解釈できないときに標準エラーへ出す 1 行を返す。
//
// ⚠️ **未設定のときはこれを呼ばない**（既定の振る舞いを 1 文字も変えないため）。
// 黙ってオフに倒さないのは、「オフだと思っていない人がオフで走り続けるのが一番害が大きい」ため。
func UnparsableNote(raw string) string {
	return fmt.Sprintf("注記: %s=%q は解釈できません（%s のいずれか）。計測はオフで実行します。\n",
		EnvVar, raw, strings.Join(LevelNames(), "|"))
}

// CountingWriter は下流へ素通ししながら、渡したバイト数と Write に掛かった時間を数える。
//
// ⚠️ **素通しであること**が要点である。返す n と err は下流のものをそのまま返し、
// 内容にも境界にも触らない——計測を入れたせいで出力が変わってはいけない。
type CountingWriter struct {
	w     io.Writer
	bytes int64
	spent time.Duration
}

// NewCountingWriter は w への書き込みを数える writer を返す。
func NewCountingWriter(w io.Writer) *CountingWriter { return &CountingWriter{w: w} }

func (c *CountingWriter) Write(p []byte) (int, error) {
	start := time.Now()
	n, err := c.w.Write(p)
	c.spent += time.Since(start)
	if n > 0 {
		c.bytes += int64(n)
	}
	return n, err
}

// Bytes は下流が受け取ったバイト数を返す。
func (c *CountingWriter) Bytes() int64 { return c.bytes }

// Spent は Write に費やした時間の合計を返す（所要の内訳に使う）。
func (c *CountingWriter) Spent() time.Duration { return c.spent }
