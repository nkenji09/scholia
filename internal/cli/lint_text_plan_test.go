package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
)

// ---------------------------------------------------------------------------
// 何に対して落ちるガードか
//
// lint のテキスト出力は「4区分 × 出力モード」で粒度が決まる。この表は3つの穴を
// 同時に塞ぐために、1つの表として書いてある。
//
//  1. planLintText の粒度が変わったら落ちる（TestLintTextPlan_GranularityTable）。
//     期待値は表に literal で書いてあるので、実装から導出されない。
//  2. renderLintText が plan を無視して描いたら落ちる
//     （TestLintTextRender_FollowsPlanForEverySection）。期待値は plan から導出する
//     ので、区分が増えても検査が素通りしない。
//  3. 既定で畳んだ区分が別の面（--ci / --json）から漏れたら落ちる
//     （TestLintCI_TextGranularityMatchesDefault / TestLintJSON_CarriesFoldedSections）。
//
// ⚠️ 落ちないこと: 「明細を出すコードがソースに書いてある／書いていない」は見て
// いない。見ているのは合成 finding を入れたときに実際に描かれた行だけなので、
// 同じ意味を別の綴りで書き直しても検査は通る——それは原理的にこの形では取れない。
// ---------------------------------------------------------------------------

// 区分ごとの marker。Message / Target / Detail に同じ値を入れるので、
// 「marker を含む行の数」＝「その finding が明細として描かれた行数」になる。
// 互いに接頭辞にならない綴りにしてある。
const (
	mkDisplayedWarn  = "MK-DISPLAYED-WARN"
	mkDisplayedInfo  = "MK-DISPLAYED-INFO"
	mkCoverageNone   = "MK-COVERAGE-NONE"
	mkCoverageViaTag = "MK-COVERAGE-VIATAG"
	mkCoverageDirect = "MK-COVERAGE-DIRECT"
	mkTypedAck       = "MK-TYPEDACK"
	mkAckOnlyA       = "MK-ACKONLY-A"
	mkAckOnlyB       = "MK-ACKONLY-B"
)

// guardFindings は4区分すべてを非空にした合成 findings。
// どの区分も空でないことが、区分ごとの検査が空振りしない前提になる。
func guardFindings() []lint.Finding {
	mark := func(f lint.Finding, m string) lint.Finding {
		f.Message, f.Target, f.Detail = m, m, m
		return f
	}
	return []lint.Finding{
		mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn}, mkDisplayedWarn),
		mark(lint.Finding{Rule: "unused-vocab", Severity: lint.SeverityInfo}, mkDisplayedInfo),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageNone}, mkCoverageNone),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageViaTag}, mkCoverageViaTag),
		mark(lint.Finding{Rule: "decision-coverage", Severity: lint.SeverityInfo, Coverage: lint.CoverageDirect}, mkCoverageDirect),
		mark(lint.Finding{Rule: "requirement-gap", Severity: lint.SeverityWarn, AcknowledgedBy: "01TESTDECISIONID"}, mkTypedAck),
		mark(lint.Finding{Rule: "why-file-line", Severity: lint.SeverityInfo, AcknowledgeOnly: true}, mkAckOnlyA),
		mark(lint.Finding{Rule: "dangling-id", Severity: lint.SeverityInfo, AcknowledgeOnly: true}, mkAckOnlyB),
	}
}

// 表1: 区分 × モード → 明細粒度。期待値は literal（実装から導出しない）。
func TestLintTextPlan_GranularityTable(t *testing.T) {
	table := []struct {
		mode    string
		verbose bool
		want    map[lintSection]lintDetail
	}{
		{"既定", false, map[lintSection]lintDetail{
			lintSectionDisplayed: lintDetailAll,  // error/warn は exit code と ratchet の根拠ゆえ畳まない
			lintSectionTypedAck:  lintDetailNone, // 件数のみ
			lintSectionAckOnly:   lintDetailNone, // 件数のみ（本 decision で明細→件数へ）
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
			lintSectionTypedAck:  {mkTypedAck},
			lintSectionAckOnly:   {mkAckOnlyA, mkAckOnlyB},
			lintSectionCoverage:  {mkCoverageViaTag}, // direct は内訳に出さない
		}
		for sec, w := range want {
			if strings.Join(got[sec], ",") != strings.Join(w, ",") {
				t.Errorf("verbose=%v: 区分 %s = %v, want %v", verbose, sec, got[sec], w)
			}
		}
		if p.CoverageDirect != 1 || p.CoverageViaTag != 1 || p.CoverageNone != 1 {
			t.Errorf("verbose=%v: 3段の件数 = direct %d / via-tag %d / none %d, want 1/1/1",
				verbose, p.CoverageDirect, p.CoverageViaTag, p.CoverageNone)
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
		var buf bytes.Buffer
		renderLintText(&buf, p)
		out := buf.String()

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

		// 件数は粒度に依らず必ず出る（畳んでも「何件あるか」は消えない）。
		for _, want := range []string{
			"typed 容認済み（decision で意図的に残す gap・#45 D6）: 1 件",
			"acknowledge-only",
			"decision-coverage: direct 1 / via-tag 1 / none 1",
		} {
			if !strings.Contains(out, want) {
				t.Errorf("verbose=%v: 件数行 %q が無い\n出力:\n%s", verbose, want, out)
			}
		}
		// 畳んだ先の案内は既定のときだけ要る。
		if !verbose && !strings.Contains(out, "scholia retrofit") {
			t.Errorf("既定出力に retrofit への導線が無い\n出力:\n%s", out)
		}
	}
}

// coverage の direct は3段の件数にだけ現れ、どのモードでも明細に出ない。
func TestLintTextRender_CoverageDirectIsNeverDetailed(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		var buf bytes.Buffer
		renderLintText(&buf, planLintText(guardFindings(), verbose))
		if n := countLinesContaining(buf.String(), mkCoverageDirect); n != 0 {
			t.Errorf("verbose=%v: direct が明細に %d 行出ている\n出力:\n%s", verbose, n, buf.String())
		}
	}
}

// 是正対象が空でも、畳んだ区分が残っていれば「問題は見つかりませんでした」とは言わない
// ——畳みが「信じられる緑」を偽らないこと。
func TestLintTextRender_FoldedSectionsDoNotClaimClean(t *testing.T) {
	only := []lint.Finding{
		{Rule: "why-file-line", Severity: lint.SeverityInfo, AcknowledgeOnly: true, Target: mkAckOnlyA, Message: mkAckOnlyA},
	}
	var buf bytes.Buffer
	renderLintText(&buf, planLintText(only, false))
	if strings.Contains(buf.String(), "問題は見つかりませんでした") {
		t.Fatalf("acknowledge-only が残っているのに緑を名乗った\n出力:\n%s", buf.String())
	}
	if !strings.Contains(buf.String(), "acknowledge-only") {
		t.Fatalf("件数行が無い\n出力:\n%s", buf.String())
	}
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

// ---------------------------------------------------------------------------
// 面ごとの検査（--ci / --json / exit code）——CLI を通した実出力で見る。
// ---------------------------------------------------------------------------

// setupAckOnlyStore は acknowledge-only（why-file-line）と typed 容認と
// coverage none を同時に含む最小 store を組む。
func setupAckOnlyStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"tx", "add", "T-bare", "--action", "act.a", "--then", "eff.a"},
		{"tag", "create", "req.gap", "--name", "未充足要件", "--kind", "requirement"},
		// why に file:line を書いた decision＝why-file-line（acknowledge-only）を生む。
		{"decide", "--on", "tag:req.gap", "--why", "# 見出し\n\n判断の根拠は internal/example/sample.go:58 にある"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

// 新しく畳んだ区分が --ci の面から漏れていないこと（--ci は既定テキストを描いてから
// CI 行を足す実装なので、既定と同じ粒度でなければならない）。
func TestLintCI_TextGranularityMatchesDefault(t *testing.T) {
	dir := setupAckOnlyStore(t)

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

	const detailMark = "  [info] why-file-line:"
	if strings.Contains(def, detailMark) {
		t.Errorf("既定出力に acknowledge-only の明細が出ている\n%s", def)
	}
	if strings.Contains(ci, detailMark) {
		t.Errorf("--ci の出力に acknowledge-only の明細が出ている（既定と粒度が違う）\n%s", ci)
	}
	if !strings.Contains(verbose, detailMark) {
		t.Errorf("--verbose で acknowledge-only の明細が出ていない\n%s", verbose)
	}
	for _, out := range []string{def, ci, verbose} {
		if !strings.Contains(out, "acknowledge-only") {
			t.Errorf("件数行が無い\n%s", out)
		}
	}
	if !strings.Contains(def, "scholia retrofit") {
		t.Errorf("既定出力に retrofit への導線が無い\n%s", def)
	}
}

// --json は畳みの影響を受けず、acknowledge-only の finding を全件透過する
// （viewer など JSON の消費者は表示粒度の変更で欠けてはいけない）。
func TestLintJSON_CarriesFoldedSections(t *testing.T) {
	dir := setupAckOnlyStore(t)

	for _, args := range [][]string{{"lint", "--json"}, {"lint", "--json", "--verbose"}, {"lint", "--json", "--ci"}} {
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
		n := 0
		for _, f := range payload.Findings {
			if f.AcknowledgeOnly {
				n++
			}
		}
		if n == 0 {
			t.Errorf("%v: --json に acknowledge-only の finding が 1 件も無い\n%s", args, out)
		}
	}
}

// 畳んだ区分は exit code に一切関与しない（どのモードでも同じ exit）。
func TestLintExitCode_UnaffectedByFoldedSections(t *testing.T) {
	dir := setupAckOnlyStore(t)

	for _, args := range [][]string{
		{"lint"}, {"lint", "--verbose"}, {"lint", "--ci"},
		{"lint", "--json"}, {"lint", "--json", "--verbose"}, {"lint", "--json", "--ci"},
	} {
		if out, err := run(t, dir, args...); err != nil {
			t.Errorf("%v が exit 非0 になった（acknowledge-only は info で exit に関与しないはず）: %v\n%s",
				args, err, out)
		}
	}
}
