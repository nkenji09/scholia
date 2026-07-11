import { useLookups } from '../../lookups';
import { strings } from '../../strings';
import type { EffectiveTag, TransitionDetail } from '../../types';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { Icon } from '../shared/Icon';

interface Props {
  detail: TransitionDetail;
  isOpen: boolean;
  cardRef: (el: HTMLElement | null) => void;
  onToggleOpen: () => void;
  onFilterVocab: (id: string) => void;
  onFilterTag: (id: string) => void;
}

export function SpecCard({ detail, isOpen, cardRef, onToggleOpen, onFilterVocab, onFilterTag }: Props) {
  const { tagById, vocabById } = useLookups();

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
    return extra.length > 0 ? strings.browse.provenanceLabel(extra) : null;
  };
  const hasDetail = (detail.tests && detail.tests.length > 0) || (detail.rules && detail.rules.length > 0);

  return (
    <article ref={cardRef} data-card-id={detail.id} class="card">
      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-act)' }}>
            <Icon name="circle-play" size={13} /> {strings.flow.trigger}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="action" anchorLabel={strings.flow.trigger} />
        </div>
        <button type="button" class="spec-card-action" onClick={() => onFilterVocab(detail.action)} title={strings.browse.clickToFilter}>
          {detail.actionLabel || detail.action}
          <Icon name="plus" size={13} class="filter-plus-icon" />
        </button>
      </div>

      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-giv)' }}>
            <Icon name="funnel" size={12} /> {strings.flow.given}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="given" anchorLabel={strings.flow.given} />
        </div>
        {(detail.given || []).length === 0 && <span class="dim spec-card-empty-given">無条件（前提なし）</span>}
        <div class="spec-card-given-list">
          {(detail.given || []).map((id, i) => (
            <button key={id} type="button" class="spec-card-cond-row" onClick={() => onFilterVocab(id)} title={strings.browse.clickToFilter}>
              <span class="spec-card-cond-label">{(detail.givenLabels || [])[i] || id}</span>
              <Icon name="plus" size={13} class="filter-plus-icon" />
            </button>
          ))}
        </div>
      </div>

      <div class="spec-card-slot">
        <div class="card-section-heading-row">
          <span class="spec-card-slot-label" style={{ color: 'var(--t-then)' }}>
            <Icon name="arrow-right-to-line" size={12} /> {strings.flow.result}
          </span>
          <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="then" anchorLabel={strings.flow.result} />
        </div>
        <div class="spec-card-then-list">
          {(detail.then || []).map((id, i) => (
            <button key={id} type="button" class="spec-card-cond-row" onClick={() => onFilterVocab(id)} title={strings.browse.clickToFilter}>
              <span class="spec-card-then-n dim">{i + 1}</span>
              <span class="spec-card-cond-label">
                {(detail.thenLabels || [])[i] || id}
                {vocabById.get(id)?.owner && <span class="spec-card-owner dim"> owner: {vocabById.get(id)?.owner}</span>}
              </span>
              <Icon name="plus" size={13} class="filter-plus-icon" />
            </button>
          ))}
        </div>
      </div>

      {hasTags && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="tags" size={14} /> {strings.browse.tagsHeading}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {strings.browse.clickToFilter}
              </span>
            </span>
            <CommentButton recordType="transition" recordId={detail.id} recordTitle={detail.actionLabel || detail.action} anchor="tags" anchorLabel={strings.browse.tagsHeading} />
          </div>
          <div class="spec-card-chip-row">
            {own.map((et) => (
              <Chip
                key={et.id}
                color={kindColor(tagById.get(et.id)?.kind)}
                onClick={() => onFilterTag(et.id)}
                filterable
                title={strings.browse.provenanceLabel(et.sources)}
              >
                {tagById.get(et.id)?.name || et.id}
                {provenanceBadge(et) && <span class="tag-provenance-badge">{provenanceBadge(et)}</span>}
              </Chip>
            ))}
          </div>
          {derived.length > 0 && (
            <div class="spec-card-derived">
              <span class="dim spec-card-derived-label">{strings.browse.derivedHeading}</span>
              <div class="spec-card-chip-row">
                {derived.map((et) => (
                  <Chip
                    key={et.id}
                    color={kindColor(tagById.get(et.id)?.kind)}
                    onClick={() => onFilterTag(et.id)}
                    filterable
                    title={strings.browse.provenanceLabel(et.sources)}
                  >
                    {tagById.get(et.id)?.name || et.id}
                    <span class="tag-provenance-badge">{strings.browse.provenanceLabel(et.sources)}</span>
                  </Chip>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {hasDetail && (
        <button type="button" class="spec-card-detail-toggle" onClick={onToggleOpen}>
          <Icon name={isOpen ? 'chevron-up' : 'chevron-down'} size={15} /> {isOpen ? strings.browse.hideDetail : strings.browse.showDetail}
        </button>
      )}

      {isOpen && hasDetail && (
        <div class="card-section spec-card-detail">
          {detail.tests && detail.tests.length > 0 && (
            <div>
              <span class="card-section-heading">
                <Icon name="flask-conical" size={14} /> {strings.browse.tests}
              </span>
              <div class="spec-card-chip-row">
                {detail.tests.map((t) => (
                  <span key={t} class="spec-card-test">
                    {t}
                  </span>
                ))}
              </div>
            </div>
          )}
          {detail.rules && detail.rules.length > 0 && (
            <div>
              <span class="card-section-heading">
                <Icon name="gavel" size={14} /> {strings.browse.rulesHeading}
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
