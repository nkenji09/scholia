import { useState } from 'preact/hooks';
import { useT } from '../../i18n';
import { useLookups } from '../../lookups';
import { loadCardSectionOpen, saveCardSectionOpen } from '../../collapseState';
import { summaryOf } from '../../decisionSummary';
import type { Decision, SupersedeLink } from '../../types';
import { Icon } from '../shared/Icon';
import { Markdown } from '../Markdown';
import { DecisionIdReveal } from './DecisionIdReveal';
import { effectOf, linkMode, relatedDecisions } from './decisionModel';

// 意思決定の一覧の行（01KYKS4Y56FAHRVCWKMQJK4RT6）。
//
// この行は**単票の代わり**である。単票（#/decision/<id>）が持っていた12種類を
// そのまま持ち、違うのは「戻る」を押さずに絞り込み条件を緩められることだけ。
// 実測で 158件中 146件が、旧一覧の行では読めない要素（変更内容84 / 参照81 /
// 実装コミット104 / 置き換え関係 両方向で各40 / 容認2）を1つ以上持っていた
// ——抜けると実害が出るので、ここが全部を負う。
//
// 形はゼロから作っていない。「要約が主表示・全文は展開の内側」（条項6）と
// 「関係は並び順ではなく明示的な導線で辿る」（条項7）は
// 01KYHW54B8ZXH0NEPH2J7N1X39 が定め、レコードカードの意思決定欄（DecisionList）と
// 概要シートの規則欄（OverviewView.renderRuleRow）が既に体現している。ここは
// その形に、単票にしか無かった5つ（変更内容・実装コミット・容認・置き換え関係の
// 両方向すべて・記録を指す文字列の開示）を足したもの。
//
// **カード側の行はこの行に寄せない。** 利用者の指示が「概要の方の意思決定は
// 短く見せて、詳細は詳細に任せる、というアプローチでいいのだけれど、意思決定
// 一覧では…その場で全文見せていい」と、面ごとに違う粒度を明示的に求めている。
// 面間整合（01KXYED62CEKBY97D7X66BMC9A）は粒度差そのものを禁じておらず、
// 「サーフェスごとの意図的 decision に従う」ことを求めている——その意図的
// decision がこれである。
//
// 生成 id の扱いは 01KYK4YNCYGZHHXB4H90Q996T2 のまま持ち越す: 裸の id を既定の
// 見え方に置かず（条項3）、実行・貼り付けのための文字列として求められたときに
// だけ出す（条項4）。単票が消えても到達手段を落とさない（条項5）ために、開示は
// 単票からこの行の中へそのまま移した。

/** 開閉の永続キー。レコード×セクション単位（collapseState と同じ名前空間）。 */
const SECTION = 'decision-row-full';

interface Props {
  d: Decision;
  /** 一覧の結果が1件のときだけ true。**初期既定**であって上書きではない——
      利用者が明示的に閉じた保存値のほうが勝つ（01KYGYYN8HRNFQEDMBS3DZRRX7）。 */
  defaultOpen: boolean;
  /** 置き換え関係のチップから、同じ仕組みの上を移動する（別画面には行かない）。 */
  onOpenDecision: (id: string) => void;
  /** 相手の decision を名前で示すための索引（生 id をラベルに使わない）。 */
  byId: Map<string, Decision>;
}

function isUrl(s: string): boolean {
  return /^https?:\/\//i.test(s.trim());
}

export function DecisionRowFull({ d, defaultOpen, onOpenDecision, byId }: Props) {
  const t = useT();
  const { tagName, vocabLabel, transitionLabel, formatDecisionAt, currencyIndex } = useLookups();
  // 保存値が最優先・無ければ defaultOpen（＝1件に絞られたら開いて着地する）。
  const [open, setOpen] = useState<boolean>(() => loadCardSectionOpen(d.id, SECTION) ?? defaultOpen);

  const toggle = () => {
    setOpen((prev) => {
      const next = !prev;
      saveCardSectionOpen(d.id, SECTION, next);
      return next;
    });
  };

  const targetLabel = (x: Decision): string => {
    if (x.target.type === 'tag') return tagName(x.target.id);
    if (x.target.type === 'vocab') return vocabLabel(x.target.id);
    return transitionLabel(x.target.id).primary;
  };
  const targetPrefix = (type: Decision['target']['type']): string =>
    type === 'tag' ? t.decisions.targetPrefixTag : type === 'vocab' ? t.decisions.targetPrefixVocab : t.decisions.targetPrefixTransition;
  const modeLabel = (mode: string | undefined): string => {
    const m = linkMode(mode);
    return m === 'supersede' ? t.decisions.modeSupersede : m === 'exception' ? t.decisions.modeException : t.decisions.modeAmend;
  };
  // 相手は名前で示す（01KYCC2TF3NW3JRSSRK9ZHN078 / 01KYK4YNCYGZHHXB4H90Q996T2 条項2）。
  // 索引に無い相手だけ、名乗るものが他に無いので id へ落ちる。
  const linkChipLabel = (id: string): string => {
    const linked = byId.get(id);
    return linked ? `${targetPrefix(linked.target.type)} ${targetLabel(linked)}` : id;
  };

  // 効力は2値（01KYHW54B8ZXH0NEPH2J7N1X39 条項1）。記録の3値はそのまま。
  const replaced = effectOf(d.id, currencyIndex) === 'replaced';
  // 条項2: 「後続に部分改訂・例外が付いている」は状態ではなく付帯情報。
  const related = relatedDecisions(d.id, currencyIndex);
  // 導出: この decision を指している側（単票の「この意思決定を置き換え/改訂」）。
  const supersededBy = currencyIndex.supersededByMap.get(d.id) || [];
  const supersedes = d.supersedes || [];
  const commits = d.commits || [];
  const acknowledges = d.acknowledges || [];

  return (
    <div class={'decision-row decision-row-expandable' + (replaced ? ' replaced' : '')}>
      <div class="decision-row-top">
        <span class="decision-row-target">
          <span class="decision-row-target-kind dim">{targetPrefix(d.target.type)}</span>
          {targetLabel(d)}
        </span>
        <span class="decision-row-spacer" />
        <span class={'decision-badge ' + (replaced ? 'decision-badge-superseded' : 'decision-badge-current')}>
          {replaced ? t.decisions.effectReplaced : t.decisions.effectInForce}
        </span>
      </div>

      {/* 条項6: 要約が主表示・全文は展開の内側。要約の切り出しは共有
          （decisionSummary）を通す——面ごとに slice すると面ごとに違う長さが出る。 */}
      {!open && <p class="decision-row-why">{summaryOf(d.why)}</p>}

      {open && (
        <div class="decision-row-body">
          <section class="decision-detail-section">
            <h3 class="decision-detail-heading">{t.decisions.whyHeading}</h3>
            <Markdown text={d.why} class="decision-detail-why" />
          </section>

          {d.changed && d.changed.trim() && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.changedHeading}</h3>
              <Markdown text={d.changed} class="decision-detail-changed" />
            </section>
          )}

          {d.ref && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.refHeading}</h3>
              {isUrl(d.ref) ? (
                <a class="decision-detail-ref-link" href={d.ref} target="_blank" rel="noopener noreferrer">
                  {d.ref} <Icon name="external-link" size={12} />
                </a>
              ) : (
                <p class="decision-detail-ref">{d.ref}</p>
              )}
            </section>
          )}

          {commits.length > 0 && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.commitsHeading}</h3>
              <div class="decision-chip-row">
                {commits.map((c) => (
                  <span key={c} class="decision-commit-chip" title={c}>
                    {c.slice(0, 8)}
                  </span>
                ))}
              </div>
            </section>
          )}

          {acknowledges.length > 0 && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.acknowledgesHeading}</h3>
              <div class="decision-chip-row">
                {acknowledges.map((a) => (
                  <span key={a} class="decision-ack-chip">
                    {a}
                  </span>
                ))}
              </div>
            </section>
          )}

          {/* 条項7: 関係は並び順ではなく明示的な導線で辿る。踏むと「その1件に
              絞り込んだ一覧」へ移る＝同じ仕組みの上の移動で、別画面には行かない。 */}
          {supersedes.length > 0 && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.supersedesHeading}</h3>
              <div class="decision-chip-row">
                {supersedes.map((link: SupersedeLink) => (
                  <button key={link.id} type="button" class="decision-link-chip" onClick={() => onOpenDecision(link.id)}>
                    <span class="decision-link-mode">{modeLabel(link.mode)}</span>
                    {linkChipLabel(link.id)}
                  </button>
                ))}
              </div>
            </section>
          )}

          {supersededBy.length > 0 && (
            <section class="decision-detail-section">
              <h3 class="decision-detail-heading">{t.decisions.supersededByHeading}</h3>
              <div class="decision-chip-row">
                {supersededBy.map((other) => {
                  const link = (other.supersedes || []).find((l) => l.id === d.id);
                  return (
                    <button key={other.id} type="button" class="decision-link-chip" onClick={() => onOpenDecision(other.id)}>
                      <span class="decision-link-mode">{modeLabel(link?.mode)}</span>
                      {`${targetPrefix(other.target.type)} ${targetLabel(other)}`}
                    </button>
                  );
                })}
              </div>
            </section>
          )}

          <DecisionIdReveal id={d.id} />
        </div>
      )}

      <div class="decision-row-meta">
        <button type="button" class="decision-row-toggle" onClick={toggle} aria-expanded={open}>
          <Icon name={open ? 'chevron-up' : 'chevron-down'} size={13} />
          {open ? t.overview.backToSummary : t.overview.readFull}
        </button>
        {/* 条項2: 付帯情報。注記ではなく辿れる導線にする（旧一覧はここが
            クリックできない <span> だった）。 */}
        {related.length > 0 && (
          <button type="button" class="decision-row-related" onClick={() => onOpenDecision(related[0].id)}>
            <Icon name="arrow-up-right" size={12} />
            {t.decisions.readTogether(related.length)}
          </button>
        )}
        <span class="decision-row-spacer" />
        <span class="decision-row-at dim">{formatDecisionAt(d.at)}</span>
      </div>
    </div>
  );
}
