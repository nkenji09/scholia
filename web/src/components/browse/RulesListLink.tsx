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
// 一覧の絞り込みはタグ単位（decision の実効タグ集合に対する一致・
// 01KXZK5BWEX3HH15B78E4Z3BXK）なので:
//
//   - タグのカード → そのタグ自身で絞る。「この記録に効く規則」と厳密に一致する。
//   - transition / vocab のカード → 一覧は「この transition を支配する規則」を
//     表現できない（タグ AND しか無い）。そこで**規則を最も多く運んでいるタグ**で
//     絞り、ラベルにそのタグ名を出して**リンクが何で絞るのかを名乗る**。
//     「この記録に効く規則ぜんぶ」と読ませない——嘘の入口を置くより、範囲を
//     明示した入口を置く。
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
