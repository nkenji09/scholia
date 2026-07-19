// Package model defines the record types persisted under .scholia/ (§3 of DESIGN.md).
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
	// Priority は同一 action 内での評価順の第一級宣言（#45 D8・additive/
	// omitempty）。nil=未宣言（従来どおり）・1 始まりの正整数・小さいほど先に
	// 評価される。同一 action 内でのみ意味を持ち、action をまたいだ比較は無意味。
	// flow は「あるグループの全遷移が相異なる priority を持つ」ときだけ overlap／
	// subset-shadow を『評価順で解決済み』に畳み、1つでも未宣言 or 同 priority が
	// 混じれば従来どおり『優先順位未定義』で報告する（保守的解決）。全遷移が
	// priority 宣言済みの action は最後尾（最大 priority 番号）が宣言的残余として
	// L-total を免除される（部分宣言は免除しない）。*int は nil=未宣言と 0 の区別が
	// 本質のため（U3）——保存されるのは 1 以上の値のみ。
	Priority *int `json:"priority,omitempty"`
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
	// Ref は外部契約・仕様本文へのアンカー（#45 D5・additive/omitempty）。契約の
	// 全文は desc に散文で埋めず、versioned 文書（DESIGN の § 参照・OpenAPI 等の
	// 外部正本）を指す1行。Tag.Ref との非対称を解消する。file:line 形式は
	// ref-freshness lint（warn）で警告される。
	Ref string `json:"ref,omitempty"`
	// AltLabels は別表記・同義語（#45 D5・additive/omitempty）。別の言い回しから
	// 既存語彙へ到達させ、重複新設を構造的に防ぐ。検索編入3面（CLI search・
	// viewer index 検索・viewer フィルタ/サジェスト）の検索対象に含まれる。
	AltLabels []string `json:"altLabels,omitempty"`
	// Establishes は「この効果が成立させる condition」の直接参照
	// （#45 D5・additive/omitempty・category=effect のみ有効）。値は現存 condition
	// の id（write-time 検証＋参照整合 lint）。transition 間の明示辺は持たないと
	// いう既決を維持したまま状態連鎖を機械可読にする（例: eff.state.save-scroll-
	// to-session が cond.view-scroll-in-session を成立させる）。
	Establishes []string `json:"establishes,omitempty"`
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
	// Total is meaningful only for kind=="axis" tags (#39 action-flow):
	// true means the axis's declared values are meant to partition every
	// world (exactly one value should hold), which is what makes a missing
	// value a sound gap (L-total). Additive/omitempty — irrelevant to any
	// other tag kind and absent from existing tag files.
	Total bool `json:"total,omitempty"`
	// Fulfillment は要件がどのように充足されるかの宣言（#45 D6・additive/
	// omitempty）。"" は "transitions" 扱い（既定＝遷移で充足される behavioral
	// 要件）・"property" は「遷移では構造的に充足されない性質型要件」（単一
	// バイナリ・ランタイム依存ゼロ等の非機能要件）。property のタグは
	// requirement-gap の遷移充足検査から外れるが、「acknowledges に
	// requirement-gap を含む decision が当該タグ宛てに存在する」ときのみ緑になる
	// （宣言だけでは畳まない＝怠慢な宣言を許さない）。
	Fulfillment string `json:"fulfillment,omitempty"`
}

// Tag.Fulfillment の値（#45 D6）。"" は FulfillmentTransitions と等価に扱う。
const (
	FulfillmentTransitions = "transitions"
	FulfillmentProperty    = "property"
)

func (t Tag) GetID() string { return t.ID }

// DecisionTarget は decision が指す対象（transition・tag・vocab）。
type DecisionTarget struct {
	Type string `json:"type"` // "transition" | "tag" | "vocab"
	ID   string `json:"id"`
}

const (
	DecisionTargetTransition = "transition"
	DecisionTargetTag        = "tag"
	// DecisionTargetVocab は decision の対象種別 vocab（#45 D5）。語彙の why・
	// 外部契約・状態連鎖の判断を語彙自身に付けられるようにする。旧バイナリは
	// 未知種別として decision-target lint で error 扱いするため、採用後は
	// 「バイナリ更新が先・レコード追加が後」の順序を守る（移行注記・D5 changed⑨）。
	DecisionTargetVocab = "vocab"
)

// Decision は意思決定 1 件（append-only・§3.5）。型定義のみ（decide コマンドは Phase 1）。
type Decision struct {
	ID      string         `json:"id"`
	Target  DecisionTarget `json:"target"`
	Why     string         `json:"why"`
	Changed string         `json:"changed,omitempty"`
	Ref     string         `json:"ref,omitempty"`
	At      string         `json:"at"` // RFC3339
	// Commits は実装来歴（git hash の集合）。判断フィールド（Target/Why/
	// Changed/Ref/At）は不変のまま、Commits だけ `scholia decision
	// add-commit` で追記できる（追加専用・単調増加・§3.5 append-only の
	// 精緻化）。omitempty により commits の無い旧 decision ファイルも無改修
	// で読める。
	Commits []string `json:"commits,omitempty"`
	// Acknowledges は「この decision が意図的に容認する finding の rule id 集合」
	// （#45 D6・additive/omitempty）。decide 時に rule id を実在照合し（typo は
	// 同一ターン error＋候補提示）、lint/flow の消費側が「当該 target 宛ての
	// acknowledges に該当 rule 名があれば finding を『容認済み（decision リンク
	// 付き）』に畳む」。祖先 decision では畳まない（無関係 decision による偽陰性
	// ＝untyped 容認を再導入しないため）。rule 改名で解決しなくなった宙吊り
	// acknowledges は lint dangling-acknowledges（info）が警告する。
	Acknowledges []string `json:"acknowledges,omitempty"`
	// Supersedes は「この decision が置き換える／改訂する／例外化する旧 decision」
	// への追記専用リンク集合（#45 D7・additive/omitempty）。旧 decision は無改変
	// のまま（新が旧を指す＝append-only 完全保持）。mode が現行性の意味を持ち、
	// derive 側は保守的に mode=supersede のみ失効扱いにする。link は
	// `scholia decision link` / `scholia decide --supersedes` で追記でき、判断
	// 欄位（why/changed/ref/at/target）は不可侵。
	Supersedes []SupersedeLink `json:"supersedes,omitempty"`
}

func (d Decision) GetID() string { return d.ID }

// SupersedeLink は decision → 旧 decision への現行性リンク（#45 D7）。Mode が
// 省略（""）のときは derive 側で ModeAmend（部分改訂）として扱う——保存は書かれた
// 値のまま（append-only なので既定補完で上書きしない）。
type SupersedeLink struct {
	ID   string `json:"id"`
	Mode string `json:"mode,omitempty"`
}

// SupersedeLink.Mode の3値（#45 D7）。
//   - ModeSupersede: 全文置換（旧を失効させる）。derive の --current で被参照を畳む唯一の mode。
//   - ModeAmend: 部分改訂（既定・旧は失効しない）。既定を amend にするのは
//     「失効させ忘れ」の系統誤りを避けるため（skill が decide 時に「全文置換か？」を必ず1問挟む）。
//   - ModeException: 一般則への意識的例外（旧は失効しない）。
const (
	ModeSupersede = "supersede"
	ModeAmend     = "amend"
	ModeException = "exception"
)

// SupersedeMode は link の mode を返す（空なら既定 ModeAmend・derive 用の
// 補完。保存値は書き換えない）。
func (l SupersedeLink) SupersedeMode() string {
	if l.Mode == "" {
		return ModeAmend
	}
	return l.Mode
}

// ValidSupersedeMode は mode 文字列が3値のいずれか（空＝既定 amend も許容）かを返す。
func ValidSupersedeMode(mode string) bool {
	switch mode {
	case "", ModeSupersede, ModeAmend, ModeException:
		return true
	}
	return false
}

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
// (e.g. white-labeling scholia for a different project). Empty string means
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
	SchemaVersion     int          `json:"schemaVersion"`
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
	// viewer (GET /api/config) and by `scholia export --html` right before
	// each response/bake, never by DefaultConfig()/SaveConfig, so it never
	// ends up written into config.json. Empty when the project isn't a
	// git repo, HEAD is detached, or git itself isn't available — callers
	// fall back to a default display value (see
	// web/src/components/layout/Header.tsx) rather than showing nothing.
	Branch string `json:"branch,omitempty"`
	// SourceRefs is additive, optional config for the source-reference
	// scanner/rewriter (`scholia {tag|vocab|tx} rename --rewrite-refs`,
	// `scholia refs scan|rewrite`). nil (the DefaultConfig()/omitted-field
	// case) means "use built-in defaults": scan the whole project root,
	// no extra excludes beyond the always-excluded .scholia/.git/_workspace/
	// .concierge (those are not configurable). This intentionally does
	// NOT reuse Roots — Roots is a separate, still-unwired concept
	// (extra *record* discovery roots, see the field below) and a past
	// decision on req.record-maintenance already ruled it out of scope
	// for rename's reference integrity, to avoid conflating record
	// discovery with source scanning under one setting.
	SourceRefs *SourceRefs `json:"sourceRefs,omitempty"`
	// IDPolicy is additive (#45 U2): id prefix declarations with write-time
	// enforcement semantics (new ids only — wired in P3). nil means "no
	// declaration" and everything behaves as before; IDPrefix above stays
	// convention-only and untouched.
	IDPolicy *IDPolicy `json:"idPolicy,omitempty"`
	// Lint is additive (#45 U2): tuning knobs for advisory lint rules.
	Lint *LintConfig `json:"lint,omitempty"`
}

// SourceRefs scopes where `scholia refs scan|rewrite` and rename's implicit
// source scan look for id references, additive to Config so existing
// config.json files decode unchanged (a nil SourceRefs is indistinguishable
// from an absent field). Scan/Exclude are project-root-relative path
// prefixes.
type SourceRefs struct {
	Scan    []string `json:"scan,omitempty"`
	Exclude []string `json:"exclude,omitempty"`
}

// IDPolicy は id prefix の宣言（#45 U2・additive）。既存 IDPrefix（vocab 3
// カテゴリの「慣例のみ・強制なし」）とは意味論の異なる別キーで非破壊に共存する:
// 宣言された prefix は書き込みゲート（P3）で新規 id にのみ強制され、既存 id は
// 対象外。lint の dangling-id は宣言 prefix を id 様トークンの候補集合に加える。
// `scholia config infer-id-policy` が既存 id 分布から宣言案を出す（書き込みは
// しない——実宣言は各 store の運用判断）。
type IDPolicy struct {
	// Transition は transition id の prefix（例 "T-"・"tx."）。
	Transition string `json:"transition,omitempty"`
	// Vocab は vocab カテゴリ（condition/action/effect）→ prefix。
	Vocab map[string]string `json:"vocab,omitempty"`
	// TagByKind は tag kind（axis/requirement/…）→ prefix。
	TagByKind map[string]string `json:"tagByKind,omitempty"`
}

// LintConfig は advisory lint（authoring 規律・#45 U2）の検出調整。additive/
// omitempty——既存 config.json は無改修で読める（nil = 全て既定値）。
type LintConfig struct {
	// StalePatternExcludes は stale-tense の除外正規表現（検出語がいずれかに
	// マッチしたら finding にしない）。初期値は空集合（最小で開始・決定⑥）。
	StalePatternExcludes []string `json:"stalePatternExcludes,omitempty"`
	// PlaceholderSegments は dangling-id の除外 (E2) プレースホルダ語彙への
	// 追加分。built-in（xxx/yyy/foo/bar/foobar/foo-bar/example/sample/dummy）
	// に加算される（置換ではない）。
	PlaceholderSegments []string `json:"placeholderSegments,omitempty"`
}

// DefaultConfig は `scholia init` が書き出す既定値（§3.6 の例そのまま）。
func DefaultConfig() Config {
	return Config{
		SchemaVersion: 1,
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
			ProductName: "scholia",
			Tagline:     "記録を、読みたくなる形で。",
			Intro:       "scholia は、プロダクトの意思決定・要件・振る舞いを原子（遷移）として記録し、構造は派生（query）で見るためのツールです。",
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
