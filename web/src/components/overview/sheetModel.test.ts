import { describe, it, expect } from 'vitest';
import { buildDirectByTag, componentBehaviorTxIds, sheetRuleCount } from './sheetModel';
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

// 見出しの「現行ルール N」。⚠️ **この足し算は OverviewView の中にあり、条項を1つ
// 落とす変異が何にも落ちなかった**（見出しの数字は変わるが、その数字が正しいかを
// 値として見ている検査がどこにも無かった）。ここへ出したので落とせる。
describe('見出しの「現行ルール N」＝シートの中で開いて読める規則の数', () => {
  // 効力の判定はここでは持たない（decisionModel の役目）。'x' で始まるものを
  // 「置き換え済み」と見なす軽い代役を渡して、数え方だけを見る。
  const inForce = (d: string) => !d.startsWith('x');
  const empty = { partBlocks: [], ownBehaviors: [], propBlocks: [] };

  it('4系統すべてを数える（構成要素・その配下の振る舞い・直下の振る舞い・制約）', () => {
    const n = sheetRuleCount(
      {
        partBlocks: [{ rules: ['a'], behaviors: [{ rules: ['b', 'c'] }] }],
        ownBehaviors: [{ rules: ['d'] }],
        propBlocks: [{ rules: ['e'] }],
      },
      inForce,
    );
    expect(n).toBe(5);
  });

  it('直下の振る舞いの分を落とさない（決定の条項そのもの）', () => {
    // ⚠️ この1件がこの describe の主眼。落とすと「N と言っているのに N 件見つからない」。
    expect(sheetRuleCount({ ...empty, ownBehaviors: [{ rules: ['a', 'b'] }] }, inForce)).toBe(2);
  });

  it('効いていない規則は数えない（見出しの件数は効いている数）', () => {
    expect(sheetRuleCount({ ...empty, ownBehaviors: [{ rules: ['a', 'x1', 'x2'] }] }, inForce)).toBe(1);
  });

  it('何も無ければ 0', () => {
    expect(sheetRuleCount(empty, inForce)).toBe(0);
  });
});
