import { describe, expect, it } from 'vitest';
import { summarizeInherited } from './inheritedSummary';
import type { GovernsRef } from '../../types';

// 継承した規則の開示に出す件数（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3）。
//
// レビューで「開示ブロックを常に空にする」変異（M4）が配線ガードを素通りした。
// 計算を純関数に切り出したので、ここは**値として**守る——描画の有無に依存しない。
// この件数が嘘をつくと開示が嘘になる（例: own を混ぜると、そのレコード自身の
// 決定が「継承した規則」に化ける）。

const entry = (id: string, provenance: GovernsRef['provenance'], viaTag?: string): GovernsRef => ({
  decisionId: id,
  provenance,
  ...(viaTag ? { viaTag } : {}),
});

const allInForce = () => true;
const name = (id: string) => id;

describe('summarizeInherited', () => {
  it('own は数えない（意思決定欄が本文で出している側）', () => {
    const got = summarizeInherited([entry('a', 'own'), entry('b', 'parent', 'p1')], allInForce, name);
    expect(got.total).toBe(1);
    expect(got.sources).toEqual([{ tagId: 'p1', count: 1 }]);
  });

  it('置き換え済みは数えない（条項5 と同じ数え方）', () => {
    const inForce = (id: string) => id !== 'dead';
    const got = summarizeInherited([entry('dead', 'parent', 'p1'), entry('live', 'parent', 'p1')], inForce, name);
    expect(got.total).toBe(1);
    expect(got.sources).toEqual([{ tagId: 'p1', count: 1 }]);
  });

  it('継承元ごとに束ね、多い順に並べる', () => {
    const got = summarizeInherited(
      [entry('a', 'parent', 'few'), entry('b', 'effective-tag', 'many'), entry('c', 'effective-tag', 'many')],
      allInForce,
      name,
    );
    expect(got.sources).toEqual([
      { tagId: 'many', count: 2 },
      { tagId: 'few', count: 1 },
    ]);
  });

  it('総数は継承元チップの合計と必ず一致する（開示した数＝たどって読める数）', () => {
    const got = summarizeInherited(
      [entry('a', 'parent', 'p1'), entry('b', 'parent', 'p2'), entry('c', 'effective-tag', 'p1')],
      allInForce,
      name,
    );
    expect(got.total).toBe(got.sources.reduce((n, s) => n + s.count, 0));
    expect(got.total).toBe(3);
  });

  it('継承が無ければ 0（呼び手はここで口を出さない）', () => {
    expect(summarizeInherited([entry('a', 'own')], allInForce, name)).toEqual({ total: 0, sources: [] });
    expect(summarizeInherited([], allInForce, name)).toEqual({ total: 0, sources: [] });
  });

  it('経由が分からないものは名乗れないので数にも入れない', () => {
    // viaTag 欠落分を total に足すと、チップの合計より多い件数を開示してしまう。
    const got = summarizeInherited([entry('a', 'parent'), entry('b', 'parent', 'p1')], allInForce, name);
    expect(got.total).toBe(1);
  });
});
