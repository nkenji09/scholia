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

export interface Config {
  pmemVersion: number;
  kinds: Kinds;
  tagKinds: string[];
  facetKinds: string[];
  traceabilityKinds: string[];
  idPrefix: IDPrefix;
  roots: string[];
  viewer: ViewerConfig;
}

export interface ConfigPatch {
  tagKinds: string[];
  facetKinds: string[];
  traceabilityKinds: string[];
  roots: string[];
  viewer: { port: number };
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

export interface TransitionDetail extends Transition {
  actionLabel?: string;
  givenLabels?: string[];
  thenLabels?: string[];
  effectiveTags?: string[];
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

export interface Change<T> {
  id: string;
  before: T;
  after: T;
}

export interface VocabDiff {
  added?: VocabEntry[];
  removed?: VocabEntry[];
  changed?: Change<VocabEntry>[];
}

export interface TagDiff {
  added?: Tag[];
  removed?: Tag[];
  changed?: Change<Tag>[];
}

export interface TransitionChange {
  id: string;
  before: Transition;
  after: Transition;
  actionChanged?: boolean;
  givenAdded?: string[];
  givenRemoved?: string[];
  thenChanged?: boolean;
  thenReordered?: boolean;
  tagsAdded?: string[];
  tagsRemoved?: string[];
  testsAdded?: string[];
  testsRemoved?: string[];
}

export interface TransitionDiff {
  added?: Transition[];
  removed?: Transition[];
  changed?: TransitionChange[];
}

export interface DecisionDiff {
  added?: Decision[];
  removed?: Decision[];
  changed?: Change<Decision>[];
}

export interface DiffResult {
  ref: string;
  vocab: VocabDiff;
  tags: TagDiff;
  transitions: TransitionDiff;
  decisions: DecisionDiff;
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
