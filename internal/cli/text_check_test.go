package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// 読み手を呼ぶ 2 つのコマンド（decide --dry-run・text-check）の受け入れ検査（nkenji09/scholia#5）。
// 読み手は sh の台本を置いた偽物で、本物の AI は呼ばない。環境変数を使うので、並列にしない。

const readerVar = "SCHOLIA_TEXT_READER"

type fakeReaderT struct {
	path, received, calls, env string
}

// fakeReader は、呼ばれるたびに calls へ 1 行足し、標準入力を received へ、環境を env へ書き、answer を標準出力へ返す読み手を置き、環境変数で指定する。
func fakeReader(t *testing.T, answer string, exit int) fakeReaderT {
	t.Helper()
	dir := t.TempDir()
	f := fakeReaderT{path: filepath.Join(dir, "reader"), received: filepath.Join(dir, "received"), calls: filepath.Join(dir, "calls"), env: filepath.Join(dir, "env")}
	script := "#!/bin/sh\necho called >> '" + f.calls + "'\ncat > '" + f.received + "'\nenv > '" + f.env + "'\n" +
		"cat <<'ANSWER_EOF'\n" + answer + "\nANSWER_EOF\nexit " + string(rune('0'+exit)) + "\n"
	if err := os.WriteFile(f.path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv(readerVar, f.path)
	return f
}

func (f fakeReaderT) callCount(t *testing.T) int {
	t.Helper()
	b, err := os.ReadFile(f.calls)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(b), "called")
}

type sentRecord struct {
	Key   string `json:"key"`
	Texts []struct {
		Field string `json:"field"`
		Text  string `json:"text"`
	} `json:"texts"`
}

func (f fakeReaderT) sent(t *testing.T) (question string, records []sentRecord) {
	t.Helper()
	b, err := os.ReadFile(f.received)
	if err != nil {
		t.Fatalf("読み手が呼ばれていない: %v", err)
	}
	var doc struct {
		Question string       `json:"question"`
		Records  []sentRecord `json:"records"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatalf("標準入力が JSON でない: %v\n%s", err, b)
	}
	return doc.Question, doc.Records
}

func sentKeys(records []sentRecord) []string {
	keys := []string{}
	for _, r := range records {
		keys = append(keys, r.Key)
	}
	sort.Strings(keys)
	return keys
}

const noCandidates = `{"candidates":[]}`

// seedTextStore は、tag t1・vocab 3 件・transition T-1・decision 1 件を持つ store を作り、decision の id を返す。
func seedTextStore(t *testing.T) (dir, decisionID string) {
	t.Helper()
	t.Setenv(readerVar, "")
	dir = t.TempDir()
	gitInitT(t, dir)
	steps := [][]string{
		{"init"},
		{"tag", "create", "t1", "--name", "検索の入口", "--kind", "concern", "--desc", "新しい方を開く"},
		{"vocab", "add", "condition", "cond.a", "--label", "古い条件", "--description", "条件の説明"},
		{"vocab", "add", "action", "act.a", "--label", "操作"},
		{"vocab", "add", "effect", "eff.a", "--label", "効果"},
		{"tx", "add", "T-1", "--action", "act.a", "--given", "cond.a", "--then", "eff.a"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v: %v\n%s", s, err, out)
		}
	}
	out, err := run(t, dir, "decide", "--on", "tag:t1", "--why", "# 見出し\n\n既存の理由", "--changed", "既存の変更", "--json")
	if err != nil {
		t.Fatalf("decide: %v\n%s", err, out)
	}
	var env struct {
		Record struct {
			ID string `json:"id"`
		} `json:"record"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatal(err)
	}
	return dir, env.Record.ID
}

const dryArgsWhy = "# 見出し\n\n新しい方を開く"

func dryRunArgs(extra ...string) []string {
	return append([]string{"decide", "--on", "tag:t1", "--why", dryArgsWhy, "--changed", "検索を撃つ", "--dry-run"}, extra...)
}

func savedDecisionCount(t *testing.T, dir string) int {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, ".scholia", "decisions", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	return len(files)
}

// 受け入れ 1: decide --dry-run が why と changed を 1 回だけ読み手に渡し、候補を「確かめ: 」の行で出す。保存しない。
func TestTextCheck_DecideDryRunSendsWhyAndChanged(t *testing.T) {
	dir, _ := seedTextStore(t)
	before := savedDecisionCount(t, dir)
	f := fakeReader(t, `{"candidates":[{"key":"new","phrase":"新しい方","reason":"何と比べるか不明"}]}`, 0)
	out, err := run(t, dir, dryRunArgs()...)
	if err != nil {
		t.Fatalf("dry-run: %v\n%s", err, out)
	}
	if f.callCount(t) != 1 {
		t.Fatalf("読み手は 1 回だけ呼ばれるはず: %d", f.callCount(t))
	}
	question, recs := f.sent(t)
	if !strings.Contains(question, "何も知らない読み手") {
		t.Errorf("既定の問いが渡るはず: %q", question)
	}
	if len(recs) != 1 || recs[0].Key != "new" || len(recs[0].Texts) != 2 ||
		recs[0].Texts[0].Field != "why" || recs[0].Texts[0].Text != dryArgsWhy ||
		recs[0].Texts[1].Field != "changed" || recs[0].Texts[1].Text != "検索を撃つ" {
		t.Errorf("records は key=new の why と changed だけのはず: %+v", recs)
	}
	for _, want := range []string{"確かめ: 問いは 既定", "確かめ: new 「新しい方」 何と比べるか不明", "advisory: なし"} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に %q が無い:\n%s", want, out)
		}
	}
	// 確かめの行は advisory の後に出る。
	if strings.Index(out, "確かめ: new") < strings.Index(out, "advisory: なし") {
		t.Errorf("確かめの行は advisory の後のはず:\n%s", out)
	}
	if savedDecisionCount(t, dir) != before {
		t.Error("dry-run は保存しないはず")
	}
}

// 受け入れ 1・5: dry-run は、候補があっても読み手が失敗しても終了コードを変えない（0 のまま）。
func TestTextCheck_DecideDryRunExitCodeIsUnchanged(t *testing.T) {
	dir, _ := seedTextStore(t)
	for _, c := range []struct {
		name, answer string
		exit         int
	}{
		{"candidates", `{"candidates":[{"key":"new","phrase":"x","reason":"y"}]}`, 0},
		{"reader-failed", "", 3},
	} {
		fakeReader(t, c.answer, c.exit)
		out, err := run(t, dir, dryRunArgs()...)
		if err != nil {
			t.Errorf("%s: dry-run は 0 のまま: %v\n%s", c.name, err, out)
		}
		if c.name == "reader-failed" && !strings.Contains(out, "確かめ: 行われなかった（終了コード 3 で終わった） new") {
			t.Errorf("行われなかった行が出るはず:\n%s", out)
		}
	}
}

// 受け入れ 2: 指定ありの tx add/edit・tag edit・vocab edit は読み手を呼ばない（本番の decide は読み手を呼ぶ。下の Decide 系）。
func TestTextCheck_WritesDoNotCallTheReader(t *testing.T) {
	dir, _ := seedTextStore(t)
	f := fakeReader(t, noCandidates, 0)
	steps := [][]string{
		{"tx", "add", "T-2", "--action", "act.a", "--then", "eff.a"},
		{"tx", "edit", "T-2", "--given", "cond.a"},
		{"tag", "edit", "t1", "--desc", "新しい方を開く（直した）"},
		{"vocab", "edit", "cond.a", "--description", "古い方の説明"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v: %v\n%s", s, err, out)
		} else if strings.Contains(out, "確かめ:") {
			t.Errorf("%v: 確かめの行は出ないはず:\n%s", s, out)
		}
	}
	if n := f.callCount(t); n != 0 {
		t.Errorf("読み手は呼ばれないはずが %d 回", n)
	}
}

// 受け入れ 3: 指定なしの decide --dry-run は今と同じ出力で、確かめの行が無い。
func TestTextCheck_DecideDryRunWithoutReaderIsUnchanged(t *testing.T) {
	dir, _ := seedTextStore(t)
	t.Setenv(readerVar, "")
	out, err := run(t, dir, dryRunArgs()...)
	if err != nil {
		t.Fatal(err)
	}
	want := "dry-run: decision は保存していません（tag:t1 への advisory プレビュー）\nadvisory: なし（このまま decide してよい）\n"
	if out != want {
		t.Errorf("出力が今と違う:\n%q\n%q", out, want)
	}
}

// 受け入れ 4: text-check は指定した記録の文の欄を渡す。transition には文の欄が無いので、実在の確認だけで渡さない。
func TestTextCheck_NamedRecordsSendTheirTextFields(t *testing.T) {
	dir, did := seedTextStore(t)
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, "text-check", "tag:t1", "vocab:cond.a", "decision:"+did, "tx:T-1")
	if err != nil {
		t.Fatalf("text-check: %v\n%s", err, out)
	}
	_, recs := f.sent(t)
	if !reflect.DeepEqual(sentKeys(recs), []string{did, "cond.a", "t1"}) {
		t.Fatalf("渡した記録: %v", sentKeys(recs))
	}
	fields := map[string]map[string]string{}
	for _, r := range recs {
		fields[r.Key] = map[string]string{}
		for _, x := range r.Texts {
			fields[r.Key][x.Field] = x.Text
		}
	}
	if fields["t1"]["name"] != "検索の入口" || fields["t1"]["description"] != "新しい方を開く" ||
		fields["cond.a"]["label"] != "古い条件" || fields["cond.a"]["description"] != "条件の説明" ||
		fields[did]["why"] != "# 見出し\n\n既存の理由" || fields[did]["changed"] != "既存の変更" {
		t.Errorf("文の欄: %v", fields)
	}
	for _, want := range []string{"確かめ: 問いは 既定", "確かめ: わかりにくい表現の候補は無い"} {
		if !strings.Contains(out, want) {
			t.Errorf("出力に %q が無い:\n%s", want, out)
		}
	}
}

// 受け入れ 4: --base は、ref 以後に足された・文が変わった記録だけを渡し、文の変わらない記録は渡さない。
func TestTextCheck_BaseSendsOnlyAddedOrTextChangedRecords(t *testing.T) {
	dir, _ := seedTextStore(t)
	gitCommitAllT(t, dir, "seed")
	for _, s := range [][]string{
		{"tag", "edit", "t1", "--desc", "変えた説明"},                                  // 文が変わる
		{"tag", "create", "t2", "--name", "足したタグ", "--kind", "concern"},           // 足される
		{"vocab", "edit", "act.a", "--ref", "https://example.com/spec"},           // 文でない欄だけ変わる
		{"tx", "add", "T-9", "--action", "act.a", "--then", "eff.a"},              // transition は文が無い
		{"decide", "--on", "tag:t1", "--why", "# 新しい決定\n\n理由", "--changed", "変更"}, // 足される decision
	} {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v: %v\n%s", s, err, out)
		}
	}
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, "text-check", "--base", "HEAD")
	if err != nil {
		t.Fatalf("text-check --base: %v\n%s", err, out)
	}
	_, recs := f.sent(t)
	keys := sentKeys(recs)
	if len(keys) != 3 || !containsStr(keys, "t1") || !containsStr(keys, "t2") {
		t.Fatalf("t1・t2・新しい decision の 3 件だけのはず: %v", keys)
	}
	for _, k := range keys {
		if k == "act.a" || k == "cond.a" || k == "T-9" || k == "T-1" {
			t.Errorf("文の変わらない記録は渡さないはず: %s", k)
		}
	}
}

// 受け入れ 5: text-check の終了コード。確かめ終えたら 0、行われなかったら 1。
func TestTextCheck_ExitCodes(t *testing.T) {
	cases := []struct {
		name, answer string
		exit         int
		wantErr      bool
		wantLine     string
	}{
		{"candidates", `{"candidates":[{"key":"t1","phrase":"新しい方","reason":"何と比べるか不明"}]}`, 0, false, "確かめ: t1 「新しい方」 何と比べるか不明"},
		{"no-candidates", noCandidates, 0, false, "確かめ: わかりにくい表現の候補は無い"},
		{"non-zero-exit", "", 3, true, "確かめ: 行われなかった（終了コード 3 で終わった） t1"},
		{"bad-shape", `{"nope":1}`, 0, true, "確かめ: 行われなかった（標準出力が candidates の形でない"},
		{"unsent-key", `{"candidates":[{"key":"zzz","phrase":"x","reason":"y"}]}`, 0, true, `渡していない key "zzz" の候補が返った`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir, _ := seedTextStore(t)
			fakeReader(t, c.answer, c.exit)
			out, err := run(t, dir, "text-check", "tag:t1")
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v（エラー有無の期待 %v）\n%s", err, c.wantErr, out)
			}
			if !strings.Contains(out, c.wantLine) {
				t.Errorf("出力に %q が無い:\n%s", c.wantLine, out)
			}
		})
	}
	t.Run("no-reader", func(t *testing.T) {
		dir, _ := seedTextStore(t)
		t.Setenv(readerVar, "")
		out, err := run(t, dir, "text-check", "tag:t1")
		if err == nil || !strings.Contains(out, "確かめ: 読み手が指定されていない") {
			t.Errorf("指定が無ければ 1 と専用の行: err=%v\n%s", err, out)
		}
	})
	t.Run("missing-record-rejects-all", func(t *testing.T) {
		dir, _ := seedTextStore(t)
		f := fakeReader(t, noCandidates, 0)
		out, err := run(t, dir, "text-check", "tag:t1", "tag:no-such", "vocab:no-such-vocab")
		if err == nil || !strings.Contains(out+err.Error(), "no-such") {
			t.Errorf("無い id を出して拒むはず: err=%v\n%s", err, out)
		}
		if f.callCount(t) != 0 {
			t.Error("名指しの記録が無ければ読み手を呼ばないはず")
		}
	})
	t.Run("no-target-is-usage-error", func(t *testing.T) {
		dir, _ := seedTextStore(t)
		f := fakeReader(t, noCandidates, 0)
		if _, err := run(t, dir, "text-check"); err == nil {
			t.Error("記録も --base も無ければ使い方の誤り")
		}
		if f.callCount(t) != 0 {
			t.Error("読み手を呼ばないはず")
		}
	})
}

// 受け入れ 6: 読み手の環境に SCHOLIA_TEXT_READER が無い。
func TestTextCheck_ReaderEnvironmentLacksTheVariable(t *testing.T) {
	dir, _ := seedTextStore(t)
	f := fakeReader(t, noCandidates, 0)
	if out, err := run(t, dir, "text-check", "tag:t1"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	b, err := os.ReadFile(f.env)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), readerVar+"=") {
		t.Errorf("読み手の環境に %s が無いはず", readerVar)
	}
}

// 受け入れ 7: readerQuestion があればその文が question に入り「問いは config」が出る。空白だけの値は拒む。
func TestTextCheck_ReaderQuestionFromConfig(t *testing.T) {
	dir, _ := seedTextStore(t)
	if out, err := run(t, dir, "config", "set", "readerQuestion", "この文で迷う語を挙げて"); err != nil {
		t.Fatalf("config set: %v\n%s", err, out)
	}
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, "text-check", "tag:t1")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if q, _ := f.sent(t); q != "この文で迷う語を挙げて" {
		t.Errorf("config の問いが渡るはず: %q", q)
	}
	if !strings.Contains(out, "確かめ: 問いは config") {
		t.Errorf("問いの出どころの行:\n%s", out)
	}
	out, err = run(t, dir, dryRunArgs()...)
	if err != nil || !strings.Contains(out, "確かめ: 問いは config") {
		t.Errorf("dry-run でも config の問い: %v\n%s", err, out)
	}
}

func TestTextCheck_WhitespaceOnlyReaderQuestionIsRejected(t *testing.T) {
	dir, _ := seedTextStore(t)
	if out, err := run(t, dir, "config", "set", "readerQuestion", " \n\t "); err == nil {
		t.Errorf("空白だけの値は書けないはず:\n%s", out)
	}
	// 手で書かれた値は、読み込みの検査で拒む。
	path := filepath.Join(dir, ".scholia", "config.json")
	b, _ := os.ReadFile(path)
	var cfg map[string]any
	_ = json.Unmarshal(b, &cfg)
	cfg["readerQuestion"] = "   "
	nb, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, nb, 0o644); err != nil {
		t.Fatal(err)
	}
	f := fakeReader(t, noCandidates, 0)
	if out, err := run(t, dir, "text-check", "tag:t1"); err == nil {
		t.Errorf("読み込んだ空白だけの値は拒むはず:\n%s", out)
	}
	if f.callCount(t) != 0 {
		t.Error("読み手を呼ばないはず")
	}
}

// 受け入れ 8: --json の textCheck の欄は decide --dry-run と text-check にだけ出る。
func TestTextCheck_JSONFieldOnlyOnTheTwoCommands(t *testing.T) {
	dir, _ := seedTextStore(t)
	fakeReader(t, `{"candidates":[{"key":"new","phrase":"新しい方","reason":"何と比べるか不明"}]}`, 0)
	out, err := run(t, dir, dryRunArgs("--json")...)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var env struct {
		Record     map[string]any `json:"record"`
		Advisories []any          `json:"advisories"`
		DryRun     bool           `json:"dryRun"`
		TextCheck  struct {
			Candidates []struct {
				Key, Phrase, Reason string
			} `json:"candidates"`
			NotChecked []any  `json:"notChecked"`
			Question   string `json:"question"`
		} `json:"textCheck"`
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("JSON でない: %v\n%s", err, out)
	}
	if !env.DryRun || len(env.TextCheck.Candidates) != 1 || env.TextCheck.Candidates[0].Phrase != "新しい方" || env.TextCheck.Question != "既定" {
		t.Errorf("dry-run の textCheck: %+v", env.TextCheck)
	}

	fakeReader(t, `{"candidates":[{"key":"t1","phrase":"新しい方","reason":"何と比べるか不明"}]}`, 0)
	out, err = run(t, dir, "text-check", "tag:t1", "--json")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var tc map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &tc); err != nil || tc["textCheck"] == nil {
		t.Errorf("text-check --json にも textCheck が出るはず: %v\n%s", err, out)
	}

	// 共有の封筒を使うほかの書き込みコマンドの形は変えない。
	for _, s := range [][]string{
		{"tx", "add", "T-3", "--action", "act.a", "--then", "eff.a", "--json"},
		{"tag", "edit", "t1", "--desc", "変えた", "--json"},
		{"decide", "--on", "tag:t1", "--why", "# 本番\n\n理由", "--accept-text-candidates", "--json"},
	} {
		out, err := run(t, dir, s...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", s, err, out)
		}
		var m map[string]json.RawMessage
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatal(err)
		}
		if _, has := m["textCheck"]; has {
			t.Errorf("%v: textCheck は出ないはず", s)
		}
		if _, has := m["record"]; !has {
			t.Errorf("%v: record は今のまま出るはず", s)
		}
	}
}

// 受け入れ 9: 相対パスの指定は起動しない。
func TestTextCheck_RelativeReaderIsNotLaunched(t *testing.T) {
	dir, _ := seedTextStore(t)
	mark := filepath.Join(t.TempDir(), "started")
	if err := os.WriteFile(filepath.Join(dir, "reader"), []byte("#!/bin/sh\necho started > '"+mark+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Setenv(readerVar, "./reader")
	out, err := run(t, dir, "text-check", "tag:t1")
	if err == nil || !strings.Contains(out, "読み手の指定が名前か絶対パスでないので起動しなかった") {
		t.Errorf("起動せず 1: err=%v\n%s", err, out)
	}
	out, err = run(t, dir, dryRunArgs()...)
	if err != nil || !strings.Contains(out, "読み手の指定が名前か絶対パスでないので起動しなかった") {
		t.Errorf("dry-run でも起動しない（終了コードは 0）: err=%v\n%s", err, out)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("相対パスの指定は起動しないはず")
	}
}

func realDecideArgs(extra ...string) []string {
	return append([]string{"decide", "--on", "tag:t1", "--why", dryArgsWhy, "--changed", "検索を撃つ"}, extra...)
}

// 受け入れ 2: 指定ありの本番 decide は読み手を 1 回呼び、候補があれば保存せず終了コード 1。拒みの文に候補と打ち直しの手順を載せる。
func TestTextCheck_DecideBlocksOnCandidates(t *testing.T) {
	dir, _ := seedTextStore(t)
	before := savedDecisionCount(t, dir)
	f := fakeReader(t, `{"candidates":[{"key":"new","phrase":"新しい方","reason":"何と比べるか不明"},{"key":"new","phrase":"検索を撃つ","reason":"比喩"}]}`, 0)
	for _, extra := range [][]string{nil, {"--json"}} {
		out, err := run(t, dir, realDecideArgs(extra...)...)
		if err == nil {
			t.Fatalf("候補があれば止まる（1）はず:\n%s", out)
		}
		for _, want := range []string{"text-check-decide-blocked", "new 「新しい方」 何と比べるか不明", "new 「検索を撃つ」 比喩",
			"直すと決めた所だけ直し、直した後は --accept-text-candidates を付けて decide を打ち直す。直した文をもう一度読ませない（読み手は言い換えるたびに別の候補を出すため）"} {
			if !strings.Contains(out, want) {
				t.Errorf("%v: 拒みに %q が無い:\n%s", extra, want, out)
			}
		}
		if savedDecisionCount(t, dir) != before {
			t.Error("止めたときは保存しないはず")
		}
	}
	if n := f.callCount(t); n != 2 {
		t.Errorf("1 回の decide で 1 回読むはずが合計 %d 回", n)
	}
}

// 候補が無ければそのまま保存し、「候補は無い」を出す。
func TestTextCheck_DecideSavesWhenThereAreNoCandidates(t *testing.T) {
	dir, _ := seedTextStore(t)
	before := savedDecisionCount(t, dir)
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, realDecideArgs()...)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	if !strings.Contains(out, "確かめ: わかりにくい表現の候補は無い") || !strings.Contains(out, "を記録しました") {
		t.Errorf("出力:\n%s", out)
	}
	if savedDecisionCount(t, dir) != before+1 || f.callCount(t) != 1 {
		t.Error("保存され、読み手は 1 回だけ呼ばれるはず")
	}
	_, recs := f.sent(t)
	if len(recs) != 1 || recs[0].Key != "new" || len(recs[0].Texts) != 2 {
		t.Errorf("why と changed を key=new で渡すはず: %+v", recs)
	}
}

// 読み手が失敗したときは止めずに保存し、行われなかった行を出す。
func TestTextCheck_DecideSavesWhenTheReaderFails(t *testing.T) {
	dir, _ := seedTextStore(t)
	before := savedDecisionCount(t, dir)
	fakeReader(t, "", 3)
	out, err := run(t, dir, realDecideArgs()...)
	if err != nil || !strings.Contains(out, "確かめ: 行われなかった（終了コード 3 で終わった） new") {
		t.Fatalf("止めずに保存するはず: %v\n%s", err, out)
	}
	t.Setenv(readerVar, "./relative-reader")
	out, err = run(t, dir, realDecideArgs()...)
	if err != nil || !strings.Contains(out, "確かめ: 行われなかった（読み手の指定が名前か絶対パスでないので起動しなかった） new") {
		t.Fatalf("相対パスでも止めずに保存するはず: %v\n%s", err, out)
	}
	if savedDecisionCount(t, dir) != before+2 {
		t.Error("2 件とも保存されるはず")
	}
}

// --accept-text-candidates は読み手を呼ばずに保存する。dry-run に付ければ読み手を呼ばずに下見する。
func TestTextCheck_AcceptFlagSkipsTheReader(t *testing.T) {
	dir, _ := seedTextStore(t)
	before := savedDecisionCount(t, dir)
	f := fakeReader(t, `{"candidates":[{"key":"new","phrase":"x","reason":"y"}]}`, 0)
	const line = "確かめ: 候補を受け入れて読み手を呼ばなかった"
	out, err := run(t, dir, realDecideArgs("--accept-text-candidates")...)
	if err != nil || !strings.Contains(out, line) || savedDecisionCount(t, dir) != before+1 {
		t.Fatalf("読み手を呼ばずに保存するはず: %v\n%s", err, out)
	}
	out, err = run(t, dir, dryRunArgs("--accept-text-candidates")...)
	if err != nil || !strings.Contains(out, line) || savedDecisionCount(t, dir) != before+1 {
		t.Fatalf("dry-run に付けても読み手を呼ばず、保存しないはず: %v\n%s", err, out)
	}
	if f.callCount(t) != 0 {
		t.Error("読み手は呼ばれないはず")
	}
}

// 指定が無ければ、本番の decide も今と同じ（確かめの行を出さず、止めない）。
func TestTextCheck_DecideWithoutReaderIsUnchanged(t *testing.T) {
	dir, _ := seedTextStore(t)
	t.Setenv(readerVar, "")
	out, err := run(t, dir, realDecideArgs()...)
	if err != nil || strings.Contains(out, "確かめ") || !strings.Contains(out, "を記録しました") {
		t.Errorf("今と同じはず: %v\n%s", err, out)
	}
}
