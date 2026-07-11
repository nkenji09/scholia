// Package model defines the record types persisted under .pmem/ (§3 of DESIGN.md).
package model

// カテゴリ（遷移の文法）は固定・設定不可（DESIGN §2）。
const (
	CategoryCondition = "condition"
	CategoryAction    = "action"
	CategoryEffect    = "effect"
)

// Transition は原子（§3.2）。given は集合（書き込み時にソート正規化）、then は順序リスト。
type Transition struct {
	ID     string   `json:"id"`
	Action string   `json:"action"`
	Given  []string `json:"given"`
	Then   []string `json:"then"`
	Tags   []string `json:"tags,omitempty"`
	Tests  []string `json:"tests,omitempty"`
}

func (t Transition) GetID() string { return t.ID }

// VocabEntry は語彙（condition/action/effect）1 件（§3.3）。
type VocabEntry struct {
	ID          string   `json:"id"`
	Category    string   `json:"category"`
	Label       string   `json:"label"`
	Kind        string   `json:"kind,omitempty"`
	Owner       string   `json:"owner,omitempty"` // effect のみ
	Tags        []string `json:"tags,omitempty"`
	Description string   `json:"description,omitempty"` // markdown・任意
}

func (v VocabEntry) GetID() string { return v.ID }

// Tag はネスト可能な横断分類（§3.4）。ParentIDs は多親 DAG。
type Tag struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Kind        string   `json:"kind,omitempty"`
	ParentIDs   []string `json:"parentIds,omitempty"`
	Description string   `json:"description,omitempty"`
	Color       string   `json:"color,omitempty"`
	Ref         string   `json:"ref,omitempty"`
}

func (t Tag) GetID() string { return t.ID }

// DecisionTarget は decision が指す対象（transition か tag）。
type DecisionTarget struct {
	Type string `json:"type"` // "transition" | "tag"
	ID   string `json:"id"`
}

const (
	DecisionTargetTransition = "transition"
	DecisionTargetTag        = "tag"
)

// Decision は意思決定 1 件（append-only・§3.5）。型定義のみ（decide コマンドは Phase 1）。
type Decision struct {
	ID      string         `json:"id"`
	Target  DecisionTarget `json:"target"`
	Why     string         `json:"why"`
	Changed string         `json:"changed,omitempty"`
	Ref     string         `json:"ref,omitempty"`
	At      string         `json:"at"` // RFC3339
}

func (d Decision) GetID() string { return d.ID }

// Kinds はカテゴリごとの kind 宣言集合（§3.6）。カテゴリ軸自体は固定のため型で表す（マップにしない）。
type Kinds struct {
	Condition []string `json:"condition"`
	Action    []string `json:"action"`
	Effect    []string `json:"effect"`
}

// IDPrefix は id の命名規約（ソフト・grep 用。強制は kind フィールドが担う）。
type IDPrefix struct {
	Condition string `json:"condition"`
	Action    string `json:"action"`
	Effect    string `json:"effect"`
}

// ViewerConfig はビューアの既定設定。
type ViewerConfig struct {
	Port int `json:"port"`
}

// DisplayConfig is additive cosmetic text the viewer shows (2026-07-11
// tweaks5 §1/§2) — HOME's tagline/intro and the header's product name.
// None of this affects record semantics; it's pure display customization
// (e.g. white-labeling pmem for a different project). Empty string means
// "use the built-in default" — the frontend resolves the fallback (see
// web/src/lookups.tsx), not this struct, so an older config.json (nil/
// zero-value fields) degrades gracefully without any Go-side migration.
type DisplayConfig struct {
	ProductName string `json:"productName,omitempty"`
	Tagline     string `json:"tagline,omitempty"`
	Intro       string `json:"intro,omitempty"`
}

// Config はプロジェクト設定（singleton・§3.6）。
type Config struct {
	PmemVersion       int          `json:"pmemVersion"`
	Kinds             Kinds        `json:"kinds"`
	TagKinds          []string     `json:"tagKinds"`
	FacetKinds        []string     `json:"facetKinds"`
	TraceabilityKinds []string     `json:"traceabilityKinds"`
	IDPrefix          IDPrefix     `json:"idPrefix"`
	Roots             []string     `json:"roots"`
	Viewer            ViewerConfig `json:"viewer"`
	// TagKindLabels is an additive, optional display-label map for
	// TagKinds entries (2026-07-11 tweaks3 §2: "requirement" → "要件" etc).
	// TagKinds itself stays the single source of truth for which kinds are
	// declared/valid — this only carries how to *show* one. A kind absent
	// here (including every kind in an older config.json predating this
	// field, which decodes it to a nil map) falls back to its bare id;
	// callers must resolve through that fallback rather than reading this
	// map directly (see web/src/lookups.tsx's tagKindLabel()).
	TagKindLabels map[string]string `json:"tagKindLabels"`
	Display       DisplayConfig     `json:"display"`
	// Branch is the current git branch name (2026-07-11 tweaks5 §2) — a
	// live derived value, NOT a stored preference. It's populated by the
	// viewer (GET /api/config) and by `pmem export --html` right before
	// each response/bake, never by DefaultConfig()/SaveConfig, so it never
	// ends up written into config.json. Empty when the project isn't a
	// git repo, HEAD is detached, or git itself isn't available — callers
	// fall back to a default display value (see
	// web/src/components/layout/Header.tsx) rather than showing nothing.
	Branch string `json:"branch,omitempty"`
}

// DefaultConfig は `pmem init` が書き出す既定値（§3.6 の例そのまま）。
func DefaultConfig() Config {
	return Config{
		PmemVersion: 1,
		Kinds: Kinds{
			Condition: []string{},
			Action:    []string{"user", "api", "lifecycle", "system", "cron", "webhook"},
			Effect:    []string{"emit", "state", "http", "storage", "log"},
		},
		TagKinds:          []string{"requirement", "concern", "subject"},
		FacetKinds:        []string{"subject", "requirement", "concern"},
		TraceabilityKinds: []string{"requirement"},
		IDPrefix: IDPrefix{
			Condition: "cond.",
			Action:    "act.",
			Effect:    "eff.",
		},
		Roots:  []string{},
		Viewer: ViewerConfig{Port: 4577},
		TagKindLabels: map[string]string{
			"requirement": "要件",
			"concern":     "関心事",
			"subject":     "主題",
		},
		Display: DisplayConfig{
			ProductName: "pmem",
			Tagline:     "記録を、読みたくなる形で。",
			Intro:       "product-memory は、プロダクトの意思決定・要件・振る舞いを原子（遷移）として記録し、構造は派生（query）で見るためのツールです。",
		},
	}
}

// KindsFor はカテゴリ名から config.kinds の該当スライスを返す（write-time / lint 共用）。
func (c Config) KindsFor(category string) []string {
	switch category {
	case CategoryCondition:
		return c.Kinds.Condition
	case CategoryAction:
		return c.Kinds.Action
	case CategoryEffect:
		return c.Kinds.Effect
	default:
		return nil
	}
}
