import { useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { routeHash } from '../../router';
import { summaryOf } from '../../decisionSummary';
import type { Decision } from '../../types';
import { HashLink } from '../shared/HashLink';
import { Icon } from '../shared/Icon';
import { Markdown } from '../Markdown';
import { CollapsibleSection } from '../shared/CollapsibleSection';
import { effectOf, relatedDecisions, replacedBy } from './decisionModel';

// 共有部品なので親から遷移コールバックを貰えない。平打ちでも hash 代入で遷移する
// ——HashLink の修飾クリック（別タブ）はそのまま効くので、リンクとしての性質
// （01KXFK3Q1NY9J8Q7FX14T31N7K）は保たれる。
const goto = (hash: string) => () => {
  window.location.hash = hash;
};

// レコード自身の意思決定欄（01KYHW54B8ZXH0NEPH2J7N1X39）。TagCard / SpecCard /
// VocabCard の3種で共有する**唯一の**描画口。面ごとに書き分けると、また面ごとに
// 違う答えが出る（面間整合 01KXYED62CEKBY97D7X66BMC9A）。
//
// 条項の対応:
//  ① 状態列は2値（効いている／置き換え済み）。効いているものどうしに序列を付けない
//  ② 「後続に部分改訂・例外が付いている」は状態ではなく付帯情報＝件数と導線
//  ③ 効いていない側の語は「置き換え済み」——理由が読める語にし、置き換えた側へ辿れる
//  ④ 効いていないものは既定で畳む（0件なら口自体を出さない）
//  ⑤ 見出しの件数は効いている数（開いて見える行数と一致する）
//  ⑥ 要約が主表示・全文は展開の内側（要約の切り出しは decisionSummary.ts）
//  ⑦ 並び順だけで関係を推測させない＝関係は明示的な導線で辿る
//
// ここに本文で並ぶのは**そのレコード自身を対象とする decision だけ**
// （01KYHW4NBNVN9BFXYZMBX8MPF8 条項2・own-only 01KXDFD2RZJ118T2VVAF5F07RW）。
// 継承した規則は本文を並べず InheritedRules が件数と導線だけを出す。

function DecisionRow({ d, replaced }: { d: Decision; replaced: boolean }) {
  const t = useT();
  const { currencyIndex, formatDecisionAt } = useLookups();
  const [open, setOpen] = useState(false);

  // ②: 後続で部分改訂・例外を付けた decision。状態ではないので状態列に置かない。
  const related = replaced ? [] : relatedDecisions(d.id, currencyIndex);
  // ③: 置き換えられた側からは、置き換えた側へ辿れること。
  const replacer = replaced ? replacedBy(d.id, currencyIndex) : undefined;

  return (
    <div class={'decision-row' + (replaced ? ' replaced' : '')}>
      <div class="decision-row-top">
        <span class={'decision-row-summary' + (replaced ? ' struck' : '')}>{summaryOf(d.why)}</span>
        {/* ①: 出せる状態は2値だけ。効いているものどうしを色の段階で分けない。 */}
        <span class={'decision-row-effect' + (replaced ? ' replaced' : '')}>
          <Icon name={replaced ? 'circle-slash' : 'circle-check'} size={11} />
          {replaced ? t.decisions.effectReplaced : t.decisions.effectInForce}
        </span>
      </div>
      {/* ⑥: 全文は展開の内側。 */}
      {open && (
        <div class="decision-row-why">
          <Markdown text={d.why} />
        </div>
      )}
      <div class="decision-row-meta">
        <button type="button" class="decision-row-toggle" onClick={() => setOpen(!open)}>
          <Icon name={open ? 'chevron-up' : 'chevron-down'} size={13} />
          {open ? t.overview.backToSummary : t.overview.readFull}
        </button>
        {/* ②⑦: 関係は並び順ではなく明示的な導線で辿る。 */}
        {related.length > 0 && (
          <HashLink
            href={routeHash({ view: 'decision', decisionId: related[0].id })}
            onNavigate={goto(routeHash({ view: 'decision', decisionId: related[0].id }))}
            class="decision-row-related"
          >
            <Icon name="arrow-up-right" size={12} />
            {t.decisions.readTogether(related.length)}
          </HashLink>
        )}
        {replacer && (
          <HashLink
            href={routeHash({ view: 'decision', decisionId: replacer.id })}
            onNavigate={goto(routeHash({ view: 'decision', decisionId: replacer.id }))}
            class="decision-row-related"
          >
            <Icon name="arrow-up-right" size={12} />
            {t.decisions.openReplacement}
          </HashLink>
        )}
        <span class="decision-row-spacer" />
        <span class="decision-row-at">
          {formatDecisionAt(d.at)}
          {d.ref && ` · ${d.ref}`}
        </span>
      </div>
    </div>
  );
}

export function DecisionList({
  recordId,
  decisions,
  section = 'decisions',
  label,
  focusOpen,
  onToggle,
  extra,
}: {
  recordId: string;
  decisions: Decision[];
  section?: string;
  label: string;
  focusOpen?: boolean;
  onToggle?: () => void;
  extra?: preact.ComponentChildren;
}) {
  const t = useT();
  const { currencyIndex } = useLookups();
  const [historyOpen, setHistoryOpen] = useState(false);

  if (decisions.length === 0) return null;

  const inForce = decisions.filter((d) => effectOf(d.id, currencyIndex) === 'in-force');
  const replaced = decisions.filter((d) => effectOf(d.id, currencyIndex) === 'replaced');

  // ⑤: 見出しの件数は効いている数。置き換え済みだけが残る（inForce が0）ときも
  // 欄自体は出す——履歴の存在が読めなくなるほうが悪い。
  return (
    <CollapsibleSection
      recordId={recordId}
      section={section}
      count={inForce.length}
      icon="gavel"
      label={label}
      // 意思決定はそのレコードの核となる履歴なので件数しきい値で隠さず既定展開
      // （01KXGATP16Z1C9GB19PV1BHR61「一番残したい履歴を折りたたまない」）。
      // localStorage 済みのユーザー操作は従来どおり最優先。
      defaultOpen={true}
      focusOpen={focusOpen}
      onToggle={onToggle}
      extra={extra}
    >
      <div class="decision-list">
        {inForce.map((d) => (
          <DecisionRow key={d.id} d={d} replaced={false} />
        ))}
        {/* ④: 効いていないものは既定で畳む。0件なら口自体を出さない。 */}
        {replaced.length > 0 && (
          <div class="decision-history">
            <button type="button" class="decision-history-toggle" onClick={() => setHistoryOpen(!historyOpen)} aria-expanded={historyOpen}>
              <Icon name={historyOpen ? 'chevron-down' : 'chevron-right'} size={13} />
              <Icon name="scroll-text" size={13} />
              {t.decisions.replacedHeading(replaced.length)}
            </button>
            {historyOpen && (
              <div class="decision-history-list">
                {replaced.map((d) => (
                  <DecisionRow key={d.id} d={d} replaced={true} />
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    </CollapsibleSection>
  );
}
