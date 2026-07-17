package cli

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nkenji09/scholia/internal/model"
	"github.com/nkenji09/scholia/internal/store"
)

type inferJSON struct {
	IDPolicy      model.IDPolicy `json:"idPolicy"`
	Distributions struct {
		Transition struct {
			Total    int            `json:"total"`
			Prefixes map[string]int `json:"prefixes"`
		} `json:"transition"`
	} `json:"distributions"`
}

func TestConfigInferIDPolicyProposesFromDistribution(t *testing.T) {
	dir := t.TempDir()
	s, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init: %v", err)
	}
	for _, v := range []model.VocabEntry{
		{ID: "cond.a", Category: model.CategoryCondition, Label: "a"},
		{ID: "cond.b", Category: model.CategoryCondition, Label: "b"},
		{ID: "act.x", Category: model.CategoryAction, Label: "x"},
	} {
		if err := s.SaveVocab(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.SaveTag(model.Tag{ID: "req.r", Name: "r", Kind: "requirement"}); err != nil {
		t.Fatal(err)
	}
	// 混在 store: T- が多数派・tx. が少数派 → T- を提案し内訳を開示する
	for _, id := range []string{"T-1", "T-2", "tx.3"} {
		if err := s.SaveTransition(model.Transition{ID: id, Action: "act.x", Then: []string{"eff.e"}}); err != nil {
			t.Fatal(err)
		}
	}

	out, err := run(t, dir, "config", "infer-id-policy")
	if err != nil {
		t.Fatalf("infer-id-policy: %v\noutput:\n%s", err, out)
	}
	for _, want := range []string{
		"transition: T- 2/3（内訳: T- 2, tx. 1）",
		"vocab condition: cond. 2/2",
		"vocab action: act. 1/1",
		"tag kind requirement: req. 1/1",
		"宣言案（config.json の idPolicy に手で追記する——このコマンドは書き込まない）:",
		`"transition": "T-"`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}

	// read-only: config.json に idPolicy が書き込まれていないこと
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.IDPolicy != nil {
		t.Fatalf("infer-id-policy must not write idPolicy: %+v", cfg.IDPolicy)
	}

	jsonOut, err := run(t, dir, "config", "infer-id-policy", "--json")
	if err != nil {
		t.Fatalf("--json: %v", err)
	}
	var resp inferJSON
	if err := json.Unmarshal([]byte(jsonOut), &resp); err != nil {
		t.Fatalf("json decode: %v\noutput:\n%s", err, jsonOut)
	}
	if resp.IDPolicy.Transition != "T-" {
		t.Fatalf("idPolicy.transition = %q, want T-", resp.IDPolicy.Transition)
	}
	if resp.IDPolicy.Vocab["condition"] != "cond." || resp.IDPolicy.TagByKind["requirement"] != "req." {
		t.Fatalf("idPolicy proposal wrong: %+v", resp.IDPolicy)
	}
	if resp.Distributions.Transition.Total != 3 || resp.Distributions.Transition.Prefixes["T-"] != 2 || resp.Distributions.Transition.Prefixes["tx."] != 1 {
		t.Fatalf("distribution wrong: %+v", resp.Distributions.Transition)
	}
}

// dogfood: 実 store の分布は全種別 100% 一貫で、宣言案は kit の infer 実測
// （kit-bundle2-retrofit-findings.md 末尾）と一致する。
func TestConfigInferIDPolicyDogfood(t *testing.T) {
	s, err := store.Discover(".")
	if err != nil {
		t.Fatalf("dogfood store not found: %v", err)
	}
	root := filepath.Dir(s.Dir)

	out, err := run(t, root, "config", "infer-id-policy", "--json")
	if err != nil {
		t.Fatalf("infer-id-policy on dogfood: %v", err)
	}
	var resp inferJSON
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("json decode: %v", err)
	}
	want := model.IDPolicy{
		Transition: "T-",
		Vocab:      map[string]string{"condition": "cond.", "action": "act.", "effect": "eff."},
		TagByKind:  map[string]string{"axis": "axis.", "concern": "concern.", "requirement": "req.", "subject": "subject."},
	}
	if resp.IDPolicy.Transition != want.Transition {
		t.Fatalf("transition = %q", resp.IDPolicy.Transition)
	}
	for k, v := range want.Vocab {
		if resp.IDPolicy.Vocab[k] != v {
			t.Fatalf("vocab[%s] = %q, want %q", k, resp.IDPolicy.Vocab[k], v)
		}
	}
	for k, v := range want.TagByKind {
		if resp.IDPolicy.TagByKind[k] != v {
			t.Fatalf("tagByKind[%s] = %q, want %q", k, resp.IDPolicy.TagByKind[k], v)
		}
	}
}
