export interface Transition {
  id: string;
  action: string;
  given: string[];
  then: string[];
  tags?: string[];
  tests?: string[];
}

export interface VocabEntry {
  id: string;
  category: string;
  label: string;
  kind?: string;
  owner?: string;
  tags?: string[];
  description?: string;
}

export interface Tag {
  id: string;
  name: string;
  kind?: string;
  parentIds?: string[];
  description?: string;
  color?: string;
  ref?: string;
}

export interface DecisionTarget {
  type: 'transition' | 'tag';
  id: string;
}

export interface Decision {
  id: string;
  target: DecisionTarget;
  why: string;
  changed?: string;
  ref?: string;
  at: string;
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
  pmemVersion: number;
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

export interface FacetsResponse {
  facetKinds: string[];
  trees: Record<string, FacetTreeNode[]>;
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

// PmemStaticData is what `pmem export --html` bakes into
// `window.__PMEM_STATIC__` — the same shapes the live /api/* endpoints
// return, precomputed for every input the SPA's read-only views can ask for
// (§7). See web/src/api.ts for how this replaces fetch() in static mode.
export interface PmemStaticData {
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
  decisions: Decision[];
}
