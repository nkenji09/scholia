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
// 実測（本 repo・2026-07-28 再実測。支配側は当時の `scholia rules --tag <T> --current`
// が返した集合＝現在の `scholia rules --tag <T>` が**集める**集合と同じ
// （01KZ06SYP12ZFDG1WPNYM529D8 以後、既定で本文が出るのはそのうち自身への分だけに
// なったが、集合そのものは変わっていない）。
// 一覧側は decision の実効タグ集合への包含を数えたもの）:
//
//   req.atoms-derive.no-spec-file : dt= のヒット  0 件 / 支配する規則 3 件
//   req.comfortable-viewer        : dt= のヒット 66 件 / 支配する規則 6 件
//   全体                          : 支配する規則があるのにヒット0 のタグ 8/75・件数不一致 54/75
//
// 01KYHW4NBNVN9BFXYZMBX8MPF8 条項5 はこの一覧を「全体を1つの並びで読む」用途の
// 受け皿と前提していたが、それは事実誤認だった——と追補 01KYJV3FYMDFRWQ939NBV2BPAC
// が確定した（条項1・2）。ここは「配下の意思決定」というそれ自体は有用な眺めへの
// 入口として残す。
//
// **支配方向で絞れる面は 01KYKS4Y56FAHRVCWKMQJK4RT6 で足された**（追補 条項4 が
// 「そのときに決める」と保留していた入口）。その向きへの導線は WholeRules が持つ
// ——判定は CLI と同じ問い合わせ（GET /api/governs）に委ねてあり、viewer 側に
// 第二実装は無い。
//
// よってラベルは**一覧が実際に見せる集合**を名乗る。「この記録に効く規則」と
// 読ませない——逆の集合を指す入口は、入口が無いより悪い。向きが2つになったので
// 0件に着く入口が唯一の入口ではなくなったが、**この入口が名乗る集合は変えていない**。
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
