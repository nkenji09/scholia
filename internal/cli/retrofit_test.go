package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/lint"
	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

// retrofitJSON は `scholia retrofit --json` の応答形。
type retrofitJSON struct {
	Rules    []string       `json:"rules"`
	Findings []lint.Finding `json:"findings"`
	Fixable  struct {
		FindingCount int            `json:"findingCount"`
		RecordCount  int            `json:"recordCount"`
		ByRule       map[string]int `json:"byRule"`
	} `json:"fixable"`
	AcknowledgeOnly struct {
		FindingCount int            `json:"findingCount"`
		RecordCount  int            `json:"recordCount"`
		ByRule       map[string]int `json:"byRule"`
	} `json:"acknowledgeOnly"`
}

// setupRetrofitStore は advisory 規則が広く発火する店構えを store API で組む
// （CLI の書き込みゲートを介さず、既存レコードの「先例汚染」を再現する）。
func setupRetrofitStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	cfg.TagKinds = append(cfg.TagKinds, "axis")
	if err := s.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	// derived-value＋stale-tense＋prose-ref＋dead-doc-ref が同時ヒットする axis desc
	// （axis-without-decision も：own decision なし）
	if err := s.SaveTag(model.Tag{ID: "axis.a", Name: "軸A", Kind: "axis", Total: true,
		Description: "値＝{cond.v1}。total=true。現状は #12 の新設。missing-doc.md を参照。"}); err != nil {
		t.Fatal(err)
	}
	for _, v := range []model.VocabEntry{
		{ID: "cond.v1", Category: model.CategoryCondition, Label: "v1", Tags: []string{"axis.a"}},
		{ID: "act.a", Category: model.CategoryAction, Label: "a"},
		{ID: "eff.a", Category: model.CategoryEffect, Label: "e"},
	} {
		if err := s.SaveVocab(v); err != nil {
			t.Fatal(err)
		}
	}
	// duplicate-atom（同一原子 2 本）
	for _, id := range []string{"T-d1", "T-d2"} {
		if err := s.SaveTransition(model.Transition{ID: id, Action: "act.a", Then: []string{"eff.a"}}); err != nil {
			t.Fatal(err)
		}
	}
	// why-file-line＋dangling-id（判断欄位＝acknowledge-only）
	if err := s.SaveDecision(model.Decision{
		ID:     "01AAAAAAAAAAAAAAAAAAAAAAAA",
		Target: model.DecisionTarget{Type: model.DecisionTargetTransition, ID: "T-d1"},
		Why:    "internal/a.go:12 を見て T-gone を廃止した",
		At:     "2026-07-17T00:00:00Z",
	}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRetrofitTextOutputSeparatesFixableAndAcknowledgeOnly(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit")
	if err != nil {
		t.Fatalf("retrofit must exit 0 even with findings: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"fixable（是正可能）:",
		"[derived-value-in-desc] tag axis.a（description）",
		"[stale-tense] tag axis.a（description）",
		"[prose-ref] tag axis.a（description）",
		"[axis-without-decision] tag axis.a",
		"[duplicate-atom] transition T-d1: T-d1・T-d2",
		"[dead-doc-ref] tag axis.a（description）: missing-doc.md",
		"acknowledge-only（decision 判断欄位・append-only により是正不能・容認で畳む対象）:",
		"[why-file-line] decision 01AAAAAAAAAAAAAAAAAAAAAAAA（why）: internal/a.go:12",
		"[dangling-id] decision 01AAAAAAAAAAAAAAAAAAAAAAAA（why）: T-gone",
		"→ 修正候補:",
		"集計: fixable 6 findings / 2 レコード・acknowledge-only 2 findings / 1 レコード",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("retrofit output missing %q:\n%s", want, out)
		}
	}
}

func TestRetrofitJSONCarriesCountsAndTier(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json: %v\noutput:\n%s", err, out)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v\noutput:\n%s", err, out)
	}
	if len(resp.Rules) != 8 {
		t.Fatalf("advisory 8 規則のはず: %v", resp.Rules)
	}
	for _, f := range resp.Findings {
		if f.Tier != lint.TierAdvisory || f.Severity != lint.SeverityInfo {
			t.Fatalf("finding must be tier=advisory severity=info: %+v", f)
		}
	}
	if resp.Fixable.FindingCount != 6 || resp.Fixable.RecordCount != 2 {
		t.Fatalf("fixable counts wrong: %+v", resp.Fixable)
	}
	if resp.AcknowledgeOnly.FindingCount != 2 || resp.AcknowledgeOnly.RecordCount != 1 {
		t.Fatalf("acknowledgeOnly counts wrong: %+v", resp.AcknowledgeOnly)
	}
	wantFixByRule := map[string]int{
		"derived-value-in-desc": 1, "stale-tense": 1, "prose-ref": 1, "why-file-line": 0,
		"axis-without-decision": 1, "duplicate-atom": 1, "dangling-id": 0, "dead-doc-ref": 1,
	}
	for rule, n := range wantFixByRule {
		if resp.Fixable.ByRule[rule] != n {
			t.Fatalf("fixable byRule[%s] = %d, want %d", rule, resp.Fixable.ByRule[rule], n)
		}
	}
	if resp.AcknowledgeOnly.ByRule["why-file-line"] != 1 || resp.AcknowledgeOnly.ByRule["dangling-id"] != 1 {
		t.Fatalf("acknowledgeOnly byRule wrong: %+v", resp.AcknowledgeOnly.ByRule)
	}
}

func TestRetrofitRuleFilterAndUnknownRule(t *testing.T) {
	dir := setupRetrofitStore(t)

	out, err := run(t, dir, "retrofit", "--rule", "dangling-id", "--json")
	if err != nil {
		t.Fatalf("retrofit --rule: %v\noutput:\n%s", err, out)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if len(resp.Rules) != 1 || resp.Rules[0] != "dangling-id" {
		t.Fatalf("rules should be filtered: %v", resp.Rules)
	}
	for _, f := range resp.Findings {
		if f.Rule != "dangling-id" {
			t.Fatalf("filtered run must not include %+v", f)
		}
	}
	if len(resp.Findings) != 1 {
		t.Fatalf("expected 1 dangling-id finding, got %+v", resp.Findings)
	}

	if _, err := run(t, dir, "retrofit", "--rule", "no-such-rule"); err == nil {
		t.Fatalf("unknown --rule must error")
	}
}

// dogfood 統合: 実 store の件数（kit-bundle2-retrofit-findings.md の実走値。
// desc-length は U3 のスコープなので 8 規則ベース＝fixable 27/18・ack 13/13）。
func TestRetrofitDogfoodCounts(t *testing.T) {
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store not found: %v", err)
	}
	root := filepath.Dir(s.Dir)

	out, err := run(t, root, "retrofit", "--json")
	if err != nil {
		t.Fatalf("retrofit --json on dogfood: %v", err)
	}
	var resp retrofitJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	if resp.Fixable.FindingCount != 27 || resp.Fixable.RecordCount != 18 {
		t.Fatalf("dogfood fixable = %d findings / %d records, want 27/18", resp.Fixable.FindingCount, resp.Fixable.RecordCount)
	}
	if resp.AcknowledgeOnly.FindingCount != 13 || resp.AcknowledgeOnly.RecordCount != 13 {
		t.Fatalf("dogfood acknowledgeOnly = %d findings / %d records, want 13/13", resp.AcknowledgeOnly.FindingCount, resp.AcknowledgeOnly.RecordCount)
	}
	if total := resp.Fixable.ByRule["dead-doc-ref"] + resp.AcknowledgeOnly.ByRule["dead-doc-ref"]; total != 19 {
		t.Fatalf("dogfood dead-doc-ref total = %d, want 19", total)
	}
}
