import type {
  Config,
  ConfigPatch,
  DiffResult,
  FacetsResponse,
  LintResult,
  PmemStaticData,
  SearchResult,
  SpecReport,
  Tag,
  TraceabilityResponse,
  TransitionDetail,
  TransitionsResponse,
  VocabEntry,
  Decision,
  Review,
} from './types';
import { loadLang } from './i18n';
import { DICTS } from './strings';

class ApiError extends Error {}

declare global {
  interface Window {
    __PMEM_STATIC__?: PmemStaticData;
  }
}

// `pmem export --html` bakes this in as an inline <script> (see
// internal/render/export.go) so a static export can serve every read-only
// view without a server. Its presence is exactly what distinguishes a
// static export from a `pmem view`-served page.
const staticData: PmemStaticData | undefined = window.__PMEM_STATIC__;

export const isStaticMode = !!staticData;

function staticUnavailable(what: string): Promise<never> {
  return Promise.reject(new ApiError(DICTS[loadLang()].api.unavailable(what)));
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body && typeof body.error === 'string') message = body.error;
    } catch {
      // response body wasn't JSON; fall back to statusText
    }
    throw new ApiError(message);
  }
  return res.json() as Promise<T>;
}

function query(params: Record<string, string | undefined>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v) q.set(k, v);
  }
  const qs = q.toString();
  return qs ? `?${qs}` : '';
}

// runStaticSearch mirrors internal/index.Search's per-candidate substring
// test over the baked corpus (window.__PMEM_STATIC__.searchCorpus) instead
// of hitting GET /api/search. The corpus itself — which candidates exist
// per transition (effective tags, vocab labels, kind) — is derived once in
// Go (index.SearchCorpus, the same function Search() itself uses); only the
// trivial "does this query substring occur" test is re-run here per
// keystroke.
function runStaticSearch(data: PmemStaticData, q: string): SearchResult {
  const query = q.trim().toLowerCase();
  const result: SearchResult = { transitions: [], matchedOn: {} };
  if (!query) return result;

  const byId = new Map(data.transitionsByTag['']?.transitions?.map((t) => [t.id, t]) ?? []);
  for (const doc of data.searchCorpus) {
    const seen = new Set<string>();
    const labels: string[] = [];
    for (const c of doc.candidates) {
      if (seen.has(c.label) || !c.text.includes(query)) continue;
      seen.add(c.label);
      labels.push(c.label);
    }
    if (labels.length === 0) continue;
    const t = byId.get(doc.transitionId);
    if (!t) continue;
    result.transitions.push(t);
    result.matchedOn[doc.transitionId] = labels;
  }
  return result;
}

function staticTraceability(data: PmemStaticData, kind?: string): TraceabilityResponse {
  if (!kind) return data.traceability;
  return {
    kinds: [kind],
    entries: data.traceability.entries.filter((e) => e.tag.kind === kind),
  };
}

export const api = {
  getConfig: () => (staticData ? Promise.resolve(staticData.config) : request<Config>('/api/config')),

  putConfig: (patch: ConfigPatch) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.configEdit);
    return request<Config>('/api/config', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(patch),
    });
  },

  getFacets: () => (staticData ? Promise.resolve(staticData.facets) : request<FacetsResponse>('/api/facets')),

  getTags: (kind?: string) => {
    if (staticData) {
      const list = kind ? staticData.tags.filter((t) => t.kind === kind) : staticData.tags;
      return Promise.resolve(list);
    }
    return request<Tag[]>('/api/tags' + query({ kind }));
  },

  getVocab: (category?: string) => {
    if (staticData) {
      const list = category ? staticData.vocab.filter((v) => v.category === category) : staticData.vocab;
      return Promise.resolve(list);
    }
    return request<VocabEntry[]>('/api/vocab' + query({ category }));
  },

  getTransitions: (params: { facet?: string; tag?: string; kind?: string }) => {
    if (staticData) {
      if (params.facet || params.kind) return staticUnavailable(DICTS[loadLang()].api.transitionsByFacetKind);
      const res = staticData.transitionsByTag[params.tag ?? ''];
      if (!res) return staticUnavailable(DICTS[loadLang()].api.transitionsForTag(params.tag ?? ''));
      return Promise.resolve(res);
    }
    return request<TransitionsResponse>('/api/transitions' + query(params));
  },

  getTransition: (id: string) => {
    if (staticData) {
      const detail = staticData.transitionDetail[id];
      return detail ? Promise.resolve(detail) : staticUnavailable(DICTS[loadLang()].api.transition(id));
    }
    return request<TransitionDetail>(`/api/transitions/${encodeURIComponent(id)}`);
  },

  getSpec: (tagId: string) => {
    if (staticData) {
      const report = staticData.spec[tagId];
      return report ? Promise.resolve(report) : staticUnavailable(DICTS[loadLang()].api.spec(tagId));
    }
    return request<SpecReport>(`/api/spec/${encodeURIComponent(tagId)}`);
  },

  getRules: (params: { tx?: string; tag?: string; facet?: string }) => {
    if (staticData) {
      // Only the no-selector ("every decision, chronological") mode is
      // baked for static exports — HOME's recent-decisions widget is the
      // only current caller and never passes tag/tx/facet. Per-selector
      // rules queries stay live-only (TransitionDetail/SpecView already get
      // their decisions embedded directly in their own static payloads).
      if (params.tag || params.tx || params.facet) return staticUnavailable(DICTS[loadLang()].api.rulesWithSelectors);
      return Promise.resolve({ decisions: staticData.decisions });
    }
    return request<{ decisions: Decision[] }>('/api/rules' + query(params));
  },

  getLint: () => (staticData ? Promise.resolve(staticData.lint) : request<LintResult>('/api/lint')),

  // 意味 diff（base ref 対 head/作業ツリー）。旧・独立 compare ビューが呼んで
  // いたが撤去（change-cockpit-design-v3.md §5 P1）、後続 P2 で Transition の
  // コメントドロワーから pending diff 表示に再利用する想定で温存。static
  // export はビルド時点の1スナップショットしか持たず ref/head 比較の材料
  // （他の ref の .pmem/ ツリー）が無いため、常に静的モード非対応（他の
  // api.* と同じ流儀で弾く）。
  getDiff: (params: { ref?: string; head?: string }) =>
    staticData ? staticUnavailable(DICTS[loadLang()].api.diff) : request<DiffResult>('/api/diff' + query(params)),

  // AI コメント配送（change-cockpit-design-v3.md §8.4）— `.pmem/reviews/` の
  // read-only サイドカーを返す。getDiff と同流儀: static export はその場限りの
  // 1スナップショットで、AI review は焼き込み対象外（§8.4 「本単位ではやらな
  // い」）なので常に静的モード非対応。人コメント（localStorage）の経路には
  // 触れない — この getter だけが新規（useComments.tsx 冒頭の「fetch を足す
  // な」は人コメント永続化の制約であり、この別系統 read には適用されない）。
  getReviews: () => (staticData ? staticUnavailable(DICTS[loadLang()].api.reviews) : request<Review[]>('/api/reviews')),

  getTraceability: (kind?: string) =>
    staticData ? Promise.resolve(staticTraceability(staticData, kind)) : request<TraceabilityResponse>('/api/traceability' + query({ kind })),

  search: (q: string) => (staticData ? Promise.resolve(runStaticSearch(staticData, q)) : request<SearchResult>('/api/search' + query({ q }))),
};

export { ApiError };
