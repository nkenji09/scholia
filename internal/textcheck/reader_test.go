package textcheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// 偽の読み手は sh の台本を一時フォルダに置いたもので、本物の AI は呼ばない。
// 環境変数を使うので、並列にしない。

func tool(t *testing.T, script string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "reader")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func records() []Record {
	return []Record{
		{ID: "01AAA", Texts: []Text{{Field: "why", Text: "改行を含む\n理由"}, {Field: "changed", Text: "変更"}}},
		{ID: "t1", Texts: []Text{{Field: "description", Text: "説明"}}},
	}
}

func notCheckedReason(t *testing.T, r Result) string {
	t.Helper()
	if len(r.NotChecked) != 1 {
		t.Fatalf("行われなかった部分が 1 つのはずが %+v", r)
	}
	return r.NotChecked[0].Reason
}

func TestVariableNameIsFixed(t *testing.T) {
	if Variable != "SCHOLIA_TEXT_READER" {
		t.Fatalf("環境変数の名前: %s", Variable)
	}
}

func TestNoReaderLaunchesNothing(t *testing.T) {
	mark := filepath.Join(t.TempDir(), "started")
	tool(t, "echo started > '"+mark+"'")
	t.Setenv(Variable, "")
	got := Reader{}.Read("問い", records())
	if !got.NoReader || got.Candidates != nil || got.NotChecked != nil {
		t.Fatalf("指定が無いことを表す結果のはずが %+v", got)
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("何も起動しないはず")
	}
}

func TestSendsOnlyQuestionAnswerFormatAndRecords(t *testing.T) {
	received := filepath.Join(t.TempDir(), "received")
	t.Setenv(Variable, tool(t, "cat > '"+received+"'\necho '{\"candidates\":[]}'"))
	got := Reader{Timeout: 30 * time.Second}.Read("  問いの文\n二行目  ", records())
	if got.NoReader || len(got.NotChecked) != 0 || len(got.Candidates) != 0 {
		t.Fatalf("候補なしの確かめた結果のはずが %+v", got)
	}
	b, _ := os.ReadFile(received)
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(b, &doc); err != nil || len(doc) != 3 {
		t.Fatalf("question・answerFormat・records の三つだけの object のはずが %q", b)
	}
	var question, format string
	_ = json.Unmarshal(doc["question"], &question)
	_ = json.Unmarshal(doc["answerFormat"], &format)
	if question != "  問いの文\n二行目  " {
		t.Errorf("question は渡した問いのままのはず: %q", question)
	}
	for _, want := range []string{"JSON の object 一つだけ", "candidates", "key", "phrase", "reason", "渡した", "引用", "空の並び"} {
		if !strings.Contains(format, want) {
			t.Errorf("answerFormat は %q を含むはず: %q", want, format)
		}
	}
	var recs []map[string]any
	_ = json.Unmarshal(doc["records"], &recs)
	want := []map[string]any{
		{"key": "01AAA", "texts": []any{map[string]any{"field": "why", "text": "改行を含む\n理由"}, map[string]any{"field": "changed", "text": "変更"}}},
		{"key": "t1", "texts": []any{map[string]any{"field": "description", "text": "説明"}}},
	}
	if !reflect.DeepEqual(recs, want) {
		t.Errorf("records: %v", recs)
	}
}

func TestReaderEnvironmentLacksTheVariable(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), "env")
	t.Setenv("SCHOLIA_KEEP_ME", "kept")
	t.Setenv(Variable, tool(t, "cat > /dev/null\nenv > '"+envFile+"'\necho '{\"candidates\":[]}'"))
	Reader{Timeout: 30 * time.Second}.Read("問い", records())
	b, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), Variable+"=") {
		t.Errorf("読み手の環境に %s が無いはず", Variable)
	}
	if !strings.Contains(string(b), "SCHOLIA_KEEP_ME=kept") {
		t.Error("ほかの環境変数は引き継ぐはず")
	}
}

func TestReturnsCandidatesInOrderAndDropsOtherFields(t *testing.T) {
	out := `{"candidates":[{"key":"t1","phrase":"説明","reason":"何の説明か不明","rewrite":"捨てる"},{"key":"01AAA","phrase":"理由","reason":"二通り"}],"comment":"捨てる"}`
	t.Setenv(Variable, tool(t, "cat > /dev/null\ncat <<'EOF'\n"+out+"\nEOF"))
	got := Reader{Timeout: 30 * time.Second}.Read("問い", records())
	want := []Candidate{{Key: "t1", Phrase: "説明", Reason: "何の説明か不明"}, {Key: "01AAA", Phrase: "理由", Reason: "二通り"}}
	if got.NoReader || len(got.NotChecked) != 0 || !reflect.DeepEqual(got.Candidates, want) {
		t.Fatalf("読み手が返した順の候補のはずが %+v", got)
	}
}

func TestNotCheckedReasons(t *testing.T) {
	cases := []struct {
		name, script, reason string
	}{
		{"exit", "cat > /dev/null\nexit 3", "終了コード 3 で終わった"},
		{"not json", "cat > /dev/null\necho hello", "標準出力が candidates の形でない"},
		{"no candidates", "cat > /dev/null\necho '{}'", "標準出力が candidates の形でない"},
		{"missing field", "cat > /dev/null\necho '{\"candidates\":[{\"key\":\"t1\",\"phrase\":\"x\"}]}'", "標準出力が candidates の形でない"},
		{"non-string field", "cat > /dev/null\necho '{\"candidates\":[{\"key\":\"t1\",\"phrase\":\"x\",\"reason\":1}]}'", "標準出力が candidates の形でない"},
		{"unsent key", "cat > /dev/null\necho '{\"candidates\":[{\"key\":\"zzz\",\"phrase\":\"x\",\"reason\":\"y\"}]}'", `渡していない key "zzz" の候補が返った`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv(Variable, tool(t, c.script))
			got := Reader{Timeout: 30 * time.Second}.Read("問い", records())
			if got.NoReader || len(got.Candidates) != 0 {
				t.Fatalf("確かめが行われなかった結果のはずが %+v", got)
			}
			if r := notCheckedReason(t, got); !strings.HasPrefix(r, c.reason) {
				t.Errorf("理由は %q で始まるはずが %q", c.reason, r)
			}
			if !reflect.DeepEqual(got.NotChecked[0].IDs, []string{"01AAA", "t1"}) {
				t.Errorf("渡した全件の id が部分になるはず: %v", got.NotChecked[0].IDs)
			}
		})
	}
}

func TestTimeoutAndLaunchFailure(t *testing.T) {
	t.Setenv(Variable, tool(t, "sleep 30"))
	start := time.Now()
	got := Reader{Timeout: 200 * time.Millisecond}.Read("問い", records())
	if r := notCheckedReason(t, got); !strings.HasPrefix(r, "上限の 200ms までに終わらなかった") {
		t.Errorf("時間切れの理由: %q", r)
	}
	if time.Since(start) > 10*time.Second {
		t.Error("上限で止まるはず")
	}
	t.Setenv(Variable, filepath.Join(t.TempDir(), "no-such-reader"))
	if r := notCheckedReason(t, Reader{}.Read("問い", records())); !strings.HasPrefix(r, "起動できなかった: ") {
		t.Errorf("起動できない理由: %q", r)
	}
}

func TestRelativePathIsNotLaunched(t *testing.T) {
	dir := t.TempDir()
	mark := filepath.Join(dir, "started")
	if err := os.WriteFile(filepath.Join(dir, "reader"), []byte("#!/bin/sh\necho started > '"+mark+"'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	wd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(wd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{"./reader", "sub/../reader"} {
		t.Setenv(Variable, rel)
		got := Reader{}.Read("問い", records())
		if r := notCheckedReason(t, got); r != "読み手の指定が名前か絶対パスでないので起動しなかった" {
			t.Errorf("%s: 理由 %q", rel, r)
		}
	}
	if _, err := os.Stat(mark); err == nil {
		t.Error("相対パスの指定は起動しないはず")
	}
}

func TestLinesWording(t *testing.T) {
	lines := Lines(Result{Candidates: []Candidate{{Key: "t1", Phrase: "新しい方", Reason: "何と比べるか不明"}}})
	if !reflect.DeepEqual(lines, []string{"確かめ: t1 「新しい方」 何と比べるか不明"}) {
		t.Errorf("候補の行: %v", lines)
	}
	if got := Lines(Result{}); !reflect.DeepEqual(got, []string{"確かめ: わかりにくい表現の候補は無い"}) {
		t.Errorf("候補が無い行: %v", got)
	}
	nc := Result{NotChecked: []NotChecked{{IDs: []string{"a", "b"}, Reason: "終了コード 3 で終わった"}}}
	if got := Lines(nc); !reflect.DeepEqual(got, []string{"確かめ: 行われなかった（終了コード 3 で終わった） a、b"}) {
		t.Errorf("行われなかった行: %v", got)
	}
	if got := Lines(Result{NoReader: true}); got != nil {
		t.Errorf("指定が無ければ何も出さない: %v", got)
	}
}

func TestDefaultQuestionWording(t *testing.T) {
	for _, want := range []string{"何も知らない読み手", "「検査を撃つ」「読む口」", "「新しい方」", "「古ければ拒む」", "reason は 15 字以内"} {
		if !strings.Contains(DefaultQuestion, want) {
			t.Errorf("既定の問いは %q を含むはず", want)
		}
	}
}
