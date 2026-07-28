import { Fragment } from 'preact';
import { useEffect, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import type { GovernsRef } from '../../types';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';
import { isInForce } from '../decisions/decisionModel';
import { countWholeInForce, shouldDiscloseWhole, summarizeInherited } from './inheritedSummary';
import { RulesListLink } from './RulesListLink';
import { WholeRules } from './WholeRules';
import type { RecordRef } from './rulesCommand';

// 継承した規則の開示（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3・4）。
//
// この欄は「この記録を支配する規則」欄の**代わり**に置かれている。あの欄は
// 支配する規則の全文をカードにもう一度並べていて、own の本文が同じカードに2度
// 出る読みづらさを生んでいたので廃止した（条項2）。ただし黙って消すと、
// 「親に decision を持ち子に無いレコードの why が viewer から不可視」という
// 01KXYED61J6QBEX75H2XHVHW7Y が診断した欠陥がそのまま復活する——実測で
// タグ9件・transition 36本・vocab 9件のカードが規則ゼロ表示になる。
//
// よってここは**本文を並べない**。出すのは条項3が要求する3つだけ:
//   ・継承した規則が何件あるか
//   ・それがどのレコードから来ているか（名前で・条項4／id は出さない
//     01KYCC2TF3NW3JRSSRK9ZHN078）
//   ・そこへ辿る導線
//
// 件数は**効いている規則の数**（01KYHW54B8ZXH0NEPH2J7N1X39 条項5 と同じ数え方。
// 置き換え済みを混ぜると開示した件数と読める件数が食い違う）。
//
// あわせて、**その全体をどこで読めるか**の開示（追補 01KYJV3FYMDFRWQ939NBV2BPAC
// 条項3＝WholeRules）もここが出す。件数と継承元だけでは「全体を通しで読む」用途に
// 答えていないからで、答える受け皿は現状 CLI だけである。取得した governs を
// 両方が使うので、口の出し分けは**この1箇所**で決める:
//
//   効いている規則が0件      → 何も出さない（読む全体が無い）
//   効いている規則あり・継承0 → 全体の開示だけ（実測で tag 21件がこの形）
//   継承あり                 → 継承の開示（件数・継承元・導線）＋全体の開示
//
// データ源は GET /api/governs＝ CLI `scholia rules` と同じ Go コア
// （index.GovernsFor*）。フロントで実効タグを再計算しない（面間整合 D10b-2）。

export function InheritedRules({ record }: { record: RecordRef }) {
  const t = useT();
  const { tagName, currencyIndex } = useLookups();
  const [entries, setEntries] = useState<GovernsRef[] | null>(null);

  useEffect(() => {
    let cancelled = false;
    const params = record.kind === 'tag' ? { tag: record.id } : record.kind === 'transition' ? { tx: record.id } : { vocab: record.id };
    api
      .getGoverns(params)
      .then((res) => {
        if (!cancelled) setEntries(res.entries);
      })
      .catch(() => {
        // 取得できなかったときは欄を出さない。開示できないのは条項3 的には
        // 望ましくないが、壊れた件数を出すよりは出さないほうがまし。
        if (!cancelled) setEntries([]);
      });
    return () => {
      cancelled = true;
    };
  }, [record.kind, record.id]);

  if (!entries) return null;

  // 口を出すかどうかは「この記録に効いている規則が1件でもあるか」で決める
  // （own を含む）。継承の件数で決めると、継承0・own ありのカードから「全体は
  // どこで読めるか」の開示ごと消える——追補 条項3 が要求するのはそのカードでも
  // 開示することなので、判定は値として検査できる純関数へ切り出してある
  // （shouldDiscloseWhole・01KYK4YTB8087JT5GNV5QB26T2）。
  if (!shouldDiscloseWhole(entries, (id) => isInForce(id, currencyIndex))) return null;

  // 継承の件数は純関数へ（inheritedSummary.ts）。own を除き、効いているものだけを
  // 継承元ごとに束ねる。0件なら継承の開示ブロックは出さない（条項3）。
  const { total, sources } = summarizeInherited(entries, (id) => isInForce(id, currencyIndex), tagName);
  // 支配する規則の全体の件数（own を含む・継承の total とは別の数）。実リンクが
  // 行き先で読める件数を名乗るために使う（01KYKS4Y56FAHRVCWKMQJK4RT6）。
  const wholeCount = countWholeInForce(entries, (id) => isInForce(id, currencyIndex));

  // 見出しの選び方。transition は**規則を運ぶのが常にタグ**（自身がタグ階層に
  // 属さない）なので、経路に parent が混ざっていても「タグから継承した規則」。
  // tag / vocab は経路で決める——vocab の継承元は自身が持つタグ（effective-tag）で
  // 祖先とは限らないのに「上位から継承」と出ていた（1回目 should-2）。
  //
  // 経路だけで決めると transition が「上位から」に振れる（2回目 nit-1 の退行）。
  // 「レコード種別」と「経路」のどちらか一方では両方を正しく言えないので、
  // transition だけ種別で決めて残りを経路で決める。
  const viaAncestor = record.kind !== 'transition' && entries.some((e) => e.provenance === 'parent');
  const heading = viaAncestor ? t.browse.inheritedFromAncestors(total) : t.browse.inheritedFromTags(total);

  return (
    <Fragment>
      {total > 0 && (
        <div class="inherited-rules">
          <div class="inherited-rules-head">
            <Icon name="gavel" size={13} />
            <span>{heading}</span>
          </div>
          <div class="inherited-rules-sources">
            {sources.map((s) => (
              <HashLink
                key={s.tagId}
                href={routeHash({ view: 'spec', tagId: s.tagId })}
                onNavigate={() => {
                  // 共有部品なので親のコールバックに頼らない。平打ちでも hash 代入で
                  // 継承元のカードへ移る（修飾クリックは HashLink が別タブに回す）。
                  window.location.hash = routeHash({ view: 'spec', tagId: s.tagId });
                }}
                class="inherited-rules-source"
                title={t.browse.inheritedSourceTitle}
              >
                {tagName(s.tagId)}
                <span class="inherited-rules-count">{s.count}</span>
                <Icon name="arrow-up-right" size={12} />
              </HashLink>
            ))}
          </div>
          {/* 配下の意思決定の一覧への入口（transition / vocab 用）。一覧はこれらの
              単位で絞れないので、規則を最も多く運んでいるタグで絞り、ラベルに範囲を
              名乗らせる。tag のカードは TagCard が自身のタグで**継承0件でも**入口を
              出すので、ここでは出さない（二重に置かない）。
              この入口は「この記録を支配する規則」を指していない（追補 条項2）——
              その用途に答えるのは下の WholeRules。 */}
          {record.kind !== 'tag' && <RulesListLink tagId={sources[0].tagId} exact={false} />}
        </div>
      )}
      {/* 支配する規則の全体への入口（01KYKS4Y56FAHRVCWKMQJK4RT6）。継承0でも出す。
          追補 条項3 の「どこで読めるかの開示」が、面ができたので実リンクになった。 */}
      <WholeRules record={record} inForceCount={wholeCount} />
    </Fragment>
  );
}
