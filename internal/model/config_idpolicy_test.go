package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// idPolicy / lint は additive（#45 U2）: 既存 config.json は無改修で読め、
// 未宣言の config を書き出しても新キーは現れない。
func TestConfigIDPolicyAndLintAreAdditive(t *testing.T) {
	// 旧形式（新キー無し）の config が黙って読めること
	legacy := `{"schemaVersion":1,"kinds":{"condition":[],"action":["user"],"effect":["log"]},` +
		`"tagKinds":["requirement"],"facetKinds":["requirement"],"traceabilityKinds":["requirement"],` +
		`"idPrefix":{"condition":"cond.","action":"act.","effect":"eff."},"roots":[],"viewer":{"port":4577}}`
	var cfg Config
	if err := json.Unmarshal([]byte(legacy), &cfg); err != nil {
		t.Fatalf("legacy config must decode: %v", err)
	}
	if cfg.IDPolicy != nil || cfg.Lint != nil {
		t.Fatalf("absent keys must decode to nil: idPolicy=%+v lint=%+v", cfg.IDPolicy, cfg.Lint)
	}

	// 未宣言のままの marshal は新キーを出さない（既存 config.json を汚さない）
	data, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"idPolicy", "lint"} {
		if strings.Contains(string(data), key) {
			t.Fatalf("undeclared %s must be omitted from marshal: %s", key, data)
		}
	}

	// 宣言の round-trip
	declared := DefaultConfig()
	declared.IDPolicy = &IDPolicy{
		Transition: "T-",
		Vocab:      map[string]string{"condition": "cond."},
		TagByKind:  map[string]string{"axis": "axis."},
	}
	declared.Lint = &LintConfig{
		StalePatternExcludes: []string{`^現在は$`},
		PlaceholderSegments:  []string{"tbd"},
	}
	data, err = json.Marshal(declared)
	if err != nil {
		t.Fatal(err)
	}
	var back Config
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back.IDPolicy, declared.IDPolicy) {
		t.Fatalf("idPolicy round-trip: %+v != %+v", back.IDPolicy, declared.IDPolicy)
	}
	if !reflect.DeepEqual(back.Lint, declared.Lint) {
		t.Fatalf("lint round-trip: %+v != %+v", back.Lint, declared.Lint)
	}
}
