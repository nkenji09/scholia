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

/** 規則（decision）を持つ欄。 */
export interface RuleBearing<D> {
  rules: readonly D[];
}
/** 構成要素の欄（自身の規則＋配下の振る舞いカードの規則）。 */
export interface PartBearing<D> extends RuleBearing<D> {
  behaviors: ReadonlyArray<RuleBearing<D>>;
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
 *  ⚠️ **4系統すべてを値で守っているのはこの file の検査だけである。**
 *  描画側（`renderWiring.test.tsx`）の「見出しの件数＝開いて読める数」の突き合わせが
 *  踏むのは **直下の振る舞い と 構成要素の自身の規則の2系統だけ**——corpus の構成要素は
 *  配下に振る舞いを持たず、制約タグは1件も無いので、その2スロットを落としても
 *  描画側は緑のままになる（実測）。**ここの検査を薄くすると、その2系統は誰も見ない。** */
export function sheetRuleCount<D>(
  sheet: {
    partBlocks: ReadonlyArray<PartBearing<D>>;
    ownBehaviors: ReadonlyArray<RuleBearing<D>>;
    propBlocks: ReadonlyArray<RuleBearing<D>>;
  },
  inForce: (d: D) => boolean,
): number {
  const count = (entries: readonly D[]) => entries.reduce((n, d) => n + (inForce(d) ? 1 : 0), 0);
  let total = 0;
  for (const p of sheet.partBlocks) {
    total += count(p.rules);
    for (const b of p.behaviors) total += count(b.rules);
  }
  for (const b of sheet.ownBehaviors) total += count(b.rules);
  for (const p of sheet.propBlocks) total += count(p.rules);
  return total;
}
