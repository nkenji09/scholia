import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';

// 「この記録に効いている規則の全体を1つの並びで読む」用途への入口
// （01KYHW4NBNVN9BFXYZMBX8MPF8 条項5）。
//
// 条項1・2 で集約2面を廃止したとき、条項5 は「その用途は専用の面（対象で絞れる
// 意思決定の一覧）と CLI が担う。**カードはその入口を持つ**」と定めた。条項3・4 と
// 同格の「廃止するなら課す条件」で、入口が無ければ全体を読む受け皿が viewer 上に
// 存在しないことになる。ここがその入口。
//
// ⚠ 向きに注意。一覧のタグ絞り込みは decision の**実効タグ集合**（own タグの
// 祖先クロージャ・01KXZK5BWEX3HH15B78E4Z3BXK）に対する一致なので、`dt=T` は
// 「T とその**配下**に付いた意思決定」を返す。条項5 が言う「この記録を支配する
// 規則」＝自身＋**祖先**とは**向きが逆**である。実測（本 repo）:
//
//   req.atoms-derive.no-spec-file : dt= のヒット 0 件 / 支配する規則 3 件
//   req.comfortable-viewer        : dt= のヒット 57 件 / 支配する規則 3 件
//
// よってラベルは**一覧が実際に見せる集合**を名乗る。「この記録に効く規則」と
// 読ませない——逆の集合を指す入口は、入口が無いより悪い。
// 条項5 が求める向きの絞り込みを一覧に足すかどうかは、観測可能な振る舞いの
// 追加なので decision を先に置く（result.md §15 の残課題）。
export function RulesListLink({ tagId, exact }: { tagId: string; exact: boolean }) {
  const t = useT();
  const { tagName } = useLookups();
  const href = routeHash({ view: 'decisions', decisionTag: tagId });
  return (
    <HashLink
      href={href}
      onNavigate={() => {
        window.location.hash = href;
      }}
      class="rules-list-link"
      title={t.browse.rulesListLinkTitle}
    >
      <Icon name="scroll-text" size={12} />
      {exact ? t.browse.rulesListLinkExact : t.browse.rulesListLinkScoped(tagName(tagId))}
    </HashLink>
  );
}
