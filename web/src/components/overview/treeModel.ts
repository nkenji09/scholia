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

/** そのタグが構造の中でどこに居るか。 */
export interface StructuralPlace {
  /** 上の段から直上までの構造上の祖先（浅い順）。**役割を持たない親は通さない。**
   *
   *  ⚠️ 多親のときここが辿るのは**記録の順で先に来た道1本だけ**である。
   *  `componentId` が**別の親の道**で見つかることがあるので、**この2つが同じ道を
   *  指しているとは限らない**（下の「答えないこと」を参照）。 */
  ancestorIds: string[];
  /** そのタグを含む**いちばん近いコンポーネント**。どの親の道にも無ければ null。 */
  componentId: string | null;
}

/** 「そのタグはどこに居るか」——概要タブがこの問いに出す**唯一の答え**。
 *
 *  ⚠️ **是正前は、同じ問いに4箇所が別々の答えを出していた**（いずれも実測）:
 *
 *    ツリーの行の指し先   … `parentIds` の中の**直上のコンポーネント**だけを見る
 *                            → 親が構成要素だと null になり、行がタグの詳細へ落ちる
 *    共有済み URL の転送  … 同じ判定を使うので、入れ子を指す URL が**黙って
 *                            既定のコンポーネントのシートを出す**
 *    シートのパンくず     … `parentIds[0]` を**素通しで**遡る（役割の資格判定を通らない）
 *                            → ツリーが役割で除いたタグがパンくずに出る
 *    ツリーの位置         … 走査順で先に降りた親の下（行の指し先とは別の答え）
 *
 *  4つは「親を遡るときに役割を見ない／直上しか見ない」という**1つの原因**から出て
 *  いる。別々に直すと、この repo が繰り返している「同じ意味の記述が2箇所にあるとき、
 *  片側だけ直す」型を踏む（`CLAUDE.md`）。だから**答えをここ1箇所に置く。**
 *
 *  ### 辿り方（決めた規則）
 *
 *  **祖先の並び（`ancestorIds`）**は、各段で親のうち**記録に書かれた順で最初に来た、
 *  役割を持つ親**を1つ選んで上へ進む。役割を持たない親（要件・軸・関心）は**飛ばす**。
 *
 *  **コンポーネント（`componentId`）**は、その道の中のいちばん近いものを採る。
 *  ⚠️ **その道に1つも無ければ、記録の順で他の親の道も辿る**（深さ優先・記録の順）。
 *  最初にコンポーネントへ行き着いた道の、そのコンポーネントが答えになる。
 *  是正前はここで諦めて null を返しており、**その構成要素の欄が別のシートに実在するのに
 *  共有 URL が転送されず、既定のコンポーネントのシートを黙って出していた**（実測）
 *  ——`01KYPFJV04R347HWHQKQ2TW275` が「URL は変わらないのに別のものが出るのが一番悪い」と
 *  名指しした状態そのものである。
 *
 *  どちらも**同じ記録なら常に同じ答え**になる——走査順にも、画面の描き順にも依らない。
 *
 *  ⚠️ **`parentIds` を呼び出し側が渡す**のは、構造ツリーが多親のタグを**親ごとに**
 *  描くためである（そのときは「その行が居る経路の親」1つだけを渡す）。こうすると
 *  **ツリーの位置と、その行の行き先が同じ答えになる**——是正前は前者が走査順・
 *  後者が `parentIds` の順で、実データで食い違っていた。
 *
 *  ### この関数が答えないこと（射程を名乗る・`CLAUDE.md` 6）
 *
 *  1. ⚠️ **`ancestorIds` と `componentId` が同じ道を指しているとは限らない。**
 *     前者は記録の順で先に来た道1本、後者は必要なら他の道も辿るからである。
 *     **`ancestorIds` を「そのコンポーネントのシートの中の間の段」として使ってはいけない**
 *     ——多親でそれをやると別のシートの段を開ける（実測でその欠陥を踏んだ）。
 *     シートの中の位置は `sheetModel.panelPathTo` が答える。
 *  2. **どのコンポーネントのシートに出るかを1つに決めているわけではない。** 多親の構成要素は
 *     案B′ のもとで**複数のシートに出る**。ここが返すのは「共有 URL のように行き先が
 *     1つに決まらなければならない場面で、記録の順が選ぶ1つ」である。 */
export function structuralPlace(args: {
  /** 上へ辿り始める親。ツリーの行なら「その行が居る経路の親」1つだけを渡す。 */
  parentIds: readonly string[];
  parentIdsOf: (id: string) => readonly string[];
  kindOf: (id: string) => string | undefined;
  roles: TreeRoles;
}): StructuralPlace {
  const { parentIds, parentIdsOf, kindOf, roles } = args;
  const chain: string[] = [];
  const guard = new Set<string>();
  let ids: readonly string[] = parentIds;
  for (;;) {
    const next = ids.find((id) => !guard.has(id) && isStructuralKind(kindOf(id), roles));
    if (!next) break;
    guard.add(next);
    chain.push(next);
    ids = parentIdsOf(next);
  }
  // chain は深い順に積んだので、浅い順へ直す。いちばん近いコンポーネントは
  // **浅い順で最後**に出るコンポーネント。
  chain.reverse();
  let componentId: string | null = null;
  for (const id of chain) if (kindOf(id) === roles.component) componentId = id;
  // 選んだ道にコンポーネントが1つも無いときだけ、他の親の道も辿る（記録の順・深さ優先）。
  // ⚠️ 探索は「見つけたら止める」ので、記録の順が答えを決める（走査順に依らない）。
  if (!componentId) {
    const seen = new Set<string>();
    const search = (from: readonly string[]): string | null => {
      for (const id of from) {
        if (seen.has(id) || !isStructuralKind(kindOf(id), roles)) continue;
        seen.add(id);
        if (kindOf(id) === roles.component) return id;
        const deeper = search(parentIdsOf(id));
        if (deeper) return deeper;
      }
      return null;
    };
    componentId = search(parentIds);
  }
  return { ancestorIds: chain, componentId };
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
 *  構成要素の親コンポーネントは `structuralPlace` の答え（**上へ辿って最初に見つかる
 *  コンポーネント**）を渡す。⚠️ **直上の親だけを見る形へ戻さないこと**——それが
 *  「入れ子の行を押すと概要から抜ける」の根だった。途中の段に構成要素が何段
 *  挟まっていても、答えは変わらない。 */
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
 *  （そこは従来どおり既定へ落ちる——射程を広げない）。
 *
 *  ⚠️ **`componentParentOf` には `structuralPlace` の答えを渡すこと。** 直上の親だけを
 *  見る形を渡すと、**入れ子の構成要素を指す URL が転送されず、既定のコンポーネントの
 *  シートを黙って出す**（実測）——`01KYPFJV04R347HWHQKQ2TW275` が「一番悪い」と
 *  名指しした状態そのものに戻る。 */
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
