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
	// dirName / fileName は計測ログの置き場所。
	//
	// ⚠️ リポジトリの外の 1 ファイルであること（正本 条項 8）。ユーザのキャッシュ配下に置くのは、
	// このログが真実の源ではなく**捨ててよい副産物**だからである（DESIGN §1 git-as-DB は不変）。
	dirName  = "scholia"
	fileName = "usage.jsonl"
)

// DefaultPath は計測ログのパスを返す。
//
// os.UserCacheDir は環境（HOME / XDG_CACHE_HOME）から決まるので、テストは環境を差し替えて確かめられる。
func DefaultPath() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, dirName, fileName), nil
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
