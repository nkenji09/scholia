import type {
  Config,
  ConfigPatch,
  DiffResult,
  FacetsResponse,
  FlowReport,
  LintResult,
  LocalConfigOverride,
  ScholiaStaticData,
  SearchResult,
  SpecReport,
  Tag,
  TraceabilityResponse,
  Transition,
  TransitionDetail,
  TransitionPostBody,
  TransitionsResponse,
  VocabEntry,
  Decision,
  DecisionPostBody,
  GovernsRef,
  Review,
} from './types';
import { loadLang } from './i18n';
import { DICTS } from './strings';

class ApiError extends Error {
  // サーバが返す機械可読な失敗種別（errorBody.code）。文言をそのまま出すと
  // 生 id が漏れる面があるので、呼び出し側はこれを見て自前の（翻訳済み・
  // id を含まない）文言を選べる（01KYCC2TF3NW3JRSSRK9ZHN078）。undefined の
  // ときは message をそのまま出してよい。
  code?: string;

  constructor(message: string, code?: string) {
    super(message);
    this.code = code;
  }
}

declare global {
  interface Window {
    __SCHOLIA_STATIC__?: ScholiaStaticData;
  }
}

// `scholia export --html` bakes this in as an inline <script> (see
// internal/render/export.go) so a static export can serve every read-only
// view without a server. Its presence is exactly what distinguishes a
// static export from a `scholia view`-served page.
const staticData: ScholiaStaticData | undefined = window.__SCHOLIA_STATIC__;

export const isStaticMode = !!staticData;

function staticUnavailable(what: string): Promise<never> {
  return Promise.reject(new ApiError(DICTS[loadLang()].api.unavailable(what)));
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, init);
  if (!res.ok) {
    let message = res.statusText;
    let code: string | undefined;
    try {
      const body = await res.json();
      if (body && typeof body.error === 'string') message = body.error;
      if (body && typeof body.code === 'string') code = body.code;
    } catch {
      // response body wasn't JSON; fall back to statusText
    }
    throw new ApiError(message, code);
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

// runStaticSearch mirrors internal/index's per-candidate substring test over
// the baked corpus (window.__SCHOLIA_STATIC__.searchCorpus) instead of hitting
// GET /api/search. The corpus itself — which candidates exist per record
// (effective tags, vocab labels, kind, decision why/changed, …) — is derived
// once in Go (index.SearchCorpus, from the same fields SearchRecords scans);
// only the trivial "does this query substring occur" test is re-run here per
// keystroke. Produces both the transition-grouped view (transitions/matchedOn,
// unchanged) and the 4-type records list (#45 D10b-3), from the one corpus.
function runStaticSearch(data: ScholiaStaticData, q: string): SearchResult {
  const query = q.trim().toLowerCase();
  const result: SearchResult = { transitions: [], matchedOn: {}, records: [] };
  if (!query) return result;

  const byId = new Map(data.transitionsByTag['']?.transitions?.map((t) => [t.id, t]) ?? []);
  const records: SearchResult['records'] = [];
  for (const doc of data.searchCorpus) {
    const seen = new Set<string>();
    const labels: string[] = [];
    for (const c of doc.candidates) {
      if (!c.text.includes(query)) continue;
      if (!seen.has(c.label)) {
        seen.add(c.label);
        labels.push(c.label);
      }
      // 4型 records: dedupe は type|id|field 単位（Go 側 SearchRecords と対称）。
      records!.push({ type: doc.type, id: doc.id, field: c.label, snippet: snippetOf(c.text, query) });
    }
    if (doc.type === 'transition' && labels.length > 0) {
      const t = byId.get(doc.id);
      if (t) {
        result.transitions.push(t);
        result.matchedOn[doc.id] = labels;
      }
    }
  }
  // dedupe records by type|id|field (a label can repeat across candidates).
  const seenRec = new Set<string>();
  result.records = records!.filter((r) => {
    const k = r.type + '|' + r.id + '|' + r.field;
    if (seenRec.has(k)) return false;
    seenRec.add(k);
    return true;
  });
  return result;
}

// snippetOf approximates the Go Snippet (a ~20-char window around the match);
// the static corpus text is already lowercased, so this is best-effort for
// display only (records' snippets aren't shown in the current UI).
function snippetOf(text: string, query: string): string {
  const idx = text.indexOf(query);
  if (idx < 0) return text.slice(0, 80);
  const start = Math.max(0, idx - 20);
  const end = Math.min(text.length, idx + query.length + 20);
  return (start > 0 ? '…' : '') + text.slice(start, end) + (end < text.length ? '…' : '');
}

function staticTraceability(data: ScholiaStaticData, kind?: string): TraceabilityResponse {
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

  // req.comfortable-viewer.config-editing amend: writes .scholia/
  // config.local.json (gitignored, this machine only), independent of
  // putConfig above (the shared project config.json). Full-replace, same
  // as putConfig — ConfigView's local-override form round-trips the whole
  // draft it loaded.
  putLocalConfig: (patch: LocalConfigOverride) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.configEdit);
    return request<LocalConfigOverride>('/api/config/local', {
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

  // subject 指定（コンポ別モード・vocab-view-p2）は「その subject に属す遷移が
  // 参照する導出語彙」を返す（category とは排他・全カテゴリを返す）。category は
  // 従来通りの全件フィルタ。static はそれぞれ vocabBySubject / vocab から解決。
  getVocab: (params?: { category?: string; subject?: string }) => {
    const { category, subject } = params ?? {};
    if (staticData) {
      if (subject) return Promise.resolve(staticData.vocabBySubject[subject] ?? []);
      const list = category ? staticData.vocab.filter((v) => v.category === category) : staticData.vocab;
      return Promise.resolve(list);
    }
    return request<VocabEntry[]>('/api/vocab' + query({ category, subject }));
  },

  // detail:true は「一覧＋全 transition の詳細」を1本で取る（正本
  // 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）。transition 1件につき getTransition を
  // 1本ずつ投げる形は、件数に比例して本数が増える——本数を件数から切り離す
  // ための口である。静的モードは焼き込み済みの transitionDetail を返すので、
  // 消費側は live/静的で同じ1つのコードパスを通る（面間整合 D10b-2）。
  getTransitions: (params: { facet?: string; tag?: string; kind?: string; detail?: boolean }) => {
    const { detail, ...selectors } = params;
    if (staticData) {
      if (selectors.facet || selectors.kind) return staticUnavailable(DICTS[loadLang()].api.transitionsByFacetKind);
      const res = staticData.transitionsByTag[selectors.tag ?? ''];
      if (!res) return staticUnavailable(DICTS[loadLang()].api.transitionsForTag(selectors.tag ?? ''));
      return Promise.resolve(detail ? { ...res, details: staticData.transitionDetail } : res);
    }
    return request<TransitionsResponse>('/api/transitions' + query({ ...selectors, detail: detail ? '1' : undefined }));
  },

  getTransition: (id: string) => {
    if (staticData) {
      const detail = staticData.transitionDetail[id];
      return detail ? Promise.resolve(detail) : staticUnavailable(DICTS[loadLang()].api.transition(id));
    }
    return request<TransitionDetail>(`/api/transitions/${encodeURIComponent(id)}`);
  },

  // 全タグの spec レポートを1本で取る（正本 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）。
  // タグ1件につき getSpec を1本ずつ投げる形の置き換えで、静的モードは焼き込み
  // 済みの map をそのまま返す（形が live と同一なので消費側は分岐しない）。
  getSpecAll: () => {
    if (staticData) return Promise.resolve(staticData.spec);
    return request<{ reports: Record<string, SpecReport> }>('/api/spec').then((r) => r.reports);
  },

  getSpec: (tagId: string) => {
    if (staticData) {
      const report = staticData.spec[tagId];
      return report ? Promise.resolve(report) : staticUnavailable(DICTS[loadLang()].api.spec(tagId));
    }
    return request<SpecReport>(`/api/spec/${encodeURIComponent(tagId)}`);
  },

  getFlow: (actionId: string) => {
    if (staticData) {
      const report = staticData.flow[actionId];
      return report ? Promise.resolve(report) : staticUnavailable(DICTS[loadLang()].api.flow(actionId));
    }
    return request<FlowReport>(`/api/flow/${encodeURIComponent(actionId)}`);
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
  // （他の ref の .scholia/ ツリー）が無いため、常に静的モード非対応（他の
  // api.* と同じ流儀で弾く）。
  getDiff: (params: { ref?: string; head?: string }) =>
    staticData ? staticUnavailable(DICTS[loadLang()].api.diff) : request<DiffResult>('/api/diff' + query(params)),

  // AI コメント配送（change-cockpit-design-v3.md §8.4）— `.scholia/reviews/` の
  // read-only サイドカーを返す。getDiff と同流儀: static export はその場限りの
  // 1スナップショットで、AI review は焼き込み対象外（§8.4 「本単位ではやらな
  // い」）なので常に静的モード非対応。人コメント（localStorage）の経路には
  // 触れない — この getter だけが新規（useComments.tsx 冒頭の「fetch を足す
  // な」は人コメント永続化の制約であり、この別系統 read には適用されない）。
  getReviews: () => (staticData ? staticUnavailable(DICTS[loadLang()].api.reviews) : request<Review[]>('/api/reviews')),

  // 昇格元コメント掃除（#35・tx.review.adopt/-reject の後半）— postDecision が
  // 成功した後にだけ呼ぶ（先に消すと why を失う）。deleteTransition と同じ
  // 流儀: static export は書込不可なので常に非対応。
  deleteReview: (id: string) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.reviewDelete);
    return request<{ id: string }>(`/api/reviews/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  // 採用（change-cockpit-design-v3.md §1/§8.5・G-1 承認済み）— viewer の書込は
  // PUT /api/config・これ・putTransition（下）の3本のみ（§7 narrow rule）。
  // static export は書込不可なので常に非対応（putConfig と同じ流儀）。
  postDecision: (body: DecisionPostBody) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.decisionAdopt);
    return request<Decision>('/api/decision', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  // 提案の手直し（change-cockpit-design-v3.md §1 (Wp)/§8.8 P3・G-1′ 承認済み）
  // — 語彙ピッカーの「反映」1 回につき 1 本呼ぶ。viewer の書込は
  // PUT /api/config・POST /api/decision・これの4本のみ（§7 narrow rule・
  // §8.8 P5 で DELETE /api/transitions/{id} が5本目に加わる・下記）。
  // static export は書込不可なので常に非対応（putConfig/postDecision と同じ流儀）。
  putTransition: (body: TransitionPostBody) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.transitionEdit);
    return request<Transition>('/api/transition', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  // 新規 Transition の提案（change-cockpit-design-v3.md §8.8 P5・M-5「追加」・
  // 同じ G-1′ 書込面）— POST /api/transition は body.id が未実在なら 201 作成
  // として扱う（internal/viewer/transition_write.go）。エンドポイント/body は
  // putTransition と同一（サーバ側が存在有無で create/edit を分岐する）ため
  // 実体は流用しつつ、呼び出し側の意図（新規作成）を名前で明確にする。
  createTransition: (body: TransitionPostBody) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.transitionCreate);
    return request<Transition>('/api/transition', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    });
  },

  // Transition の削除提案（change-cockpit-design-v3.md §8.8 P5・M-5「削除」・
  // G-1′ 拡張）— 作業ツリーの transition ファイルのみ除去（未コミット・git は
  // 人）。decision がまだ対象にしている transition は 409 で拒否される
  // （internal/store.RemoveTransitionUnlinked）。static export は書込不可
  // なので常に非対応。
  deleteTransition: (id: string) => {
    if (staticData) return staticUnavailable(DICTS[loadLang()].api.transitionDelete);
    return request<{ id: string }>(`/api/transitions/${encodeURIComponent(id)}`, { method: 'DELETE' });
  },

  getTraceability: (kind?: string) =>
    staticData ? Promise.resolve(staticTraceability(staticData, kind)) : request<TraceabilityResponse>('/api/traceability' + query({ kind })),

  search: (q: string) => (staticData ? Promise.resolve(runStaticSearch(staticData, q)) : request<SearchResult>('/api/search' + query({ q }))),

  // per-record governs（#45 D10b-1）— exactly one of tag/tx/vocab. static は
  // record ref（"tag:<id>" 等）で焼き込み済み map を引く（transitionsByTag と
  // 同流儀）。live は GET /api/governs に委譲。
  // 全レコードの governs を1本で取る（正本 01KZ5N5CJ2VFMZAGSFPSCZAMTZ 条項1）。
  // カード1枚につき getGoverns を1本ずつ投げる形の置き換え。key は
  // "tag:<id>" / "transition:<id>" / "vocab:<id>" ——静的の焼き込みと同じ綴りで、
  // live 側もこの綴りで返す（GovernsProvider が1回だけ呼ぶ）。
  getGovernsAll: () => {
    if (staticData) return Promise.resolve(staticData.governs);
    return request<{ byRef: Record<string, GovernsRef[]> }>('/api/governs?all=1').then((r) => r.byRef);
  },

  getGoverns: (ref: { tag?: string; tx?: string; vocab?: string }) => {
    if (staticData) {
      const key = ref.tag ? `tag:${ref.tag}` : ref.tx ? `transition:${ref.tx}` : ref.vocab ? `vocab:${ref.vocab}` : '';
      return Promise.resolve({ entries: staticData.governs[key] ?? [] });
    }
    return request<{ entries: GovernsRef[] }>('/api/governs' + query(ref));
  },
};

export { ApiError };
