import type { Transition, VocabEntry } from '../../types';

// 仕様シートが「どの遷移を、どのコンポーネントの振る舞いとして描くか」の判断。
//
// 画面から切り離してここに置く理由: この判断は入力（遷移のタグ付け・構成要素の
// 有無）に対する答えとして検査できる形にしておかないと、間違いが画面を起こさない
// と分からない（CLAUDE.md「配線ガードの書き方」1）。実際、**コンポーネント直下の
// 遷移がシートに1本も現れない**という欠陥は、画面を起こしても「空のシート」に
// しか見えず、原因がここにあることは分からなかった。

/** タグ id → その**タグ自身に直接付いた**遷移 id。
 *
 *  ⚠️ **祖先展開を通さない**のがこの関数の要点。概要の index が持つ `satByTag` は
 *  effByTx（tx.tags ∪ 参照 vocab の tags ∪ **祖先展開**）から作られているので、
 *  親コンポーネントを引くと**子孫に付いた遷移まで返る**。それを振る舞いカードに
 *  使うと、親のシートに子の振る舞いが再掲され、同じ遷移が2つのシートに出る。
 *  「そのコンポーネント自身の振る舞い」を問うているのだから、直接付いた分だけを返す。
 *
 *  遷移が持つタグは tx.tags だけではない——参照している vocab に付いたタグも
 *  その遷移の実効タグである（index の effByTx と同じ合成規則）。祖先展開だけを
 *  外して同じ合成を行う。 */
export function buildDirectByTag(
  transitions: Iterable<Transition>,
  vocabById: Map<string, VocabEntry>,
): Map<string, string[]> {
  const out = new Map<string, string[]>();
  for (const tx of transitions) {
    const raw = new Set<string>(tx.tags || []);
    for (const vid of [tx.action, ...(tx.given || []), ...(tx.then || [])]) {
      const v = vocabById.get(vid);
      if (v && v.tags) for (const g of v.tags) raw.add(g);
    }
    for (const g of raw) {
      const arr = out.get(g) || [];
      arr.push(tx.id);
      out.set(g, arr);
    }
  }
  return out;
}

/** そのコンポーネントのシートが、**自身の振る舞い**として描く遷移 id。
 *
 *  構成要素（part）を持つコンポーネントでは空を返す——構成要素側のカードが
 *  同じ遷移を描くので、二重に出さない。構成要素を持たないコンポーネントでは、
 *  直下の遷移をそのまま振る舞いとして描く。
 *
 *  ⚠️ **構成要素を持つコンポーネントの、どの構成要素にも属さない直下の遷移は、
 *  この関数では拾われない**（＝シートに出ない）。今回はそこまで広げないと決めた
 *  範囲であって、「そういう遷移は無い」という主張ではない。 */
export function componentBehaviorTxIds(opts: { partCount: number; directTxIds: readonly string[] }): string[] {
  if (opts.partCount > 0) return [];
  return [...opts.directTxIds];
}

/** 仕様シートに置く構成要素の欄1つ。記録の親子どおりに入れ子になる。 */
export interface PartNode {
  id: string;
  /** その欄が出す遷移＝**その構成要素に直接付いた分だけ**（祖先展開を通さない）。 */
  txIds: string[];
  /** 配下の構成要素の欄。 */
  children: PartNode[];
  /** この欄を置いた親**以外**の親（記録の順）。「もう一方の親」の開示に使う。 */
  otherParentIds: string[];
}

/** 仕様シートの構成要素の欄を、記録の親子どおりに組む。
 *
 *  ## この関数が決めていること
 *
 *  1. **入れ子**（①）。構成要素の下の構成要素も欄になる。段の深さに上限を置かない。
 *  2. **各欄が出すのは「その構成要素に直接付いた分」だけ**（`directTxIdsOf`）。
 *     祖先展開込みの索引を渡すと、親の欄に配下の振る舞いが再掲され、**同じ遷移が
 *     1枚のシートの中で二重に出る**（実測: 親子の二重が30組・最悪の1本は同じシートに6枚）。
 *  3. **多親は、シートごとに1回だけ**（③・案B′）。1つの構成要素が複数の親を持つとき、
 *     **親が属するそれぞれのコンポーネントのシートには1回ずつ出る**が、
 *     **1枚のシートの中では1回だけ**出す。
 *  4. **どの親の下に置くかは「記録に書かれた親の順」で決める。**
 *     ⚠️ **走査順に依らない。** シートを上から降りて最初に出会った親に置く形は、
 *     欄の描き順という**実装の偶然**に居場所を委ねることになる。記録が正本である。
 *  5. **置かなかった親は `otherParentIds` に残す**（位置で言えないものを言葉で言うため）。
 *     ⚠️ ここには**別のコンポーネントに属する親も入る**——それが「1つの部品が複数の
 *     コンポーネントにまたがっている」という、この単位が表現したい事実そのものだから。
 *
 *  ## この関数が答えないこと（射程・`CLAUDE.md` 6）
 *
 *  ・**記録が祖先と子孫の両方に同じ遷移を直接貼った形は、ここでは消えない。**
 *    消すのは「索引が持ち上げたぶん」だけである（実測: 直接分だけにしても、そういう
 *    記録では親子の二重が6組残った）。**歯止めは置いていない。**
 *  ・**親子関係の無い欄どうしに同じ遷移が出ることも消さない**（実測で20組残る）。
 *    別々の構成要素の要件を1本の振る舞いが満たす形は、消してはいけないものである。
 *  ・欄が実際に描かれるか・畳まれているかは見ない（描画側の配線）。 */
export function buildPartTree(args: {
  componentId: string;
  /** タグ id → その子タグ id（**記録の順**）。 */
  childIdsOf: (id: string) => readonly string[];
  /** タグ id → その親タグ id（**記録の順**）。 */
  parentIdsOf: (id: string) => readonly string[];
  /** その id が構成要素の役割を担うか。 */
  isPart: (id: string) => boolean;
  /** その id が「居場所」になりうる役割（構成要素／コンポーネント）を担うか。
      `otherParentIds` に何を残すかを決めるのに使う——要件タグを親に持つ構成要素の
      「もう一方の親」に要件タグを出しても、居場所の話にならない。 */
  isPlace: (id: string) => boolean;
  /** タグ id → その構成要素に**直接付いた**遷移 id。 */
  directTxIdsOf: (id: string) => readonly string[];
}): PartNode[] {
  const { componentId, childIdsOf, parentIdsOf, isPart, isPlace, directTxIdsOf } = args;

  // (1) このシートに属する構成要素を、コンポーネントから記録の親子を降りて集める。
  const members = new Set<string>();
  const stack = [componentId];
  const walked = new Set<string>();
  while (stack.length) {
    const cur = stack.pop()!;
    if (walked.has(cur)) continue;
    walked.add(cur);
    for (const kid of childIdsOf(cur)) {
      if (!isPart(kid)) continue;
      members.add(kid);
      stack.push(kid);
    }
  }

  // (2) 各構成要素の居場所を「記録に書かれた親の順」で1つに決める。
  //     このシートの中に居る親（コンポーネント自身、またはこのシートの構成要素）だけが候補。
  const placeUnder = new Map<string, string>();
  const others = new Map<string, string[]>();
  for (const id of members) {
    const parents = parentIdsOf(id);
    const inSheet = parents.filter((p) => p === componentId || members.has(p));
    const chosen = inSheet[0];
    if (chosen === undefined) continue; // 到達したのに親が居ない＝起こらないが、黙って落とす
    placeUnder.set(id, chosen);
    others.set(
      id,
      parents.filter((p) => p !== chosen && isPlace(p)),
    );
  }

  // (3) 欄の木を組む。並びは**記録の子の順**に従う（描き順を決める規則も記録に置く）。
  const emitted = new Set<string>();
  const build = (parentId: string): PartNode[] => {
    const out: PartNode[] = [];
    for (const kid of childIdsOf(parentId)) {
      if (!members.has(kid) || emitted.has(kid)) continue;
      if (placeUnder.get(kid) !== parentId) continue;
      emitted.add(kid);
      out.push({
        id: kid,
        txIds: [...directTxIdsOf(kid)],
        children: build(kid),
        otherParentIds: others.get(kid) || [],
      });
    }
    return out;
  };
  return build(componentId);
}

/** そのシートの欄の木の中で、目当ての欄までの**間の段**（浅い順・目当て自身は含まない）。
 *
 *  ⚠️ **「そのタグはどこに居るか」（`treeModel.structuralPlace`）を、ここに使ってはいけない。**
 *  あちらは**記録を上へ辿って**答えを出すので、多親では**いま見ているシートとは別の道**を
 *  指すことがある。その答えで間の段を開けると、**別のシートの段を開けてしまい、いま見て
 *  いるシートの段は畳まれたまま**になる——実測: `#/overview/comp.export/part/part.shared.index`
 *  で、欄はそのシートに在るのに寄せ先が DOM に存在せず、1px も寄らなかった。
 *
 *  **どのシートを見ているかに依存する問いは、そのシートの欄の木に答えさせる。**
 *  こうすると **行の行き先・欄の組み立て・間の段の3つが同じ答え**になる。
 *
 *  そのシートに目当ての欄が無ければ null（＝転送も寄せも起きない形。呼び出し側は何もしない）。 */
export function panelPathTo(nodes: readonly PartNode[], partId: string): string[] | null {
  for (const node of nodes) {
    if (node.id === partId) return [];
    const deeper = panelPathTo(node.children, partId);
    if (deeper) return [node.id, ...deeper];
  }
  return null;
}

/** 規則（decision）を持つ欄。 */
export interface RuleBearing<D> {
  rules: readonly D[];
}
/** 構成要素の欄（自身の規則＋配下の振る舞いカードの規則＋**配下の構成要素の欄**）。 */
export interface PartBearing<D> extends RuleBearing<D> {
  behaviors: ReadonlyArray<RuleBearing<D>>;
  children: ReadonlyArray<PartBearing<D>>;
}

/** シートの見出しに出す「現行ルール N」。
 *
 *  ⚠️ **シートの中で実際に開いて読める規則だけを数える**
 *  （`01KYHW54B8ZXH0NEPH2J7N1X39` 条項5: 見出しの件数と、開いて見える行数を一致させる）。
 *  数えるのは構成要素／その配下の振る舞い／**直下の振る舞い**／制約の4系統で、
 *  どれか1つを落とすと「N と言っているのに N 件見つからない」になる。
 *
 *  ⚠️ この足し算は `OverviewView` の中に直接書いていた。**そこに書くと、条項を1つ
 *  落とす変異が何にも落ちない**——見出しの数字は変わるが、その数字が正しいかを
 *  値として見ている検査がどこにも無かったため（レビュアの変異1件がそこを通った）。
 *
 *  ⚠️ **入れ子の欄は5系統目である。** 欄が入れ子になった以上、**配下の欄の中の規則も
 *  「シートの中で開いて読める」**——数えないと見出しの数が足りなくなる。逆に、数え方を
 *  「その構成要素に直接付いた分」に変えたので、**同じ規則を2回数えることは無い**
 *  （素直に祖先展開込みで描くと、実測で 20 が 28 になり、実在する20件と食い違った）。
 *
 *  ⚠️ **5系統すべてを値で守っているのはこの file の検査だけである。**
 *  描画側（`renderWiring.test.tsx`）の「見出しの件数＝開いて読める数」の突き合わせが
 *  踏むのは **直下の振る舞い／構成要素の自身の規則／入れ子の欄の自身の規則の3系統**で、
 *  残る2系統（**構成要素配下の振る舞いの規則・制約の規則**）は corpus にその形が無いので
 *  落としても緑のままになる。**ここの検査を薄くすると、その2系統は誰も見ない。** */
export function sheetRuleCount<D>(
  sheet: {
    partBlocks: ReadonlyArray<PartBearing<D>>;
    ownBehaviors: ReadonlyArray<RuleBearing<D>>;
    propBlocks: ReadonlyArray<RuleBearing<D>>;
  },
  inForce: (d: D) => boolean,
): number {
  const count = (entries: readonly D[]) => entries.reduce((n, d) => n + (inForce(d) ? 1 : 0), 0);
  const countPart = (p: PartBearing<D>): number => {
    let n = count(p.rules);
    for (const b of p.behaviors) n += count(b.rules);
    // ⚠️ **入れ子の欄まで降りる。** ここを1段で止めると、入れ子の中の規則が
    // 見出しの数に入らず「N と言っているのに N 件より多く見つかる」になる。
    for (const c of p.children) n += countPart(c);
    return n;
  };
  let total = 0;
  for (const p of sheet.partBlocks) total += countPart(p);
  for (const b of sheet.ownBehaviors) total += count(b.rules);
  for (const p of sheet.propBlocks) total += count(p.rules);
  return total;
}

/** シートに出る構成要素の欄の総数（入れ子を含む）。
 *
 *  ⚠️ ヘッダの「構成要素 N」は**シートに実際に出る欄の数**でなければならない
 *  （`01KYHW54B8ZXH0NEPH2J7N1X39` 条項5「見出しの件数と、開いて見える数を一致させる」）。
 *  直下の子の数で数えると、入れ子の欄が数に入らない。 */
export function countPartPanels(nodes: readonly NestedPanel[]): number {
  let n = 0;
  for (const node of nodes) n += 1 + countPartPanels(node.children);
  return n;
}
/** 数えるのに要るのは「配下の欄」だけ（`PartNode` も描画側の欄もこの形を満たす）。 */
export interface NestedPanel {
  children: readonly NestedPanel[];
}
