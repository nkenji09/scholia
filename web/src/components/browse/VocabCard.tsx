import { useLookups } from '../../lookups';
import { usePendingDiff } from '../../pendingDiff';
import { useT } from '../../i18n';
import type { Transition, VocabEntry } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor, OWNER_COLOR } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { useComments } from '../comments/useComments';
import { Icon } from '../shared/Icon';
import type { IconName } from '../shared/Icon';
import { CollapsibleSection } from '../shared/CollapsibleSection';
import { HashLink } from '../shared/HashLink';
import { routeHash } from '../../router';

interface Props {
  entry: VocabEntry;
  uses: Transition[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterTag: (id: string) => void;
  onFilterOwner: (owner: string) => void;
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
export function VocabCard({ entry, uses, cardRef, onFilterTag, onFilterOwner, onSelectTx }: Props) {
  const t = useT();
  const { tagById, transitionLabel } = useLookups();
  const { changedVocabIds } = usePendingDiff();
  const { openComposer, comments } = useComments();
  const tags = entry.tags || [];
  // §8.8 P5 vocab/tag（generalized from SpecCard's hasUncommentedChange・
  // §8.3）: a pending change with no comment yet is a quiet pending-change
  // flag, not a "proposal" — it steps aside once someone comments on this
  // entry (the comment itself then carries the diff card and badge count).
  const hasUncommentedChange = changedVocabIds.has(entry.id) && !comments.some((c) => c.recordType === 'vocab' && c.recordId === entry.id);

  return (
    <article ref={cardRef} data-card-id={entry.id} class="card" title={entry.id}>
      {hasUncommentedChange && (
        <button
          type="button"
          class="spec-card-clean-flag"
          onClick={() => openComposer({ recordType: 'vocab', recordId: entry.id, recordTitle: entry.label, anchor: 'card', anchorLabel: t.comments.cardAnchorLabel })}
        >
          <Icon name="git-compare" size={12} /> {t.comments.proposalCleanFlag}
        </button>
      )}
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(entry.category)}>
            <Icon name={CATEGORY_ICON[entry.category]} size={12} /> {t.vocab.categoryLabel(entry.category)}
          </Chip>
          {entry.kind && <span class="vocab-card-kind dim">{entry.kind}</span>}
          <span class="tag-card-spacer" />
          <CommentButton recordType="vocab" recordId={entry.id} recordTitle={entry.label} anchor="card" anchorLabel={t.comments.cardAnchorLabel} />
        </div>
        {/* Unlike TagCard's name (clicking narrows to a tag's subtree —
            meaningful because tags nest), a vocab entry has no hierarchy to
            drill into: filtering "to just this one entry" would be a
            no-op AND-condition on the card you're already looking at. So
            this is a plain heading, not a filter button — the design's +
            mark is reserved for clicks that actually narrow something
            (the tag chips below). */}
        <span class="tag-card-name vocab-card-name">{entry.label}</span>
      </div>

      {entry.description && (
        <div class="tag-card-body">
          <Markdown text={entry.description} />
        </div>
      )}

      {entry.owner && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="user" size={14} /> {t.vocab.owner}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {t.browse.clickToFilter}
              </span>
            </span>
          </div>
          <div class="spec-card-chip-row">
            <Chip color={OWNER_COLOR} onClick={() => onFilterOwner(entry.owner!)} filterable>
              {entry.owner}
            </Chip>
          </div>
        </div>
      )}

      {tags.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="tags" size={14} /> {t.browse.tagsHeading}{' '}
              <span class="spec-card-hint dim">
                <Icon name="plus" size={11} class="filter-plus-icon" /> {t.browse.clickToFilter}
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

      {uses.length === 0 ? (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="scroll-text" size={14} /> {t.vocab.usageCount(uses.length)}
            </span>
          </div>
          <span class="vocab-card-no-usage dim">{t.vocab.noUsage}</span>
        </div>
      ) : (
        <CollapsibleSection recordId={entry.id} section="usage" count={uses.length} icon="scroll-text" label={t.vocab.usageHeading}>
          <div class="tag-card-spec-list">
            {uses.map((tx) => {
              const label = transitionLabel(tx.id);
              return (
                <HashLink key={tx.id} href={routeHash({ view: 'browse', txId: tx.id })} class="tag-card-spec-row" onNavigate={() => onSelectTx(tx.id)} title={tx.id}>
                  <span class="tag-card-spec-label">
                    {label.primary}
                    {label.secondary && <span class="dim"> {label.secondary}</span>}
                  </span>
                </HashLink>
              );
            })}
          </div>
        </CollapsibleSection>
      )}
    </article>
  );
}
