import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';

// 「そのタグと配下に付いた意思決定」の一覧への入口。
//
// ⚠ この入口は「この記録を支配する規則（自身＋祖先）」を**指していない**。
// 一覧のタグ絞り込みは decision の**実効タグ集合**（own タグの祖先クロージャ・
// 01KXZK5BWEX3HH15B78E4Z3BXK）に対する一致なので、`dt=T` が返すのは
// 「T 自身と、その**配下**に付いた意思決定」で、支配する方向とは**逆**である。
// 実測（本 repo）:
//
//   req.atoms-derive.no-spec-file : dt= のヒット 0 件 / 支配する規則 3 件
//   req.comfortable-viewer        : dt= のヒット 57 件 / 支配する規則 3 件
//
// 01KYHW4NBNVN9BFXYZMBX8MPF8 条項5 はこの一覧を「全体を1つの並びで読む」用途の
// 受け皿と前提していたが、それは事実誤認だった——と追補 01KYJV3FYMDFRWQ939NBV2BPAC
// が確定した（条項1・2）。受け皿は現状 CLI だけで、その所在の開示は WholeRules が
// 担う。ここは「配下の意思決定」というそれ自体は有用な眺めへの入口として残す。
//
// よってラベルは**一覧が実際に見せる集合**を名乗る。「この記録に効く規則」と
// 読ませない——逆の集合を指す入口は、入口が無いより悪い。支配方向で絞れる面を
// viewer に足すかどうかは追補では決めていない（同 条項4・見直しの入口）。
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
