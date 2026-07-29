import { describe, expect, it } from 'vitest';
import { forwardedOverviewTarget, isStructuralKind, structuralRootIds, treeRowAction } from './treeModel';
import type { TreeRoles } from './treeModel';

// ===========================================================================
// 構造ツリーの「並べる資格」と「押した結果」を、入力に対する答えとして検査する
// ===========================================================================
//
// ## このガードが落とすもの（射程・`CLAUDE.md` 6）
//
//   ・**役割を持たない種類がツリーに混ざる。** 綴りではなく「役割3つ以外は通さない」と
//     いう向きで見るので、**新しい種類が増えても**同じ答えになる（列挙で追わない）。
//   ・**起点と子で違う規則を使う。** 同じ述語を両方に効かせていることを、
//     「起点にも子にも居る同じタグ」で確かめる。
//   ・**押しても何も起きない行が作れる形。** 返り値を4通りに閉じ、
//     `toggle` が返るのは開閉できるときだけ、を値で見る。
//   ・**役割の解決をリテラル kind id に戻す。** 役割 id を慣用名と**別のもの**にした
//     世界で同じ答えが出ることを見る（`01KYCC2THS5RX3HB27SQGFWSA5`）。
//   ・**構成要素へ移したタグを指す共有 URL の転送を落とす／効かせすぎる。**
//
// ## このガードが落とさないもの（名指しする）
//
//   1. **配線。** ここは純関数の答えだけを見る。画面がこの答えを**呼ばない**／
//      呼んで**捨てる**／**痩せた材料を渡す**形は、**ここでは1件も落ちない。**
//      ⚠️ これは絵空事ではない——転送について、答えを計算して使わない／転送先から
//      構成要素の id を落とす／親を探す材料を常に null にする、の3通りが**すべて
//      このファイルを素通りした**（レビュー実測）。**配線は `renderWiring.test.tsx` が
//      描画を起こして見る**。片方だけ当てて「red を実見した」と書かないこと。
//   2. **行き先の正しさ。** 「タグの詳細へ送る」と決めたことは見るが、その URL を
//      組み立てる `routeHash` の綴りは見ていない。
//   3. **構成要素の入れ子。** 構成要素の下の構成要素は本実装の範囲外で、
//      いまは「親コンポーネントが見つからない」として詳細へ落ちる。**その落ち方が
//      正しいと主張しているのではなく、範囲外だと名乗っている**（別単位で扱う）。
//   4. **要件タグをツリーに出すかどうかの根拠**そのもの。ここが見ているのは
//      「役割3つだけを通す」という形であって、正本がそう定めていることは見ていない。

/** この repo の実データと同じ形——役割 component を `subject` が担い、
    part / group は別 id、慣用 id（`component` 等）はどこにも無い世界。 */
const ROLES: TreeRoles = { group: 'grp', component: 'subject', part: 'piece' };

/** 慣用 id をそのまま使う世界（宣言していないプロジェクト）。 */
const LITERAL_ROLES: TreeRoles = { group: 'group', component: 'component', part: 'part' };

describe('構造ツリーに並ぶ資格は、役割3つだけが持つ', () => {
  it('役割3つは通り、それ以外はどんな種類でも通らない', () => {
    for (const k of [ROLES.group, ROLES.component, ROLES.part]) {
      expect(isStructuralKind(k, ROLES), `役割 ${k} が落ちている`).toBe(true);
    }
    // ⚠️ **列挙ではなく向きで見る。** 実データに在る3種（軸・関心・要件）だけを並べると、
    // 「その3つを除く」という実装でも通ってしまう。**この場で作った知らない種類**も
    // 落ちることを見て、「役割以外は通さない」という向きを固定する。
    for (const k of ['axis', 'concern', 'requirement', 'property', 'note', 'zz-未知の種類', '']) {
      expect(isStructuralKind(k, ROLES), `役割でない ${k} が通っている`).toBe(false);
    }
    expect(isStructuralKind(undefined, ROLES), '種類の無いタグが通っている').toBe(false);
  });

  it('役割 id をリテラルに固定していない（慣用 id が1つも無い世界でも同じ答え）', () => {
    // ROLES の世界に `component` という id は無い。リテラルで判定していると通ってしまう。
    expect(isStructuralKind('component', ROLES), 'リテラル component を通している').toBe(false);
    expect(isStructuralKind('subject', ROLES), '宣言された役割 id を落としている').toBe(true);
    // 逆向き: 宣言していないプロジェクトでは慣用 id が通る。
    expect(isStructuralKind('component', LITERAL_ROLES)).toBe(true);
    expect(isStructuralKind('subject', LITERAL_ROLES)).toBe(false);
  });
});

describe('起点と子は、同じ資格判定を通る', () => {
  // ⚠️ **是正前の欠陥そのものの形**: 要件タグが「子」としては除かれるのに
  // 「いちばん上」としては除かれず、最上段に残っていた。
  const TAGS = [
    { id: 'subject.cli', kind: 'subject' },
    { id: 'subject.viewer', kind: 'subject' },
    { id: 'piece.tags', kind: 'piece', parentIds: ['subject.viewer'] },
    // 親を持たない要件（＝起点の候補になってしまう形）
    { id: 'req.top', kind: 'requirement' },
    // 同じ種類が「子」の位置にも居る
    { id: 'req.child', kind: 'requirement', parentIds: ['subject.cli'] },
    { id: 'axis.x', kind: 'axis' },
  ];
  const kindOf = (id: string) => TAGS.find((t) => t.id === id)?.kind;

  it('親を持たないタグを起点にするとき、役割を持たないものは起点にならない', () => {
    const roots = structuralRootIds({ tags: TAGS, configRoots: [], kindOf, roles: ROLES });
    expect(roots).toEqual(['subject.cli', 'subject.viewer']);
  });

  it('config が起点を指定していても、同じ資格判定が効く', () => {
    // ⚠️ 是正前は config 側の経路にだけ絞りが無い状態も作れた。両方の経路を踏む。
    const roots = structuralRootIds({
      tags: TAGS,
      configRoots: ['subject.cli', 'req.top', 'axis.x', 'piece.tags'],
      kindOf,
      roles: ROLES,
    });
    expect(roots).toEqual(['subject.cli', 'piece.tags']);
  });

  it('同じタグが起点の位置でも子の位置でも、同じ答えになる', () => {
    // 「要件は子としては除くが起点では除かない」という**非対称そのもの**を落とす。
    const asRoot = structuralRootIds({ tags: TAGS, configRoots: ['req.top'], kindOf, roles: ROLES }).length > 0;
    const asChild = isStructuralKind(kindOf('req.child'), ROLES);
    expect(asRoot, '起点としては通っている').toBe(false);
    expect(asChild, '子としては通っている').toBe(false);
    expect(asRoot).toBe(asChild);
  });
});

describe('行を押したときの答えは4通りで、「何も起きない」は無い', () => {
  const call = (tag: { id: string; kind?: string }, structuralChildCount: number, componentParentId: string | null = null) =>
    treeRowAction({ tag, structuralChildCount, componentParentId, roles: ROLES });

  it('コンポーネントは自分の仕様シートを開く', () => {
    expect(call({ id: 'subject.cli', kind: 'subject' }, 0)).toEqual({ kind: 'component', componentId: 'subject.cli' });
    // 子を持っていてもシートを開く（開閉に化けない）。
    expect(call({ id: 'subject.viewer', kind: 'subject' }, 9)).toEqual({ kind: 'component', componentId: 'subject.viewer' });
  });

  it('構成要素は親コンポーネントのシートの該当箇所へ寄る', () => {
    expect(call({ id: 'piece.tags', kind: 'piece' }, 0, 'subject.viewer')).toEqual({
      kind: 'part',
      componentId: 'subject.viewer',
      partId: 'piece.tags',
    });
  });

  it('親コンポーネントが見つからない構成要素は、詳細へ送る（無反応にしない）', () => {
    // ⚠️ これは「入れ子を正しく扱っている」ではなく「範囲外を無反応にしない」という主張。
    expect(call({ id: 'piece.orphan', kind: 'piece' }, 0, null)).toEqual({ kind: 'detail', tagId: 'piece.orphan' });
  });

  it('束ねる段は、開閉できるときだけ開閉になる', () => {
    expect(call({ id: 'grp.a', kind: 'grp' }, 3)).toEqual({ kind: 'toggle' });
    // 空の束は記録側の穴だが、**押して何も起きない行にはしない**。
    expect(call({ id: 'grp.empty', kind: 'grp' }, 0)).toEqual({ kind: 'detail', tagId: 'grp.empty' });
  });

  it('どんな入力でも「何も起きない」答えは返らない', () => {
    // ⚠️ 是正前の欠陥は「三角も出ず・リンクでもない」という**組み合わせ**だった。
    // toggle が返るのは開閉できるときだけ、という対応が崩れると再発する。
    const kinds = [ROLES.group, ROLES.component, ROLES.part, 'requirement', 'axis', undefined];
    for (const kind of kinds) {
      for (const n of [0, 1, 5]) {
        for (const parent of [null, 'subject.viewer']) {
          const a = call({ id: 'x', kind }, n, parent);
          expect(['component', 'part', 'toggle', 'detail']).toContain(a.kind);
          if (a.kind === 'toggle') {
            expect(n, `開閉できないのに toggle を返した（kind=${kind}）`).toBeGreaterThan(0);
          }
        }
      }
    }
  });
});

describe('構成要素へ移したタグを指す共有 URL は、転送で生きる', () => {
  const KIND: Record<string, string> = {
    'subject.viewer': 'subject',
    'subject.viewer.tags': 'piece',
    'subject.cli': 'subject',
    'req.x': 'requirement',
  };
  // ⚠️ **転送されない側にも「親が見つかる」ものを置く。** ここに構成要素の1件しか
  // 入れていなかったとき、「転送しない」を見る検査は**4つとも『親が見つからない』だけで
  // 満たされ**、種別の判定をまるごと外しても緑のままだった（レビュアの変異 R9）。
  // 種別で弾いていることを見たいなら、**種別以外の条件は満たしている**入力が要る。
  const PARENT: Record<string, string | null> = {
    'subject.viewer.tags': 'subject.viewer',
    'subject.cli': 'grp.entry',
    'req.x': 'subject.viewer',
  };
  const call = (componentId: string | undefined) =>
    forwardedOverviewTarget({
      componentId,
      kindOf: (id) => KIND[id],
      componentParentOf: (id) => PARENT[id] ?? null,
      roles: ROLES,
    });

  it('コンポーネントだったものが構成要素になっていたら、親＋その箇所へ転送する', () => {
    expect(call('subject.viewer.tags')).toEqual({ componentId: 'subject.viewer', partId: 'subject.viewer.tags' });
  });

  it('転送するのは構成要素のときだけ（射程を広げない）', () => {
    expect(call('subject.cli'), 'コンポーネントを転送している').toBeNull();
    expect(call('req.x'), '役割を持たないタグを転送している').toBeNull();
    expect(call('存在しない.id'), '解決しない id を転送している').toBeNull();
    expect(call(undefined), '現在地が無いのに転送している').toBeNull();
  });

  it('親コンポーネントが見つからない構成要素は転送しない（行き先の無い URL を作らない）', () => {
    expect(call('subject.viewer.tags')).not.toBeNull();
    PARENT['subject.viewer.tags'] = null;
    expect(call('subject.viewer.tags')).toBeNull();
    PARENT['subject.viewer.tags'] = 'subject.viewer';
  });
});
