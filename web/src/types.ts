export interface Transition {
  id: string;
  action: string;
  given: string[];
  then: string[];
  tags?: string[];
}

export interface VocabEntry {
  id: string;
  category: string;
  label: string;
  kind?: string;
  owner?: string;
  tags?: string[];
  description?: string;
  // #45 D5 の関係スロット（additive/omitempty・Go 側 model.VocabEntry と同期）。
  /** 外部契約・仕様本文へのアンカー（1行・本文は versioned 文書へ）。 */
  ref?: string;
  /** 別表記・同義語（検索編入で重複新設を防ぐ）。 */
  altLabels?: string[];
  /** この効果が成立させる condition の id（effect のみ・状態連鎖の機械可読辺）。 */
  establishes?: string[];
}

export interface Tag {
  id: string;
  name: string;
  kind?: string;
  parentIds?: string[];
  description?: string;
  color?: string;
  ref?: string;
  /** 軸タグの網羅性宣言（#39・kind=axis のみ意味を持つ）。 */
  total?: boolean;
  /** 要件の充足型（#45 D6）: "property"（遷移では充足されない性質型要件）・
      "transitions"（既定）。Go 側 model.Tag.Fulfillment と同期。 */
  fulfillment?: string;
}

/** decision → 旧 decision への現行性リンク（#45 D7・Go 側 model.SupersedeLink と同期）。 */
export interface SupersedeLink {
  id: string;
  /** supersede | amend | exception（省略時は amend 扱い）。 */
  mode?: string;
}

export interface DecisionTarget {
  type: 'transition' | 'tag' | 'vocab';
  id: string;
}

export interface Decision {
  id: string;
  target: DecisionTarget;
  why: string;
  changed?: string;
  ref?: string;
  at: string;
  commits?: string[];
  /** 意図的に容認する finding の rule id 集合（#45 D6）。 */
  acknowledges?: string[];
  /** 置き換え/改訂/例外化する旧 decision へのリンク（#45 D7）。 */
  supersedes?: SupersedeLink[];
}

// POST /api/decision body（change-cockpit-design-v3.md §1/§8.5・採用フロー）。
// commits は採用時点では常に空 — 着地（人が commit）後に
// `scholia decision add-commit` で追記される後工程（本 P4 の実装範囲外）。
export interface DecisionPostBody {
  on: string;
  why: string;
  changed?: string;
  ref?: string;
  commits: string[];
}

// POST /api/transition body（change-cockpit-design-v3.md §1 (Wp)/§8.8 P3・
// 提案の手直し・G-1′ 承認済み）。action/given/then/tags は vocab-id/tag-id
// のみ（自由記述の label/description フィールドは無い — 構造ガードは型その
// ものが担う。internal/viewer/transition_write.go の transitionPostBody と
// 1:1）。id が既存なら編集（200）、未実在なら新規作成（201・§8.8 P5・
// api.ts の createTransition/putTransition はどちらもこの型・同一
// エンドポイントを使う）。given/then/tags は常にフル置換（tx add と同じ
// 全体指定、tx edit の部分更新ではない）。
export interface TransitionPostBody {
  id: string;
  action: string;
  given: string[];
  then: string[];
  tags: string[];
}

export interface Kinds {
  condition: string[];
  action: string[];
  effect: string[];
}

export interface IDPrefix {
  condition: string;
  action: string;
  effect: string;
}

export interface ViewerConfig {
  port: number;
}

/** Additive cosmetic display text (2026-07-11 tweaks5 §1/§2) — HOME's
    tagline/intro and the header's product name. Empty/missing means "use
    the built-in default"; never read directly — resolve through
    useLookups() so the fallback rule lives in one place (lookups.tsx). */
export interface DisplayConfig {
  productName?: string;
  tagline?: string;
  intro?: string;
}

export interface Config {
  schemaVersion: number;
  kinds: Kinds;
  tagKinds: string[];
  facetKinds: string[];
  traceabilityKinds: string[];
  idPrefix: IDPrefix;
  roots: string[];
  viewer: ViewerConfig;
  /** Additive display-label map for tagKinds (2026-07-11 tweaks3 §2) —
      tagKinds alone still decides which kinds are valid; this only carries
      how to show one. May be null/undefined for a config predating this
      field. Never read directly — resolve through useLookups().tagKindLabel
      so the id-fallback lives in one place (lookups.tsx). */
  tagKindLabels?: Record<string, string> | null;
  display?: DisplayConfig | null;
  /** Current git branch name — a live derived value computed server-side on
      every GET/PUT (2026-07-11 tweaks5 §2), never persisted to config.json.
      Empty/missing when the project isn't a git repo or HEAD is detached. */
  branch?: string;
}

export interface ConfigPatch {
  tagKinds: string[];
  facetKinds: string[];
  traceabilityKinds: string[];
  roots: string[];
  viewer: { port: number };
  tagKindLabels: Record<string, string>;
  display: DisplayConfig;
}

export interface FacetTreeNode {
  tag: Tag;
  children?: FacetTreeNode[];
}

// browse ナビの「1本の統一ツリー」（§3.8）。roots は kind 非依存に parentIds で
// 入れ子にした単一フォレスト。kind はノードの属性（バッジ/色・tag.kind）で、
// 木を分割する軸ではない。facetKinds は「その kind だけ表示」フィルタ（chips）。
export interface FacetsResponse {
  facetKinds: string[];
  roots: FacetTreeNode[];
}

export interface FacetNode {
  tag: Tag;
  transitions?: Transition[];
  children?: FacetNode[];
}

export interface TransitionsResponse {
  transitions?: Transition[];
  facet?: string;
  roots?: FacetNode[];
  untagged?: Transition[];
}

/** How a tag became effective on a transition (§3.7) — a tag can arrive via
    more than one path at once (e.g. directly assigned AND an ancestor of
    another effective tag), so EffectiveTag.sources is a set, not a single
    winner. Computed backend-side only; never re-derive this client-side
    (§9 single source of truth, gap G11). */
export type TagSource = 'own' | 'vocab' | 'ancestor';

export interface EffectiveTag {
  id: string;
  sources: TagSource[];
}

export interface TransitionDetail extends Transition {
  actionLabel?: string;
  givenLabels?: string[];
  thenLabels?: string[];
  effectiveTags?: EffectiveTag[];
  rules?: Decision[];
}

export interface SpecEntry {
  transition: Transition;
  actionLabel: string;
  givenLabels?: string[];
  thenLabels?: string[];
  decisions?: Decision[];
}

export interface SpecReport {
  tag: Tag;
  entries: SpecEntry[];
  // このタグ自体を target とする decision（§3.5 cross-cutting）。Go 側の
  // render.SpecReport.TagDecisions（omitempty）と同期。tag が transition を
  // 持つか（entries が空か）に関係なく常に載る第一級レコード。従来は entries を
  // flatMap して拾っていたが、0-tx タグでは entries が空で完全に消えていた
  // （tag-decision-visibility）。該当なしは省略される。
  tagDecisions?: Decision[];
  // このタグを直接持つ語彙（VocabEntry.Tags 逆引き・H3 の関連語彙）。Go 側の
  // render.SpecReport.RelatedVocab（omitempty）と同期。該当なしは省略される。
  relatedVocab?: VocabEntry[];
}

export interface LintFinding {
  rule: string;
  severity: 'error' | 'warn' | 'info';
  message: string;
  target?: string;
}

export interface LintResult {
  findings: LintFinding[];
  errorCount: number;
  warnCount: number;
  infoCount: number;
}

export interface TraceabilityEntry {
  tag: Tag;
  satisfiedBy: string[];
  gap: boolean;
}

export interface TraceabilityResponse {
  kinds: string[];
  entries: TraceabilityEntry[];
}

export interface SearchResult {
  transitions: Transition[];
  matchedOn: Record<string, string[]>;
}

export interface SearchCandidate {
  label: string;
  text: string;
}

export interface TransitionSearchDoc {
  transitionId: string;
  candidates: SearchCandidate[];
}

// diff-viz types (§2 of change-cockpit-design-v2.md) — mirror
// internal/diff.Result 1:1 (JSON field names, additive-only optional
// fields). This is a server-mode-only surface: `scholia export --html` never
// bakes diff data into ScholiaStaticData, so api.getDiff() always hits
// `GET /api/diff` and any caller must not invoke it when isStaticMode
// (the former CompareView did this; P2's comment-drawer diff card will).
export interface DiffChange<T> {
  id: string;
  before: T;
  after: T;
}

export interface VocabDiff {
  added?: VocabEntry[];
  removed?: VocabEntry[];
  changed?: DiffChange<VocabEntry>[];
}

export interface TagDiff {
  added?: Tag[];
  removed?: Tag[];
  changed?: DiffChange<Tag>[];
}

export interface TransitionChange {
  id: string;
  before: Transition;
  after: Transition;
  actionChanged?: boolean;
  givenAdded?: string[];
  givenRemoved?: string[];
  thenChanged?: boolean;
  tagsAdded?: string[];
  tagsRemoved?: string[];
}

export interface TransitionDiff {
  added?: Transition[];
  removed?: Transition[];
  changed?: TransitionChange[];
}

export interface DecisionDiff {
  added?: Decision[];
  removed?: Decision[];
  changed?: DiffChange<Decision>[];
}

export interface DiffResult {
  ref: string;
  vocab: VocabDiff;
  tags: TagDiff;
  transitions: TransitionDiff;
  decisions: DecisionDiff;
  /** True when `ref` fell back to an empty baseline because the caller
      didn't pass one explicitly and it doesn't resolve yet (first-run UX) —
      never set on the `head`-present (DiffRefs) path, which always errors
      instead of falling back. */
  baselineMissing?: boolean;
  /** Set only on the ref-vs-ref (DiffRefs) path — its presence is what
      distinguishes "landed task: commit vs parent" from the default
      "pending task: working tree vs base" comparison. */
  afterRef?: string;
}

// AI コメント配送（change-cockpit-design-v3.md §8.4）— `GET /api/reviews` が
// 返す read-only サイドカー。internal/review.Review の JSON shape と 1:1。
// static export には焼き込まない（本単位のスコープ外・§8.4/handoff 参照）ため
// ScholiaStaticData には含まれない。
export interface ReviewRecordRef {
  type: 'transition' | 'vocab' | 'tag';
  id: string;
}

export interface Review {
  id: string;
  recordRef: ReviewRecordRef;
  body: string;
  source: string;
  createdAt: string;
}

// flow.Report types (internal/flow/analyze.go) mirror 1:1 — T-viewer-action-
// flow-render's GET /api/flow/<action> response. Field names match the Go
// json tags exactly (camelCase); optional (`omitempty`) fields are optional
// here too.
export interface FlowMatrixRow {
  transitionId: string;
  given: string[];
  then: string[];
  // #45 D8: 同一 action 内の評価順（未宣言なら省略）。評価順バッジ描画用。
  priority?: number;
}

export interface FlowMatrix {
  conditions: string[];
  rows: FlowMatrixRow[];
}

export interface FlowSubsetShadow {
  subset: string;
  superset: string;
  // #45 D8: 評価順で解決済みか（異 priority ペア）。true なら既定の穴カウントから
  // 畳まれ、winner が先に評価される遷移。
  resolved?: boolean;
  winner?: string;
}

export interface FlowAxis {
  id: string;
  name: string;
  total: boolean;
  values: string[];
}

export interface FlowCell {
  values: Record<string, string>;
  transitions: string[];
}

export interface FlowTotalGap {
  axisId: string;
  value: string;
}

export interface FlowEffectiveGiven {
  transitionId: string;
  priority: number;
  given: string[];
  // 先行 priority 遷移群の given（否定される world・else の導出源）。
  excludes?: string[];
}

export interface FlowOverlap {
  cell: Record<string, string>;
  transitions: string[];
  // #45 D8: 評価順で解決済みか（関与遷移が全て相異なる priority）。true なら
  // 既定の穴カウントから畳まれ、effectiveGiven に derive した else が載る。
  resolved?: boolean;
  effectiveGiven?: FlowEffectiveGiven[];
}

export interface FlowRemainder {
  transitionId: string;
}

export interface FlowScopeDisclosure {
  declaredAxes: string[];
  undeclaredGiven: string[];
  hasRemainder: boolean;
  outOfGuarantee: string[];
}

export interface FlowReport {
  action: string;
  actionLabel: string;
  matrix: FlowMatrix;
  subsetShadows?: FlowSubsetShadow[];
  axes?: FlowAxis[];
  cells?: FlowCell[];
  totalGaps?: FlowTotalGap[];
  overlaps?: FlowOverlap[];
  remainder?: FlowRemainder[];
  scope: FlowScopeDisclosure;
}

// ScholiaStaticData is what `scholia export --html` bakes into
// `window.__SCHOLIA_STATIC__` — the same shapes the live /api/* endpoints
// return, precomputed for every input the SPA's read-only views can ask for
// (§7). See web/src/api.ts for how this replaces fetch() in static mode.
export interface ScholiaStaticData {
  config: Config;
  facets: FacetsResponse;
  traceability: TraceabilityResponse;
  transitionsByTag: Record<string, TransitionsResponse>;
  transitionDetail: Record<string, TransitionDetail>;
  searchCorpus: TransitionSearchDoc[];
  lint: LintResult;
  spec: Record<string, SpecReport>;
  tags: Tag[];
  vocab: VocabEntry[];
  // コンポ別 vocab（vocab-view-p2）: subject タグ id → その subject に属す遷移が
  // 参照する導出語彙。live の GET /api/vocab?subject=<id> と同じ形を、SPA が
  // 要求しうる各 tag id について焼き込む（transitionsByTag と対称）。
  vocabBySubject: Record<string, VocabEntry[]>;
  decisions: Decision[];
  // tx.viewer.action-flow-render, baked per distinct action id used by a
  // transition (internal/render/export.go's flowReports) — the only action
  // ids the SpecCard kebab can ever link to.
  flow: Record<string, FlowReport>;
}
