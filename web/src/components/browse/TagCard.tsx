import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Decision, SpecReport, Tag, VocabEntry } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { CollapsibleSection } from '../shared/CollapsibleSection';
import { HashLink } from '../shared/HashLink';
import { KebabMenu } from '../shared/KebabMenu';
import { routeHash } from '../../router';
import { DecisionList } from '../decisions/DecisionList';
import { InheritedRules } from './InheritedRules';

// VocabCard と同じ category→アイコン対応（きっかけ/前提/結果 = action/
// condition/effect の固定3軸）。関連語彙行（H3）で流用する。
const CATEGORY_ICON: Record<VocabEntry['category'], IconName> = {
  action: 'circle-play',
  condition: 'funnel',
  effect: 'arrow-right-to-line',
};

interface Props {
  tag: Tag;
  report: SpecReport | undefined;
  isGap: boolean | undefined; // undefined = this tag's kind isn't traceability-tracked
  /** 祖先の連なり（遠い側 → 直接の親）。直接の親だけでは、祖父に書かれた規則へ
      カードから辿れない（01KYHW4NBNVN9BFXYZMBX8MPF8 条項4）。 */
  ancestors: Tag[];
  children: Tag[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterSelf: () => void;
  onSelectParent: (tagId: string) => void;
  onSelectChild: (tagId: string) => void;
  onSelectSpec: (txId: string) => void;
  onSelectVocab: (vocabId: string) => void;
}

function dedupeDecisions(decisions: Decision[]): Decision[] {
  const seen = new Set<string>();
  const out: Decision[] = [];
  for (const d of decisions) {
    if (seen.has(d.id)) continue;
    seen.add(d.id);
    out.push(d);
  }
  return out;
}

export function TagCard({ tag, report, isGap, ancestors, children, cardRef, onFilterSelf, onSelectParent, onSelectChild, onSelectSpec, onSelectVocab }: Props) {
  const t = useT();
  const { tagKindLabel, tagKindDescription, vocabLabel } = useLookups();
  const { changedTagIds } = usePendingDiff();
  const { openComposer, comments } = useComments();
  const entries = report?.entries || [];
  // 軸カードの構造表示（#45 D10b-6）。backend が axis kind＋値ありのタグにだけ
  // report.axis を載せる（フロントは axis 判定を再計算しない・§9）。
  const axis = report?.axis;
  // H3: このタグを直接持つ語彙（Go 側 render.SpecReport.RelatedVocab・
  // VocabEntry.Tags の逆引き）。関連仕様の"上"に常時開きで出す。
  const relatedVocab = report?.relatedVocab || [];
  // このタグ自身に直接ぶら下がる decision（own-only・祖先/子孫の cross-cutting は
  // 出さない・req.comfortable-viewer.decision-display）。Go 側 render.SpecReport の
  // トップレベル tagDecisions を読む。従来は entries を flatMap して拾っていたが、
  // transition を持たないタグでは entries が空で decision が完全に消えていた
  // （tag-decision-visibility）。target.id 明示照合は「そのレコード自身の意思決定
  // だけ」を保証する保険として残す。
  const tagDecisions = dedupeDecisions((report?.tagDecisions || []).filter((d) => d.target.type === 'tag' && d.target.id === tag.id));
  // §8.8 P5 vocab/tag（generalized from SpecCard's hasUncommentedChange・
  // §8.3）: see VocabCard.tsx's identical comment.
  const hasUncommentedChange = changedTagIds.has(tag.id) && !comments.some((c) => c.recordType === 'tag' && c.recordId === tag.id);

  return (
    <article ref={cardRef} data-card-id={tag.id} class="card" title={tag.id}>
      {hasUncommentedChange && (
        <button
          type="button"
          class="spec-card-clean-flag"
          onClick={() =>
            openComposer({ recordType: 'tag', recordId: tag.id, recordTitle: tag.name || tag.id, anchor: 'card', anchorLabel: t.comments.cardAnchorLabel })
          }
        >
          <Icon name="git-compare" size={12} /> {t.comments.proposalCleanFlag}
        </button>
      )}
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(tag.kind)} title={tagKindDescription(tag.kind)}>{tag.kind ? tagKindLabel(tag.kind) : '?'}</Chip>
          {ancestors.length > 0 && (
            /* 祖先の連なり全体（条項4）。遠い祖先から順に並べ、区切りで階層が
               読めるようにする——タグ id のドットは階層ではないので、id を見ても
               祖先は分からない。ここが直接の親だけだと、祖父に書かれた規則へ
               カードから到達する手段が画面に無くなる。 */
            <span class="tag-card-parents dim">
              <Icon name="corner-down-right" size={13} />
              {ancestors.map((p, i) => (
                <>
                  {i > 0 && <span class="tag-card-parent-sep">›</span>}
                  <HashLink key={p.id} href={routeHash({ view: 'spec', tagId: p.id })} class="tag-card-parent-link" onNavigate={() => onSelectParent(p.id)} title={t.browse.parentLinkTitle}>
                    {p.name || p.id}
                  </HashLink>
                </>
              ))}
            </span>
          )}
          <span class="tag-card-spacer" />
          {isGap && (
            <span class="tag-card-gap-badge">
              <Icon name="triangle-alert" size={12} /> {t.browse.gapBadge}
            </span>
          )}
          {!isGap && entries.length > 0 && (
            <span class="tag-card-sat-badge">
              <Icon name="check" size={12} /> {t.browse.satBadge(entries.length)}
            </span>
          )}
          <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="card" anchorLabel={t.comments.cardAnchorLabel} />
        </div>
        <div class="tag-card-name-row">
          <span class="tag-card-name">{tag.name || tag.id}</span>
          <KebabMenu
            triggerLabel={t.browse.menuTrigger}
            items={[
              { key: 'filter', label: t.browse.menuAddFilter, icon: 'plus', onSelect: onFilterSelf },
              { key: 'open', label: t.browse.menuOpenLink, icon: 'external-link', href: routeHash({ view: 'spec', tagId: tag.id }) },
            ]}
          />
        </div>
      </div>

      {tag.description && (
        <div class="tag-card-body">
          <div class="card-section-heading-row">
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="body" anchorLabel={t.comments.descriptionAnchorLabel} />
          </div>
          <Markdown text={tag.description} />
        </div>
      )}

      {/* 軸の構造表示（#45 D10b-6）: 状態次元バッジ・total（宣言由来・非検証と
          明示）・値の一覧（軸タグ付き condition・各値の語彙カードへリンク）・
          効いている action（その値が given に現れる action・#/flow/<action> へ
          リンク）。表示が入ることで desc への値列挙の複製の動機が UI から消える
          （retrofit が値列挙 desc を安全に是正できる受け皿）。 */}
      {axis && (
        <div class="card-section axis-structure">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="git-fork" size={14} /> {t.browse.axisStructureHeading}
            </span>
          </div>
          <div class="axis-structure-meta">
            <span class="axis-dimension-badge">{t.browse.axisDimensionBadge}</span>
            <span class={'axis-total-badge ' + (axis.total ? 'axis-total-true' : 'axis-total-false')}>
              {axis.total ? t.browse.axisTotalTrue : t.browse.axisTotalFalse}
            </span>
          </div>
          {axis.values.length === 0 ? (
            <span class="dim axis-no-values">{t.browse.axisNoValues}</span>
          ) : (
            <ul class="axis-value-list">
              {axis.values.map((v) => (
                <li key={v.condition.id} class="axis-value">
                  <HashLink
                    href={routeHash({ view: 'vocab', vocabId: v.condition.id })}
                    class="axis-value-label"
                    onNavigate={() => onSelectVocab(v.condition.id)}
                    title={v.condition.id}
                  >
                    {v.condition.label || v.condition.id}
                  </HashLink>
                  {v.actions.length > 0 && (
                    <span class="axis-value-actions">
                      <span class="dim axis-value-actions-label">{t.browse.axisValueActions}:</span>
                      {v.actions.map((a) => (
                        <HashLink
                          key={a}
                          href={routeHash({ view: 'flow', actionId: a })}
                          class="axis-value-action-chip"
                          onNavigate={() => {
                            window.location.hash = routeHash({ view: 'flow', actionId: a });
                          }}
                          title={a}
                        >
                          <Icon name="git-fork" size={11} /> {vocabLabel(a)}
                        </HashLink>
                      ))}
                    </span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {/* H3: 関連語彙（このタグを直接持つ vocab）。関連仕様の"上"・常時開き
          （ユーザー明示「開閉できなくて良い」＝CollapsibleSection ではなく素の
          card-section）。各行は category バッジ（きっかけ/前提/結果）＋kind
          （api/prop/user 等）＋ラベルで VocabCard の vocab 行を踏襲。すぐ下の
          関連仕様行（HashLink→#/vocab/<id>）と同じく語彙カードへのリンクにする
          （related-vocab-row-linkable）。 */}
      {relatedVocab.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="book-open" size={14} /> {t.browse.relatedVocab} <span class="card-section-count dim">({relatedVocab.length})</span>
            </span>
          </div>
          <div class="tag-card-spec-list">
            {relatedVocab.map((v) => (
              <HashLink key={v.id} href={routeHash({ view: 'vocab', vocabId: v.id })} class="tag-card-vocab-row" onNavigate={() => onSelectVocab(v.id)} title={v.id}>
                <Chip color={kindColor(v.category)}>
                  <Icon name={CATEGORY_ICON[v.category]} size={12} /> {t.vocab.categoryLabel(v.category)}
                </Chip>
                {v.kind && <span class="vocab-card-kind dim">{v.kind}</span>}
                <span class="tag-card-vocab-label">{v.label}</span>
              </HashLink>
            ))}
          </div>
        </div>
      )}

      {entries.length > 0 && (
        <CollapsibleSection
          recordId={tag.id}
          section="specs"
          count={entries.length}
          icon="scroll-text"
          label={t.browse.satisfiedSpecs}
          extra={<CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="specs" anchorLabel={t.browse.satisfiedSpecs} />}
        >
          <div class="tag-card-spec-list">
            {entries.map((e) => {
              // 同一 action 複数遷移が同文に潰れる縮退の解消（D10a item 7）: 同じ
              // action を持つ複数の遷移は actionLabel だけでは全く同じ行に見えて
              // しまうので、区別している given のラベルを dim の接尾に添える
              // （SpecEntry.givenLabels — SpecCard/VocabCard と同じ由来。無条件
              // 遷移は given が無いので接尾も付かない）。
              const given = (e.givenLabels || []).filter((g) => !!g);
              return (
                <HashLink key={e.transition.id} href={routeHash({ view: 'browse', txId: e.transition.id })} class="tag-card-spec-row" onNavigate={() => onSelectSpec(e.transition.id)} title={e.transition.id}>
                  <span class="tag-card-spec-label">
                    {e.actionLabel}
                    {given.length > 0 && <span class="dim tag-card-spec-given"> · {given.join('、')}</span>}
                  </span>
                </HashLink>
              );
            })}
          </div>
        </CollapsibleSection>
      )}

      {/* 意思決定欄（01KYHW54B8ZXH0NEPH2J7N1X39）: 効力2値・付帯情報・履歴を畳む
          までを DecisionList が1箇所で担う。ここに本文で並ぶのはこのタグ自身を
          対象とする decision だけ（own-only 01KXDFD2RZJ118T2VVAF5F07RW）。 */}
      <DecisionList
        recordId={tag.id}
        decisions={tagDecisions}
        label={t.browse.relatedDecisions}
        extra={<CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="decisions" anchorLabel={t.browse.relatedDecisions} />}
      />

      {/* 継承した規則の開示（01KYHW4NBNVN9BFXYZMBX8MPF8 条項3）: 「この記録を
          支配する規則」欄（全文を再掲していた）を廃止した代わり。本文は並べず、
          件数と継承元と導線だけを出す。 */}
      <InheritedRules record={{ kind: 'tag', id: tag.id }} />

      {/* H2: 下位のタグを件数付きで開閉可能に（5件以上で既定折りたたみ＝
          CollapsibleSection の既定しきい値そのまま）。specs/decisions と同じ
          パターン。CommentButton は extra prop で維持。 */}
      {children.length > 0 && (
        <CollapsibleSection
          recordId={tag.id}
          section="children"
          count={children.length}
          icon="list-tree"
          label={t.browse.childTags}
          extra={<CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="children" anchorLabel={t.browse.childTags} />}
        >
          <div class="tag-card-children">
            {children.map((c) => (
              <HashLink key={c.id} href={routeHash({ view: 'spec', tagId: c.id })} class="tag-card-child-chip" onNavigate={() => onSelectChild(c.id)} title={t.browse.childLinkTitle}>
                <Icon name="corner-down-right" size={12} /> {c.name || c.id}
              </HashLink>
            ))}
          </div>
        </CollapsibleSection>
      )}
    </article>
  );
}
