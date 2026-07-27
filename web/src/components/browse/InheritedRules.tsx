import { useEffect, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import type { GovernsRef } from '../../types';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';
import { isInForce } from '../decisions/decisionModel';
import { summarizeInherited } from './inheritedSummary';
import { RulesListLink } from './RulesListLink';

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
// 置き換え済みを混ぜると開示した件数と読める件数が食い違う）。0件なら何も出さない。
//
// データ源は GET /api/governs＝ CLI `scholia rules` と同じ Go コア
// （index.GovernsFor*）。フロントで実効タグを再計算しない（面間整合 D10b-2）。

type RecordRef =
  | { kind: 'tag'; id: string }
  | { kind: 'transition'; id: string }
  | { kind: 'vocab'; id: string };

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

  // 計算は純関数へ（inheritedRules.ts）。own を除き、効いているものだけを
  // 継承元ごとに束ねる。0件なら口自体を出さない（条項3）。
  const { total, sources } = summarizeInherited(entries, (id) => isInForce(id, currencyIndex), tagName);
  if (total === 0) return null;

  // 見出しは record.kind ではなく**実際の継承経路**で決める。vocab の継承元は
  // 自身が持つタグ（effective-tag）で祖先とは限らないのに「上位から継承」と
  // 出ていた（レビュー should-2）。祖先経由が1つも無ければ「タグから」。
  const viaAncestor = entries.some((e) => e.provenance === 'parent');
  const heading = viaAncestor ? t.browse.inheritedFromAncestors(total) : t.browse.inheritedFromTags(total);

  return (
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
      {/* 条項5 の入口（transition / vocab 用）。一覧はこれらの単位で絞れないので、
          規則を最も多く運んでいるタグで絞り、ラベルに範囲を名乗らせる。tag の
          カードは TagCard が自身のタグで**継承0件でも**入口を出すので、ここでは
          出さない（二重に置かない）。 */}
      {record.kind !== 'tag' && <RulesListLink tagId={sources[0].tagId} exact={false} />}
    </div>
  );
}
