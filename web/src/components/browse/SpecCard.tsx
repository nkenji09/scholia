import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { EffectiveTag, TransitionDetail } from '../../types';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';
import { HashLink } from '../shared/HashLink';
import { routeHash } from '../../router';
import { CollapsibleSection } from '../shared/CollapsibleSection';

interface Props {
  detail: TransitionDetail;
  isOpen: boolean;
  cardRef: (el: HTMLElement | null) => void;
  onToggleOpen: () => void;
  onFilterVocab: (id: string) => void;
  onFilterTag: (id: string) => void;
  // 結果(Then)スロットの owner チップ廃止に伴い未使用だが、呼び出し側
  // (BrowseView) は並行トラックの領域で今回触らないため prop は残す。
  onFilterOwner: (owner: string) => void;
  /** ↗ 詳細リンクの平打ち左クリック時に走る SPA 内遷移（card-detail-link）。
      語彙 → 語彙フォーカス（#/vocab/<id>）、タグ → タグ詳細（#/spec/<id>）。
      modified/中クリックは HashLink 側で preventDefault せず別タブに委ねる。 */
  onGoToVocab: (id: string) => void;
  onGoToTag: (id: string) => void;
}

// filter（⊕）と詳細リンク（↗）の2アイコンだけをクリック領域にする affordance 対
// （card-detail-link）。ラベル本体は非クリックの <span> に退避し、誤タップを避け
// つつ deep-linking を SpecCard の tag/vocab へ延長する（item3 除外条項の精緻化：
// filter は button のまま「遷移ではない」を保ち、遷移は別の専用 <a href> で提供）。
// ⊕/↗ とも最小 44×44 CSS px 相当のタップターゲット（CSS 側 padding で確保）。
function SpecAffordances({
  onFilter,
  filterLabel,
  detailHref,
  onNavigate,
  detailLabel,
}: {
  onFilter: () => void;
  filterLabel: string;
  detailHref: string;
  onNavigate: () => void;
  detailLabel: string;
}) {
  return (
    <span class="spec-affordances">
      <button type="button" class="spec-affordance-btn spec-filter-btn" onClick={onFilter} aria-label={filterLabel} title={filterLabel}>
        <Icon name="plus" size={15} class="filter-plus-icon" />
      </button>
      <HashLink class="spec-affordance-btn spec-detail-link" href={detailHref} onNavigate={onNavigate} ariaLabel={detailLabel} title={detailLabel}>
        <Icon name="external-link" size={14} />
      </HashLink>
    </span>
  );
}

export function SpecCard({ detail, isOpen, cardRef, onToggleOpen, onFilterVocab, onFilterTag, onGoToVocab, onGoToTag }: Props) {
  const t = useT();
  const { tagById } = useLookups();
  const { changedTransitionIds, addedTransitionIds } = usePendingDiff();
  const { openComposer, comments } = useComments();
  // #27 P2′-rework (change-cockpit-design-v3.md §8.3): a pending change with
  // no comment yet isn't a "proposal" (that's derived from comment × change
  // in CommentPanel) — it's just a quiet pending-change flag here, and it
  // steps aside once someone comments on this record (the comment itself
  // then carries the inline diff card and counts toward the badge).
  const hasUncommentedChange = changedTransitionIds.has(detail.id) && !comments.some((c) => c.recordType === 'transition' && c.recordId === detail.id);
  // §8.8 P5・M-5「追加」（§3 の 3種別表）: this transition exists in the
  // working tree but not at base — always badged (not gated on comment
  // state like hasUncommentedChange above, since "this is new" is a
  // structural fact about the record, not an attention-needed nudge).
  const isAdded = addedTransitionIds.has(detail.id);

  // own/derived split reads straight off backend-computed provenance (gap
  // G11) — no re-derivation client-side (§9). "own" is any tag directly
  // assigned (detail.tags), even if it's *also* reachable via vocab/ancestor
  // (multi-path); the badge below surfaces that extra provenance.
  const effective = detail.effectiveTags || [];
  const own = effective.filter((et) => et.sources.includes('own'));
  const derived = effective.filter((et) => !et.sources.includes('own'));
  const provenanceBadge = (et: EffectiveTag) => {
    const extra = et.sources.filter((s) => s !== 'own');
    return extra.length > 0 ? t.browse.provenanceLabel(extra) : null;
  };
  const hasDetail = detail.rules && detail.rules.length > 0;

  return (
    <article ref={cardRef} data-card-id={detail.id} class={'card' + (isAdded ? ' card-added-proposal' : '')}>
      {isAdded && (
        <button
          type="button"
          class="spec-card-added-badge"
          onClick={() =>
            openComposer({
              recordType: 'transition',
              recordId: detail.id,
              recordTitle: detail.actionLabel || detail.action,
              anchor: 'action',
              anchorLabel: t.flow.trigger,
            })
          }
        >
          <Icon name="plus" size={12} /> {t.comments.proposalAddedBadge}
        </button>
      )}
      {hasUncommentedChange && (
        <button
          type="button"
          class="spec-card-clean-flag"
          onClick={() =>
            openComposer({
              recordType: 'transition',
              recordId: detail.id,
              recordTitle: detail.actionLabel || detail.action,
              anchor: 'action',
              anchorLabel: t.flow.trigger,
            })
          }
        >
          <Icon name="git-compare" size={12} /> {t.comments.proposalCleanFlag}
        </button>
      )}
      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-act)' }}>
            <Icon name="circle-play" size={13} /> {t.flow.trigger}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="action" anchorLabel={t.flow.trigger} />
        </div>
        <div class="spec-card-action">
          <span class="spec-card-action-label">{detail.actionLabel || detail.action}</span>
          <SpecAffordances
            onFilter={() => onFilterVocab(detail.action)}
            filterLabel={t.browse.clickToFilter}
            detailHref={routeHash({ view: 'vocab', vocabId: detail.action })}
            onNavigate={() => onGoToVocab(detail.action)}
            detailLabel={t.browse.openDetail}
          />
        </div>
      </div>

      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-giv)' }}>
            <Icon name="funnel" size={12} /> {t.flow.given}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="given" anchorLabel={t.flow.given} />
        </div>
        {(detail.given || []).length === 0 && <span class="dim spec-card-empty-given">{t.flow.noGiven}</span>}
        <div class="spec-card-given-list">
          {(detail.given || []).map((id, i) => (
            <div key={id} class="spec-card-cond-row">
              <span class="spec-card-cond-label">{(detail.givenLabels || [])[i] || id}</span>
              <SpecAffordances
                onFilter={() => onFilterVocab(id)}
                filterLabel={t.browse.clickToFilter}
                detailHref={routeHash({ view: 'vocab', vocabId: id })}
                onNavigate={() => onGoToVocab(id)}
                detailLabel={t.browse.openDetail}
              />
            </div>
          ))}
        </div>
      </div>

      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-then)' }}>
            <Icon name="arrow-right-to-line" size={12} /> {t.flow.result}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="then" anchorLabel={t.flow.result} />
        </div>
        <div class="spec-card-then-list">
          {(detail.then || []).map((id, i) => (
            <div key={id} class="spec-card-cond-row">
              <span class="spec-card-then-n dim">{i + 1}</span>
              <span class="spec-card-cond-label">{(detail.thenLabels || [])[i] || id}</span>
              <SpecAffordances
                onFilter={() => onFilterVocab(id)}
                filterLabel={t.browse.clickToFilter}
                detailHref={routeHash({ view: 'vocab', vocabId: id })}
                onNavigate={() => onGoToVocab(id)}
                detailLabel={t.browse.openDetail}
              />
            </div>
          ))}
        </div>
      </div>

      {own.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="tags" size={14} /> {t.browse.tagsHeading}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {t.browse.clickToFilter}
              </span>
            </span>
            <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="tags" anchorLabel={t.browse.tagsHeading} />
          </div>
          <div class="spec-card-chip-row">
            {own.map((et) => (
              <span key={et.id} class="spec-tag-item">
                <Chip color={kindColor(tagById.get(et.id)?.kind)} title={t.browse.provenanceLabel(et.sources)}>
                  {tagById.get(et.id)?.name || et.id}
                  {provenanceBadge(et) && <span class="tag-provenance-badge">{provenanceBadge(et)}</span>}
                </Chip>
                <SpecAffordances
                  onFilter={() => onFilterTag(et.id)}
                  filterLabel={t.browse.clickToFilter}
                  detailHref={routeHash({ view: 'spec', tagId: et.id })}
                  onNavigate={() => onGoToTag(et.id)}
                  detailLabel={t.browse.openDetail}
                />
              </span>
            ))}
          </div>
        </div>
      )}

      {/* H1: 継承タグ（own でない実効タグ）は既定で閉じておく（ユーザー明示
          「デフォルトでは閉じておく」＝件数に関わらず常に既定閉じ・H2 の
          5件しきい値とは別扱い）。defaultOpen={false} で件数しきい値を上書き、
          localStorage 済みのユーザー操作は尊重（一度開けば次回復元）。 */}
      {derived.length > 0 && (
        <CollapsibleSection recordId={detail.id} section="derived" count={derived.length} icon="tags" label={t.browse.derivedHeading} defaultOpen={false}>
          <div class="spec-card-chip-row">
            {derived.map((et) => (
              <span key={et.id} class="spec-tag-item">
                <Chip color={kindColor(tagById.get(et.id)?.kind)} title={t.browse.provenanceLabel(et.sources)}>
                  {tagById.get(et.id)?.name || et.id}
                  <span class="tag-provenance-badge">{t.browse.provenanceLabel(et.sources)}</span>
                </Chip>
                <SpecAffordances
                  onFilter={() => onFilterTag(et.id)}
                  filterLabel={t.browse.clickToFilter}
                  detailHref={routeHash({ view: 'spec', tagId: et.id })}
                  onNavigate={() => onGoToTag(et.id)}
                  detailLabel={t.browse.openDetail}
                />
              </span>
            ))}
          </div>
        </CollapsibleSection>
      )}

      {hasDetail && (
        <CollapsibleSection
          recordId={detail.id}
          section="rules"
          count={detail.rules!.length}
          icon="gavel"
          label={t.browse.rulesHeading}
          focusOpen={isOpen}
          onToggle={onToggleOpen}
        >
          {detail.rules!.map((d) => (
            <div key={d.id} class="tag-card-decision">
              <p>{d.why}</p>
              <span class="dim">
                {d.at.slice(0, 10)} {d.ref && `· ${d.ref}`}
              </span>
            </div>
          ))}
        </CollapsibleSection>
      )}
    </article>
  );
}
