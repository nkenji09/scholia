import { useEffect, useState } from 'preact/hooks';
import { api } from '../../api';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import type { GovernsEntry } from '../../types';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';
import { isInForce } from '../decisions/decisionModel';

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

/** 継承元1件ぶん（名前・件数・辿る先）。 */
interface Source {
  tagId: string;
  count: number;
}

export function InheritedRules({ record }: { record: RecordRef }) {
  const t = useT();
  const { tagName, currencyIndex } = useLookups();
  const [entries, setEntries] = useState<GovernsEntry[] | null>(null);

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

  // own（このレコード自身への決定）は意思決定欄が本文で出しているので数えない。
  // タグカードで own が effective-tag として返っていたバグ（internal/index/
  // query.go の GovernsForTag）を直したので、ここは3種のカードで同じ意味になる。
  const inherited = entries.filter((e) => e.provenance !== 'own' && isInForce(e.decision.id, currencyIndex));
  if (inherited.length === 0) return null;

  const byTag = new Map<string, number>();
  for (const e of inherited) {
    const via = e.viaTag || '';
    if (!via) continue;
    byTag.set(via, (byTag.get(via) || 0) + 1);
  }
  const sources: Source[] = [...byTag.entries()]
    .map(([tagId, count]) => ({ tagId, count }))
    .sort((a, b) => b.count - a.count || tagName(a.tagId).localeCompare(tagName(b.tagId)));

  const heading = record.kind === 'transition' ? t.browse.inheritedFromTags(inherited.length) : t.browse.inheritedFromAncestors(inherited.length);

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
    </div>
  );
}
