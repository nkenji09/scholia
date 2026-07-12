import { useLookups } from '../../lookups';
import { useT } from '../../i18n';
import type { Decision, SpecReport, Tag } from '../../types';
import { Markdown } from '../Markdown';
import { Chip, kindColor } from '../shared/Chip';
import { CommentButton } from '../comments/CommentButton';
import { Icon } from '../shared/Icon';

interface Props {
  tag: Tag;
  report: SpecReport | undefined;
  isGap: boolean | undefined; // undefined = this tag's kind isn't traceability-tracked
  parents: Tag[];
  children: Tag[];
  cardRef: (el: HTMLElement | null) => void;
  onFilterSelf: () => void;
  onSelectParent: (tagId: string) => void;
  onSelectChild: (tagId: string) => void;
  onSelectSpec: (txId: string) => void;
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

export function TagCard({ tag, report, isGap, parents, children, cardRef, onFilterSelf, onSelectParent, onSelectChild, onSelectSpec }: Props) {
  const t = useT();
  const { tagKindLabel } = useLookups();
  const entries = report?.entries || [];
  const tagDecisions = dedupeDecisions(entries.flatMap((e) => (e.decisions || []).filter((d) => d.target.type === 'tag')));

  return (
    <article ref={cardRef} data-card-id={tag.id} class="card" title={tag.id}>
      <div class="tag-card-head">
        <div class="tag-card-badges">
          <Chip color={kindColor(tag.kind)} onClick={onFilterSelf}>
            {tag.kind ? tagKindLabel(tag.kind) : '?'}
          </Chip>
          {parents.length > 0 && (
            <span class="tag-card-parents dim">
              <Icon name="corner-down-right" size={13} />
              {parents.map((p) => (
                <button key={p.id} type="button" class="tag-card-parent-link" onClick={() => onSelectParent(p.id)} title={t.browse.parentLinkTitle}>
                  {p.name || p.id}
                </button>
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
        <button type="button" class="tag-card-name" onClick={onFilterSelf} title={t.browse.clickToFilter}>
          {tag.name || tag.id}
          <Icon name="plus" size={13} class="filter-plus-icon" />
        </button>
      </div>

      {tag.description && (
        <div class="tag-card-body">
          <div class="card-section-heading-row">
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="body" anchorLabel={t.comments.descriptionAnchorLabel} />
          </div>
          <Markdown text={tag.description} />
        </div>
      )}

      {entries.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="scroll-text" size={14} /> {t.browse.satisfiedSpecs}
            </span>
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="specs" anchorLabel={t.browse.satisfiedSpecs} />
          </div>
          <div class="tag-card-spec-list">
            {entries.map((e) => (
              <button key={e.transition.id} type="button" class="tag-card-spec-row" onClick={() => onSelectSpec(e.transition.id)} title={e.transition.id}>
                <span class="tag-card-spec-label">{e.actionLabel}</span>
              </button>
            ))}
          </div>
        </div>
      )}

      {tagDecisions.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="gavel" size={14} /> {t.browse.relatedDecisions}
            </span>
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="decisions" anchorLabel={t.browse.relatedDecisions} />
          </div>
          {tagDecisions.map((d) => (
            <div key={d.id} class="tag-card-decision">
              <p>{d.why}</p>
              <span class="dim">
                {d.at.slice(0, 10)} {d.ref && `· ${d.ref}`}
              </span>
            </div>
          ))}
        </div>
      )}

      {children.length > 0 && (
        <div class="card-section">
          <div class="card-section-heading-row">
            <span class="card-section-heading">
              <Icon name="list-tree" size={14} /> {t.browse.childTags}
            </span>
            <CommentButton recordType="tag" recordId={tag.id} recordTitle={tag.name || tag.id} anchor="children" anchorLabel={t.browse.childTags} />
          </div>
          <div class="tag-card-children">
            {children.map((c) => (
              <button key={c.id} type="button" class="tag-card-child-chip" onClick={() => onSelectChild(c.id)} title={t.browse.childLinkTitle}>
                <Icon name="corner-down-right" size={12} /> {c.name || c.id}
              </button>
            ))}
          </div>
        </div>
      )}
    </article>
  );
}
