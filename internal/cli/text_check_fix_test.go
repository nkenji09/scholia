package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 差し戻し 1 回目の指摘に対する検査。text_check_test.go（受け入れテスト）は変えず、ここに足す。

func writeRawReaderQuestion(t *testing.T, dir, q string) {
	t.Helper()
	path := filepath.Join(dir, ".scholia", "config.json")
	b, _ := os.ReadFile(path)
	var cfg map[string]any
	_ = json.Unmarshal(b, &cfg)
	cfg["readerQuestion"] = q
	nb, _ := json.Marshal(cfg)
	if err := os.WriteFile(path, nb, 0o644); err != nil {
		t.Fatal(err)
	}
}

// 指摘 2: 空白だけの readerQuestion でほかのコマンドは止まらない。decide は止めずに保存し、text-check は 1。
func TestTextCheckFix_WhitespaceQuestionDoesNotStopOtherCommands(t *testing.T) {
	dir, _ := seedTextStore(t)
	writeRawReaderQuestion(t, dir, "   ")
	f := fakeReader(t, noCandidates, 0)
	if out, err := run(t, dir, "tag", "list"); err != nil {
		t.Fatalf("ほかのコマンドは止まらないはず: %v\n%s", err, out)
	}
	before := savedDecisionCount(t, dir)
	out, err := run(t, dir, realDecideArgs()...)
	if err != nil || savedDecisionCount(t, dir) != before+1 ||
		!strings.Contains(out, "確かめ: 行われなかった（readerQuestion が空白だけ） new") {
		t.Errorf("decide は止めずに保存するはず: %v\n%s", err, out)
	}
	out, err = run(t, dir, "text-check", "tag:t1")
	if err == nil || !strings.Contains(out, "確かめ: 行われなかった（readerQuestion が空白だけ） t1") {
		t.Errorf("text-check は 1: %v\n%s", err, out)
	}
	if f.callCount(t) != 0 {
		t.Error("読み手は呼ばないはず")
	}
}

// 指摘 3: 本番の decide --json も、読み手の指定があれば textCheck を出す（失敗の理由・accepted を含む）。人向けの行は標準エラーへ。
func TestTextCheckFix_RealDecideJSONCarriesTextCheck(t *testing.T) {
	dir, _ := seedTextStore(t)
	type env struct {
		TextCheck *struct {
			Checked    bool                      `json:"checked"`
			Accepted   bool                      `json:"accepted"`
			Question   string                    `json:"question"`
			NotChecked []struct{ Reason string } `json:"notChecked"`
		} `json:"textCheck"`
	}
	decode := func(out string) env {
		var e env
		if err := json.Unmarshal([]byte(out), &e); err != nil {
			t.Fatalf("標準出力が JSON でない: %v\n%s", err, out)
		}
		return e
	}
	fakeReader(t, noCandidates, 0)
	out, errOut, err := runSplit(t, dir, realDecideArgs("--json")...)
	if e := decode(out); err != nil || e.TextCheck == nil || !e.TextCheck.Checked || e.TextCheck.Question != "既定" ||
		!strings.Contains(errOut, "確かめ: わかりにくい表現の候補は無い") {
		t.Errorf("確かめた結果: %v\n%s\n%s", err, out, errOut)
	}
	fakeReader(t, "", 3)
	out, errOut, err = runSplit(t, dir, realDecideArgs("--json")...)
	if e := decode(out); err != nil || e.TextCheck == nil || e.TextCheck.Checked || len(e.TextCheck.NotChecked) != 1 ||
		e.TextCheck.NotChecked[0].Reason != "終了コード 3 で終わった" || !strings.Contains(errOut, "確かめ: 行われなかった") {
		t.Errorf("失敗の理由: %v\n%s\n%s", err, out, errOut)
	}
	out, _, err = runSplit(t, dir, realDecideArgs("--json", "--accept-text-candidates")...)
	if e := decode(out); err != nil || e.TextCheck == nil || !e.TextCheck.Accepted || e.TextCheck.Checked {
		t.Errorf("accepted: %v\n%s", err, out)
	}
	// 指定が無ければ欄は出ない。
	t.Setenv(readerVar, "")
	out, _, err = runSplit(t, dir, realDecideArgs("--json")...)
	if e := decode(out); err != nil || e.TextCheck != nil {
		t.Errorf("指定が無ければ textCheck は出ないはず: %v\n%s", err, out)
	}
}

// 指摘 4・8: 読ませる文が無い text-check。指定の検査が先で、読み手を呼ばないときは問いの行を出さない。
func TestTextCheckFix_NothingToRead(t *testing.T) {
	dir, _ := seedTextStore(t)
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, "text-check", "tx:T-1")
	if err != nil || !strings.Contains(out, "確かめ: 読ませる文が無い（読み手は呼んでいない）") ||
		strings.Contains(out, "候補は無い") || strings.Contains(out, "問いは") || f.callCount(t) != 0 {
		t.Errorf("0 で終わり、専用の行だけのはず: %v\n%s", err, out)
	}
	out, err = run(t, dir, "text-check", "tx:T-1", "--json")
	var j struct {
		TextCheck struct {
			Checked  bool   `json:"checked"`
			Reason   string `json:"reason"`
			Question string `json:"question"`
		} `json:"textCheck"`
	}
	if jerr := json.Unmarshal([]byte(out), &j); jerr != nil || err != nil || j.TextCheck.Checked || j.TextCheck.Reason == "" || j.TextCheck.Question != "" {
		t.Errorf("JSON: %v %v\n%s", err, jerr, out)
	}
	t.Setenv(readerVar, "")
	if out, err := run(t, dir, "text-check", "tx:T-1"); err == nil || !strings.Contains(out, "読み手が指定されていない") {
		t.Errorf("指定が無ければ 1: %v\n%s", err, out)
	}
	t.Setenv(readerVar, "./relative")
	if out, err := run(t, dir, "text-check", "tx:T-1"); err == nil || !strings.Contains(out, "起動しなかった") {
		t.Errorf("相対パスは文の有無より先に拒む（1）: %v\n%s", err, out)
	}
}

func TestTextCheckFix_QuestionLineOnlyWhenTheReaderRan(t *testing.T) {
	dir, _ := seedTextStore(t)
	t.Setenv(readerVar, "./relative")
	out, err := run(t, dir, "text-check", "tag:t1")
	if err == nil || strings.Contains(out, "問いは") {
		t.Errorf("起動していないので問いの行は出ないはず: %v\n%s", err, out)
	}
	out, _ = run(t, dir, "text-check", "tag:t1", "--json")
	if strings.Contains(out, `"question"`) {
		t.Errorf("JSON の question も出ないはず:\n%s", out)
	}
}

// 指摘 7: 保存の口が拒む入力は、読み手を呼ぶ前にその拒みで終える。
func TestTextCheckFix_StoreRejectionComesBeforeTheReader(t *testing.T) {
	dir, _ := seedTextStore(t)
	f := fakeReader(t, noCandidates, 0)
	out, err := run(t, dir, realDecideArgs("--commit", "abcdef1234567")...)
	if err == nil {
		t.Fatalf("存在しない commit は拒まれるはず:\n%s", out)
	}
	if f.callCount(t) != 0 {
		t.Error("読み手を呼ばずに終えるはず")
	}
	// 見出しの無い why も同じ。
	out, err = run(t, dir, "decide", "--on", "tag:t1", "--why", "見出しなし")
	if err == nil || f.callCount(t) != 0 {
		t.Errorf("見出しの拒みが先: %v\n%s", err, out)
	}
}

// 指摘 9: 名指しと --base の両方に入った記録の key は、名指しだけのときと同じ。
func TestTextCheckFix_KeyIsStableWhenBothNamedAndInTheDiff(t *testing.T) {
	dir, _ := seedTextStore(t)
	gitCommitAllT(t, dir, "seed")
	if out, err := run(t, dir, "tag", "edit", "t1", "--desc", "変えた説明"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	f := fakeReader(t, noCandidates, 0)
	if out, err := run(t, dir, "text-check", "tag:t1"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	_, a := f.sent(t)
	if out, err := run(t, dir, "text-check", "tag:t1", "--base", "HEAD"); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	_, b := f.sent(t)
	if len(a) != 1 || len(b) != 1 || a[0].Key != "t1" || b[0].Key != "t1" {
		t.Errorf("key は t1 のまま 1 件のはず: %+v / %+v", a, b)
	}
}

// 指摘 1 の補助: 本物の環境にある読み手の指定は、この検査一式に持ち込まれない（TestMain が消している）。
func TestTextCheckFix_TestMainClearsTheReaderVariable(t *testing.T) {
	if v := os.Getenv(readerVar); v != "" {
		t.Errorf("TestMain が %s を消しているはずが %q", readerVar, v)
	}
}
