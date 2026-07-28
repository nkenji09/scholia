import { describe, expect, it } from 'vitest';
import type { ViewName } from '../../router';
import { NAV_FAMILY, activeNavKey, isNavActive } from './navActive';
import type { NavKey } from './navActive';

// どのタブが点灯するかを**値として**検査する（CLAUDE.md「配線ガードの書き方」1）。
//
// 差し戻し1回目で、**家族の定数はそのまま・判定の分岐だけを `return false` に潰す**
// 変異がテスト緑で素通りした——意思決定タブが一度も点灯しなくなるのに、定数だけを
// 見るガードは無傷だった。全ルートを列挙して「どのタブが点くか」を固定すれば、
// 綴りに関係なく落ちる。

/** router の ViewName 全部と、そこで点灯するべきタブ。
    ⚠️ ルートを足したらここにも足すこと——足し忘れは下の網羅テストで落ちる。 */
const EXPECTED: Record<ViewName, NavKey | null> = {
  overview: 'overview',
  home: 'overview',
  tags: 'tags',
  spec: 'tags',
  browse: 'tags',
  vocab: 'tags',
  flow: 'tags',
  decisions: 'decisions',
  decision: 'decisions',
  // 設定は歯車が別に点く既存の意匠なので、ナビのタブは点かない。
  config: null,
};

describe('ナビの点灯', () => {
  for (const [view, expected] of Object.entries(EXPECTED) as Array<[ViewName, NavKey | null]>) {
    it(`${view} では ${expected ?? '（どのタブも点かない）'}`, () => {
      expect(activeNavKey(view)).toBe(expected);
    });
  }

  it('1つの面で2つのタブが同時に点灯しない（同じタブが両方を名乗らない）', () => {
    for (const view of Object.keys(EXPECTED) as ViewName[]) {
      const lit = (Object.keys(NAV_FAMILY) as NavKey[]).filter((k) => isNavActive(k, view));
      expect(lit.length, `${view} で ${lit.length} 個のタブが点灯している`).toBeLessThanOrEqual(1);
    }
  });

  it('意思決定の面は「意思決定」で点く（転送で残した旧単票の URL も）', () => {
    expect(isNavActive('decisions', 'decisions')).toBe(true);
    expect(isNavActive('decisions', 'decision')).toBe(true);
    expect(isNavActive('tags', 'decisions')).toBe(false);
    expect(isNavActive('tags', 'decision')).toBe(false);
  });

  it('語彙・フロー・遷移はタブを名乗らず「タグ」側のレンズとして残る', () => {
    for (const v of ['vocab', 'flow', 'browse', 'spec'] as ViewName[]) expect(isNavActive('tags', v), v).toBe(true);
  });

  it('点灯しない面は設定だけ（新しい面を足して「どのタブも点かない」を増やさない）', () => {
    const dark = (Object.keys(EXPECTED) as ViewName[]).filter((v) => activeNavKey(v) === null);
    expect(dark).toEqual(['config']);
  });
});
