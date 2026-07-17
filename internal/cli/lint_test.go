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
func setupLintCoverageStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	steps := [][]string{
		{"init"},
		{"vocab", "add", "action", "act.a", "--label", "a"},
		{"vocab", "add", "effect", "eff.a", "--label", "e"},
		{"tag", "create", "req.a", "--name", "要件A", "--kind", "requirement"},
		{"tx", "add", "T-covered", "--action", "act.a", "--then", "eff.a", "--tags", "req.a"},
		{"tx", "add", "T-bare", "--action", "act.a", "--then", "eff.a"},
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
