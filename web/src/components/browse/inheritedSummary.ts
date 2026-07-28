import type { GovernsRef } from '../../types';

// 継承した規則の開示に出す値の計算（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3）。
//
// UI から切り離した純関数にしてあるのは、この計算が**嘘をつくと開示が嘘になる**
// 一方で、DOM を起こさないと守れない形にしておくと結局テストが書かれないから。
// レビューで「開示ブロックを常に空にする」変異がテストを素通りした（M4）——
// 描画の有無に依存しない形で件数と束ね方を固定する。
//
// 数える対象:
//   - own は除く（そのレコード自身の決定は意思決定欄が本文で出している）
//   - 効いているものだけ（条項5 と同じ数え方。置き換え済みを混ぜると、開示した
//     件数と継承元をたどって読める件数が食い違う）

export interface InheritedSource {
  tagId: string;
  count: number;
}

export interface InheritedSummary {
  /** 効いている継承規則の総数。0 なら開示ブロック自体を出さない。 */
  total: number;
  /** 継承元ごとの件数（多い順・同数はタグ名順）。合計は total と一致する。 */
  sources: InheritedSource[];
}

export function summarizeInherited(
  entries: GovernsRef[],
  isInForce: (id: string) => boolean,
  tagName: (id: string) => string,
): InheritedSummary {
  const inherited = entries.filter((e) => e.provenance !== 'own' && isInForce(e.decisionId));
  const byTag = new Map<string, number>();
  for (const e of inherited) {
    const via = e.viaTag || '';
    if (!via) continue; // 経由が分からないものは継承元を名乗れないので出さない
    byTag.set(via, (byTag.get(via) || 0) + 1);
  }
  const sources = [...byTag.entries()]
    .map(([tagId, count]) => ({ tagId, count }))
    .sort((a, b) => b.count - a.count || tagName(a.tagId).localeCompare(tagName(b.tagId)));
  // total は viaTag を持つものの合計＝チップの合計。開示した数と、たどって読める
  // 数を一致させる（inherited.length にすると viaTag 欠落分だけ多く名乗る）。
  return { total: sources.reduce((n, s) => n + s.count, 0), sources };
}

// 「全体をどこで読めるか」の開示（WholeRules）を出すかどうか（追補
// 01KYJV3FYMDFRWQ939NBV2BPAC 条項3）。継承の件数（total）ではなく
// **効いている規則が1件でもあるか**（own を含む）で決める——継承0・own ありの
// カード（実測 tag 21件）でも読む全体はあるので、total で決めると消えてしまう。
//
// 呼び手（InheritedRules）はこの真偽だけで WholeRules を出すかどうかを決める。
// JSX 側で「total > 0 のブロックの内側に置く」ような形に戻すと、出し分けの
// 判断がここではなく JSX の入れ子に暗黙で移り、01KYK4YTB8087JT5GNV5QB26T2 が
// 禁じた「ソース文字列は残るが構造が壊れる」変異が緑のまま通る。
export function shouldDiscloseWhole(entries: GovernsRef[], isInForce: (id: string) => boolean): boolean {
  return entries.some((e) => isInForce(e.decisionId));
}

// 「この記録を支配する規則」の件数（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// お詫びが実リンクに変わったので、そのリンクは行き先で読める件数を名乗る。
// summarizeInherited の total とは**別の数**である点に注意——あちらは継承分だけ
// （own を除く）で、こちらは own を含む全体。リンク先の絞り込み（対象＝この記録・
// 向き＝自身＋祖先）が返すのは own も含む集合なので、名乗る数もそちらに合わせる。
//
// 効いているものだけ数える（条項5 と同じ数え方）。リンク先の既定の絞り込みも
// 「効いているものだけ」なので、これで名乗る数と着いた先の件数が一致する。
//
// **decisionId で重複を落とす。** 1つの decision が own と parent の両方の経路で
// 届くことがあり、entries はそのぶん重複する。行き先が並べるのは decision なので、
// 経路の数を件数として名乗ると多く言うことになる。
export function countWholeInForce(entries: GovernsRef[], isInForce: (id: string) => boolean): number {
  const seen = new Set<string>();
  for (const e of entries) if (isInForce(e.decisionId)) seen.add(e.decisionId);
  return seen.size;
}
