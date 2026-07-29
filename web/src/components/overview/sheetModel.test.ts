import { describe, it, expect } from 'vitest';
import { buildDirectByTag, componentBehaviorTxIds } from './sheetModel';
import type { Transition, VocabEntry } from '../../types';

// 「どの遷移を、どのコンポーネントの振る舞いとして描くか」を、**入力に対する答え**
// として検査する（CLAUDE.md「配線ガードの書き方」1）。
//
// **何に落ちるか**: 祖先展開の混入（親のシートに子の振る舞いが出る）、vocab 経由の
// タグの取りこぼし、構成要素があるときの二重表示。
// **何に落ちないか**: 出した遷移が画面のカードとして実際に描かれるか（描画ガードが
// 見る）。この file は選択だけを見る。

const vocab = (...vs: VocabEntry[]) => new Map(vs.map((v) => [v.id, v]));

describe('直下の遷移の索引（祖先展開を通さない）', () => {
  const TX: Transition[] = [
    { id: 'T-child', action: 'a', given: [], then: ['e'], tags: ['comp.parent.child'] },
    { id: 'T-parent', action: 'a', given: [], then: ['e'], tags: ['comp.parent'] },
  ];
  const V = vocab({ id: 'a', category: 'action', label: 'A' }, { id: 'e', category: 'effect', label: 'E' });

  it('親タグには、子タグに付いた遷移が入らない', () => {
    const m = buildDirectByTag(TX, V);
    // ⚠️ ここが satByTag（祖先展開込み）との違い。祖先展開を使うと親は
    // ['T-parent','T-child'] になり、親のシートに子の振る舞いが再掲される。
    expect(m.get('comp.parent')).toEqual(['T-parent']);
    expect(m.get('comp.parent.child')).toEqual(['T-child']);
  });

  it('遷移が参照する vocab に付いたタグも、その遷移の実効タグとして拾う', () => {
    const m = buildDirectByTag([{ id: 'T', action: 'a', given: ['g'], then: ['e'] }], vocab(
      { id: 'a', category: 'action', label: 'A', tags: ['comp.x'] },
      { id: 'g', category: 'condition', label: 'G', tags: ['comp.y'] },
      { id: 'e', category: 'effect', label: 'E', tags: ['comp.z'] },
    ));
    expect(m.get('comp.x')).toEqual(['T']);
    expect(m.get('comp.y')).toEqual(['T']);
    expect(m.get('comp.z')).toEqual(['T']);
  });

  it('同じタグが tx.tags と vocab.tags の両方から来ても、遷移は1回だけ数える', () => {
    const m = buildDirectByTag([{ id: 'T', action: 'a', given: [], then: [], tags: ['comp.x'] }], vocab({ id: 'a', category: 'action', label: 'A', tags: ['comp.x'] }));
    expect(m.get('comp.x')).toEqual(['T']);
  });

  it('どのタグにも付いていない遷移は、どのコンポーネントにも現れない', () => {
    const m = buildDirectByTag([{ id: 'T', action: 'a', given: [], then: [] }], vocab({ id: 'a', category: 'action', label: 'A' }));
    expect(m.size).toBe(0);
  });
});

describe('コンポーネント自身の振る舞いとして描く遷移', () => {
  it('構成要素を持たないコンポーネントでは、直下の遷移をそのまま描く', () => {
    expect(componentBehaviorTxIds({ partCount: 0, directTxIds: ['T1', 'T2'] })).toEqual(['T1', 'T2']);
  });

  it('構成要素を持つコンポーネントでは描かない（構成要素側が描くので二重に出さない）', () => {
    expect(componentBehaviorTxIds({ partCount: 1, directTxIds: ['T1', 'T2'] })).toEqual([]);
  });

  it('直下の遷移が無ければ空（記録側の穴は埋めずに空のまま見せる）', () => {
    expect(componentBehaviorTxIds({ partCount: 0, directTxIds: [] })).toEqual([]);
  });
});
