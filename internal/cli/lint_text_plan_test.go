package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
)

// ---------------------------------------------------------------------------
// このガードが何に対して落ちるか（射程）
//
// lint のテキスト出力は「4区分 × 面」で粒度が決まる。区分は displayed /
// typed-ack / ack-only / coverage、面は 既定 / --verbose / --ci / --json。
//
// 落ちるもの:
//   - 区分ごとの粒度の方針が変わったとき（GranularityTable・期待値は literal）。
//   - 描画が plan の表を無視したとき（FollowsPlanForEverySection・期待値は plan から導出）。
//   - **畳んだ後に残る唯一の情報である「件数」の値が狂ったとき**（SummaryCountsAreValueChecked）。
//     語が在るかではなく数字を読み、区分ごとに突き合わせる。区分の件数は互いに
//     相異なる値にしてあるので、他区分の件数と取り違える変異も落ちる。
//   - 件数に依存して粒度が変わる分岐を足したとき（GranularityIsIndependentOfCount）。
//     0〜40 件で粒度が一定であることを見るので、閾値を変えても落ちる。
//   - 述語が重なる finding（AcknowledgedBy と AcknowledgeOnly が同時に立つ
//     decision-stale）の振り分けが変わったとき（OverlappingPredicates…）。
//   - **畳んだ明細が --ci の面から、どんな書式であれ漏れたとき**
//     （FoldedDetailsNeverLeakIntoCIFace）。綴りの照合ではなく、
//     ①--ci は既定テキストを接頭辞として含む ②CI が足す行数は上限内
//     ③畳んだ finding を指す語が1つも出ない、の3点で見る。
//   - --json から畳んだ区分が欠けたとき（JSON_FindingsAreIdenticalAcrossFaces）。
//     3つの面の findings 配列が完全一致することを見る。
//   - 畳んだ区分が exit code に効いたとき（ExitCode_…）。baseline を張った状態で
//     見て、末尾に「baseline に無い warn なら --ci だけ exit 1」の対照を置く。
//
// 🔴 落ちないもの（名乗っておく）:
//   - **CLI 面の fixture で述語の重なりを踏めていない。** decision-stale は git の
//     commit 履歴から出る finding で、t.TempDir() の非 git ストアでは作れない。
//     重なりは合成 finding 側でしか通していない。
//   - **件数行や導線の文面が正しいか**は見ていない（在ること・数字が合うことまで）。
//   - **retrofit が実際にその明細を出せるか**は見ていない（導線の行き先の検証は
//     このファイルの外）。
// ---------------------------------------------------------------------------

// 区分ごとの marker。Message / Target / Detail に同じ値を入れるので、
// 「marker を含む行の数」＝「その finding が明細として描かれた行数」になる。
// 互いに接頭辞にならない綴りにしてある。
const (
	mkDisplayedWarn  = "MK-DISPLAYED-WARN"
	mkDisplayedInfo  = "MK-DISPLAYED-INFO"
	mkCoverageNone   = "MK-COVERAGE-NONE"
	mkCoverageViaA   = "MK-COVERAGE-VIATAG-A"
	mkCoverageViaB   = "MK-COVERAGE-VIATAG-B"
	mkCoverageDirect = "MK-COVERAGE-DIRECT"
	mkTypedAck       = "MK-TYPEDACK"
	mkOverlap        = "MK-OVERLAP-BOTH-FLAGS"
)

func mark(f lint.Finding, m string) lint.Finding {
	f.Message, f.Target, f.Detail = m, m, m
	return f
}

func ackOnlyFinding(i int) lint.Finding {
	return mark(lint.Finding{Rule: "why-file-line", Severity: lint.SeverityInfo, AcknowledgeOnly: true},
		fmt.Sprintf("MK-ACKONLY-%02d", i))
}

// guardFindings は4区分すべてを非空にした合成 findings。
// ⚠️ 区分ごとの件数は **互いに相異なる値** にしてある（displayed 3 / typed-ack 2 /
// ack-only 5）。同じ数にすると「他区分の件数を出す」変異が数字の一致で素通りする。
func guardFindings() []lint.Finding {
	fs := []lint.Finding{
		mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn}, mkDisplayedWarn),
		mark(lint.Finding{Rule: "unused-vocab", Severity: lint.SeverityInfo}, mkDisplayedInfo),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageNone}, mkCoverageNone),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageViaTag}, mkCoverageViaA),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageViaTag}, mkCoverageViaB),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageDirect}, mkCoverageDirect),
		mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn, AcknowledgedBy: "01TESTDECISIONID"}, mkTypedAck),
		// 述語の重なり: decision-stale は是正不能（AcknowledgeOnly）でありながら
		// acknowledges で容認されうる（AcknowledgedBy）。両方立つ実在の形。
		mark(lint.Finding{Rule: "decision-stale", Severity: lint.SeverityInfo,
			AcknowledgeOnly: true, AcknowledgedBy: "01TESTDECISIONID"}, mkOverlap),
	}
	for i := 1; i <= 5; i++ {
		fs = append(fs, ackOnlyFinding(i))
	}
	return fs
}

func ackOnlyMarkers() []string {
	out := make([]string, 0, 5)
	for i := 1; i <= 5; i++ {
		out = append(out, ackOnlyFinding(i).Target)
	}
	return out
}

// 表1: 区分 × 面 → 明細粒度。期待値は literal（実装から導出しない）。
func TestLintTextPlan_GranularityTable(t *testing.T) {
	table := []struct {
		mode    string
		verbose bool
		want    map[lintSection]lintDetail
	}{
		{"既定", false, map[lintSection]lintDetail{
			lintSectionDisplayed: lintDetailAll,  // error/warn は exit code と ratchet の根拠ゆえ畳まない
			lintSectionTypedAck:  lintDetailNone, // 件数のみ
			lintSectionAckOnly:   lintDetailNone, // 件数のみ
			lintSectionCoverage:  lintDetailNone, // 3段の件数のみ
		}},
		{"--verbose", true, map[lintSection]lintDetail{
			lintSectionDisplayed: lintDetailAll,
			lintSectionTypedAck:  lintDetailAll,
			lintSectionAckOnly:   lintDetailAll,
			lintSectionCoverage:  lintDetailAll,
		}},
	}
	for _, tc := range table {
		p := planLintText(guardFindings(), tc.verbose)
		if len(p.Detail) != len(tc.want) {
			t.Fatalf("%s: 区分の数が表と違う: got %d (%v), want %d",
				tc.mode, len(p.Detail), p.Detail, len(tc.want))
		}
		for sec, want := range tc.want {
			if got := p.Detail[sec]; got != want {
				t.Errorf("%s: 区分 %s の粒度 = %q, want %q", tc.mode, sec, got, want)
			}
		}
	}
}

// 表2: 区分への振り分け。finding がどの区分に落ちるかは verbose に依らない。
func TestLintTextPlan_SectionMembership(t *testing.T) {
	wantAckOnly := ackOnlyMarkers()
	for _, verbose := range []bool{false, true} {
		p := planLintText(guardFindings(), verbose)
		got := map[lintSection][]string{
			lintSectionDisplayed: markersOf(p.Displayed),
			lintSectionTypedAck:  markersOf(p.TypedAck),
			lintSectionAckOnly:   markersOf(p.AckOnly),
			lintSectionCoverage:  markersOf(p.ViaTag),
		}
		want := map[lintSection][]string{
			// coverage none は「info に出すのは none の実数のみ」ゆえ displayed 側。
			lintSectionDisplayed: {mkDisplayedWarn, mkDisplayedInfo, mkCoverageNone},
			// 述語が重なる finding は typed 容認側（下の専用テストに理由あり）。
			lintSectionTypedAck: {mkTypedAck, mkOverlap},
			lintSectionAckOnly:  wantAckOnly,
			lintSectionCoverage: {mkCoverageViaA, mkCoverageViaB}, // direct は内訳に出さない
		}
		for sec, w := range want {
			if !reflect.DeepEqual(got[sec], w) {
				t.Errorf("verbose=%v: 区分 %s = %v, want %v", verbose, sec, got[sec], w)
			}
		}
		if p.CoverageDirect != 1 || p.CoverageViaTag != 2 || p.CoverageNone != 1 {
			t.Errorf("verbose=%v: 3段の件数 = direct %d / via-tag %d / none %d, want 1/2/1",
				verbose, p.CoverageDirect, p.CoverageViaTag, p.CoverageNone)
		}
	}
}

// 述語の重なり: AcknowledgedBy と AcknowledgeOnly が同時に立つ finding は
// **typed 容認側**に落ちる。
//
// なぜ typed 容認が勝つか: acknowledges で容認された finding は evaluateCI でも
// baseline update でも「容認済み」として除外される。ack-only 側に落とすと、
// 容認済みのものを「まだ容認していない残件」として数える——件数行が黙って嘘をつく。
// planLintText の switch は上から評価するので、**case の順序がこの振る舞いを決める**。
func TestLintTextPlan_OverlappingPredicatesGoToTypedAck(t *testing.T) {
	both := mark(lint.Finding{Rule: "decision-stale", Severity: lint.SeverityInfo,
		AcknowledgeOnly: true, AcknowledgedBy: "01ACKDECISION"}, mkOverlap)

	for _, verbose := range []bool{false, true} {
		p := planLintText([]lint.Finding{both}, verbose)
		if len(p.TypedAck) != 1 {
			t.Errorf("verbose=%v: 両方立った finding が typed 容認に入っていない（TypedAck=%d）", verbose, len(p.TypedAck))
		}
		if len(p.AckOnly) != 0 {
			t.Errorf("verbose=%v: 両方立った finding が acknowledge-only にも入っている（AckOnly=%d）＝容認済みを残件として数える",
				verbose, len(p.AckOnly))
		}
		if len(p.Displayed) != 0 {
			t.Errorf("verbose=%v: 両方立った finding が是正対象に混ざった（Displayed=%d）", verbose, len(p.Displayed))
		}
	}
}

// 描画は plan の表に従う。期待値を plan から導出するので、区分が増えても
// 「新しい区分だけ表を通らない」経路を作れない。
func TestLintTextRender_FollowsPlanForEverySection(t *testing.T) {
	sections := []struct {
		sec      lintSection
		findings func(lintTextPlan) []lint.Finding
	}{
		{lintSectionDisplayed, func(p lintTextPlan) []lint.Finding { return p.Displayed }},
		{lintSectionTypedAck, func(p lintTextPlan) []lint.Finding { return p.TypedAck }},
		{lintSectionAckOnly, func(p lintTextPlan) []lint.Finding { return p.AckOnly }},
		{lintSectionCoverage, func(p lintTextPlan) []lint.Finding { return p.ViaTag }},
	}
	for _, verbose := range []bool{false, true} {
		p := planLintText(guardFindings(), verbose)
		out := renderToString(p)

		for _, s := range sections {
			// plan が All と言うなら 1 行、None と言うなら 0 行——区分の全 finding について。
			want := 0
			if p.Detail[s.sec] == lintDetailAll {
				want = 1
			}
			for _, f := range s.findings(p) {
				if got := countLinesContaining(out, f.Target); got != want {
					t.Errorf("verbose=%v: 区分 %s の %s を含む行 = %d, want %d（plan は %q）\n出力:\n%s",
						verbose, s.sec, f.Target, got, want, p.Detail[s.sec], out)
				}
			}
		}

		// 畳んだ先の案内は既定のときだけ出す（--verbose を付けているのに
		// 「明細は --verbose」と案内し続けるのは自己矛盾）。
		hasHint := strings.Contains(out, "明細は --verbose")
		if verbose && hasHint {
			t.Errorf("--verbose なのに「明細は --verbose」と案内している\n出力:\n%s", out)
		}
		if !verbose && !hasHint {
			t.Errorf("既定出力に明細の開き方の案内が無い\n出力:\n%s", out)
		}
		if !strings.Contains(out, "scholia retrofit") {
			t.Errorf("verbose=%v: retrofit への導線が無い\n出力:\n%s", verbose, out)
		}
	}
}

// 🔴 畳んだ後に残るのは件数だけ。その**値**を区分ごとに突き合わせる。
// 語が在るかではなく数字を読む。件数は区分ごとに相異なる値なので、
// 他区分の件数で置き換える変異も落ちる。
func TestLintTextRender_SummaryCountsAreValueChecked(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		p := planLintText(guardFindings(), verbose)
		got := summaryCounts(t, renderToString(p))
		want := map[lintSection][]int{
			lintSectionTypedAck: {len(p.TypedAck)},
			lintSectionAckOnly:  {len(p.AckOnly)},
			lintSectionCoverage: {p.CoverageDirect, p.CoverageViaTag, p.CoverageNone},
		}
		for sec, w := range want {
			g, ok := got[sec]
			if !ok {
				t.Errorf("verbose=%v: 区分 %s の件数行が出力に無い", verbose, sec)
				continue
			}
			if !reflect.DeepEqual(g, w) {
				t.Errorf("verbose=%v: 区分 %s の件数 = %v, want %v", verbose, sec, g, w)
			}
		}
		// 件数が互いに相異なることを前提にしているので、前提自体を検査する
		// （同じ値にすると取り違えの変異が素通りするため）。
		if len(p.TypedAck) == len(p.AckOnly) || len(p.Displayed) == len(p.AckOnly) {
			t.Fatalf("fixture の前提が崩れている: 区分ごとの件数が相異ならない（displayed %d / typed-ack %d / ack-only %d）",
				len(p.Displayed), len(p.TypedAck), len(p.AckOnly))
		}
	}
}

// 粒度は件数に依存しない——「N 件以上なら既定でも明細を出す」という分岐を作らない。
// 閾値がいくつであっても落ちるように、0 件から 40 件まで見る。
func TestLintTextPlan_GranularityIsIndependentOfCount(t *testing.T) {
	build := map[lintSection]func(n int) []lint.Finding{
		lintSectionAckOnly: func(n int) []lint.Finding {
			fs := make([]lint.Finding, 0, n)
			for i := 1; i <= n; i++ {
				fs = append(fs, ackOnlyFinding(i))
			}
			return fs
		},
		lintSectionTypedAck: func(n int) []lint.Finding {
			fs := make([]lint.Finding, 0, n)
			for i := 1; i <= n; i++ {
				fs = append(fs, mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn,
					AcknowledgedBy: "01TESTDECISIONID"}, fmt.Sprintf("MK-TA-%02d", i)))
			}
			return fs
		},
	}
	for sec, mk := range build {
		for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 9, 10, 17, 40} {
			fs := mk(n)
			for _, verbose := range []bool{false, true} {
				p := planLintText(fs, verbose)
				wantDetail := lintDetailNone
				if verbose {
					wantDetail = lintDetailAll
				}
				if got := p.Detail[sec]; got != wantDetail {
					t.Errorf("区分 %s・n=%d・verbose=%v: 粒度 = %q, want %q（件数で粒度が変わっている）",
						sec, n, verbose, got, wantDetail)
				}
				// 実際に描かれた明細行数も件数に依らない。
				out := renderToString(p)
				wantLines := 0
				if verbose {
					wantLines = n
				}
				lines := 0
				for _, f := range fs {
					lines += countLinesContaining(out, f.Target)
				}
				if lines != wantLines {
					t.Errorf("区分 %s・n=%d・verbose=%v: 明細行 = %d, want %d\n出力:\n%s",
						sec, n, verbose, lines, wantLines, out)
				}
			}
		}
	}
}

// coverage の direct は3段の件数にだけ現れ、どの面でも明細に出ない。
func TestLintTextRender_CoverageDirectIsNeverDetailed(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		out := renderToString(planLintText(guardFindings(), verbose))
		if n := countLinesContaining(out, mkCoverageDirect); n != 0 {
			t.Errorf("verbose=%v: direct が明細に %d 行出ている\n出力:\n%s", verbose, n, out)
		}
	}
}

// 是正対象が空でも、畳んだ区分が残っていれば「問題は見つかりませんでした」とは言わない
// ——畳みが「信じられる緑」を偽らないこと。
func TestLintTextRender_FoldedSectionsDoNotClaimClean(t *testing.T) {
	cases := map[string][]lint.Finding{
		"ack-only のみ": {ackOnlyFinding(1)},
		"typed 容認のみ":  {mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn, AcknowledgedBy: "01X"}, mkTypedAck)},
		"述語が重なるもののみ":  {mark(lint.Finding{Rule: "decision-stale", Severity: lint.SeverityInfo, AcknowledgeOnly: true, AcknowledgedBy: "01X"}, mkOverlap)},
	}
	for name, fs := range cases {
		out := renderToString(planLintText(fs, false))
		if strings.Contains(out, "問題は見つかりませんでした") {
			t.Errorf("%s: 畳んだ区分が残っているのに緑を名乗った\n出力:\n%s", name, out)
		}
	}
}

func renderToString(p lintTextPlan) string {
	var buf bytes.Buffer
	renderLintText(&buf, p)
	return buf.String()
}

func markersOf(fs []lint.Finding) []string {
	out := make([]string, 0, len(fs))
	for _, f := range fs {
		out = append(out, f.Target)
	}
	return out
}

func countLinesContaining(out, needle string) int {
	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, needle) {
			n++
		}
	}
	return n
}

var (
	reCount    = regexp.MustCompile(`: (\d+) 件`)
	reCoverage = regexp.MustCompile(`direct (\d+) / via-tag (\d+) / none (\d+)`)
)

// summaryCounts は出力の件数行から区分ごとの**数字**を読み取る。
// 「その語が在るか」ではなく値を返すので、他区分の件数と取り違えた変異が落ちる。
func summaryCounts(t *testing.T, out string) map[lintSection][]int {
	t.Helper()
	got := make(map[lintSection][]int)
	for _, line := range strings.Split(out, "\n") {
		var sec lintSection
		switch {
		case strings.HasPrefix(line, "typed "):
			sec = lintSectionTypedAck
		case strings.HasPrefix(line, "acknowledge-only"):
			sec = lintSectionAckOnly
		case strings.HasPrefix(line, "decision-coverage: "):
			m := reCoverage.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("decision-coverage のサマリ行から3段の件数を読めない: %q", line)
			}
			got[lintSectionCoverage] = []int{atoi(t, m[1]), atoi(t, m[2]), atoi(t, m[3])}
			continue
		default:
			continue
		}
		m := reCount.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("区分 %s のサマリ行から件数を読めない: %q", sec, line)
		}
		got[sec] = []int{atoi(t, m[1])}
	}
	return got
}

func atoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("数字として読めない: %q", s)
	}
	return n
}

// ---------------------------------------------------------------------------
// 面ごとの検査（--ci / --json / exit code）——CLI を通した実出力で見る。
// ---------------------------------------------------------------------------

// setupAckOnlyStore は acknowledge-only を **5 件**（件数依存の分岐が踏める数）、
// typed 容認 1 件、是正対象 warn 1 件、coverage none 1 件を同時に含む store を組む。
// ⚠️ 区分ごとの件数を相異ならせてある（ack-only 5 / typed 容認 1）。
func setupAckOnlyStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"tx", "add", "T-bare", "--action", "act.a", "--then", "eff.a"},
		// 是正対象の warn（requirement-gap・容認しない）
		{"tag", "create", "req.open", "--name", "未充足要件", "--kind", "requirement"},
		// typed 容認 1 件（acknowledges で畳む）
		{"tag", "create", "req.acked", "--name", "容認する未充足要件", "--kind", "requirement"},
		{"decide", "--on", "tag:req.acked", "--why", "# 容認の見出し\n\n意図して残す gap である", "--acknowledges", "requirement-gap"},
	}
	// why に file:line を書いた decision＝why-file-line（acknowledge-only）を 5 件。
	for i := 1; i <= 5; i++ {
		steps = append(steps, []string{"decide", "--on", "tag:req.open",
			"--why", fmt.Sprintf("# 見出し %d\n\n判断の根拠は internal/example/sample%d.go:%d にある", i, i, 10+i)})
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

// foldedFindings は --json から「畳んだ区分」の finding を返す（面の検査の marker 源）。
func foldedFindings(t *testing.T, dir string) []lint.Finding {
	t.Helper()
	out, err := run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json failed: %v\n%s", err, out)
	}
	var payload struct {
		Findings []lint.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("JSON decode failed: %v\n%s", err, out)
	}
	var folded []lint.Finding
	for _, f := range payload.Findings {
		if f.AcknowledgeOnly || f.AcknowledgedBy != "" {
			folded = append(folded, f)
		}
	}
	if len(folded) < 5 {
		t.Fatalf("fixture の前提が崩れている: 畳んだ区分の finding が %d 件（5 件以上を期待）", len(folded))
	}
	return folded
}

// 🔴 畳んだ明細が --ci の面から、**どんな書式であれ**漏れないこと。
//
// 綴りの照合ではなく次の3点で見る:
//  1. --ci の出力は既定の出力を**接頭辞として含む**（既定テキストを描いてから CI 行を足す）。
//  2. CI が足す行数が上限内（この fixture では stale 0・新規 warn 0 なのでサマリ 1 行だけ）。
//  3. 畳んだ finding を指す語（target / message）が既定にも --ci にも 1 行も出ない。
//
// 2 があるので、printLintCIText に別書式で明細を吐く変異は綴りに関係なく落ちる。
func TestLintCI_FoldedDetailsNeverLeakIntoCIFace(t *testing.T) {
	dir := setupAckOnlyStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("baseline update failed: %v\n%s", err, out)
	}
	folded := foldedFindings(t, dir)

	def, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint failed: %v\n%s", err, def)
	}
	ci, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("lint --ci failed: %v\n%s", err, ci)
	}
	verbose, err := run(t, dir, "lint", "--verbose")
	if err != nil {
		t.Fatalf("lint --verbose failed: %v\n%s", err, verbose)
	}

	// 1. 構造
	if !strings.HasPrefix(ci, def) {
		t.Fatalf("--ci が既定テキストを接頭辞として含まない\n=== 既定 ===\n%s\n=== --ci ===\n%s", def, ci)
	}
	// 2. CI が足す行数（stale 0・新規 warn 0 ならサマリ 1 行）
	extra := strings.TrimSuffix(strings.TrimPrefix(ci, def), "\n")
	if n := len(strings.Split(extra, "\n")); n != 1 {
		t.Errorf("--ci が既定に足した行数 = %d, want 1（畳んだ明細が別書式で漏れていないか）\n足された分:\n%s", n, extra)
	}
	// 3. 畳んだ finding を指す語が既定・--ci に出ない／--verbose には出る
	for _, f := range folded {
		for _, needle := range []string{f.Target, f.Message} {
			if needle == "" {
				continue
			}
			if n := countLinesContaining(def, needle); n != 0 {
				t.Errorf("既定出力に畳んだ finding が %d 行漏れている（%s）\n%s", n, needle, def)
			}
			if n := countLinesContaining(ci, needle); n != 0 {
				t.Errorf("--ci の出力に畳んだ finding が %d 行漏れている（%s）\n%s", n, needle, ci)
			}
		}
		// 明細の書式は区分で違う（ack-only は Message、typed 容認は Target を描く）ので、
		// どちらかでその finding に届いていることを見る。
		if countLinesContaining(verbose, f.Message)+countLinesContaining(verbose, f.Target) == 0 {
			t.Errorf("--verbose で畳んだ finding の明細が出ていない（rule=%s target=%s）\n%s", f.Rule, f.Target, verbose)
		}
	}

	// 件数行は3つの面すべてに、同じ**値**で出る。
	for name, out := range map[string]string{"既定": def, "--ci": ci, "--verbose": verbose} {
		got := summaryCounts(t, out)
		if g, ok := got[lintSectionAckOnly]; !ok || len(g) != 1 || g[0] != 5 {
			t.Errorf("%s: acknowledge-only の件数 = %v, want [5]\n%s", name, got[lintSectionAckOnly], out)
		}
		if g, ok := got[lintSectionTypedAck]; !ok || len(g) != 1 || g[0] != 1 {
			t.Errorf("%s: typed 容認の件数 = %v, want [1]\n%s", name, got[lintSectionTypedAck], out)
		}
	}
}

// --json は面によらず findings が完全一致する（表示粒度の変更で欠けない）。
func TestLintJSON_FindingsAreIdenticalAcrossFaces(t *testing.T) {
	dir := setupAckOnlyStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("baseline update failed: %v\n%s", err, out)
	}

	decode := func(args ...string) []lint.Finding {
		out, err := run(t, dir, args...)
		if err != nil {
			t.Fatalf("%v failed: %v\n%s", args, err, out)
		}
		var payload struct {
			Findings []lint.Finding `json:"findings"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			t.Fatalf("%v: JSON decode failed: %v\n%s", args, err, out)
		}
		return payload.Findings
	}

	// ⚠️ 面どうしの一致だけでは、**全部の面から等しく欠ける**変異が取れない。
	// 畳んだ区分それぞれについて、fixture が決めた件数を値で押さえる。
	base := decode("lint", "--json")
	nAck, nTyped := 0, 0
	for _, f := range base {
		if f.AcknowledgeOnly && f.AcknowledgedBy == "" {
			nAck++
		}
		if f.AcknowledgedBy != "" {
			nTyped++
		}
	}
	if nAck != 5 {
		t.Errorf("--json の acknowledge-only = %d 件, want 5", nAck)
	}
	if nTyped != 1 {
		t.Errorf("--json の typed 容認 = %d 件, want 1", nTyped)
	}
	for _, args := range [][]string{
		{"lint", "--json", "--verbose"},
		{"lint", "--json", "--ci"},
		{"lint", "--json", "--verbose", "--ci"},
	} {
		if got := decode(args...); !reflect.DeepEqual(got, base) {
			t.Errorf("%v: findings が既定の --json と一致しない（%d 件 vs %d 件）", args, len(got), len(base))
		}
	}
}

// 畳んだ区分（typed 容認 / acknowledge-only）と info は --ci の ratchet に載らない
// ＝ exit code を動かさない。
//
// ⚠️ 射程: **baseline を張った状態でしか意味を持たない。** baseline 不在だと
// evaluateCI が BaselinePresent==false で早期 return するので ratchet 自体が非活性
// になり、「info を ratchet 対象にする」変異を入れても exit 0 のまま＝この検査は
// 空振りする（実際に一度そうなった）。空振りしていないことは、末尾の対照
// 「baseline に無い warn を足すと --ci だけ exit 1 になる」で示す。
func TestLintExitCode_FoldedSectionsDoNotAffectRatchet(t *testing.T) {
	dir := setupAckOnlyStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("baseline update failed: %v\n%s", err, out)
	}

	// acknowledge-only と info が残ったまま、どの面でも exit 0。
	for _, args := range [][]string{
		{"lint"}, {"lint", "--verbose"}, {"lint", "--ci"}, {"lint", "--verbose", "--ci"},
		{"lint", "--json"}, {"lint", "--json", "--verbose"}, {"lint", "--json", "--ci"},
	} {
		if out, err := run(t, dir, args...); err != nil {
			t.Errorf("%v が exit 非0 になった（acknowledge-only は info で exit に関与しないはず）: %v\n%s",
				args, err, out)
		}
	}

	// 対照: baseline に無い warn を 1 件足すと --ci だけ exit 1 になる。
	// これが落ちるなら、上の exit 0 は「ratchet が動いていないから」ではない。
	if out, err := run(t, dir, "tag", "create", "req.fresh", "--name", "新しい未充足要件", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create failed: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "lint", "--ci"); err == nil {
		t.Fatalf("baseline に無い warn を足したのに --ci が exit 0 のまま＝ratchet が動いていない\n%s", out)
	}
	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("既定の lint は warn で exit 0 のはず: %v\n%s", err, out)
	}
}
