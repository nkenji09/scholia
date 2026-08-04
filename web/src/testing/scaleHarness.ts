import type { Config, Decision, FacetTreeNode, GovernsRef, SpecReport, Tag, Transition, TransitionDetail, VocabEntry } from '../types';

// 「画面1枚がサーバに何本投げるか」を、**規模を変えて**測るための足場。
//
// **ここは検査ではない。** 検査は pageRequestCost.test.tsx が持つ。ここが持つのは
// 2つだけ:
//
//   1. レコード件数 N を引数に取る合成 corpus（本番と同じ**4カテゴリ**の形）
//   2. 製品が実際に叩く HTTP の口を、その corpus で答える偽サーバ
//      （**来た要求を順序つきで記録する**＝数えるのはこの記録であって、
//       製品が自分で申告した数ではない）
//
// ⚠️ **偽サーバは Go 側の選択規則を再実装しない**（renderHarness.tsx と同じ線）。
// governs は「そのレコード自身への decision」だけを機械的に返す。ここで守れるのは
// 「画面が何本投げるか」であって「Go の答えが正しいか」ではない。

// ---------------------------------------------------------------------------
// 合成 corpus
// ---------------------------------------------------------------------------

export interface Corpus {
  scale: number;
  config: Config;
  tags: Tag[];
  vocab: VocabEntry[];
  transitions: Transition[];
  decisions: Decision[];
  roots: FacetTreeNode[];
}

const KIND_LABELS: Record<string, string> = { requirement: '要件', component: 'コンポーネント', part: '構成要素' };

/**
 * scale=1 で本 repo の `.scholia` とおおよそ同じ件数と形（タグ 84 / 語彙 168 /
 * 遷移 72 / 意思決定 180）になり、scale 倍で件数だけ増える。
 *
 * 🔴 **4カテゴリすべてを持つ**（単位AU の fixture は `.scholia/tags/` だけで、
 * 「カテゴリ2種以上なら旧経路」の変異が全ゲート緑・本番 100% 発火だった）。
 * 🔴 **参照の形も本番のものを踏む**——タグの親子と多親、vocab 側にだけ付いた
 * タグ、transition の action/given/then、tag/transition/vocab それぞれを対象に
 * した decision。どれかを欠くと、その形のレコードだけ別経路を通る実装が
 * 素通りする。
 */
export function makeCorpus(scale: number): Corpus {
  const tags: Tag[] = [];
  const vocab: VocabEntry[] = [];
  const transitions: Transition[] = [];
  const decisions: Decision[] = [];
  const roots: FacetTreeNode[] = [];

  // 1 クラスタ = タグ7・語彙14・遷移6・決定15（本 repo の比率）。
  const clusters = 12 * scale;
  for (let c = 0; c < clusters; c++) {
    const grp = `grp.c${c}`;
    const req = `req.c${c}`;
    const reqChild = `req.c${c}.sub`;
    const comp = `comp.c${c}`;
    tags.push({ id: grp, name: `束ね${c}`, kind: 'requirement' });
    tags.push({ id: req, name: `要件${c}`, kind: 'requirement', parentIds: [grp] });
    tags.push({ id: reqChild, name: `要件${c}の下位`, kind: 'requirement', parentIds: [req] });
    tags.push({ id: comp, name: `コンポ${c}`, kind: 'component', parentIds: [grp] });
    const partIds: string[] = [];
    for (let p = 0; p < 3; p++) {
      const part = `part.c${c}.p${p}`;
      partIds.push(part);
      // 多親（本番にある形）: 2つ目は別の要件にもぶら下がる。
      tags.push({ id: part, name: `構成要素${c}-${p}`, kind: 'part', parentIds: p === 1 ? [comp, req] : [comp] });
    }

    // 語彙 14 件。**1件目の action はタグを語彙側にだけ持つ**（遷移の実効タグが
    // vocab 経由で決まる形——これが無いと実効タグの合成に vocab を渡し忘れる
    // 変異が素通りする）。
    for (let v = 0; v < 5; v++) {
      vocab.push({ id: `act.c${c}.v${v}`, category: 'action', label: `動作${c}-${v}`, tags: v === 0 ? [comp] : [] });
    }
    for (let v = 0; v < 4; v++) {
      vocab.push({ id: `cond.c${c}.v${v}`, category: 'condition', label: `条件${c}-${v}`, tags: [] });
    }
    for (let v = 0; v < 5; v++) {
      vocab.push({ id: `eff.c${c}.v${v}`, category: 'effect', label: `効果${c}-${v}`, tags: [] });
    }

    // 遷移 6 本。1本は**自分のタグを持たず語彙経由でだけ属す**。
    for (let x = 0; x < 5; x++) {
      transitions.push({
        id: `T.c${c}.x${x}`,
        action: `act.c${c}.v${(x % 4) + 1}`,
        given: [`cond.c${c}.v${x % 4}`],
        then: [`eff.c${c}.v${x % 5}`],
        tags: [partIds[x % 3], reqChild],
      });
    }
    transitions.push({ id: `T.c${c}.viaVocab`, action: `act.c${c}.v0`, given: [], then: [`eff.c${c}.v1`] });

    // 決定 15 件。tag / transition / vocab の3種の target を必ず踏む。
    const dec = (n: number, target: Decision['target'], at: string, why: string) =>
      decisions.push({ id: `01DEC${String(c).padStart(7, '0')}${String(n).padStart(2, '0')}${'0'.repeat(12)}`.slice(0, 26), target, at, why });
    for (let d = 0; d < 5; d++) dec(d, { type: 'tag', id: d % 2 === 0 ? req : partIds[d % 3] }, `2026-01-0${(d % 9) + 1}T00:00:00Z`, `# 要件${c}の判断${d}\n\n本文。`);
    for (let d = 0; d < 6; d++) dec(5 + d, { type: 'transition', id: `T.c${c}.x${d % 5}` }, `2026-02-0${(d % 9) + 1}T00:00:00Z`, `# 遷移${c}の判断${d}\n\n本文。`);
    for (let d = 0; d < 4; d++) dec(11 + d, { type: 'vocab', id: `act.c${c}.v${d}` }, `2026-03-0${(d % 9) + 1}T00:00:00Z`, `# 語彙${c}の判断${d}\n\n本文。`);

    const base = tags.length - 7;
    roots.push({
      tag: tags[base],
      children: [
        { tag: tags[base + 1], children: [{ tag: tags[base + 2] }] },
        { tag: tags[base + 3], children: [{ tag: tags[base + 4] }, { tag: tags[base + 5] }, { tag: tags[base + 6] }] },
      ],
    });
  }

  const config: Config = {
    schemaVersion: 1,
    kinds: { condition: ['condition'], action: ['action'], effect: ['effect'] },
    tagKinds: ['requirement', 'component', 'part'],
    facetKinds: ['requirement'],
    traceabilityKinds: ['requirement'],
    idPrefix: { condition: 'cond.', action: 'act.', effect: 'eff.' },
    roots: [],
    viewer: { port: 4577 },
    tagKindLabels: KIND_LABELS,
    display: { productName: 'scholia' },
    branch: 'scale-harness',
    localOverride: {},
    effectiveTimezone: 'UTC',
  };

  return { scale, config, tags, vocab, transitions, decisions, roots };
}

// ---------------------------------------------------------------------------
// 偽サーバ
// ---------------------------------------------------------------------------

export interface ScaleServer {
  /** 来た要求（順序つき・`path + search`）。**数えるのはこれ。** */
  requests: string[];
  /** 答えを持っていなかった要求。空でなければ harness が製品の経路を
      取りこぼしている（新しい fetch が足されたのに追随していない）。 */
  unhandled: string[];
  restore: () => void;
}

function specFor(c: Corpus, tagId: string): SpecReport {
  const tag = c.tags.find((t) => t.id === tagId)!;
  const entries = c.transitions
    .filter((tx) => (tx.tags || []).includes(tagId))
    .map((tx) => ({
      transition: tx,
      actionLabel: c.vocab.find((v) => v.id === tx.action)?.label || tx.action,
      givenLabels: tx.given.map((g) => c.vocab.find((v) => v.id === g)?.label || g),
      thenLabels: tx.then.map((e) => c.vocab.find((v) => v.id === e)?.label || e),
      decisions: c.decisions.filter((d) => d.target.type === 'transition' && d.target.id === tx.id),
    }));
  return {
    tag,
    entries,
    tagDecisions: c.decisions.filter((d) => d.target.type === 'tag' && d.target.id === tagId),
    relatedVocab: c.vocab.filter((v) => (v.tags || []).includes(tagId)),
  };
}

function detailFor(c: Corpus, tx: Transition): TransitionDetail {
  return {
    ...tx,
    actionLabel: c.vocab.find((v) => v.id === tx.action)?.label || tx.action,
    givenLabels: tx.given.map((g) => c.vocab.find((v) => v.id === g)?.label || g),
    thenLabels: tx.then.map((e) => c.vocab.find((v) => v.id === e)?.label || e),
    effectiveTags: (tx.tags || []).map((id) => ({ id, sources: ['own' as const] })),
    rules: c.decisions.filter((d) => d.target.type === 'transition' && d.target.id === tx.id),
  };
}

/** そのレコード自身への decision だけを own として返す（Go の選択規則は再実装しない）。 */
function governsFor(c: Corpus, type: 'tag' | 'transition' | 'vocab', id: string): GovernsRef[] {
  return c.decisions.filter((d) => d.target.type === type && d.target.id === id).map((d) => ({ decisionId: d.id, provenance: 'own' as const }));
}

function allGoverns(c: Corpus): Record<string, GovernsRef[]> {
  const out: Record<string, GovernsRef[]> = {};
  for (const t of c.tags) out[`tag:${t.id}`] = governsFor(c, 'tag', t.id);
  for (const tx of c.transitions) out[`transition:${tx.id}`] = governsFor(c, 'transition', tx.id);
  for (const v of c.vocab) out[`vocab:${v.id}`] = governsFor(c, 'vocab', v.id);
  return out;
}

const EMPTY_DIFF = {
  transitions: { added: [], changed: [], removed: [] },
  vocab: { added: [], changed: [], removed: [] },
  tags: { added: [], changed: [], removed: [] },
  decisions: { added: [], changed: [], removed: [] },
};

function body(c: Corpus, path: string, params: URLSearchParams): unknown | undefined {
  switch (path) {
    case '/api/config':
      return c.config;
    case '/api/facets':
      return { facetKinds: c.config.facetKinds, roots: c.roots };
    case '/api/tags': {
      const kind = params.get('kind');
      return kind ? c.tags.filter((t) => t.kind === kind) : c.tags;
    }
    case '/api/vocab': {
      const subject = params.get('subject');
      if (subject) {
        const ids = new Set(c.transitions.filter((tx) => (tx.tags || []).includes(subject)).flatMap((tx) => [tx.action, ...tx.given, ...tx.then]));
        return c.vocab.filter((v) => ids.has(v.id));
      }
      const category = params.get('category');
      return category ? c.vocab.filter((v) => v.category === category) : c.vocab;
    }
    case '/api/transitions': {
      const tag = params.get('tag');
      const list = tag ? c.transitions.filter((tx) => (tx.tags || []).includes(tag)) : c.transitions;
      const out: Record<string, unknown> = { transitions: list };
      if (params.get('detail')) {
        const details: Record<string, TransitionDetail> = {};
        for (const tx of list) details[tx.id] = detailFor(c, tx);
        out.details = details;
      }
      return out;
    }
    case '/api/rules': {
      const tag = params.get('tag');
      const tx = params.get('tx');
      if (tag) return { decisions: c.decisions.filter((d) => d.target.type === 'tag' && d.target.id === tag) };
      if (tx) return { decisions: c.decisions.filter((d) => d.target.type === 'transition' && d.target.id === tx) };
      return { decisions: c.decisions };
    }
    case '/api/spec': {
      const reports: Record<string, SpecReport> = {};
      for (const t of c.tags) reports[t.id] = specFor(c, t.id);
      return { reports };
    }
    case '/api/governs': {
      if (params.get('all')) return { byRef: allGoverns(c) };
      const tag = params.get('tag');
      const tx = params.get('tx');
      const vocab = params.get('vocab');
      if (tag) return { entries: governsFor(c, 'tag', tag) };
      if (tx) return { entries: governsFor(c, 'transition', tx) };
      if (vocab) return { entries: governsFor(c, 'vocab', vocab) };
      return { entries: [] };
    }
    case '/api/traceability':
      return { kinds: c.config.traceabilityKinds, entries: [] };
    case '/api/reviews':
      return [];
    case '/api/diff':
      return EMPTY_DIFF;
    case '/api/lint':
      return { findings: [], errorCount: 0, warnCount: 0, infoCount: 0 };
    case '/api/search':
      return { transitions: [], matchedOn: {}, records: [] };
  }
  if (path.startsWith('/api/spec/')) {
    const id = decodeURIComponent(path.slice('/api/spec/'.length));
    return c.tags.some((t) => t.id === id) ? specFor(c, id) : undefined;
  }
  if (path.startsWith('/api/transitions/')) {
    const id = decodeURIComponent(path.slice('/api/transitions/'.length));
    const tx = c.transitions.find((t) => t.id === id);
    return tx ? detailFor(c, tx) : undefined;
  }
  if (path.startsWith('/api/flow/')) {
    return { action: decodeURIComponent(path.slice('/api/flow/'.length)), actionLabel: '', rows: [], givens: [], thens: [], matrix: [] };
  }
  return undefined;
}

export function installScaleServer(c: Corpus): ScaleServer {
  const original = globalThis.fetch;
  const server: ScaleServer = {
    requests: [],
    unhandled: [],
    restore: () => {
      globalThis.fetch = original;
    },
  };
  globalThis.fetch = (async (input: RequestInfo | URL) => {
    const raw = typeof input === 'string' ? input : input instanceof URL ? input.toString() : (input as Request).url;
    const url = new URL(raw, 'http://scale.local');
    server.requests.push(url.pathname + url.search);
    const payload = body(c, url.pathname, url.searchParams);
    if (payload === undefined) {
      server.unhandled.push(url.pathname + url.search);
      return { ok: false, status: 404, statusText: `scale harness has no answer for ${url.pathname}`, json: async () => ({}) } as Response;
    }
    return { ok: true, status: 200, statusText: 'OK', json: async () => payload } as Response;
  }) as typeof fetch;
  return server;
}
