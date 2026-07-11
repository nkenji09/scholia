import { useLookups } from '../../lookups';
import { strings } from '../../strings';
import type { Transition, VocabEntry } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';

interface Props {
  entry: VocabEntry;
  uses: Transition[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterTag: (id: string) => void;
  onSelectTx: (txId: string) => void;
}

const CATEGORY_ICON: Record<VocabEntry['category'], IconName> = {
  action: 'circle-play',
  condition: 'funnel',
  effect: 'arrow-right-to-line',
};

// Mirrors TagCard's layout (kind badge → filterable name/id → description →
// card-sections) so Vocab reads as the same design language as タグ/仕様
// rather than its own bespoke list — see .concierge/decision.md's tweaks2
// handoff §4. Several classnames (tag-card-head/-badges/-name/-id,
// tag-card-spec-list/-row/-label/-id) are reused as-is from TagCard's CSS;
// they're generic "card head" / "row of records" patterns, not actually
// tag-specific.
export function VocabCard({ entry, uses, cardRef, onFilterTag, onSelectTx }: Props) {
  const { tagById, transitionLabel } = useLookups();
  const tags = entry.tags || [];

  return (
    <article ref={cardRef} data-card-id={entry.id} class="card" title={entry.id}>
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(entry.category)}>
            <Icon name={CATEGORY_ICON[entry.category]} size={12} /> {strings.vocab.categoryLabel(entry.category)}
          </Chip>
          {entry.kind && <span class="vocab-card-kind dim">{entry.kind}</span>}
          <span class="tag-card-spacer" />
          <CommentButton recordType="vocab" recordId={entry.id} recordTitle={entry.label} anchor="card" anchorLabel="カード全体" />
        </div>
        {/* Unlike TagCard's name (clicking narrows to a tag's subtree —
            meaningful because tags nest), a vocab entry has no hierarchy to
            drill into: filtering "to just this one entry" would be a
            no-op AND-condition on the card you're already looking at. So
            this is a plain heading, not a filter button — the design's +
            mark is reserved for clicks that actually narrow something
            (the tag chips below). */}
        <span class="tag-card-name vocab-card-name">{entry.label}</span>
        {entry.owner && (
          <span class="vocab-card-owner">
            {strings.vocab.owner}: {entry.owner}
          </span>
        )}
      </div>

      {entry.description && (
        <div class="tag-card-body">
          <Markdown text={entry.description} />
        </div>
      )}

      {tags.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="tags" size={14} /> {strings.browse.tagsHeading}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {strings.browse.clickToFilter}
              </span>
            </span>
          </div>
          <div class="spec-card-chip-row">
            {tags.map((id) => (
              <Chip key={id} color={kindColor(tagById.get(id)?.kind)} onClick={() => onFilterTag(id)} filterable>
                {tagById.get(id)?.name || id}
              </Chip>
            ))}
          </div>
        </div>
      )}

      <div class="card-section">
        <div class="card-section-heading-row">
          <span class="card-section-heading">
            <Icon name="scroll-text" size={14} /> {strings.vocab.usageCount(uses.length)}
          </span>
        </div>
        {uses.length === 0 ? (
          <span class="vocab-card-no-usage dim">{strings.vocab.noUsage}</span>
        ) : (
          <div class="tag-card-spec-list">
            {uses.map((t) => {
              const label = transitionLabel(t.id);
              return (
                <button key={t.id} type="button" class="tag-card-spec-row" onClick={() => onSelectTx(t.id)} title={t.id}>
                  <span class="tag-card-spec-label">
                    {label.primary}
                    {label.secondary && <span class="dim"> {label.secondary}</span>}
                  </span>
                </button>
              );
            })}
          </div>
        )}
      </div>
    </article>
  );
}
