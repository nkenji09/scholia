import { describe, it, expect } from 'vitest';
import { buildDirectByTag, buildPartTree, componentBehaviorTxIds, countPartPanels, panelPathTo, sheetRuleCount } from './sheetModel';
import type { Transition, VocabEntry } from '../../types';

// 「どの遷移を、どのコンポーネントの振る舞いとして描くか」と「構成要素の欄を
// どう組むか」を、**入力に対する答え**として検査する（CLAUDE.md「配線ガードの書き方」1）。
//
// **何に落ちるか**: 祖先展開の混入（親のシートに子の振る舞いが出る）、vocab 経由の
// タグの取りこぼし、構成要素があるときの二重表示、**入れ子の欄を組まない／1段で止める**、
// **多親を同じシートに2回出す／どこにも出さない**、**置き場所を走査順に委ねる**、
// **見出しの件数が入れ子の中を数えない**。
// **何に落ちないか**: 出した遷移が画面のカードとして実際に描かれるか・欄が畳まれて
// いるか・開示の行が実アンカーで描かれるか（`renderWiring.test.tsx` が描画を起こして見る）。
// この file は選択と構造だけを見る。
//
// ⚠️ **この file が緑でも「配線が生きている」ことにはならない。** 答えを1つ外側で
// 捨てる／痩せた材料を渡す形はここでは1件も落ちない——この repo が4回以上踏んでいる型。

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

// ===========================================================================
// 構成要素の欄を、記録の親子どおりに組む（①入れ子 ＋ ③多親 B′）
// ===========================================================================
//
// ## この describe が落とすもの（射程・`CLAUDE.md` 6）
//
//   ・**入れ子の欄を組まない／1段で止める。** 3段以上の入力で構造そのものを固定する。
//   ・**欄に祖先展開込みの索引を使う。** 欄が出す遷移は `directTxIdsOf` の答えちょうど
//     でなければならない（親の欄に配下の分が混ざる形が落ちる）。
//   ・**多親を1枚のシートに2回出す**（案B へ戻す変異）。
//   ・**多親をどこにも出さない／片方のシートから落とす**（案A へ戻す変異）。
//   ・**置き場所を走査順に委ねる。** 記録の親の順を逆にすると答えが変わることを見るので、
//     「シートを上から降りて最初に出会った親」に置く実装は落ちる。
//   ・**もう一方の親の開示を落とす／居場所でない親（要件タグ）まで開示に混ぜる。**
//
// ## この describe が落とさないもの（名指しする）
//
//   1. **配線。** 欄が実際に描かれるか、初期表示で畳まれているか、開示が実アンカーか。
//   2. **記録が祖先と子孫の両方に同じ遷移を直接貼った形。** それはここで消す対象では
//      ないので、**歯止めも無い**（実測でそういう記録では二重が6組残る）。
//   3. **親子関係の無い欄どうしに同じ遷移が出ること**（実測20組）。消してはいけない。
describe('構成要素の欄を、記録の親子どおりに入れ子で組む', () => {
  /** 記録（親→子・子→親・直接付いた遷移）から欄の木を組む小さな足場。 */
  function build(
    tags: Array<{ id: string; kind: string; parentIds?: string[]; tx?: string[] }>,
    componentId: string,
  ) {
    const byId = new Map(tags.map((t) => [t.id, t]));
    const kids = new Map<string, string[]>();
    for (const t of tags) for (const p of t.parentIds || []) kids.set(p, [...(kids.get(p) || []), t.id]);
    return buildPartTree({
      componentId,
      childIdsOf: (id) => kids.get(id) || [],
      parentIdsOf: (id) => byId.get(id)?.parentIds || [],
      isPart: (id) => byId.get(id)?.kind === 'piece',
      isPlace: (id) => byId.get(id)?.kind === 'piece' || byId.get(id)?.kind === 'subject',
      directTxIdsOf: (id) => byId.get(id)?.tx || [],
    });
  }
  /** 木を `親>子` の連なりに畳んで、構造そのものを1つの値として突き合わせる。 */
  function shape(nodes: ReturnType<typeof build>, prefix = ''): string[] {
    const out: string[] = [];
    for (const n of nodes) {
      const here = prefix ? `${prefix}>${n.id}` : n.id;
      out.push(here);
      out.push(...shape(n.children, here));
    }
    return out;
  }

  it('3段以上の入れ子を、記録の親子どおりに組む（段の深さに上限を置かない）', () => {
    const tree = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'p1', kind: 'piece', parentIds: ['c'] },
        { id: 'p2', kind: 'piece', parentIds: ['p1'] },
        { id: 'p3', kind: 'piece', parentIds: ['p2'] },
        { id: 'p4', kind: 'piece', parentIds: ['p3'] },
      ],
      'c',
    );
    // ⚠️ **1段で止める実装は `['p1']` を返す。** 構造そのものを固定して落とす。
    expect(shape(tree)).toEqual(['p1', 'p1>p2', 'p1>p2>p3', 'p1>p2>p3>p4']);
  });

  it('各欄が出す遷移は「その構成要素に直接付いた分」だけ', () => {
    const tree = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'p1', kind: 'piece', parentIds: ['c'], tx: ['T-p1'] },
        { id: 'p2', kind: 'piece', parentIds: ['p1'], tx: ['T-p2'] },
      ],
      'c',
    );
    // ⚠️ 祖先展開込みの索引を渡すと p1 は ['T-p1','T-p2'] になり、**同じ遷移が
    // 1枚のシートの中で二重に出る**。答えちょうどであることを見る。
    expect(tree[0].txIds).toEqual(['T-p1']);
    expect(tree[0].children[0].txIds).toEqual(['T-p2']);
  });

  it('役割を持たない子は欄にならない（要件タグが欄に化けない）', () => {
    const tree = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'req.x', kind: 'requirement', parentIds: ['c'] },
        { id: 'p1', kind: 'piece', parentIds: ['c'] },
        // 構成要素の下の要件タグも欄にしない。
        { id: 'req.y', kind: 'requirement', parentIds: ['p1'] },
      ],
      'c',
    );
    expect(shape(tree)).toEqual(['p1']);
  });

  it('別のコンポーネントのシートには、それぞれ1回ずつ出る（案A へ戻す変異を落とす）', () => {
    const TAGS = [
      { id: 'c1', kind: 'subject' },
      { id: 'c2', kind: 'subject' },
      { id: 'shared', kind: 'piece', parentIds: ['c1', 'c2'] },
    ];
    // ⚠️ **「記録の1つ目の親に固定する」実装は c2 のシートから欄を消す**（実測で、
    // いま既に読めているものを失う形）。両方のシートに出ることを見る。
    expect(shape(build(TAGS, 'c1'))).toEqual(['shared']);
    expect(shape(build(TAGS, 'c2'))).toEqual(['shared']);
  });

  it('親が別コンポーネントの構成要素どうしでも、両方のシートから読める', () => {
    const TAGS = [
      { id: 'c1', kind: 'subject' },
      { id: 'c2', kind: 'subject' },
      { id: 'a', kind: 'piece', parentIds: ['c1'] },
      { id: 'b', kind: 'piece', parentIds: ['c2'] },
      { id: 'shared', kind: 'piece', parentIds: ['a', 'b'] },
    ];
    expect(shape(build(TAGS, 'c1'))).toEqual(['a', 'a>shared']);
    expect(shape(build(TAGS, 'c2'))).toEqual(['b', 'b>shared']);
  });

  it('1枚のシートの中では1回だけ出す（案B へ戻す変異を落とす）', () => {
    // 同じシートに2つの親が居る2形——(i) 親子（コンポーネントとその構成要素）と
    // (ii) 兄弟（同じコンポーネントの2つの構成要素）。どちらも欄は1つ。
    const nested = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'p1', kind: 'piece', parentIds: ['c'] },
        { id: 'mixed', kind: 'piece', parentIds: ['c', 'p1'] },
      ],
      'c',
    );
    expect(shape(nested)).toEqual(['p1', 'mixed']);
    const siblings = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'p1', kind: 'piece', parentIds: ['c'] },
        { id: 'p2', kind: 'piece', parentIds: ['c'] },
        { id: 'pair', kind: 'piece', parentIds: ['p1', 'p2'] },
      ],
      'c',
    );
    expect(shape(siblings)).toEqual(['p1', 'p1>pair', 'p2']);
  });

  it('どの親の下に置くかは「記録の親の順」で決まる（走査順に依らない）', () => {
    // ⚠️ **これがモックの規則を落とす1件。** モックは「シートを上から降りて最初に
    // 出会った親」に置いていた＝欄の描き順という実装の偶然に居場所を委ねる形で、
    // その実装では下の2つが**同じ答え**になる（どちらも p1 が先に描かれるため）。
    // 記録の順に従う実装だけが、2つを別の答えにできる。
    const TAGS = (parents: string[]) => [
      { id: 'c', kind: 'subject' },
      { id: 'p1', kind: 'piece', parentIds: ['c'] },
      { id: 'p2', kind: 'piece', parentIds: ['c'] },
      { id: 'pair', kind: 'piece', parentIds: parents },
    ];
    expect(shape(build(TAGS(['p1', 'p2']), 'c'))).toEqual(['p1', 'p1>pair', 'p2']);
    expect(shape(build(TAGS(['p2', 'p1']), 'c'))).toEqual(['p1', 'p2', 'p2>pair']);
  });

  it('置かなかった親は開示に残る（別のコンポーネントに属する親も含む）', () => {
    const tree = build(
      [
        { id: 'c1', kind: 'subject' },
        { id: 'c2', kind: 'subject' },
        { id: 'a', kind: 'piece', parentIds: ['c1'] },
        { id: 'b', kind: 'piece', parentIds: ['c2'] },
        { id: 'shared', kind: 'piece', parentIds: ['a', 'b'] },
      ],
      'c1',
    );
    // ⚠️ **c1 のシートでは b（別のコンポーネントの構成要素）を開示する。**
    // 同じシートの中だけを開示する実装は、この1件で落ちる——そして「1つの部品が
    // 複数のコンポーネントにまたがっている」という、この単位の要件が読めなくなる。
    expect(tree[0].children[0].otherParentIds).toEqual(['b']);
    // 置いた側の親は開示に出さない（同じことを位置と言葉で2回言わない）。
    expect(tree[0].otherParentIds).toEqual([]);
  });

  it('居場所にならない親（要件タグ）は開示に混ぜない', () => {
    const tree = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'req.x', kind: 'requirement' },
        { id: 'p1', kind: 'piece', parentIds: ['c', 'req.x'] },
      ],
      'c',
    );
    expect(tree[0].otherParentIds).toEqual([]);
  });

  it('循環しても止まる（記録側の穴で画面を落とさない）', () => {
    const tree = build(
      [
        { id: 'c', kind: 'subject' },
        { id: 'p1', kind: 'piece', parentIds: ['c', 'p2'] },
        { id: 'p2', kind: 'piece', parentIds: ['p1'] },
      ],
      'c',
    );
    // p1 は記録の順で c の下（`['c','p2']` の最初）、p2 はその下。欄は各1つ。
    expect(shape(tree)).toEqual(['p1', 'p1>p2']);
  });
});

// ===========================================================================
// そのシートの中で、目当ての欄までの間の段
// ===========================================================================
//
// ## この describe が落とすもの（射程・`CLAUDE.md` 6）
//
//   ・**間の段を1段しか返さない／自分を含めてしまう／深い順で返す。**
//   ・**そのシートに無い欄について、空配列を返してしまう**（＝「間の段は無い」と
//     「このシートに無い」を取り違える形）。呼び出し側はこの2つで別のことをする。
//
// ## この describe が落とさないもの（名指しする）
//
//   ・**呼び出し側がこの答えを使わず、記録を上へ辿った道を使う形**（＝F1 の欠陥そのもの）。
//     それは `renderWiring.test.tsx` が描画を起こして落とす。
describe('シートの欄の木の中で、目当ての欄までの間の段', () => {
  /** `panelPathTo` は `PartNode` の `id`/`children` だけを見る。 */
  type Node = { id: string; txIds: string[]; children: Node[]; otherParentIds: string[] };
  const n = (id: string, children: Node[] = []): Node => ({ id, txIds: [], children, otherParentIds: [] });

  it('浅い順に、目当て自身を含まない経路を返す', () => {
    const tree = [n('a', [n('b', [n('c')])]), n('d')];
    expect(panelPathTo(tree, 'c')).toEqual(['a', 'b']);
    expect(panelPathTo(tree, 'b')).toEqual(['a']);
    // シート直下の欄には間の段が無い＝空配列（null ではない）。
    expect(panelPathTo(tree, 'a')).toEqual([]);
    expect(panelPathTo(tree, 'd')).toEqual([]);
  });

  it('そのシートに無い欄は null（「間の段が無い」と区別する）', () => {
    // ⚠️ ここで空配列を返すと、呼び出し側は「間の段は無いが欄は在る」と読む。
    // **多親では「この欄はこのシートには無い」が起きる**ので、その2つは区別が要る。
    expect(panelPathTo([n('a', [n('b')])], 'zz')).toBeNull();
    expect(panelPathTo([], 'a')).toBeNull();
  });

  it('多親の欄は、シートごとに違う経路を返す（同じ id でも道が違う）', () => {
    // 2つのシートの木を、`buildPartTree` が実際に返す形で作って突き合わせる。
    const TAGS = [
      { id: 'c1', kind: 'subject' },
      { id: 'c2', kind: 'subject' },
      { id: 'a', kind: 'piece', parentIds: ['c1'] },
      { id: 'b', kind: 'piece', parentIds: ['c2'] },
      { id: 'shared', kind: 'piece', parentIds: ['a', 'b'] },
    ];
    const byId = new Map(TAGS.map((t) => [t.id, t]));
    const kids = new Map<string, string[]>();
    for (const t of TAGS) for (const p of t.parentIds || []) kids.set(p, [...(kids.get(p) || []), t.id]);
    const treeOf = (componentId: string) =>
      buildPartTree({
        componentId,
        childIdsOf: (id) => kids.get(id) || [],
        parentIdsOf: (id) => byId.get(id)?.parentIds || [],
        isPart: (id) => byId.get(id)?.kind === 'piece',
        isPlace: (id) => byId.get(id)?.kind === 'piece' || byId.get(id)?.kind === 'subject',
        directTxIdsOf: () => [],
      });
    // ⚠️ **これが F1 の核心。** 同じ `shared` について、シートごとに間の段が違う。
    // 記録を上へ辿る判定は片方（記録の順＝`a`）しか返さないので、**もう一方のシートでは
    // 間の段を開けられない**。そのシートの木に聞けば、そのシートの答えが出る。
    expect(panelPathTo(treeOf('c1'), 'shared')).toEqual(['a']);
    expect(panelPathTo(treeOf('c2'), 'shared')).toEqual(['b']);
  });
});

describe('シートに出る欄の総数（見出しの「構成要素 N」）', () => {
  it('入れ子の欄まで数える', () => {
    // ⚠️ 直下の子の数で数える実装は 1 を返す。見出しの数と、開いて見える欄の数が
    // 食い違う（`01KYHW54B8ZXH0NEPH2J7N1X39` 条項5）。
    expect(countPartPanels([{ children: [{ children: [{ children: [] }] }] }])).toBe(3);
  });
  it('欄が無ければ 0', () => {
    expect(countPartPanels([])).toBe(0);
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
  type Bearing = { rules: string[]; behaviors: Array<{ rules: string[] }>; children: Bearing[] };
  const part = (rules: string[], behaviors: string[][] = [], children: Bearing[] = []): Bearing => ({
    rules,
    behaviors: behaviors.map((r) => ({ rules: r })),
    children,
  });

  it('5系統すべてを数える（構成要素・その配下の振る舞い・入れ子の欄・直下の振る舞い・制約）', () => {
    const n = sheetRuleCount(
      {
        partBlocks: [part(['a'], [['b', 'c']], [part(['f'], [['g']])])],
        ownBehaviors: [{ rules: ['d'] }],
        propBlocks: [{ rules: ['e'] }],
      },
      inForce,
    );
    expect(n).toBe(7);
  });

  it('入れ子の欄の分を落とさない（1段で止める変異を落とす）', () => {
    // ⚠️ **入れ子は5系統目である。** 1段で止めると「N と言っているのに、開いて
    // 読める規則のほうが多い」になる。
    expect(sheetRuleCount({ ...empty, partBlocks: [part([], [], [part(['a'], [['b']])])] }, inForce)).toBe(2);
    // 段が深くなっても数える（2段目で止める変異も落とす）。
    expect(sheetRuleCount({ ...empty, partBlocks: [part([], [], [part([], [], [part(['a'])])])] }, inForce)).toBe(1);
  });

  it('直下の振る舞いの分を落とさない（決定の条項そのもの）', () => {
    // ⚠️ この1件がこの describe の主眼。落とすと「N と言っているのに N 件見つからない」。
    expect(sheetRuleCount({ ...empty, ownBehaviors: [{ rules: ['a', 'b'] }] }, inForce)).toBe(2);
  });

  it('効いていない規則は数えない（見出しの件数は効いている数）', () => {
    expect(sheetRuleCount({ ...empty, ownBehaviors: [{ rules: ['a', 'x1', 'x2'] }] }, inForce)).toBe(1);
    // 入れ子の中でも同じ判定が効く。
    expect(sheetRuleCount({ ...empty, partBlocks: [part([], [], [part(['a', 'x1'])])] }, inForce)).toBe(1);
  });

  it('何も無ければ 0', () => {
    expect(sheetRuleCount(empty, inForce)).toBe(0);
  });
});
