package usage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// fullObservation は全項目に**互いに異なる非ゼロの値**を入れた観測。
//
// ゼロ値のままだと「記録していない（null）」と「記録したが空だった」の区別が
// 検査に見えなくなるので、どの項目にも値を入れる。
func fullObservation() Observation {
	return Observation{
		Timestamp:     time.Date(2026, 7, 30, 12, 34, 56, 0, time.UTC),
		Command:       "scholia rules",
		FlagNames:     []string{"json", "tag"},
		SelectorKind:  "tag",
		ArgCount:      2,
		ExitCode:      1,
		StdoutBytes:   12599,
		Duration:      42 * time.Millisecond,
		Caller:        "cli",
		SessionID:     "01JXXXXXXXXXXXXXXXXXXXXXXX",
		ToolVersion:   "v0.9.9",
		RecordIDs:     []string{"req.some-requirement"},
		ProjectRoot:   "/somewhere/some-project",
		FlagValues:    map[string]any{"json": true, "sort": "chrono"},
		FreeTextLens:  map[string]int{"why": 6138},
		StderrBytes:   77,
		DurationParts: map[string]int64{"write": 1200, "rest": 40800},
	}
}

// decodeLine は行を復号し、キーの並びも返す（キーの集合と順序を検査するため）。
func decodeLine(t *testing.T, line []byte) (map[string]json.RawMessage, []string) {
	t.Helper()
	if len(line) == 0 || line[len(line)-1] != '\n' {
		t.Fatalf("行が改行で終わっていない: %q", string(line))
	}
	if strings.Count(string(line), "\n") != 1 {
		t.Fatalf("1 起動 1 行のはずが改行が複数ある: %q", string(line))
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(line, &m); err != nil {
		t.Fatalf("行が JSON として読めない: %v\n%s", err, line)
	}
	dec := json.NewDecoder(strings.NewReader(string(line)))
	tok, err := dec.Token()
	if err != nil || tok != json.Delim('{') {
		t.Fatalf("行が JSON オブジェクトで始まっていない: %v", err)
	}
	var order []string
	for dec.More() {
		k, err := dec.Token()
		if err != nil {
			t.Fatalf("キーを読めない: %v", err)
		}
		order = append(order, k.(string))
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			t.Fatalf("値を読めない: %v", err)
		}
	}
	return m, order
}

// TestLine_NullIffNotRecorded は **4 段 × 全項目の対**を、組み立てた行の値で検査する。
//
// 正本の歯止め 1（判断を純関数へ切り出し、入力と出力の対で検査する）と
// 歯止め 2（検査は 4 段 × 全項目の対で行う）。
// 「記録していない」を表す形は null ただ 1 つで、記録すると決めた項目は必ず非 null になる
// ——この 2 方向を同時に見るので、Records と行の組み立てがずれたら落ちる。
func TestLine_NullIffNotRecorded(t *testing.T) {
	obs := fullObservation()
	for _, l := range AllLevels() {
		if l == Off {
			// オフは行そのものを書かない（Record 側で検査する）。
			continue
		}
		line, err := Line(l, obs)
		if err != nil {
			t.Fatalf("Line(%s) が失敗: %v", l, err)
		}
		m, _ := decodeLine(t, line)
		for _, f := range AllFields() {
			raw, ok := m[f.Key()]
			if !ok {
				t.Fatalf("段 %s の行にキー %q が無い（キーの集合は段によって変わらない）", l, f.Key())
			}
			isNull := string(raw) == "null"
			if want := Records(l, f); isNull == want {
				t.Errorf("段 %s・項目 %q: Records=%v なのに値は %s。"+
					"null は「記録していない」のただ 1 つの形である", l, f.Key(), want, raw)
			}
		}
	}
}

// TestLine_KeySetAndOrderDoNotDependOnLevel は、キーの集合と並びが段によらないこと。
//
// 集計する側が段ごとに別の読み方をしなくて済むための条項（正本 条項 7）。
func TestLine_KeySetAndOrderDoNotDependOnLevel(t *testing.T) {
	obs := fullObservation()
	var want []string
	for _, f := range AllFields() {
		want = append(want, f.Key())
	}
	for _, l := range AllLevels() {
		if l == Off {
			continue
		}
		line, err := Line(l, obs)
		if err != nil {
			t.Fatalf("Line(%s) が失敗: %v", l, err)
		}
		_, order := decodeLine(t, line)
		if len(order) != len(want) {
			t.Fatalf("段 %s のキー数 %d が項目数 %d と違う", l, len(order), len(want))
		}
		for i := range want {
			if order[i] != want[i] {
				t.Fatalf("段 %s のキーの並びが AllFields と違う: %v", l, order)
			}
		}
	}
}

// TestLine_LevelNameIsOnTheLine は、その行が何を記録していない行かを行だけで判定できること。
func TestLine_LevelNameIsOnTheLine(t *testing.T) {
	obs := fullObservation()
	for _, l := range AllLevels() {
		if l == Off {
			continue
		}
		line, err := Line(l, obs)
		if err != nil {
			t.Fatalf("Line(%s) が失敗: %v", l, err)
		}
		m, _ := decodeLine(t, line)
		var got string
		if err := json.Unmarshal(m["level"], &got); err != nil {
			t.Fatalf("level を読めない: %v", err)
		}
		if got != l.String() {
			t.Errorf("段 %s の行の level が %q", l, got)
		}
	}
}

// projectNamedStrings は「プロジェクトが名付けたもの」として観測に入れる文字列。
// マスクの行にこれらが現れてはいけない。
var projectNamedStrings = []string{
	"req.comfortable-viewer.decision-display",
	"tx.viewer.render-drawer",
	"cond.acme-billing-enabled",
	"/Users/someone/workspace/acme-secret-product",
	"01KYSKM4T0RWRY1N7407KZSZ17",
}

// TestLine_MaskedDoesNotContainProjectNamedValues は、マスクの非漏洩を**値で**検査する。
//
// 正本の歯止め 3。プロジェクトが名付けた文字列を観測のあらゆる欄に入れて行を組み立て、
// マスクの行にその文字列が**部分文字列としても**現れないことを見る。
//
// ⚠️ **この検査の射程**（正本の歯止め 4・CLAUDE.md 6）:
// ここが捕まえるのは「**値がそのまま出ること**」だけである。
// 値から**導いたもの**——長さ・先頭数文字・ダイジェスト——が出ることは捕まえない。
// 本 repo のタグは 78 件しかなく、長さはほぼ一意なので、長さが出れば名前は指せてしまう。
// だからマスクの境界は性質として「導いたものも書かない」と Records / NamesProject の側に書いてあり、
// この検査はその一部しか担保しない。**ここを埋めたつもりになってはいけない。**
func TestLine_MaskedDoesNotContainProjectNamedValues(t *testing.T) {
	obs := fullObservation()
	obs.RecordIDs = append([]string(nil), projectNamedStrings...)
	obs.ProjectRoot = projectNamedStrings[3]
	obs.SelectorKind = "tag" // 選択子の**種類**は道具の側の語彙なので残ってよい
	obs.FlagValues = map[string]any{"tag": projectNamedStrings[0], "json": true}
	obs.FreeTextLens = map[string]int{"why": 6138}
	obs.Command = "scholia rules"
	obs.Caller = "cli"

	line, err := Line(Masked, obs)
	if err != nil {
		t.Fatalf("Line(masked) が失敗: %v", err)
	}
	got := string(line)
	for _, s := range projectNamedStrings {
		if strings.Contains(got, s) {
			t.Errorf("マスクの行にプロジェクトが名付けた文字列 %q が現れた:\n%s", s, got)
		}
	}
	// 道具の側の語彙は残っていること（漏らさないだけなら空行を書けば済むので、
	// 「残るべきものが残る」も同時に見る）。
	for _, s := range []string{`"masked"`, `"scholia rules"`, `"tag"`, `"cli"`} {
		if !strings.Contains(got, s) {
			t.Errorf("マスクの行に道具の側の語彙 %s が無い:\n%s", s, got)
		}
	}
}

// TestLine_MaskedDoesNotContainFreeTextValues は、貫く原理（ログは .scholia の中身を写さない）を
// マスクの行で見る。自由文の値は段によらず記録しない。
func TestLine_MaskedDoesNotContainFreeTextValues(t *testing.T) {
	const body = "この decision の本文がログに写ってはいけない"
	obs := fullObservation()
	obs.FlagValues = map[string]any{"why": body}
	line, err := Line(Masked, obs)
	if err != nil {
		t.Fatalf("Line(masked) が失敗: %v", err)
	}
	if strings.Contains(string(line), body) {
		t.Errorf("マスクの行に自由文の値が現れた:\n%s", line)
	}
}

func TestLine_RecordedButEmptyIsNotNull(t *testing.T) {
	obs := Observation{Timestamp: time.Unix(0, 0).UTC()}
	line, err := Line(Detailed, obs)
	if err != nil {
		t.Fatalf("Line(detailed) が失敗: %v", err)
	}
	m, _ := decodeLine(t, line)
	for key, want := range map[string]string{
		"flagNames":       "[]",
		"recordIds":       "[]",
		"flagValues":      "{}",
		"freeTextLens":    "{}",
		"durationPartsUs": "{}",
		"selectorKind":    `""`,
		"projectRoot":     `""`,
	} {
		if got := string(m[key]); got != want {
			t.Errorf("%q は %s であるべき（記録したが空、と読める形）。実際は %s", key, want, got)
		}
	}
}

func TestRecord_OffWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	Record(Off, fullObservation())

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("オフなのにファイルが作られた（%s・err=%v）", path, err)
	}
	// ディレクトリごと作られていないこと（生成物が 1 つも増えない）。
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("オフなのにディレクトリが作られた: %s", filepath.Dir(path))
	}
}

func TestRecord_AppendsOneLinePerCall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	Record(Normal, fullObservation())
	Record(Detailed, fullObservation())

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ログを読めない: %v", err)
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("2 起動なのに %d 行: %q", len(lines), data)
	}
	var levels []string
	for _, l := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("行が JSON として読めない: %v (%q)", err, l)
		}
		levels = append(levels, m["level"].(string))
	}
	sort.Strings(levels)
	if levels[0] != "detailed" || levels[1] != "normal" {
		t.Errorf("段の名前が行に入っていない: %v", levels)
	}
}

// TestRecord_FailureDoesNotEscape は、記録が失敗しても呼び出し側へ何も返らないこと
// （正本 条項 11: 記録の失敗が本業を落とさない）。
// 置き場所にファイルではなくディレクトリを置いて、書き込みを必ず失敗させる。
func TestRecord_FailureDoesNotEscape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("邪魔なディレクトリを作れない: %v", err)
	}

	// panic も戻り値も無いこと。Record は何も返さないので、ここまで到達すれば通過。
	Record(Detailed, fullObservation())
}

func TestAppendLine_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "usage.jsonl")
	if err := AppendLine(path, []byte("{}\n")); err != nil {
		t.Fatalf("AppendLine: %v", err)
	}
	if err := AppendLine(path, []byte("{}\n")); err != nil {
		t.Fatalf("2 回目の AppendLine: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("読めない: %v", err)
	}
	if string(data) != "{}\n{}\n" {
		t.Errorf("追記になっていない: %q", data)
	}
}

func TestDefaultPath_IsASingleFileOutsideTheProject(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(dir, "cache"))

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	if filepath.Base(path) != "usage.jsonl" {
		t.Errorf("ログのファイル名が %q", filepath.Base(path))
	}
	if strings.Contains(path, ".scholia") {
		t.Errorf(".scholia 配下に置いている: %s", path)
	}
	if !filepath.IsAbs(path) {
		t.Errorf("絶対パスでない: %s", path)
	}
}

// TestCountingWriter_PassesThroughUnchanged は、計数 writer が
// **バイト列にも境界にも触らない**こと。
//
// ⚠️ 末尾の改行・空の書き込み・多バイト文字を入れてあるのは、
// 「素通し」と称する検査が実際には落ちない綴りを通した前例があるため
// （最初の版は `bytes.TrimRight(p, "\n")` という変異を素通りさせた）。
func TestCountingWriter_PassesThroughUnchanged(t *testing.T) {
	var sink strings.Builder
	c := NewCountingWriter(&sink)
	payloads := []string{
		"rules: 該当する decision はありません\n", // 末尾改行
		"\n\n",                          // 改行だけ
		"",                              // 空
		"あいう",                           // 多バイト・末尾改行なし
		"  末尾に空白 \t ",                   // 末尾の空白
	}
	for _, p := range payloads {
		n, err := c.Write([]byte(p))
		if err != nil {
			t.Fatalf("Write(%q): %v", p, err)
		}
		if n != len(p) {
			t.Errorf("Write(%q) が %d を返した（want %d）", p, n, len(p))
		}
	}
	want := strings.Join(payloads, "")
	if got := sink.String(); got != want {
		t.Errorf("下流の内容が変わった:\n got=%q\nwant=%q", got, want)
	}
	if got := c.Bytes(); got != int64(len(want)) {
		t.Errorf("数えたバイト数が %d（want %d）", got, len(want))
	}
}

// errWriter は下流の失敗をそのまま返すかを見るための writer。
type errWriter struct{ err error }

func (e errWriter) Write(p []byte) (int, error) { return 3, e.err }

func TestCountingWriter_PropagatesDownstreamResult(t *testing.T) {
	want := os.ErrClosed
	c := NewCountingWriter(errWriter{err: want})
	n, err := c.Write([]byte("0123456789"))
	if err != want {
		t.Errorf("下流のエラーが返らない: %v", err)
	}
	if n != 3 {
		t.Errorf("下流が返した n をそのまま返していない: %d", n)
	}
	if c.Bytes() != 3 {
		t.Errorf("実際に渡ったバイト数を数えていない: %d", c.Bytes())
	}
}

func TestUnparsableNote_MentionsTheVariableAndTheLevels(t *testing.T) {
	note := UnparsableNote("verbose")
	if !strings.HasSuffix(note, "\n") || strings.Count(note, "\n") != 1 {
		t.Errorf("注記は 1 行であること: %q", note)
	}
	if !strings.Contains(note, EnvVar) {
		t.Errorf("注記に変数名が無い: %q", note)
	}
	for _, name := range LevelNames() {
		if !strings.Contains(note, name) {
			t.Errorf("注記に段の名前 %q が無い: %q", name, note)
		}
	}
}
