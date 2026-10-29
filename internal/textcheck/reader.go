// Package textcheck は、記録の文を、経緯を知らない読み手の AI に読ませ、
// 読む人によって意味が分かれうる言葉の候補を受け取る。
//
// 読み手は利用者の環境変数で指定した実行ファイル 1 つで、入出力の形は telos と同じにしてある
// （同じ読み手の実行ファイルを両方で使えるようにするため）。
// 指定はリポジトリの設定からは読まない。他人のリポジトリでコマンドを実行しただけで、
// 利用者の知らない実行ファイルが走らないようにするため。
package textcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Variable は、読み手を指定する環境変数の名前。
const Variable = "SCHOLIA_TEXT_READER"

// DefaultTimeout は、読み手を待つ上限。利用者が変える口は持たない。
const DefaultTimeout = 120 * time.Second

// AnswerFormat は、読み手に答えの形を頼む固定の文。問いに左右されない。
const AnswerFormat = `答えは JSON の object 一つだけで返してください。前後に説明や記号を付けないでください。
object は candidates の並びを持ちます。candidates の各件は、key（渡した key のどれか）、phrase（引っかかった言葉か、文の引用）、reason（なぜわかりにくいか）の三つの文字列を持ちます。
わかりにくい所が無ければ、candidates を空の並びにしてください。`

// Text は、記録の文 1 欄。
type Text struct {
	Field string `json:"field"`
	Text  string `json:"text"`
}

// Record は、読み手に渡す記録 1 件。ID が読み手に渡す key になる。
type Record struct {
	ID    string
	Texts []Text
}

// Candidate は、読み手が挙げた、わかりにくい言葉 1 件。
type Candidate struct {
	Key    string `json:"key"`
	Phrase string `json:"phrase"`
	Reason string `json:"reason"`
}

// NotChecked は、確かめが行われなかった部分。
type NotChecked struct {
	IDs    []string `json:"ids"`
	Reason string   `json:"reason"`
}

// Result は読ませた結果。NoReader は読み手の指定が無かったことを表す。
// NotChecked があれば確かめは行われていない（候補は空）。
type Result struct {
	NoReader   bool
	Candidates []Candidate
	NotChecked []NotChecked
}

// Checked は、読み手が答えを返し、確かめが終わったことを返す。
func (r Result) Checked() bool { return !r.NoReader && len(r.NotChecked) == 0 }

// Reader は、環境変数で指定された実行ファイルを子のプロセスとして起動する。
type Reader struct {
	// Stderr は、読み手の標準エラーの流し先。nil なら捨てる。
	Stderr io.Writer
	// Timeout は、読み手を待つ上限。0 なら DefaultTimeout。
	Timeout time.Duration
}

// Named は、指定された読み手を環境変数から変えずに返す。指定が無ければ空。
func (Reader) Named() string { return os.Getenv(Variable) }

type request struct {
	Question     string          `json:"question"`
	AnswerFormat string          `json:"answerFormat"`
	Records      []requestRecord `json:"records"`
}

type requestRecord struct {
	Key   string `json:"key"`
	Texts []Text `json:"texts"`
}

// Read は、問いと記録の文を読み手の標準入力に渡し、標準出力の候補を受け取る。
// 読み手には、Variable だけを抜いた環境を引き継ぐ。失敗は誤りにせず、当たった理由を 1 つ言う、確かめが行われなかった結果で答える。
func (r Reader) Read(question string, records []Record) Result {
	command := r.Named()
	if command == "" {
		return Result{NoReader: true}
	}
	// 相対パスは撃った場所から解かれ、開いたリポジトリの中のファイルが走るので、起動しない。
	if strings.ContainsAny(command, "/"+string(filepath.Separator)) && !filepath.IsAbs(command) {
		return notChecked(records, "読み手の指定が名前か絶対パスでないので起動しなかった")
	}
	req := request{Question: question, AnswerFormat: AnswerFormat, Records: make([]requestRecord, 0, len(records))}
	for _, rec := range records {
		req.Records = append(req.Records, requestRecord{Key: rec.ID, Texts: rec.Texts})
	}
	stdin, err := json.Marshal(req)
	if err != nil {
		return notChecked(records, "読み手に渡す入力を作れなかった: "+firstLine(err.Error()))
	}
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, command)
	// 読み手が中で scholia を呼んでも、同じ読み手が呼ばれ続けないようにする。
	cmd.Env = environWithout(Variable)
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if r.Stderr != nil {
		// 包んで管にする。読み手が起こしたプロセスが標準エラーそのものを受け継ぐと、
		// 上限で読み手を止めた後も握り続け、呼んだ側がその終わりを待つため。
		cmd.Stderr = writerOnly{r.Stderr}
	}
	// 上限か読み手の終わりから 1 秒で管を閉じ、読み手が起こしたプロセスを待たない。
	cmd.WaitDelay = time.Second
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.Is(err, exec.ErrWaitDelay):
		return notChecked(records, fmt.Sprintf("読み手は終わったが、読み手が起こしたプロセスが %v のうちに出力を閉じなかった", cmd.WaitDelay))
	case err != nil && ctx.Err() == context.DeadlineExceeded:
		return notChecked(records, fmt.Sprintf("上限の %v までに終わらなかった", timeout))
	case errors.As(err, &exit):
		return notChecked(records, fmt.Sprintf("終了コード %d で終わった", exit.ExitCode()))
	case err != nil:
		return notChecked(records, "起動できなかった: "+firstLine(err.Error()))
	}
	return parse(stdout.Bytes(), records)
}

// parse は、読み手の標準出力を、candidates を持つ JSON の object として読む。
// 各件が key、phrase、reason の空白だけでない文字列を持ち、key が渡した記録のどれかであるものだけを確かめた結果にする。
func parse(stdout []byte, records []Record) Result {
	const badShape = "標準出力が candidates の形でない（candidates の各件が key、phrase、reason の文字列を持つ JSON の object を期待する）"
	var answer struct {
		Candidates *[]struct {
			Key    *string `json:"key"`
			Phrase *string `json:"phrase"`
			Reason *string `json:"reason"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(stdout), &answer); err != nil || answer.Candidates == nil {
		return notChecked(records, badShape)
	}
	sent := map[string]bool{}
	for _, rec := range records {
		sent[rec.ID] = true
	}
	candidates := make([]Candidate, 0, len(*answer.Candidates))
	for _, c := range *answer.Candidates {
		if blank(c.Key) || blank(c.Phrase) || blank(c.Reason) {
			return notChecked(records, badShape)
		}
		if !sent[*c.Key] {
			return notChecked(records, fmt.Sprintf("渡していない key %q の候補が返った", *c.Key))
		}
		candidates = append(candidates, Candidate{Key: *c.Key, Phrase: *c.Phrase, Reason: *c.Reason})
	}
	return Result{Candidates: candidates}
}

func blank(s *string) bool { return s == nil || strings.TrimSpace(*s) == "" }

// notChecked は、渡した全件の ID をその部分とする、確かめが行われなかった結果を作る。
func notChecked(records []Record, reason string) Result {
	ids := make([]string, 0, len(records))
	for _, rec := range records {
		ids = append(ids, rec.ID)
	}
	return Result{NotChecked: []NotChecked{{IDs: ids, Reason: reason}}}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// environWithout は、自分の環境から name だけを抜いたものを返す。
func environWithout(name string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if !strings.HasPrefix(kv, name+"=") {
			env = append(env, kv)
		}
	}
	return env
}

// writerOnly は、書き込みだけを見せる包み（exec が *os.File を直接渡さず管を作るようにする）。
type writerOnly struct{ w io.Writer }

func (o writerOnly) Write(p []byte) (int, error) { return o.w.Write(p) }
