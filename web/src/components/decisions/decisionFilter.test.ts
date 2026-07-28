import { describe, expect, it } from 'vitest';
import type { Decision, GovernsRef } from '../../types';
import {
  EMPTY_CONDITIONS,
  conditionsFromRoute,
  matchesTags,
  namesOneDecision,
  routeParamsFromConditions,
  sameConditions,
  selectBase,
  selectDecisions,
} from './decisionFilter';
import type { DecisionConditions, DecisionRouteParams, SelectContext } from './decisionFilter';

// 絞り込み条件そのものを**値として**検査する（CLAUDE.md「配線ガードの書き方」1）。
//
// 差し戻し1回目で、次の4つがテスト緑のまま素通りした。いずれも「ソース文字列が
// そこに在るか」を見る検査では捕まらない型で、**判断を値で検査できる形に移した**
// のがこのファイルの対象:
//
//   R1 URL → 画面 の読みを握り潰す（対象の条件が画面に一切届かない）
//   R2 同・向きを握り潰す（向きが全部 subtree に落ちる＝0件到達の回帰が戻る）
//   R3 画面 → URL の書き戻しから対象と向きを落とす
//   R4 照合の呼び出しは残して**適用の一行だけ削る**（governing が全件を返す）

const dec = (id: string, type: Decision['target']['type'], targetId: string, at = '2026-07-01T00:00:00Z'): Decision => ({
  id,
  target: { type, id: targetId },
  why: 'w',
  at,
});

const ctx = (over: Partial<SelectContext> = {}): SelectContext => ({
  effectOf: () => 'in-force',
  effTagsById: new Map(),
  governs: undefined,
  now: Date.parse('2026-07-28T00:00:00Z'),
  tierOf: () => 1,
  ...over,
});

const cond = (over: Partial<DecisionConditions> = {}): DecisionConditions => ({ ...EMPTY_CONDITIONS, ...over });

// ---------------------------------------------------------------------------
// URL ⇄ 条件（R1・R2・R3）

describe('URL から条件を起こす', () => {
  it('7つの条件すべてを読む（1つでも読み落とすと落ちる）', () => {
    const p: DecisionRouteParams = {
      searchQuery: 'viewer',
      decisionTargetKind: 'tag',
      decisionTag: 'req.x,req.y',
      decisionCurrency: 'all',
      decisionPeriod: '30d',
      decisionOn: 'tag:req.z',
      decisionScope: 'governing',
    };
    expect(conditionsFromRoute(p)).toEqual({
      query: 'viewer',
      targetKind: 'tag',
      tags: ['req.x', 'req.y'],
      currency: 'all',
      period: '30d',
      on: 'tag:req.z',
      scope: 'governing',
    });
  });

  it('省略された条件は既定に落ちる', () => {
    expect(conditionsFromRoute({})).toEqual(EMPTY_CONDITIONS);
    // 効力の既定だけ 'all' ではなく 'current'（一覧には畳む器が無いので、既定の
    // 絞り込みが「効いていないものを本文に混ぜない」役目を果たす・条項4）。
    expect(conditionsFromRoute({}).currency).toBe('current');
  });

  it('知らない値は既定に落ちる（壊れた URL でも一覧は開く）', () => {
    const c = conditionsFromRoute({ decisionTargetKind: 'nope', decisionCurrency: 'nope', decisionPeriod: 'nope' });
    expect([c.targetKind, c.currency, c.period]).toEqual(['all', 'current', 'all']);
  });
});

describe('条件を URL へ書き戻す', () => {
  it('7つの条件すべてを書く（1つでも落とすと落ちる）', () => {
    const p: DecisionRouteParams = {
      searchQuery: 'viewer',
      decisionTargetKind: 'tag',
      decisionTag: 'req.x,req.y',
      decisionCurrency: 'all',
      decisionPeriod: '30d',
      decisionOn: 'tag:req.z',
      decisionScope: 'governing',
    };
    expect(routeParamsFromConditions(conditionsFromRoute(p))).toEqual(p);
  });

  it('既定は書かない（URL を汚さない）', () => {
    const out = routeParamsFromConditions(EMPTY_CONDITIONS);
    for (const [k, v] of Object.entries(out)) expect(v, k).toBeUndefined();
  });

  it('既定の向き（subtree）は書かない', () => {
    expect(routeParamsFromConditions(cond({ on: 'tag:req.x', scope: 'subtree' })).decisionScope).toBeUndefined();
    expect(routeParamsFromConditions(cond({ on: 'tag:req.x', scope: 'own' })).decisionScope).toBe('own');
  });

  it('条件を1つずつ変えると、その分だけ URL が変わる（どの条件も握り潰されない）', () => {
    const changes: Array<[keyof DecisionConditions, unknown, keyof DecisionRouteParams]> = [
      ['query', 'x', 'searchQuery'],
      ['targetKind', 'vocab', 'decisionTargetKind'],
      ['tags', ['req.x'], 'decisionTag'],
      ['currency', 'all', 'decisionCurrency'],
      ['period', '1y', 'decisionPeriod'],
      ['on', 'tag:req.x', 'decisionOn'],
      ['scope', 'governing', 'decisionScope'],
    ];
    for (const [field, value, param] of changes) {
      const out = routeParamsFromConditions(cond({ [field]: value } as Partial<DecisionConditions>));
      expect(out[param], `${String(field)} が URL に載っていない`).toBeDefined();
    }
  });
});

describe('条件の同一判定（書き戻しを、本当に変わったときだけ走らせる）', () => {
  it('同じなら true', () => {
    expect(sameConditions(cond({ tags: ['a'] }), cond({ tags: ['a'] }))).toBe(true);
  });
  it('どの条件が変わっても false になる', () => {
    const base = cond({ query: 'q', targetKind: 'tag', tags: ['a'], currency: 'all', period: '30d', on: 'tag:x', scope: 'own' });
    const diffs: Array<Partial<DecisionConditions>> = [
      { query: 'q2' },
      { targetKind: 'vocab' },
      { tags: ['b'] },
      { tags: ['a', 'b'] },
      { currency: 'current' },
      { period: '1y' },
      { on: 'tag:y' },
      { scope: 'governing' },
    ];
    for (const d of diffs) expect(sameConditions(base, { ...base, ...d }), JSON.stringify(d)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 照合（R4）

describe('条件が実際に集合を絞る', () => {
  const dTag = dec('d1', 'tag', 'req.x');
  const dChild = dec('d2', 'tag', 'req.x.child');
  const dVocab = dec('d3', 'vocab', 'act.user.z');
  const all = [dTag, dChild, dVocab];
  const effTagsById = new Map<string, Set<string>>([
    ['d1', new Set(['req.x'])],
    ['d2', new Set(['req.x.child', 'req.x'])],
    ['d3', new Set(['req.other'])],
  ]);
  const ids = (ds: Decision[]) => ds.map((d) => d.id).sort();

  it('対象の種別で絞る', () => {
    expect(ids(selectDecisions(all, cond({ targetKind: 'vocab' }), ctx({ effTagsById })))).toEqual(['d3']);
  });

  it('効力で絞る（既定は効いているものだけ）', () => {
    const c = ctx({ effTagsById, effectOf: (id) => (id === 'd2' ? 'replaced' : 'in-force') });
    expect(ids(selectDecisions(all, cond(), c))).toEqual(['d1', 'd3']);
    expect(ids(selectDecisions(all, cond({ currency: 'superseded' }), c))).toEqual(['d2']);
    expect(ids(selectDecisions(all, cond({ currency: 'all' }), c))).toEqual(['d1', 'd2', 'd3']);
  });

  it('期間で絞る', () => {
    const old = dec('old', 'tag', 'req.x', '2020-01-01T00:00:00Z');
    expect(ids(selectDecisions([...all, old], cond({ period: '30d' }), ctx({ effTagsById })))).toEqual(['d1', 'd2', 'd3']);
  });

  it('タグ AND で絞る', () => {
    expect(ids(selectDecisions(all, cond({ tags: ['req.x'] }), ctx({ effTagsById })))).toEqual(['d1', 'd2']);
    expect(ids(selectDecisions(all, cond({ tags: ['req.x', 'req.x.child'] }), ctx({ effTagsById })))).toEqual(['d2']);
  });

  it('フリーワードで絞る', () => {
    const c = ctx({ effTagsById, tierOf: (d) => (d.id === 'd1' ? 1 : null) });
    expect(ids(selectDecisions(all, cond({ query: 'x' }), c))).toEqual(['d1']);
  });

  // ⚠️ R4 の型: 「照合の呼び出しは残して適用の一行だけ削る」変異はここで落ちる。
  it('**向きが実際に適用される**（own / subtree）', () => {
    expect(ids(selectDecisions(all, cond({ on: 'tag:req.x', scope: 'own' }), ctx({ effTagsById })))).toEqual(['d1']);
    expect(ids(selectDecisions(all, cond({ on: 'tag:req.x', scope: 'subtree' }), ctx({ effTagsById })))).toEqual(['d1', 'd2']);
  });

  it('**governing が実際に適用される**（問い合わせの結果だけを通す）', () => {
    const governs: GovernsRef[] = [{ decisionId: 'd3', provenance: 'parent', viaTag: 'req.x' }];
    expect(ids(selectDecisions(all, cond({ on: 'tag:req.x', scope: 'governing' }), ctx({ effTagsById, governs })))).toEqual(['d3']);
    // 未取得のあいだは1件も通さない（「支配する規則が全部」と一瞬でも嘘をつかない）
    expect(ids(selectDecisions(all, cond({ on: 'tag:req.x', scope: 'governing' }), ctx({ effTagsById })))).toEqual([]);
  });

  it('候補の母数（selectBase）にも向きが適用される', () => {
    expect(ids(selectBase(all, cond({ on: 'tag:req.x', scope: 'own' }), ctx({ effTagsById })))).toEqual(['d1']);
  });

  it('候補の母数はフリーワードとタグ AND では縮まない（打った瞬間に候補が消えない）', () => {
    const c = ctx({ effTagsById, tierOf: () => null });
    expect(ids(selectBase(all, cond({ query: 'zzz', tags: ['req.x'] }), c))).toEqual(['d1', 'd2', 'd3']);
  });

  it('新しい順に並ぶ', () => {
    const a = dec('a', 'tag', 'r', '2026-01-01T00:00:00Z');
    const b = dec('b', 'tag', 'r', '2026-02-01T00:00:00Z');
    expect(selectDecisions([a, b], cond({ period: 'all' }), ctx()).map((d) => d.id)).toEqual(['b', 'a']);
  });

  it('フリーワードがあるときは関連度順', () => {
    const a = dec('a', 'tag', 'r');
    const b = dec('b', 'tag', 'r');
    const c = ctx({ tierOf: (d) => (d.id === 'b' ? 1 : 3) });
    expect(selectDecisions([a, b], cond({ query: 'x' }), c).map((d) => d.id)).toEqual(['b', 'a']);
  });

  it('matchesTags は実効タグ集合を見る', () => {
    expect(matchesTags(dTag, [], effTagsById)).toBe(true);
    expect(matchesTags(dTag, ['req.x'], effTagsById)).toBe(true);
    expect(matchesTags(dTag, ['req.x.child'], effTagsById)).toBe(false);
  });
});

// ---------------------------------------------------------------------------
// 1件を名指しした URL（旧単票の permalink）

describe('1件を名指しした URL には他の条件を掛けない', () => {
  const target = dec('d1', 'tag', 'req.x');
  const other = dec('d2', 'tag', 'req.y');
  const all = [target, other];

  it('名指しかどうかを判定できる', () => {
    expect(namesOneDecision(cond({ on: 'decision:d1' }))).toBe(true);
    expect(namesOneDecision(cond({ on: 'tag:req.x' }))).toBe(false);
    expect(namesOneDecision(cond())).toBe(false);
  });

  it('置き換え済みでも消えない（既定の効力フィルタを掛けない）', () => {
    // ⚠️ ここが効かないと、**置き換え済みの意思決定を指す共有リンクが0件に着く**。
    // 改訂チェーンを辿る導線は置き換え済みの相手を指すので、日常的に踏まれる経路。
    const c = ctx({ effectOf: () => 'replaced' });
    expect(selectDecisions(all, cond({ on: 'decision:d1' }), c).map((d) => d.id)).toEqual(['d1']);
  });

  it('種別・期間・タグ AND・フリーワードのいずれでも消えない', () => {
    const c = ctx({ tierOf: () => null, effTagsById: new Map([['d1', new Set<string>()]]) });
    const noisy = cond({ on: 'decision:d1', targetKind: 'vocab', period: '30d', tags: ['req.zzz'], query: 'zzzznotfound' });
    expect(selectDecisions(all, noisy, c).map((d) => d.id)).toEqual(['d1']);
  });

  it('存在しない id なら 0 件（呼び手はここで「見つかりません」と言う）', () => {
    expect(selectDecisions(all, cond({ on: 'decision:nope' }), ctx())).toEqual([]);
  });
});
