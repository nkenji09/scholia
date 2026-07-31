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

// unsetEnv は環境変数を**未設定**にする。
//
// t.Setenv を一度通してから消すのは、テスト終了時に元の値へ戻す後始末を testing に登録するため
// （t.Unsetenv は無い）。実環境の $XDG_STATE_HOME が検査へ漏れ込むのを止めるのにも要る。
func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "この値は Unsetenv で消える（後始末の登録のためだけに一度置く）")
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("Unsetenv(%s): %v", key, err)
	}
}

// usageLogEnv は計測ログの置き場所をテスト用に閉じ込め、そのパスを返す。
//
// ⚠️ **$XDG_STATE_HOME を明示的に未設定にする。** これをしないと、実環境で設定している人の
// 手元だけ検査が別の場所を見る（＝手元では緑・CI では赤、あるいはその逆）。
func usageLogEnv(t *testing.T) string {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	unsetEnv(t, StateHomeEnv)
	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	return path
}

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
//
// 導いたものの側は cli の TestUsage_MaskedLineIsNonInterferingWithProjectNames が差分で見ている
// （プロジェクトが名付けたものだけを変えて 2 回走らせ、行がバイト同一であること）。
// あちらは配線した経路を実際に走らせる検査なので、Line だけを見るこちらでは代われない。
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
	path := usageLogEnv(t)

	Record(Off, fullObservation())

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("オフなのにファイルが作られた（%s・err=%v）", path, err)
	}
	// ディレクトリごと作られていないこと（生成物が 1 つも増えない）。
	if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
		t.Errorf("オフなのにディレクトリが作られた: %s", filepath.Dir(path))
	}
}

func TestRecord_AppendsOneLinePerCall(t *testing.T) {
	path := usageLogEnv(t)

	Record(Normal, fullObservation())
	Record(Detailed, fullObservation())

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
	path := usageLogEnv(t)
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

// seedProject は `.scholia` を持つ「プロジェクトらしいディレクトリ」を作って返す。
// store の中身までは作らない（見たいのは置き場所がここに入らないことだけ）。
func seedProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".scholia", "decisions"), 0o755); err != nil {
		t.Fatalf("プロジェクトを作れない: %v", err)
	}
	return root
}

// TestDefaultPath_IsASingleFileOutsideTheProject は、正本 条項 8
// 「置き場所はリポジトリの外の 1 ファイル。`.scholia/` 配下には置かない」を**値で**検査する。
//
// ⚠️ 以前の版は「パスに `.scholia` という文字列を含まない」しか見ていなかった。
// **名乗り（プロジェクトの外）より狭い**——文字列が一致しなければ、プロジェクトの中に
// 置いていても通ってしまう。ここでは「外である」ことそのものを 3 つの性質で見る。
//
// ⚠️ **この検査の射程**: 見ているのは「解決したパスがプロジェクトの位置に依存しない」ことで、
// **どのディレクトリがプロジェクトかを知っているわけではない。** 利用者が state ディレクトリの
// 中で `scholia init` すれば、そこはプロジェクトにもなる（そのときの置き場所は下の
// TestDefaultPath_DoesNotCreateAStoreShapedDirectory が別の角度から見ている）。
func TestDefaultPath_IsASingleFileOutsideTheProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetEnv(t, StateHomeEnv)

	project := seedProject(t)

	// プロジェクトの中に降りて解決する。
	t.Chdir(project)
	inProject, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	// 別の場所（プロジェクトのさらに内側）へ移って、もう一度解決する。
	deeper := filepath.Join(project, ".scholia", "decisions")
	t.Chdir(deeper)
	inDeeper, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}

	if filepath.Base(inProject) != "usage.jsonl" {
		t.Errorf("ログのファイル名が %q", filepath.Base(inProject))
	}
	if !filepath.IsAbs(inProject) {
		t.Errorf("絶対パスでない: %s", inProject)
	}
	// 1) cwd を変えても同じパスを返す ＝ プロジェクトの位置に依存しない。
	if inProject != inDeeper {
		t.Errorf(`cwd によって置き場所が変わる（プロジェクトの位置に依存している）
  %s で解決: %s
  %s で解決: %s`, project, inProject, deeper, inDeeper)
	}
	// 2) プロジェクトルートの下にいない。
	if isUnder(inProject, project) {
		t.Errorf("プロジェクトの中に置いている:\n  ログ: %s\n  プロジェクト: %s", inProject, project)
	}
	// 3) プロジェクトのストア（<root>/.scholia）の下にいない。
	if store := filepath.Join(project, ".scholia"); isUnder(inProject, store) {
		t.Errorf(".scholia 配下に置いている:\n  ログ: %s\n  ストア: %s", inProject, store)
	}
}

// isUnder は path が root の下にあるかを、パスの成分単位で判定する。
// 文字列の前方一致だと "/a/bc" が "/a/b" の下だと誤判定するので、Rel で見る。
func isUnder(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// TestDefaultPath_DoesNotCreateAStoreShapedDirectory は、置き場所の**どの段にも**
// `.scholia` という名前のディレクトリが現れないことを見る。
//
// ⚠️ これは見た目の問題ではない。store.Discover は名前が `.scholia` のディレクトリを
// **中身を見ずに**ストアとして拾うので、置き場所の途中に 1 つでもあると、
// その親から下で走らせた scholia がそれをストアとして開こうとする——
// **しかも計測をオフに戻しても消えない。**
func TestDefaultPath_DoesNotCreateAStoreShapedDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	unsetEnv(t, StateHomeEnv)

	path, err := DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	rel, err := filepath.Rel(home, path)
	if err != nil {
		t.Fatalf("Rel: %v", err)
	}
	for _, seg := range strings.Split(rel, string(filepath.Separator)) {
		if seg == ".scholia" {
			t.Errorf("置き場所の途中に `.scholia` というディレクトリを作っている（store.Discover が拾う）: %s", path)
		}
	}
}

// TestDefaultPath_OneRuleForEveryOS は、置き場所を決める規則が 1 つであること
// （$XDG_STATE_HOME か、無ければ $HOME/.local/state）を入力と出力の対で検査する。
//
// ⚠️ **OS で分岐しない。** os.UserConfigDir / os.UserCacheDir を使うと darwin と linux で
// 別の場所になり、置き場所のガードも OS で分岐する。
func TestDefaultPath_OneRuleForEveryOS(t *testing.T) {
	home := t.TempDir()
	abs := t.TempDir()

	cases := []struct {
		name     string
		stateEnv string // "" は未設定を表す
		want     func() string
	}{
		{"XDG_STATE_HOME があればそれ", abs,
			func() string { return filepath.Join(abs, "scholia", "usage.jsonl") }},
		{"未設定なら $HOME/.local/state", "",
			func() string { return filepath.Join(home, ".local", "state", "scholia", "usage.jsonl") }},
		// 相対パスは XDG の規定で無効。ここで使ってしまうと基点が cwd になり、
		// ログがプロジェクトの中に落ちる（条項 8 違反）。
		{"相対パスは無効として無視する", "relative/state",
			func() string { return filepath.Join(home, ".local", "state", "scholia", "usage.jsonl") }},
		{"空文字も無効として無視する", "",
			func() string { return filepath.Join(home, ".local", "state", "scholia", "usage.jsonl") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("HOME", home)
			if c.stateEnv == "" {
				unsetEnv(t, StateHomeEnv)
			} else {
				t.Setenv(StateHomeEnv, c.stateEnv)
			}
			got, err := DefaultPath()
			if err != nil {
				t.Fatalf("DefaultPath: %v", err)
			}
			if want := c.want(); got != want {
				t.Errorf("DefaultPath() = %q, want %q", got, want)
			}
		})
	}
}

// TestDefaultPath_UnresolvableEnvironmentIsAnErrorNotAGuess は、基点が決まらないときに
// **推測でどこかへ書かない**こと。
//
// ⚠️ ここでエラーを返すのは「記録しない」に倒すためである（条項 11: 記録の失敗が本業を落とさない）。
// 相対パスや空文字を掴んで cwd 起点のパスを組み立てると、**プロジェクトの中に書いてしまう。**
func TestDefaultPath_UnresolvableEnvironmentIsAnErrorNotAGuess(t *testing.T) {
	unsetEnv(t, StateHomeEnv)
	unsetEnv(t, "HOME")

	path, err := DefaultPath()
	if err == nil {
		t.Fatalf("基点が決まらないのにパスを返した: %q", path)
	}
	if path != "" {
		t.Errorf("エラーなのにパスも返している: %q", path)
	}
}

// TestRecord_UnresolvableEnvironmentWritesNothingAndDoesNotPanic は、基点が決まらない環境で
// **どの段でも**本業が落ちないこと（条項 11）。オフでも段が立っていても、外へは何も出ない。
func TestRecord_UnresolvableEnvironmentWritesNothingAndDoesNotPanic(t *testing.T) {
	cwd := t.TempDir()
	t.Chdir(cwd)
	unsetEnv(t, StateHomeEnv)
	unsetEnv(t, "HOME")

	for _, l := range AllLevels() {
		Record(l, fullObservation())
	}

	// 推測で cwd 起点のどこかへ書いていないこと。
	var wrote []string
	if err := filepath.WalkDir(cwd, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			wrote = append(wrote, p)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}
	if len(wrote) > 0 {
		t.Errorf("基点が決まらないのに何かを書いた: %v", wrote)
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
		"\n\n",        // 改行だけ
		"",            // 空
		"あいう",         // 多バイト・末尾改行なし
		"  末尾に空白 \t ", // 末尾の空白
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
