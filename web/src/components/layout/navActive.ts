import type { ViewName } from '../../router';

// どのナビタブが点灯するか（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// **なぜ純関数にするか。** CLAUDE.md「配線ガードの書き方」1 の適用である。
// 差し戻し1回目のレビューで、**定数（家族の一覧）はそのまま・判定の分岐だけを
// `return false` に潰す**変異がテスト緑で素通りした——意思決定タブが一度も点灯
// しなくなるのに、定数だけを見るガードは無傷だった。
// 判定そのものを値として検査できる形にすれば、綴りに関係なく落ちる。

export type NavKey = 'overview' | 'tags' | 'decisions';

/** 各タブが担う面。旧「ブラウザ」1つが7画面で点灯していた状態を、
    読む目的（俯瞰する／分類から降りる／規則を読む）で3つに割ったもの。 */
export const NAV_FAMILY: Record<NavKey, ViewName[]> = {
  // 概要 ＋ 旧ランディング #/home（既存ブックマークの意味を変えない）。
  overview: ['overview', 'home'],
  // タグの一覧・詳細に加え、語彙・フロー・遷移のレンズ。これらはタブを名乗らず
  // タグ・遷移のカードから降りるが、deep link で来ても点灯するタブが無い状態には
  // しない。
  tags: ['tags', 'spec', 'browse', 'vocab', 'flow'],
  // 一覧と、転送で残した旧単票の URL。
  decisions: ['decisions', 'decision'],
};

export function isNavActive(key: NavKey, view: ViewName): boolean {
  return NAV_FAMILY[key].includes(view);
}

/** その面で点灯するタブ（無ければ null＝設定画面。歯車が別に点く既存の意匠）。 */
export function activeNavKey(view: ViewName): NavKey | null {
  for (const key of Object.keys(NAV_FAMILY) as NavKey[]) if (isNavActive(key, view)) return key;
  return null;
}
