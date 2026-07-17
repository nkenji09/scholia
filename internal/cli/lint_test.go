package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
)

// setupLintCoverageStore は decision-coverage の3段（via-tag / none →
// decide 後に direct）を再現する最小 store を組む。
//   - T-covered: req.a を own タグに持ち、req.a 宛 decision に via-tag で到達
//   - T-bare:    own にも実効タグにも decision なし（none）
//
// T-bare の then は eff.b（T-covered と別）にして、U2 の duplicate-atom
// advisory（同一 action＋given＋then の複製検出）に掛からないようにする。
func setupLintCoverageStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"vocab", "add", "effect", "eff.b", "--label", "e2"},
		{"tag", "create", "req.a", "--name", "要件A", "--kind", "requirement"},
		{"tx", "add", "T-covered", "--action", "act.a", "--then", "eff.a", "--tags", "req.a"},
		{"tx", "add", "T-bare", "--action", "act.a", "--then", "eff.b"},
		{"decide", "--on", "tag:req.a", "--why", "タグ側の決定"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

// 既定出力: decision-coverage は none のみ列挙され、3段の件数がサマリ行に出る。
func TestLintDefaultShowsOnlyNoneCoverageWithSummary(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage: direct 0 / via-tag 1 / none 1（via-tag の内訳は --verbose）") {
		t.Fatalf("summary line missing or wrong:\n%s", out)
	}
	if !strings.Contains(out, "transition T-bare: own にも実効タグにも decision が 1 件もありません（none・why 未記録）") {
		t.Fatalf("none finding for T-bare missing:\n%s", out)
	}
	if strings.Contains(out, "transition T-covered") {
		t.Fatalf("via-tag finding must not be listed in default output:\n%s", out)
	}

	// direct 化: T-bare に own decision を付けると none が消え direct が増える。
	if o, err := run(t, dir, "decide", "--on", "transition:T-bare", "--why", "遷移固有の決定"); err != nil {
		t.Fatalf("decide on transition failed: %v\noutput:\n%s", err, o)
	}
	out, err = run(t, dir, "lint")
	if err != nil {
		t.Fatalf("lint failed after decide: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage: direct 1 / via-tag 1 / none 0") {
		t.Fatalf("summary after decide missing or wrong:\n%s", out)
	}
	if strings.Contains(out, "transition T-bare") {
		t.Fatalf("direct-covered transition must not be listed:\n%s", out)
	}
}

// --verbose: via-tag の内訳（どのタグ経由か・decision 件数）が展開される。
func TestLintVerboseExpandsViaTagProvenance(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint", "--verbose")
	if err != nil {
		t.Fatalf("lint --verbose failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "decision-coverage via-tag の内訳:") {
		t.Fatalf("verbose breakdown header missing:\n%s", out)
	}
	if !strings.Contains(out, "  T-covered: via req.a (1)") {
		t.Fatalf("verbose breakdown line for T-covered missing:\n%s", out)
	}
}

// --json: decision-coverage は direct/via-tag/none の全件が coverage（と via-tag
// の detail）付きで出る。封筒の形は不変（findings + counts）。
func TestLintJSONCarriesAllCoverageFindings(t *testing.T) {
	dir := setupLintCoverageStore(t)

	out, err := run(t, dir, "lint", "--json")
	if err != nil {
		t.Fatalf("lint --json failed: %v\noutput:\n%s", err, out)
	}
	var resp struct {
		Findings []lint.Finding `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode failed: %v\noutput:\n%s", err, out)
	}
	coverage := make(map[string]lint.Finding)
	for _, f := range resp.Findings {
		if f.Coverage != "" {
			coverage[f.Target] = f
		}
	}
	if len(coverage) != 2 {
		t.Fatalf("expected coverage findings for all 2 transitions, got %+v", coverage)
	}
	if f := coverage["T-covered"]; f.Coverage != lint.CoverageViaTag || f.Detail != "via req.a (1)" {
		t.Fatalf("T-covered = %+v, want via-tag with detail 'via req.a (1)'", f)
	}
	if f := coverage["T-bare"]; f.Coverage != lint.CoverageNone {
		t.Fatalf("T-bare = %+v, want none", f)
	}
}

// --- #45 U4: lint --ci（baseline ratchet）と lint baseline update ---

// setupRatchetStore は requirement-gap warn（req.gap: 充足遷移 0 件）が 1 件
// 出る最小 store を組む。
func setupRatchetStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"tag", "create", "req.gap", "--name", "未充足要件", "--kind", "requirement"},
	}
	for _, s := range steps {
		if out, err := run(t, dir, s...); err != nil {
			t.Fatalf("%v failed: %v\noutput:\n%s", s, err, out)
		}
	}
	return dir
}

func TestLintCI_NoBaselineIsInactiveRatchet(t *testing.T) {
	dir := setupRatchetStore(t)

	// baseline 不在: warn が出ていても --ci は fail しない（opt-in）。
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("baseline 不在の lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "非活性") {
		t.Fatalf("expected inactive-ratchet note:\n%s", out)
	}
}

func TestLintCI_BaselineRatchet(t *testing.T) {
	dir := setupRatchetStore(t)

	// baseline update で現在の warn 1 件（requirement-gap req.gap）を吸収。
	out, err := run(t, dir, "lint", "baseline", "update")
	if err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warn 1 件") {
		t.Fatalf("expected 1-entry baseline summary:\n%s", out)
	}

	// baseline に吸収済み → exit 0。
	out, err = run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("baseline 吸収済みの lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0（baseline 1 件・stale 0 件）") {
		t.Fatalf("expected ratchet summary:\n%s", out)
	}

	// 新規 warn（もう 1 つの未充足 requirement タグ）を作る → exit 1。
	if out, err := run(t, dir, "tag", "create", "req.gap2", "--name", "新規未充足", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}
	out, err = run(t, dir, "lint", "--ci")
	if err == nil {
		t.Fatalf("baseline に無い新規 warn で lint --ci が exit 0 になった:\n%s", out)
	}
	if !strings.Contains(out, "requirement-gap: req.gap2") {
		t.Fatalf("expected the new warn to be listed by rule+target:\n%s", out)
	}
	if !strings.Contains(out, "baseline update") {
		t.Fatalf("expected remediation hint:\n%s", out)
	}

	// 既定の lint は従来契約のまま（warn は exit 0）。
	if out, err := run(t, dir, "lint"); err != nil {
		t.Fatalf("既定 lint の exit 契約が変わっている: %v\n%s", err, out)
	}
}

func TestLintCI_StaleEntryIsInfoOnly(t *testing.T) {
	dir := setupRatchetStore(t)

	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	// warn の原因を解消（req.gap を充足する遷移を追加）→ baseline entry が stale 化。
	if out, err := run(t, dir, "tx", "add", "T-fill", "--action", "act.a", "--then", "eff.a", "--tags", "req.gap"); err != nil {
		t.Fatalf("tx add: %v\n%s", err, out)
	}

	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("stale entry だけの lint --ci が fail した: %v\n%s", err, out)
	}
	if !strings.Contains(out, "stale baseline entry: requirement-gap req.gap") {
		t.Fatalf("expected stale info line:\n%s", out)
	}

	// 次の baseline update で自然消滅（削除 1）。
	out, err = run(t, dir, "lint", "baseline", "update")
	if err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if !strings.Contains(out, "warn 0 件") || !strings.Contains(out, "削除 1") {
		t.Fatalf("expected shrink summary:\n%s", out)
	}
}

func TestLintCI_InfoAndAdvisoryAreNotRatcheted(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	// info（unused-vocab 等）や advisory を増やしても --ci は fail しない。
	// cond.unused はどの遷移からも参照されない語彙 → unused-vocab info。
	if out, err := run(t, dir, "vocab", "add", "condition", "cond.unused", "--label", "未使用"); err != nil {
		t.Fatalf("vocab add: %v\n%s", err, out)
	}
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("info の増加で lint --ci が fail した（ratchet は warn 専用のはず）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0") {
		t.Fatalf("expected zero new warns:\n%s", out)
	}
}

func TestLintCI_JSONCarriesCIResult(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}
	if out, err := run(t, dir, "tag", "create", "req.gap2", "--name", "新規未充足", "--kind", "requirement"); err != nil {
		t.Fatalf("tag create: %v\n%s", err, out)
	}

	out, err := run(t, dir, "lint", "--ci", "--json")
	if err == nil {
		t.Fatalf("新規 warn ありの lint --ci --json が exit 0:\n%s", out)
	}
	// エラーメッセージ行が JSON の後に混ざるため、先頭の JSON オブジェクトだけを decode する。
	dec := json.NewDecoder(strings.NewReader(out))
	var parsed struct {
		CI struct {
			BaselinePresent bool `json:"baselinePresent"`
			BaselineCount   int  `json:"baselineCount"`
			NewWarns        []struct {
				Rule   string `json:"rule"`
				Target string `json:"target"`
			} `json:"newWarns"`
		} `json:"ci"`
	}
	if err := dec.Decode(&parsed); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if !parsed.CI.BaselinePresent || parsed.CI.BaselineCount != 1 {
		t.Fatalf("ci envelope = %+v", parsed.CI)
	}
	if len(parsed.CI.NewWarns) != 1 || parsed.CI.NewWarns[0].Rule != "requirement-gap" || parsed.CI.NewWarns[0].Target != "req.gap2" {
		t.Fatalf("newWarns = %+v", parsed.CI.NewWarns)
	}
}

func TestLintBaseline_RenameRetargetsEntries(t *testing.T) {
	dir := setupRatchetStore(t)
	if out, err := run(t, dir, "lint", "baseline", "update"); err != nil {
		t.Fatalf("lint baseline update: %v\n%s", err, out)
	}

	// tag rename → baseline 内の target id が追随し、新 id では新規 warn に
	// ならない（旧 id のままなら req.gap-renamed が新規 warn になり exit 1）。
	if out, err := run(t, dir, "tag", "rename", "req.gap", "req.gap-renamed", "--no-refs"); err != nil {
		t.Fatalf("tag rename: %v\n%s", err, out)
	}
	out, err := run(t, dir, "lint", "--ci")
	if err != nil {
		t.Fatalf("rename 後の lint --ci が fail した（baseline 追随漏れ）: %v\n%s", err, out)
	}
	if !strings.Contains(out, "新規 warn 0（baseline 1 件・stale 0 件）") {
		t.Fatalf("expected retargeted baseline to absorb the renamed warn:\n%s", out)
	}
}
