package model

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// KindDecl は string|object の union（#45 D9）。string は id のみに縮退宣言、
// object は全欄。UnmarshalJSON はどちらからも復元し、MarshalJSON は縮退で書く。
func TestKindDeclUnmarshalStringAndObject(t *testing.T) {
	var s KindDecl
	if err := json.Unmarshal([]byte(`"axis"`), &s); err != nil {
		t.Fatalf("string decode: %v", err)
	}
	if s.ID != "axis" || s.Label != "" || s.Description != "" || len(s.Behaviors) != 0 {
		t.Fatalf("string decode = %+v, want bare {ID:axis}", s)
	}

	var o KindDecl
	obj := `{"id":"env","label":"環境","description":"プロセス外","behaviors":["axis"]}`
	if err := json.Unmarshal([]byte(obj), &o); err != nil {
		t.Fatalf("object decode: %v", err)
	}
	if o.ID != "env" || o.Label != "環境" || o.Description != "プロセス外" || !reflect.DeepEqual(o.Behaviors, []string{"axis"}) {
		t.Fatalf("object decode = %+v, want all fields", o)
	}
}

// 縮退 Marshal: label/description/behaviors がいずれも空なら string ID に縮退する
// （既存 string 宣言を round-trip で object に膨らませない＝git diff を汚さない・不変条件①）。
func TestKindDeclMarshalDegradesToString(t *testing.T) {
	bare := KindDecl{ID: "requirement"}
	data, err := json.Marshal(bare)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `"requirement"` {
		t.Fatalf("bare KindDecl marshaled to %s, want \"requirement\" (string form)", data)
	}

	full := KindDecl{ID: "input", Label: "入力", Description: "呼び出しの形", Behaviors: []string{"axis"}}
	data, err = json.Marshal(full)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(data), "{") {
		t.Fatalf("full KindDecl marshaled to %s, want object form", data)
	}
	var back KindDecl
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(back, full) {
		t.Fatalf("object round-trip = %+v, want %+v", back, full)
	}
}

// 後方互換: 旧 string 形式の config.json（tagKinds が string 配列・axis 含む・
// ownerKind なし）が従来と同一に parse され、Marshal で string 形に戻る（不変条件①）。
func TestConfigLegacyStringTagKindsRoundTrip(t *testing.T) {
	legacy := `{"schemaVersion":1,"kinds":{"condition":[],"action":["user"],"effect":["log"]},` +
		`"tagKinds":["requirement","concern","subject","axis"],"facetKinds":["subject"],` +
		`"traceabilityKinds":["requirement"],"idPrefix":{"condition":"cond.","action":"act.","effect":"eff."},` +
		`"roots":[],"viewer":{"port":4577}}`
	var cfg Config
	if err := json.Unmarshal([]byte(legacy), &cfg); err != nil {
		t.Fatalf("legacy config must decode: %v", err)
	}
	if got := cfg.TagKindIDs(); !reflect.DeepEqual(got, []string{"requirement", "concern", "subject", "axis"}) {
		t.Fatalf("TagKindIDs = %v, want [requirement concern subject axis]", got)
	}
	if cfg.OwnerKind != "" {
		t.Fatalf("legacy config OwnerKind = %q, want empty (unwired)", cfg.OwnerKind)
	}
	// Marshal で tagKinds は string 配列に戻る（object に膨らまない）。
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tagKinds":["requirement","concern","subject","axis"]`) {
		t.Fatalf("re-marshaled tagKinds not in string form: %s", data)
	}
	// ownerKind は未宣言なら omitempty で現れない。
	if strings.Contains(string(data), "ownerKind") {
		t.Fatalf("ownerKind must be omitted when empty: %s", data)
	}
}

// DefaultConfig の tagKinds は縮退 Marshal で string 形になる（既定 config が
// object に膨らまない・不変条件①）。ownerKind は既定空で現れない。
func TestDefaultConfigTagKindsMarshalString(t *testing.T) {
	data, err := json.Marshal(DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"tagKinds":["requirement","concern","subject"]`) {
		t.Fatalf("default tagKinds not string form: %s", data)
	}
	if strings.Contains(string(data), "ownerKind") {
		t.Fatalf("default ownerKind must be omitted: %s", data)
	}
}

// object 宣言（description 付き condition kind）が round-trip で保全される。
func TestConfigObjectConditionKindsRoundTrip(t *testing.T) {
	src := `{"schemaVersion":1,"kinds":{"condition":[` +
		`{"id":"input","label":"入力","description":"呼び出しの形"},` +
		`{"id":"env","label":"環境","description":"プロセス外"}],` +
		`"action":["user"],"effect":["log"]},` +
		`"tagKinds":["subject"],"facetKinds":["subject"],"traceabilityKinds":["requirement"],` +
		`"idPrefix":{"condition":"cond.","action":"act.","effect":"eff."},"roots":[],"viewer":{"port":4577}}`
	var cfg Config
	if err := json.Unmarshal([]byte(src), &cfg); err != nil {
		t.Fatalf("object condition kinds must decode: %v", err)
	}
	if len(cfg.Kinds.Condition) != 2 || cfg.Kinds.Condition[0].Description != "呼び出しの形" {
		t.Fatalf("condition kinds = %+v, want 2 with descriptions", cfg.Kinds.Condition)
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"description":"呼び出しの形"`) {
		t.Fatalf("object condition kind description lost on round-trip: %s", data)
	}
}

// TagKindLabel の3段解決（object Label → 互換 map → 素の id・#45 D9）。
func TestTagKindLabelResolution(t *testing.T) {
	cfg := Config{
		TagKinds:      []KindDecl{{ID: "env", Label: "環境"}, {ID: "subject"}},
		TagKindLabels: map[string]string{"subject": "主題"},
	}
	if got := cfg.TagKindLabel("env"); got != "環境" {
		t.Fatalf("object Label wins: got %q, want 環境", got)
	}
	if got := cfg.TagKindLabel("subject"); got != "主題" {
		t.Fatalf("compat map fallback: got %q, want 主題", got)
	}
	if got := cfg.TagKindLabel("unknown"); got != "unknown" {
		t.Fatalf("bare id fallback: got %q, want unknown", got)
	}
}

// KindHasBehavior: 明示宣言・axis 互換・非該当（#45 D9・不変条件③）。
func TestKindHasBehavior(t *testing.T) {
	cfg := Config{TagKinds: []KindDecl{
		{ID: "axis"}, // 旧 string axis 宣言相当（Behaviors 未宣言）
		{ID: "dimension", Behaviors: []string{"axis"}}, // 別名 kind の明示 axis 宣言
		{ID: "subject"}, // axis 挙動なし
	}}
	if !cfg.KindHasBehavior("axis", "axis") {
		t.Fatal("compat: kind=axis without explicit behaviors must have axis behavior")
	}
	if !cfg.KindHasBehavior("dimension", "axis") {
		t.Fatal("explicit behaviors:[axis] must have axis behavior")
	}
	if cfg.KindHasBehavior("subject", "axis") {
		t.Fatal("subject must not have axis behavior")
	}
	if cfg.KindHasBehavior("axis", "exclusive") {
		t.Fatal("unknown behavior must be false")
	}
}
