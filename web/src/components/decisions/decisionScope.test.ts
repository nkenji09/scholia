import { describe, expect, it } from 'vitest';
import type { Decision, GovernsRef } from '../../types';
import {
  DEFAULT_SCOPE,
  formatScopeTarget,
  governsParams,
  needsGoverns,
  parseScopeDirection,
  parseScopeTarget,
  scopeMatcher,
} from './decisionScope';

// 絞り込み条件の「どの対象か」「どの向きか」の値の正しさ（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// ここが嘘をつくと、支配する規則の一覧が違う集合を出す／permalink が1件を出さない
// ——どちらも画面を見ないと分からない形で壊れる。値として検査できる純関数に
// 切り出してあるのはそのため（inheritedSummary と同じ理由）。

const dec = (id: string, type: Decision['target']['type'], targetId: string): Decision => ({
  id,
  target: { type, id: targetId },
  why: 'w',
  at: '2026-01-01T00:00:00Z',
});

describe('対象の綴り', () => {
  it('tag / transition / vocab / decision を読む', () => {
    expect(parseScopeTarget('tag:req.x')).toEqual({ type: 'tag', id: 'req.x' });
    expect(parseScopeTarget('transition:tx.y')).toEqual({ type: 'transition', id: 'tx.y' });
    expect(parseScopeTarget('vocab:act.user.z')).toEqual({ type: 'vocab', id: 'act.user.z' });
    expect(parseScopeTarget('decision:01KX')).toEqual({ type: 'decision', id: '01KX' });
  });

  it('id に含まれるコロンを落とさない（最初の1つだけを区切りに使う）', () => {
    expect(parseScopeTarget('tag:req.a:b')).toEqual({ type: 'tag', id: 'req.a:b' });
  });

  it('旧綴り tx: も読むが、書くときは transition: に正規化する', () => {
    // 2通りの綴りが URL に出回らないように、受けるのは読むときだけ。
    expect(parseScopeTarget('tx:tx.y')).toEqual({ type: 'transition', id: 'tx.y' });
    expect(formatScopeTarget({ type: 'transition', id: 'tx.y' })).toBe('transition:tx.y');
  });

  it('解釈できない値は条件なしに縮退する（壊れた URL でも一覧は開く）', () => {
    for (const raw of [undefined, '', 'req.x', 'nope:req.x', 'tag:', ':req.x']) {
      expect(parseScopeTarget(raw), String(raw)).toBeUndefined();
    }
  });

  it('往復する', () => {
    for (const raw of ['tag:req.x', 'transition:tx.y', 'vocab:act.user.z', 'decision:01KX']) {
      expect(formatScopeTarget(parseScopeTarget(raw)!)).toBe(raw);
    }
  });
});

describe('向き', () => {
  it('3つの値を読む', () => {
    expect(parseScopeDirection('own')).toBe('own');
    expect(parseScopeDirection('governing')).toBe('governing');
    expect(parseScopeDirection('subtree')).toBe('subtree');
  });

  it('省略・未知の値は既定（subtree＝現行の絞り込みが返しているもの）', () => {
    expect(parseScopeDirection(undefined)).toBe(DEFAULT_SCOPE);
    expect(parseScopeDirection('')).toBe(DEFAULT_SCOPE);
    expect(parseScopeDirection('sideways')).toBe(DEFAULT_SCOPE);
    expect(DEFAULT_SCOPE).toBe('subtree');
  });
});

describe('支配する規則の判定は Go コアへ委ねる（viewer に第二実装を置かない）', () => {
  it('governing のときだけ問い合わせが要る', () => {
    expect(needsGoverns({ type: 'tag', id: 'req.x' }, 'governing')).toBe(true);
    expect(needsGoverns({ type: 'tag', id: 'req.x' }, 'own')).toBe(false);
    expect(needsGoverns({ type: 'tag', id: 'req.x' }, 'subtree')).toBe(false);
    expect(needsGoverns(undefined, 'governing')).toBe(false);
  });

  it('1件を名指ししたときは向きが意味を持たない（問い合わせない）', () => {
    expect(needsGoverns({ type: 'decision', id: '01KX' }, 'governing')).toBe(false);
  });

  it('種別ごとに正しいパラメータを組む（CLI の --tag/--tx/--vocab と対応する）', () => {
    expect(governsParams({ type: 'tag', id: 'req.x' }, 'governing')).toEqual({ tag: 'req.x' });
    expect(governsParams({ type: 'transition', id: 'tx.y' }, 'governing')).toEqual({ tx: 'tx.y' });
    expect(governsParams({ type: 'vocab', id: 'act.user.z' }, 'governing')).toEqual({ vocab: 'act.user.z' });
    expect(governsParams({ type: 'tag', id: 'req.x' }, 'own')).toBeUndefined();
  });
});

describe('照合', () => {
  const dOnX = dec('d1', 'tag', 'req.x');
  const dOnChild = dec('d2', 'tag', 'req.x.child');
  const dOnTx = dec('d3', 'transition', 'tx.y');
  const all = [dOnX, dOnChild, dOnTx];
  // 実効タグ集合（own タグの祖先クロージャ）。子は親を含む。
  const effTagsById = new Map<string, Set<string>>([
    ['d1', new Set(['req.x'])],
    ['d2', new Set(['req.x.child', 'req.x'])],
    ['d3', new Set(['req.other'])],
  ]);
  const match = (target: string | undefined, direction: string, governs?: GovernsRef[]) =>
    all
      .filter(
        scopeMatcher({
          target: parseScopeTarget(target),
          direction: parseScopeDirection(direction),
          effTagsById,
          governs,
        }),
      )
      .map((d) => d.id);

  it('対象が無ければ全部通す（この条件は掛かっていない）', () => {
    expect(match(undefined, 'subtree')).toEqual(['d1', 'd2', 'd3']);
  });

  it('1件を名指ししたらその1件だけ（向きは見ない）', () => {
    expect(match('decision:d2', 'governing')).toEqual(['d2']);
    expect(match('decision:d2', 'own')).toEqual(['d2']);
    expect(match('decision:nope', 'own')).toEqual([]);
  });

  it('own はその対象ちょうど（祖先展開なし・CLI の decision list --on と同じ）', () => {
    expect(match('tag:req.x', 'own')).toEqual(['d1']);
    expect(match('transition:tx.y', 'own')).toEqual(['d3']);
  });

  it('own は種別まで見る（同名 id の別種別を拾わない）', () => {
    expect(match('vocab:req.x', 'own')).toEqual([]);
  });

  it('subtree はその対象と配下（実効タグ集合への包含＝現行の絞り込みと同じ）', () => {
    expect(match('tag:req.x', 'subtree')).toEqual(['d1', 'd2']);
  });

  it('配下の概念を持たない対象では subtree は own に落ちる', () => {
    // 遷移・語彙にタグ階層のような配下は無い。黙って全件へ広げない。
    expect(match('transition:tx.y', 'subtree')).toEqual(['d3']);
    expect(match('vocab:act.user.z', 'subtree')).toEqual([]);
  });

  it('governing は問い合わせの結果だけを通す（viewer で導出しない）', () => {
    const governs: GovernsRef[] = [
      { decisionId: 'd1', provenance: 'own' },
      { decisionId: 'd3', provenance: 'parent', viaTag: 'req.x' },
    ];
    expect(match('tag:req.x', 'governing', governs)).toEqual(['d1', 'd3']);
  });

  it('governing は取得前に1件も通さない（「支配する規則が全部」と一瞬でも嘘をつかない）', () => {
    expect(match('tag:req.x', 'governing', undefined)).toEqual([]);
  });

  it('governing が空集合なら空（実効タグ方向へフォールバックしない）', () => {
    // ここでフォールバックすると、向きが逆の集合を「支配する規則」として出す
    // ——追補 01KYJV3FYMDFRWQ939NBV2BPAC が確定した誤りそのものが復活する。
    expect(match('tag:req.x', 'governing', [])).toEqual([]);
  });
});
