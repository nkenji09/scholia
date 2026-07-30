package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"github.com/nkenji09/scholia/internal/usage"
)

// runnableSurfaces は走査時点で「実行できるコマンド」すべて。
// 子を持ちつつ自分でも走るコマンド（scholia lint など）も 1 つの面である。
func usageRunnableSurfaces() []string {
	var leaves []string
	var walk func(c *cobra.Command, path string)
	walk = func(c *cobra.Command, path string) {
		name := strings.TrimSpace(path + " " + c.Name())
		if c.Runnable() {
			leaves = append(leaves, name)
		}
		for _, k := range c.Commands() {
			if k.Name() == "help" || k.Name() == "completion" || k.Hidden {
				continue
			}
			walk(k, name)
		}
	}
	walk(newRootCmd(), "")
	sort.Strings(leaves)
	return leaves
}

// TestUsage_EveryRunnableSurfaceIsClassified は、位置引数の分類に載っていない面が無いこと。
//
// ⚠️ CLAUDE.md 5「新しく作った面には、ガードを置き忘れる」。
// 未分類の面は値を記録しない（安全側に倒れる）ので**壊れはしない**が、
// 黙って記録が欠ける状態は、ログを読む側から見れば「引かれていない」と読める。
func TestUsage_EveryRunnableSurfaceIsClassified(t *testing.T) {
	var unclassified []string
	for _, s := range usageRunnableSurfaces() {
		if _, ok := positionalSpecs[s]; !ok {
			unclassified = append(unclassified, s)
		}
	}
	if len(unclassified) > 0 {
		t.Fatalf(`位置引数の分類が無い面がある: %v

位置引数を取らないなら空スライス {} を、取るなら位置ごとの argSpec を
usage_args.go の positionalSpecs に足すこと（最後の要素は残りの位置すべてに適用される）。
レコード id を取る位置なら classRecordID と選択子の種類を、
自由文なら classFreeText を宣言する。`, unclassified)
	}

	present := map[string]bool{}
	for _, s := range usageRunnableSurfaces() {
		present[s] = true
	}
	for s := range positionalSpecs {
		if !present[s] {
			t.Errorf("positionalSpecs に載っている %q は実在しない（改名・削除したなら分類も直す）", s)
		}
	}
}

// TestUsage_EveryStringFlagIsClassified は、文字列を取るフラグで分類の無いものが無いこと。
//
// 真偽・数値のフラグは型から扱えるので宣言が要らない（structuralFlagValue）。
// 宣言が要るのは、値がプロジェクトを指しうる文字列フラグだけである。
func TestUsage_EveryStringFlagIsClassified(t *testing.T) {
	found := map[string]bool{}
	var unclassified []string
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.LocalFlags().VisitAll(func(f *flag.Flag) {
			if !isStringLikeFlag(f) {
				return
			}
			found[f.Name] = true
			if _, ok := stringFlagSpecs[f.Name]; !ok {
				unclassified = append(unclassified, f.Name+" ("+c.CommandPath()+")")
			}
		})
		c.PersistentFlags().VisitAll(func(f *flag.Flag) {
			if !isStringLikeFlag(f) {
				return
			}
			found[f.Name] = true
			if _, ok := stringFlagSpecs[f.Name]; !ok {
				unclassified = append(unclassified, f.Name+" ("+c.CommandPath()+")")
			}
		})
		for _, k := range c.Commands() {
			if k.Name() == "help" || k.Name() == "completion" || k.Hidden {
				continue
			}
			walk(k)
		}
	}
	walk(newRootCmd())

	sort.Strings(unclassified)
	if len(unclassified) > 0 {
		t.Fatalf(`分類の無い文字列フラグがある: %v

usage_args.go の stringFlagSpecs に足すこと。
・レコード id を取る → classRecordID ＋ 選択子の種類
・道具の側の閉じた集合 → classToolVocab ＋ values（集合の外の値は自由文へ倒れる）
・それ以外（自由文・config が宣言する値・パス） → classFreeText`, unclassified)
	}

	for name := range stringFlagSpecs {
		if !found[name] {
			t.Errorf("stringFlagSpecs に載っている %q は実在しない（改名・削除したなら分類も直す）", name)
		}
	}
}

// --- 配線した経路を、実際に走らせて値で見る ---

// usageTestEnv はログの置き場所をテスト用に閉じ込め、パスを返す。
func usageTestEnv(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, "cache"))
	// 呼び出し元の名乗りは環境に左右されるので、テストでは固定する。
	t.Setenv("CLAUDE_CODE_ENTRYPOINT", "test-harness")
	t.Setenv("CLAUDE_CODE_SESSION_ID", "session-under-test")
	t.Setenv("AI_AGENT", "")
	usageProjectRoot = ""
	path, err := usage.DefaultPath()
	if err != nil {
		t.Fatalf("DefaultPath: %v", err)
	}
	return path
}

// runMeasured は段を立てて 1 回実行し、書かれた行を返す（1 行だけのはず）。
func runMeasured(t *testing.T, level usage.Level, args ...string) (out string, line map[string]any) {
	t.Helper()
	path := usageTestEnv(t)
	var stdout, stderr bytes.Buffer
	_ = executeWithUsage(level, usage.Record, args, &stdout, &stderr)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("計測ログを読めない（%s）: %v\nstdout:\n%s\nstderr:\n%s", path, err, stdout.String(), stderr.String())
	}
	rows := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(rows) != 1 {
		t.Fatalf("1 起動なのに %d 行書かれた:\n%s", len(rows), data)
	}
	if err := json.Unmarshal([]byte(rows[0]), &line); err != nil {
		t.Fatalf("行が JSON として読めない: %v\n%s", err, rows[0])
	}
	return stdout.String(), line
}

// seedStore は検査用の .scholia を作り、プロジェクトが名付けたレコードを 1 件置く。
func seedStore(t *testing.T, tagID string) string {
	t.Helper()
	dir := t.TempDir()
	if out, err := run(t, dir, "init"); err != nil {
		t.Fatalf("init: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", tagID, "--name", "検査用", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	return dir
}

// projectNamedArg はプロジェクトが名付けた文字列。マスクの行に現れてはいけない。
const projectNamedArg = "req.acme-confidential-billing"

// TestUsage_MaskedLineDoesNotContainProjectNamedArguments は、正本の歯止め 3 を
// **配線した経路を実際に走らせて**値で検査する。
//
// ⚠️ **この検査の射程**（正本の歯止め 4・CLAUDE.md 6）:
// 落ちるのは「**引数の値がそのままログに出ること**」だけである。
// 値から**導いたもの**——長さ・先頭数文字・ダイジェスト——が出ても、この検査は落ちない。
// 本 repo のタグは 78 件しかなく長さはほぼ一意なので、長さが出れば名前は指せてしまう。
// マスクの境界は性質として usage.Records / Field.NamesProject の側に書いてあり、
// **ここはその一部しか担保しない。ここを埋めたつもりになってはいけない。**
//
// 導いたものの側は TestUsage_MaskedLineIsNonInterferingWithProjectNames が差分で見ている
// （2 つの入力についてバイト同一であること）。**両方合わせても「すべての入力の証明」にはならない。**
func TestUsage_MaskedLineDoesNotContainProjectNamedArguments(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	_, line := runMeasured(t, usage.Masked, "--dir", dir, "rules", "--tag", projectNamedArg)

	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("再符号化できない: %v", err)
	}
	for _, s := range []string{projectNamedArg, dir} {
		if strings.Contains(string(raw), s) {
			t.Errorf("マスクの行にプロジェクトが名付けた文字列 %q が現れた:\n%s", s, raw)
		}
	}
	// 漏らさないだけなら空行を書けば済むので、「残るべきものが残る」も同時に見る。
	if got := line["command"]; got != "scholia rules" {
		t.Errorf("command が %v", got)
	}
	if got := line["selectorKind"]; got != "tag" {
		t.Errorf("選択子の種類（道具の側の語彙）が残っていない: %v", got)
	}
	if got := line["level"]; got != "masked" {
		t.Errorf("段の名前が行に無い: %v", got)
	}
	if got := line["sessionId"]; got != "session-under-test" {
		t.Errorf("セッション識別子はマスクでも残るはず: %v", got)
	}
	if got := line["caller"]; got != "test-harness" {
		t.Errorf("呼び出し元の名乗りはマスクでも残るはず: %v", got)
	}
	for _, key := range []string{"recordIds", "projectRoot", "flagValues", "freeTextLens"} {
		if line[key] != nil {
			t.Errorf("マスクの行で %q が null でない: %v", key, line[key])
		}
	}
}

// usageNonInterferenceHoldOut は「プロジェクトが名付けたものと無関係に、あるいは量そのものとして
// 実行ごとに変わってよい」と**明示的に宣言した**項目。
//
// ⚠️ **ここに無い項目は、2 回の実行でバイト同一でなければならない。**
// Field.NamesProject の手宣言（項目ごと・18 個）と違い、**新しい項目はここに足さない限り
// 自動的に検査対象になる**——閉じ方がこちらのほうが強い。
var usageNonInterferenceHoldOut = map[string]bool{
	"ts":         true, // 時刻。実行ごとに進む
	"durationUs": true, // 所要。実行ごとに揺れる
	// stdoutBytes は「量」そのもの（正本がマスクで残すと決めた項目）で、原則は検査対象である。
	// レコード id の長さが違えば出力の長さも変わるので、**長さ違いの対でだけ** hold-out する。
}

// TestUsage_MaskedLineIsNonInterferingWithProjectNames は、マスクの行が
// **プロジェクトが名付けた入力の関数になっていない**ことを差分で見る。
//
// 正本の歯止め 3 の値検査（TestUsage_MaskedLineDoesNotContainProjectNamedArguments /
// TestLine_MaskedDoesNotContainProjectNamedValues）が捕まえるのは「値がそのまま出ること」だけで、
// 値から**導いたもの**——長さ・先頭数文字・ダイジェスト——は捕まえない。
// こちらはプロジェクトが名付けたものだけを変えて 2 回走らせ、行がバイト同一であることを見るので、
// 導いたものが 1 ビットでも行に漏れれば落ちる。
//
// **これは「捕まえられない綴りの列挙」ではなく 1 つの性質である**（CLAUDE.md 2）。
//
// ⚠️ **この検査の射程**（CLAUDE.md 6）:
//   - 言えるのは「**この 2 つの入力について行が同じ**」までで、すべての入力についての証明ではない。
//   - hold-out（上の 3 つ）に載せた項目は見ていない。
//   - **マスクの段だけを見る。** 通常・詳細はプロジェクトが名付けたものを載せると決めた段なので、
//     この性質は成り立たないし、成り立ってはいけない。
func TestUsage_MaskedLineIsNonInterferingWithProjectNames(t *testing.T) {
	const (
		shortID = "req.a"
		longID  = "req.bbbbbbbbbbbbbbbbbbbbbbbbbbbb"
		sameA   = "req.aaaaaaaaaaaaaaaa"
		sameB   = "req.zzzzzzzzzzzzzzzz"
	)
	cases := []struct {
		name       string
		a, b       string
		holdStdout bool
	}{
		{"同じ長さ・違う中身（値・先頭・ダイジェストの漏れを捕まえる）", sameA, sameB, false},
		{"違う長さ（長さの漏れを捕まえる）", shortID, longID, true},
		// レコード id を同じにして、違うのはプロジェクトのパスだけ。
		// 正本の「マスクでは複数プロジェクトを区別できない」が実装で成立していること。
		{"プロジェクトのパスだけ違う", sameA, sameA, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			la := maskedLineFor(t, c.a)
			lb := maskedLineFor(t, c.b)
			for _, m := range []map[string]any{la, lb} {
				for k := range usageNonInterferenceHoldOut {
					delete(m, k)
				}
				if c.holdStdout {
					delete(m, "stdoutBytes")
				}
			}
			ja, err := json.Marshal(la)
			if err != nil {
				t.Fatalf("再符号化できない: %v", err)
			}
			jb, err := json.Marshal(lb)
			if err != nil {
				t.Fatalf("再符号化できない: %v", err)
			}
			if string(ja) != string(jb) {
				t.Errorf(`マスクの行がプロジェクトが名付けたものに依存している（＝導いたものが漏れている）
  %q の行: %s
  %q の行: %s`, c.a, ja, c.b, jb)
			}
			// 空の行どうしを比べても同一になるので、「残るべきものが残る」も同時に見る。
			if la["level"] != "masked" || la["command"] != "scholia rules" {
				t.Errorf("比べた行が空になっている: %s", ja)
			}
		})
	}
}

// maskedLineFor は、プロジェクトが名付けたレコード 1 件だけを置いた新しい store に対して
// マスクで 1 回走らせ、その行を返す。**store は呼び出しごとに別の場所に作られる**ので、
// レコード id とプロジェクトのパスの両方が呼び出しごとに変わる。
func maskedLineFor(t *testing.T, tagID string) map[string]any {
	t.Helper()
	dir := seedStore(t, tagID)
	_, line := runMeasured(t, usage.Masked, "--dir", dir, "rules", "--tag", tagID)
	return line
}

// TestUsage_NormalLineNamesTheRecordAndTheProject は、通常が
// 「どのレコードが太いか」に答えられること。
func TestUsage_NormalLineNamesTheRecordAndTheProject(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	out, line := runMeasured(t, usage.Normal, "--dir", dir, "rules", "--tag", projectNamedArg)

	ids, _ := line["recordIds"].([]any)
	if len(ids) != 1 || ids[0] != projectNamedArg {
		t.Errorf("recordIds が %v", line["recordIds"])
	}
	root, _ := line["projectRoot"].(string)
	if root == "" || !strings.HasSuffix(dir, filepath.Base(root)) {
		t.Errorf("projectRoot が %q（--dir は %q）", root, dir)
	}
	if got, want := int(line["stdoutBytes"].(float64)), len(out); got != want {
		t.Errorf("stdoutBytes=%d だが実際に渡したのは %d バイト", got, want)
	}
	// 詳細だけの項目は通常では null のまま。
	for _, key := range []string{"flagValues", "freeTextLens", "stderrBytes", "durationPartsUs"} {
		if line[key] != nil {
			t.Errorf("通常の行で %q が null でない: %v", key, line[key])
		}
	}
}

// TestUsage_DetailedRecordsFreeTextLengthNotValue は、貫く原理
// 「ログは .scholia の中身を写さない」を最上段で検査する。
// 詳細は**呼び出しの形と量の解像度**を上げる段であって、自由文の値を書く段ではない。
func TestUsage_DetailedRecordsFreeTextLengthNotValue(t *testing.T) {
	dir := seedStore(t, projectNamedArg)
	body := strings.Repeat("秘", 137)

	_, line := runMeasured(t, usage.Detailed,
		"--dir", dir, "decide", "--on", "tag:"+projectNamedArg, "--why", body, "--json")

	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatalf("再符号化できない: %v", err)
	}
	if strings.Contains(string(raw), body) {
		t.Errorf("詳細の行に自由文の値が写っている:\n%s", raw)
	}
	lens, _ := line["freeTextLens"].(map[string]any)
	if got, _ := lens["why"].(float64); int(got) != 137 {
		t.Errorf("自由文の長さが %v（137 文字のはず）: %v", lens["why"], lens)
	}
	values, _ := line["flagValues"].(map[string]any)
	if values["json"] != true {
		t.Errorf("真偽のフラグ値が残っていない: %v", values)
	}
	if values["on"] != "tag:"+projectNamedArg {
		t.Errorf("レコードを指す値が残っていない: %v", values)
	}
	if got := line["selectorKind"]; got != "tag" {
		t.Errorf("--on の前置から選択子の種類を取れていない: %v", got)
	}
}

// TestUsage_UnknownSubcommandStillRecordsALine は、本業が失敗した起動も 1 行残ること。
func TestUsage_UnknownSubcommandStillRecordsALine(t *testing.T) {
	_, line := runMeasured(t, usage.Detailed, "no-such-command")
	if got := line["exitCode"]; got != float64(1) {
		t.Errorf("exitCode が %v", got)
	}
	if got, _ := line["stderrBytes"].(float64); got <= 0 {
		t.Errorf("標準エラーへ渡した量が数えられていない: %v", line["stderrBytes"])
	}
}

// --- 段の解決 ---

func TestResolveUsageLevel(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		set      bool
		want     usage.Level
		wantNote bool
	}{
		{"未設定は注記も出さない", "", false, usage.Off, false},
		{"空文字は設定されているので注記する", "", true, usage.Off, true},
		{"未知の値は注記してオフへ倒す", "verbose", true, usage.Off, true},
		{"明示のオフは注記しない", "off", true, usage.Off, false},
		{"マスク", "masked", true, usage.Masked, false},
		{"通常", "normal", true, usage.Normal, false},
		{"詳細", "detailed", true, usage.Detailed, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			level, note := resolveUsageLevel(func(string) (string, bool) { return c.raw, c.set })
			if level != c.want {
				t.Errorf("段が %s（want %s）", level, c.want)
			}
			if (note != "") != c.wantNote {
				t.Errorf("注記が %q（出すべきか: %v）", note, c.wantNote)
			}
			if note != "" && strings.Count(note, "\n") != 1 {
				t.Errorf("注記は 1 行であること: %q", note)
			}
		})
	}
}

// --- 既定 off の不変（正本 条項 10: オフのときは何もしない）---

// unsetUsageEnv は SCHOLIA_USAGE_LEVEL を**未設定**にする。
//
// t.Setenv を一度通してから消すのは、テスト終了時に元の値へ戻す後始末を testing に登録するため
// （t.Unsetenv は無い）。ここで os.LookupEnv を本番と同じまま使いたいので、
// lookup を偽装せずに環境そのものを未設定にする。
func unsetUsageEnv(t *testing.T) {
	t.Helper()
	t.Setenv(usage.EnvVar, "この値は Unsetenv で消える（後始末の登録のためだけに一度置く）")
	if err := os.Unsetenv(usage.EnvVar); err != nil {
		t.Fatalf("Unsetenv: %v", err)
	}
}

// usageOffCase は「既定 off で何も変わらない」を確かめる 1 つの起動。
type usageOffCase struct {
	name string
	args func(dir string) []string
}

// usageOffCases は起動の並び。
//
// ⚠️ **読み取りだけを並べない。** 単位AI の手測り 16 通りは全部読み取り系で、
// 書き込み系が 1 つも無かった（レビュアが 22 通りへ広げて埋めた軸）。
// 書き込み・失敗する起動・引数不足・`--help`・store を開かない起動を入れてある。
var usageOffCases = []usageOffCase{
	{"読み取り（出力がある）", func(dir string) []string {
		return []string{"--dir", dir, "rules", "--tag", projectNamedArg}
	}},
	{"読み取り（一覧）", func(dir string) []string { return []string{"--dir", dir, "list"} }},
	{"書き込み", func(dir string) []string {
		return []string{"--dir", dir, "tag", "create", "req.written-while-off", "--name", "オフで書く", "--kind", "requirement"}
	}},
	{"失敗する起動", func(dir string) []string { return []string{"--dir", dir, "no-such-command"} }},
	{"引数が足りない起動", func(dir string) []string { return []string{"--dir", dir, "show"} }},
	{"--help", func(string) []string { return []string{"--help"} }},
	{"store を開かない起動", func(string) []string { return []string{"version"} }},
}

// TestUsage_DefaultOffDoesNotEnterTheMeasuredPath は、正本で**最も重い不変**
// （条項 10: 環境変数が未設定なら出力・exit code・生成物のいずれも変わらない）を自動で守る。
//
// 2 つを**対で**見る。
//
//  1. **計測経路に入らないこと**——sink が 1 度も呼ばれない。
//     ⚠️ 「ログのファイルが作られない」だけでは足りない。usage.Record 自身がオフを弾くので、
//     オフで計測経路を通ってしまう変異（レビュアの R-1: off 分岐を丸ごと消す）でもファイルは増えない。
//  2. **出力が計測層を通さない経路とバイト同一であること**——注記の混入・writer の非素通しを捕まえる。
//
// ⚠️ **この検査の射程**（CLAUDE.md 6）:
// 落ちるのは「オフなのに観測を組み立てて sink へ渡すこと」と「オフなのに出力・エラー・生成物が変わること」である。
// **sink を呼ばずに writer を包むだけの変異は落ちない**——ただしそれは出力・exit code・生成物の
// どれも変えないので、条項 10 が名指しする観測可能な差にはならない（変わるのは所要だけである）。
// 「包んでいないこと」自体は TestUsage_PlainRootHandsCobraTheWritersUnwrapped が同一性で見ている。
func TestUsage_DefaultOffDoesNotEnterTheMeasuredPath(t *testing.T) {
	for _, c := range usageOffCases {
		t.Run(c.name, func(t *testing.T) {
			logPath := usageTestEnv(t)
			unsetUsageEnv(t)

			// 1) 計測層をまったく通さない経路（＝計測を入れる前の Execute と同じこと）。
			baseDir := seedStore(t, projectNamedArg)
			var baseOut bytes.Buffer
			baseRoot := newRootCmd()
			baseRoot.SetArgs(c.args(baseDir))
			baseRoot.SetOut(&baseOut)
			baseRoot.SetErr(&baseOut)
			baseErr := baseRoot.Execute()

			// 2) 環境変数が未設定のまま、本番の入口（Execute の中身）を通す。
			offDir := seedStore(t, projectNamedArg)
			var offOut bytes.Buffer
			sinkCalls := 0
			sink := func(l usage.Level, _ usage.Observation) {
				sinkCalls++
				t.Errorf("環境変数が未設定なのに計測経路を通り、sink が段 %s で呼ばれた", l)
			}
			offErr := execute(os.LookupEnv, sink, c.args(offDir), &offOut, &offOut)

			if sinkCalls != 0 {
				t.Errorf("sink が %d 回呼ばれた（オフでは 0 回）", sinkCalls)
			}
			if _, err := os.Stat(logPath); !os.IsNotExist(err) {
				t.Errorf("オフなのに計測ログが作られた（%s・err=%v）", logPath, err)
			}
			if _, err := os.Stat(filepath.Dir(logPath)); !os.IsNotExist(err) {
				t.Errorf("オフなのに計測ログのディレクトリが作られた: %s", filepath.Dir(logPath))
			}

			// プロジェクトルートが違うので、そこだけ正規化してからバイト比較する。
			want := strings.ReplaceAll(baseOut.String(), baseDir, "<DIR>")
			got := strings.ReplaceAll(offOut.String(), offDir, "<DIR>")
			if got != want {
				t.Errorf("オフで出力が変わった\n計測層なし: %q\nオフ経路  : %q", want, got)
			}
			if (baseErr == nil) != (offErr == nil) {
				t.Errorf("オフでエラーの有無が変わった: 計測層なし=%v / オフ経路=%v", baseErr, offErr)
			}
			if baseErr != nil && offErr != nil {
				wantErr := strings.ReplaceAll(baseErr.Error(), baseDir, "<DIR>")
				gotErr := strings.ReplaceAll(offErr.Error(), offDir, "<DIR>")
				if gotErr != wantErr {
					t.Errorf("オフでエラーが変わった\n計測層なし: %q\nオフ経路  : %q", wantErr, gotErr)
				}
			}
		})
	}
}

// TestUsage_PlainRootHandsCobraTheWritersUnwrapped は、オフ経路が cobra へ渡す writer が
// **渡されたものそのもの**であること——包んでいない＝観測していないこと——を同一性で見る。
func TestUsage_PlainRootHandsCobraTheWritersUnwrapped(t *testing.T) {
	var out, errw bytes.Buffer
	root := newPlainRoot(nil, &out, &errw)
	if got := root.OutOrStdout(); got != io.Writer(&out) {
		t.Errorf("オフ経路が標準出力を包んでいる: %T", got)
	}
	if got := root.ErrOrStderr(); got != io.Writer(&errw) {
		t.Errorf("オフ経路が標準エラーを包んでいる: %T", got)
	}
}

// TestUsage_RootSilencesUsageOnError は、オフ経路で SetOut していることが振る舞いを変えない
// **前提**を固定する。
//
// cobra の `cmd.Print*` と既定の UsageFunc は `OutOrStderr()` へ書く——SetOut していなければ
// 標準エラー、していれば標準出力である。計測を入れる前のオフ経路は SetOut していなかったので、
// **失敗した起動で usage が出ないこと**が「オフで出力が変わらない」の前提になっている。
// SilenceUsage が false に戻ると、この前提が崩れて宛先が stderr → stdout に動く。
func TestUsage_RootSilencesUsageOnError(t *testing.T) {
	if root := newRootCmd(); !root.SilenceUsage {
		t.Errorf("SilenceUsage が false。オフ経路は SetOut しているので、" +
			"失敗した起動の usage が標準エラーではなく標準出力へ出るようになる（計測前との差になる）")
	}
}

// TestUsage_CountingDoesNotChangeOutput は、計測を入れた経路と入れない経路で
// 標準出力のバイト列が同一であること（計数 writer が素通しであること）。
func TestUsage_CountingDoesNotChangeOutput(t *testing.T) {
	dir := seedStore(t, projectNamedArg)

	plain, err := run(t, dir, "rules", "--tag", projectNamedArg)
	if err != nil {
		t.Fatalf("計測なしの実行が失敗: %v", err)
	}

	usageTestEnv(t)
	var stdout, stderr bytes.Buffer
	if err := executeWithUsage(usage.Detailed, usage.Record, []string{"--dir", dir, "rules", "--tag", projectNamedArg}, &stdout, &stderr); err != nil {
		t.Fatalf("計測ありの実行が失敗: %v", err)
	}
	if got := stdout.String() + stderr.String(); got != plain {
		t.Errorf("計測を入れると出力が変わった\n計測なし: %q\n計測あり: %q", plain, got)
	}
}

// --- 分類の性質 ---

// TestUsage_ClosedSetValuesOnly は、閉じた集合の外の値が「道具の側の語彙」として
// 書かれないこと。**分類を間違えても、書けるのは宣言した語彙だけ**という性質を見る。
func TestUsage_ClosedSetValuesOnly(t *testing.T) {
	spec := stringFlagSpecs["sort"]
	if spec.class != classToolVocab {
		t.Fatalf("--sort は道具の側の語彙のはず")
	}
	shape := invocationShape{flagValues: map[string]any{}, freeTextLens: map[string]int{}}
	shape.apply("sort", "chrono", spec, map[string]bool{})
	shape.apply("sort", "プロジェクトが名付けた何か", spec, map[string]bool{})

	if got := shape.flagValues["sort"]; got != "chrono" {
		t.Errorf("閉じた集合の中の値は残るはず: %v", got)
	}
	if shape.freeTextLens["sort"] == 0 {
		t.Errorf("閉じた集合の外の値は長さへ倒れるはず: %v", shape.freeTextLens)
	}
	for _, v := range shape.flagValues {
		if s, ok := v.(string); ok && strings.Contains(s, "プロジェクト") {
			t.Errorf("閉じた集合の外の値が書かれた: %v", v)
		}
	}
}

// TestUsage_UnclassifiedStringFlagFallsBackToLength は、未分類の文字列フラグが
// 値ではなく長さへ倒れること（安全側の既定）。
func TestUsage_UnclassifiedStringFlagFallsBackToLength(t *testing.T) {
	cmd := &cobra.Command{Use: "probe", Run: func(*cobra.Command, []string) {}}
	var v string
	cmd.Flags().StringVar(&v, "brand-new-flag", "", "分類表に無いフラグ")
	if err := cmd.Flags().Parse([]string{"--brand-new-flag", "req.secret"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	shape := observeInvocation(cmd)
	if _, ok := shape.flagValues["brand-new-flag"]; ok {
		t.Errorf("未分類のフラグの値が書かれた: %v", shape.flagValues)
	}
	if got := shape.freeTextLens["brand-new-flag"]; got != len("req.secret") {
		t.Errorf("長さへ倒れていない: %v", shape.freeTextLens)
	}
	if len(shape.recordIDs) != 0 {
		t.Errorf("未分類のフラグが recordIds に入った: %v", shape.recordIDs)
	}
}
