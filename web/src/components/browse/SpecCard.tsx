import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { EffectiveTag, TransitionDetail } from '../../types';
import { Chip, kindColor, OWNER_COLOR } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';

interface Props {
  detail: TransitionDetail;
  isOpen: boolean;
  cardRef: (el: HTMLElement | null) => void;
  onToggleOpen: () => void;
  onFilterVocab: (id: string) => void;
  onFilterTag: (id: string) => void;
  onFilterOwner: (owner: string) => void;
}

export function SpecCard({ detail, isOpen, cardRef, onToggleOpen, onFilterVocab, onFilterTag, onFilterOwner }: Props) {
  const t = useT();
  const { tagById, vocabById } = useLookups();
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
  const hasTags = own.length > 0 || derived.length > 0;
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
        <button type="button" class="spec-card-action" onClick={() => onFilterVocab(detail.action)} title={t.browse.clickToFilter}>
          {detail.actionLabel || detail.action}
          <Icon name="plus" size={13} class="filter-plus-icon" />
        </button>
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
            <button key={id} type="button" class="spec-card-cond-row" onClick={() => onFilterVocab(id)} title={t.browse.clickToFilter}>
              <span class="spec-card-cond-label">{(detail.givenLabels || [])[i] || id}</span>
              <Icon name="plus" size={13} class="filter-plus-icon" />
            </button>
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
          {(detail.then || []).map((id, i) => {
            const owner = vocabById.get(id)?.owner;
            return (
              <div key={id} class="spec-card-cond-row">
                <span class="spec-card-then-n dim">{i + 1}</span>
                <button type="button" class="spec-card-cond-label-btn" onClick={() => onFilterVocab(id)} title={t.browse.clickToFilter}>
                  <span class="spec-card-cond-label">{(detail.thenLabels || [])[i] || id}</span>
                  <Icon name="plus" size={13} class="filter-plus-icon" />
                </button>
                {owner && (
                  <Chip color={OWNER_COLOR} onClick={() => onFilterOwner(owner)} filterable title={t.browse.clickToFilter}>
                    {owner}
                  </Chip>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {hasTags && (
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
              <Chip
                key={et.id}
                color={kindColor(tagById.get(et.id)?.kind)}
                onClick={() => onFilterTag(et.id)}
                filterable
                title={t.browse.provenanceLabel(et.sources)}
              >
                {tagById.get(et.id)?.name || et.id}
                {provenanceBadge(et) && <span class="tag-provenance-badge">{provenanceBadge(et)}</span>}
              </Chip>
            ))}
          </div>
          {derived.length > 0 && (
            <div class="spec-card-derived">
              <span class="dim spec-card-derived-label">{t.browse.derivedHeading}</span>
              <div class="spec-card-chip-row">
                {derived.map((et) => (
                  <Chip
                    key={et.id}
                    color={kindColor(tagById.get(et.id)?.kind)}
                    onClick={() => onFilterTag(et.id)}
                    filterable
                    title={t.browse.provenanceLabel(et.sources)}
                  >
                    {tagById.get(et.id)?.name || et.id}
                    <span class="tag-provenance-badge">{t.browse.provenanceLabel(et.sources)}</span>
                  </Chip>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {hasDetail && (
        <button type="button" class="spec-card-detail-toggle" onClick={onToggleOpen}>
          <Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={15} /> {isOpen ? t.browse.hideDetail : t.browse.showDetail}
        </button>
      )}

      {isOpen && hasDetail && (
        <div class="card-section spec-card-detail">
          {detail.rules && detail.rules.length > 0 && (
            <div>
              <span class="card-section-heading">
                <Icon name="gavel" size={14} /> {t.browse.rulesHeading}
              </span>
              {detail.rules.map((d) => (
                <div key={d.id} class="tag-card-decision">
                  <p>{d.why}</p>
                  <span class="dim">
                    {d.at.slice(0, 10)} {d.ref && `· ${d.ref}`}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </article>
  );
}
