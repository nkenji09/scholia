import type { SheetRole } from '../../roleKinds';

// 構造ツリー（概要タブの左）が「何を並べ、行を押すと何が起きるか」の判断。
//
// 正本は `01KYCC2TDC6PGKPVV6DY90BHR4`（現行）——**構造ツリーは group>component>part を並べる**。
// 軸・関心・要件はそのどれでもない。
//
// ⚠️ **画面から切り離してここに置く理由**は、この判断が**場所ごとにバラバラに書かれていた**
// ことにある。是正前は、同じ「ツリーに並ぶか」の問いに対して4箇所が3通りの集合を見ていた:
//
//   起点にするか       … 絞っていない（親を持たない全タグ）
//   子として降りるか   … 要件系を除いた子
//   開閉の三角を出すか … 要件系を除いた子
//   リンクにするか     … **要件系を含む**全部の子
//
// 最後の1つがズレていたせいで、**三角も出ず・リンクでもない＝押しても何も起きない行**が
// 実データで4件できていた（`補助機能`／`設計原理`／`非機能要件`／`ビューア機能`）。
// 画面を起こしても「並んでいる行」にしか見えず、押して初めて分かる欠陥だった
// （`CLAUDE.md`「配線ガードの書き方」1）。

/** 構造ツリーが並べる3役割。`roleKinds.resolveRoleKinds` が解決した実 kind id を渡す。 */
export type TreeRoles = Pick<Record<SheetRole, string>, 'group' | 'component' | 'part'>;

/** その種類が構造ツリーに並ぶ資格を持つか。
 *
 *  ⚠️ **列挙ではなく役割で決める。** 「軸と関心と要件を除く」と書くと、**次に増えた種類が
 *  また混ざる**（要件を子として除いていたのに起点では除いていなかったのが、まさにその形）。
 *  正本が並べると言った3役割**だけ**を通す、という向きで書く。
 *
 *  役割はリテラル kind id ではなく宣言で解決したものを受け取る
 *  （`01KYCC2THS5RX3HB27SQGFWSA5`）——別 id で表すプロジェクトでも同じ答えになる。 */
export function isStructuralKind(kind: string | undefined, roles: TreeRoles): boolean {
  if (!kind) return false;
  return kind === roles.group || kind === roles.component || kind === roles.part;
}

/** 構造ツリーの起点。
 *
 *  `configRoots` があればそれを、無ければ親を持たないタグを起点にする（従来どおり）。
 *  ⚠️ **どちらの経路にも同じ資格判定を効かせる**のがこの関数の要点——是正前は
 *  フォールバック側だけが絞られておらず、要件タグ6件が最上段に残っていた。 */
export function structuralRootIds(args: {
  tags: ReadonlyArray<{ id: string; kind?: string; parentIds?: readonly string[] }>;
  configRoots: readonly string[];
  kindOf: (id: string) => string | undefined;
  roles: TreeRoles;
}): string[] {
  const { tags, configRoots, kindOf, roles } = args;
  const ids = configRoots.length ? configRoots : tags.filter((t) => !(t.parentIds && t.parentIds.length)).map((t) => t.id);
  return ids.filter((id) => isStructuralKind(kindOf(id), roles));
}

/** 行を押したときに起きること。
 *
 *  ⚠️ **「何も起きない」という選択肢を型として持たない。** 是正前の欠陥は、2つの判定が
 *  別々の集合を見た結果として**どの枝にも入らない行**が生まれたことだった。答えを1つの
 *  関数に集約し、返り値を4通りに閉じておけば、その組み合わせは作れない。 */
export type TreeRowAction =
  /** そのコンポーネントの仕様シートを開く（概要の中）。 */
  | { kind: 'component'; componentId: string }
  /** 親コンポーネントのシートの、その構成要素まで寄せる（概要の中）。 */
  | { kind: 'part'; componentId: string; partId: string }
  /** その場で開閉するだけ（別レコードへ移動しないので、アンカーにしない）。 */
  | { kind: 'toggle' }
  /** タグの詳細へ移動する（概要の外）。 */
  | { kind: 'detail'; tagId: string };

/** 1行ぶんの答え。
 *
 *  ⚠️ **`structuralChildCount` は「ツリーに並ぶ資格を持つ子」の数**でなければならない
 *  （`isStructuralKind` で数えたもの）。ここに「全部の子」を渡すと、
 *  **開閉の三角は出ないのに toggle を返す**＝押しても何も起きない行が復活する。
 *  呼び出し側が三角を出す条件と、この関数へ渡す数は、**同じ集合から採ること。**
 *
 *  構成要素の親コンポーネントは**直上の親**だけを見る（`componentParentId`）。
 *  構成要素の入れ子（構成要素の下の構成要素）は本実装の範囲外で、そのときは
 *  `componentParentId` が null になり、行はタグの詳細へ落ちる。**これは「入れ子を
 *  正しく扱っている」という主張ではない**——別単位で扱うと決めた範囲である
 *  （`01KYPFJV04R347HWHQKQ2TW275`「構成要素の入れ子は本決定の範囲外」）。 */
export function treeRowAction(args: {
  tag: { id: string; kind?: string };
  structuralChildCount: number;
  componentParentId: string | null;
  roles: TreeRoles;
}): TreeRowAction {
  const { tag, structuralChildCount, componentParentId, roles } = args;
  if (tag.kind === roles.component) return { kind: 'component', componentId: tag.id };
  if (tag.kind === roles.part) {
    if (componentParentId) return { kind: 'part', componentId: componentParentId, partId: tag.id };
    return { kind: 'detail', tagId: tag.id };
  }
  // 束ねる段（および将来の構造 kind）。子を持つなら開閉が押した意味になる。
  // 子を持たない束ねる段は記録側の穴だが、**行を無反応にはしない**——せめて詳細へ辿れる。
  if (structuralChildCount > 0) return { kind: 'toggle' };
  return { kind: 'detail', tagId: tag.id };
}

/** URL の「コンポーネント」欄が、いまは構成要素になっているタグを指しているときの転送先。
 *
 *  ⚠️ **共有済みの URL を黙って殺さないための転送**（`01KYKS4Y56FAHRVCWKMQJK4RT6` 条項4 の
 *  「共有済みの URL は転送で生かす」を概要タブの現在地へ及ぼす）。あるコンポーネントを
 *  構成要素へ移すと、それを指していた `#/overview/<id>` は**そのタグをコンポーネントとして
 *  解決できなくなり、既定のコンポーネントへ黙って落ちる**（実測: `#/overview/subject.viewer.tags`
 *  が「CLI コマンド」のシートを出した）。URL は変わらないのに別のものが出る、が一番悪い。
 *
 *  転送しないなら null。**転送するのは「構成要素になっていた」場合だけ**で、
 *  存在しない id や、コンポーネントでも構成要素でもないタグは対象外
 *  （そこは従来どおり既定へ落ちる——射程を広げない）。 */
export function forwardedOverviewTarget(args: {
  componentId: string | undefined;
  kindOf: (id: string) => string | undefined;
  componentParentOf: (id: string) => string | null;
  roles: TreeRoles;
}): { componentId: string; partId: string } | null {
  const { componentId, kindOf, componentParentOf, roles } = args;
  if (!componentId) return null;
  if (kindOf(componentId) !== roles.part) return null;
  const parent = componentParentOf(componentId);
  if (!parent) return null;
  return { componentId: parent, partId: componentId };
}
